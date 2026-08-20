package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeeth/rss-reader/backend/internal/ai"
	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/crawl"
	"github.com/jeeth/rss-reader/backend/internal/ipc"
	"github.com/jeeth/rss-reader/backend/internal/rss"
	"github.com/jeeth/rss-reader/backend/internal/scheduler"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

const version = "0.1.0"

func main() {
	dbPath := flag.String("db", "", "path to sqlite database")
	seed := flag.Bool("seed", false, "seed development sample data")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dbPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			log.Error("user config dir", "err", err)
			os.Exit(1)
		}
		*dbPath = filepath.Join(dir, "rss-reader", "rss.db")
	}
	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		log.Error("mkdir db", "err", err)
		os.Exit(1)
	}

	db, err := sqlite.Open(*dbPath)
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	feeds := sqlite.NewFeedRepo(db)
	articles := sqlite.NewArticleRepo(db)
	folders := sqlite.NewFolderRepo(db)
	settings := sqlite.NewSettingsRepo(db)
	stories := sqlite.NewStoryRepo(db)
	queue := sqlite.NewAIQueueRepo(db)
	aiLogs := sqlite.NewAILogRepo(db)

	crawlSvc := crawl.New(articles, feeds, log)
	aiSvc := ai.New(articles, stories, settings, feeds, queue, aiLogs, log)

	svc := &application.Service{
		Feeds:    feeds,
		Articles: articles,
		Folders:  folders,
		Settings: settings,
		Stories:  stories,
		RSS:      rss.NewFetcher(),
		AI:       aiSvc,
		Crawler:  crawlSvc,
		Log:      log,
		Version:  version,
		DBPath:   *dbPath,
	}

	if _, err := feeds.EnsureReadLater(context.Background()); err != nil {
		log.Warn("ensure read later feed", "err", err)
	}

	if *seed {
		if err := seedDev(context.Background(), svc, log); err != nil {
			log.Warn("seed failed", "err", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	sched := scheduler.New(svc, feeds, log)
	go sched.Run(ctx)

	server := ipc.NewServer(svc, log, os.Stdout)
	aiSvc.Emit = server.Emit
	crawlSvc.Emit = server.Emit
	aiSvc.Resume(ctx)
	crawlSvc.EnqueueAndKick(ctx)

	log.Info("backend started", "version", version, "db", *dbPath)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(ctx, os.Stdin)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal")
	case <-server.Done():
		log.Info("shutdown requested via ipc")
		cancel()
	case err := <-errCh:
		if err != nil {
			log.Error("ipc serve", "err", err)
			os.Exit(1)
		}
	}

	log.Info("backend stopped")
}

func seedDev(ctx context.Context, svc *application.Service, log *slog.Logger) error {
	feeds, err := svc.ListFeeds(ctx)
	if err != nil {
		return err
	}
	normal := 0
	for _, f := range feeds {
		if !f.IsReadLater {
			normal++
		}
	}
	if normal > 0 {
		return nil
	}
	log.Info("seeding sample feed")
	_, err = svc.AddFeed(ctx, "https://hnrss.org/frontpage")
	if err != nil {
		return fmt.Errorf("seed add: %w", err)
	}
	return nil
}
