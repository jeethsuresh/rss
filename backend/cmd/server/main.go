package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/ai"
	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/cluster"
	"github.com/jeeth/rss-reader/backend/internal/crawl"
	"github.com/jeeth/rss-reader/backend/internal/httpapi"
	"github.com/jeeth/rss-reader/backend/internal/mlb"
	"github.com/jeeth/rss-reader/backend/internal/netguard"
	"github.com/jeeth/rss-reader/backend/internal/openf1"
	"github.com/jeeth/rss-reader/backend/internal/rss"
	"github.com/jeeth/rss-reader/backend/internal/serverfetch"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

const version = "0.2.0-server"

func main() {
	defaultDB := env("RSS_SERVER_DB", "rss-server.db")
	defaultAddr := env("RSS_SERVER_ADDR", "127.0.0.1:8787")
	dbPath := flag.String("db", defaultDB, "path to the server SQLite database")
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	registration := flag.Bool("registration", envBool("RSS_SERVER_REGISTRATION_ENABLED", false), "allow account registration")
	webDir := flag.String("web-dir", env("RSS_SERVER_WEB_DIR", ""), "path to built web application assets")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if dir := filepath.Dir(*dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Error("create database directory", "err", err)
			os.Exit(1)
		}
	}
	db, err := sqlite.Open(*dbPath)
	if err != nil {
		log.Error("open server database", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	// This database is the canonical server store, not a desktop replica.
	// Suppress desktop-only feed triggers to avoid an unused local operation log.
	_, _ = db.SQL.Exec(`UPDATE local_sync_apply_guard SET applying=1 WHERE id=1`)

	feeds := sqlite.NewFeedRepo(db)
	articles := sqlite.NewArticleRepo(db)
	folders := sqlite.NewFolderRepo(db)
	settings := sqlite.NewSettingsRepo(db)
	stories := sqlite.NewStoryRepo(db)
	queue := sqlite.NewAIQueueRepo(db)
	aiLogs := sqlite.NewAILogRepo(db)
	errorLogs := sqlite.NewErrorLogRepo(db)

	crawlSvc := crawl.New(articles, feeds, log)
	crawlSvc.Client = netguard.NewClient(60 * time.Second)
	clusterSvc := cluster.New(articles, stories, log)
	clusterSvc.Logs = aiLogs
	aiSvc := ai.New(articles, stories, settings, feeds, queue, aiLogs, log)
	aiSvc.Suggester = clusterSvc
	aiSvc.APIKey = strings.TrimSpace(os.Getenv("RSS_SERVER_AI_API_KEY"))
	configureAI(settings, log)

	sportsSvc := application.NewSportsService(
		sqlite.NewSportsRepo(db), sqlite.NewSportsCacheRepo(db), mlb.NewClient(), openf1.NewClient(),
	)
	rssFetcher := rss.NewFetcher()
	rssFetcher.Client = netguard.NewClient(30 * time.Second)
	svc := &application.Service{
		Feeds: feeds, Articles: articles, Folders: folders, Settings: settings, Stories: stories,
		Errors: errorLogs, Sports: sportsSvc, RSS: rssFetcher, AI: aiSvc,
		Crawler: crawlSvc, Cluster: clusterSvc, Log: log, Version: version, DBPath: *dbPath,
	}
	crawlSvc.OnReady = func(ctx context.Context, articleID string) { svc.ClusterArticle(ctx, articleID) }

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	events := httpapi.NewEventHub()
	svc.Emit = events.Emit
	crawlSvc.Emit = events.Emit
	clusterSvc.Emit = events.Emit
	sportsSvc.Emit = events.Emit
	store := serverstore.New(db)
	crawlSvc.Shared = store
	crawlSvc.SharedOwner = "rss-server-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	feedScheduler := &serverfetch.FeedScheduler{Service: svc, Store: store, Log: log, Workers: 4}
	documentScheduler := serverfetch.NewDocumentScheduler(store, log)
	documentScheduler.Client = netguard.NewClient(60 * time.Second)
	go feedScheduler.Run(ctx)
	go documentScheduler.Run(ctx)
	aiSvc.Resume(ctx)
	crawlSvc.EnqueueAndKick(ctx)
	go crawlSvc.BackfillExtracts(ctx)

	resolvedWebDir := resolveWebDir(*webDir)
	if resolvedWebDir == "" {
		log.Warn("web application assets not found; API-only mode enabled")
	}
	handler := httpapi.New(store, svc, log, httpapi.Config{
		RegistrationEnabled: *registration,
		Version:             version,
		Context:             ctx,
		WebDir:              resolvedWebDir,
		CookieSecure:        envBool("RSS_SERVER_COOKIE_SECURE", false),
		Events:              events,
	})
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Streaming browser events intentionally keep responses open.
		WriteTimeout: 0,
		IdleTimeout:  2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("rss server started", "version", version, "addr", *addr, "db", *dbPath, "registration", *registration, "webDir", resolvedWebDir)
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		log.Info("rss server shutting down")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "err", err)
	}
}

func configureAI(settings *sqlite.SettingsRepo, log *slog.Logger) {
	ctx := context.Background()
	current, err := settings.Get(ctx)
	if err != nil {
		log.Warn("read AI settings", "err", err)
		return
	}
	changed := false
	if raw, ok := os.LookupEnv("RSS_SERVER_AI_ENABLED"); ok {
		value, err := strconv.ParseBool(raw)
		if err == nil {
			current.AIEnabled = value
			changed = true
		}
	}
	if value := strings.TrimSpace(os.Getenv("RSS_SERVER_AI_BASE_URL")); value != "" {
		current.AIBaseURL = value
		changed = true
	}
	if value := strings.TrimSpace(os.Getenv("RSS_SERVER_AI_MODEL")); value != "" {
		current.AIModel = value
		changed = true
	}
	if changed {
		if err := settings.Update(ctx, current); err != nil {
			log.Warn("configure AI from environment", "err", err)
		}
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func resolveWebDir(configured string) string {
	candidates := []string{}
	if strings.TrimSpace(configured) != "" {
		candidates = append(candidates, configured)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(executable), "web"),
			filepath.Join(filepath.Dir(executable), "..", "web"),
		)
	}
	candidates = append(candidates,
		filepath.Join("apps", "desktop", "dist-renderer"),
		filepath.Join("..", "apps", "desktop", "dist-renderer"),
	)
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(filepath.Join(absolute, "index.html")); err == nil && !info.IsDir() {
			return absolute
		}
	}
	return ""
}
