package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

type Config struct {
	RegistrationEnabled bool
	Version             string
	Context             context.Context
	WebDir              string
	CookieSecure        bool
	Events              *EventHub
}

type API struct {
	store    *serverstore.Store
	svc      *application.Service
	log      *slog.Logger
	cfg      Config
	ctx      context.Context
	events   *EventHub
	start    time.Time
	limitMu  sync.Mutex
	attempts map[string]authAttempts
}

type authAttempts struct {
	Count int
	Reset time.Time
}

type userContextKey struct{}

func New(store *serverstore.Store, svc *application.Service, log *slog.Logger, cfg Config) http.Handler {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.Events == nil {
		cfg.Events = NewEventHub()
	}
	a := &API{store: store, svc: svc, log: log, cfg: cfg, ctx: cfg.Context, events: cfg.Events, start: time.Now().UTC(), attempts: map[string]authAttempts{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /v1/auth/register", a.register)
	mux.HandleFunc("POST /v1/auth/login", a.login)
	mux.HandleFunc("GET /v1/web/config", a.webConfig)
	mux.HandleFunc("POST /v1/web/register", a.webRegister)
	mux.HandleFunc("POST /v1/web/login", a.webLogin)

	mux.Handle("GET /v1/me", a.auth(http.HandlerFunc(a.me)))
	mux.Handle("GET /v1/web/session", a.auth(http.HandlerFunc(a.webSession)))
	mux.Handle("POST /v1/web/logout", a.auth(a.requireCSRF(http.HandlerFunc(a.webLogout))))
	mux.Handle("POST /v1/rpc", a.auth(a.requireCSRF(http.HandlerFunc(a.rpc))))
	mux.Handle("GET /v1/events", a.auth(http.HandlerFunc(a.eventStream)))
	mux.Handle("POST /v1/sync/feeds", a.auth(http.HandlerFunc(a.syncFeeds)))
	mux.Handle("POST /v1/sync/state", a.auth(http.HandlerFunc(a.syncState)))
	mux.Handle("GET /v1/feeds", a.auth(http.HandlerFunc(a.listFeeds)))
	mux.Handle("GET /v1/articles", a.auth(http.HandlerFunc(a.listArticles)))
	mux.Handle("GET /v1/articles/{id}", a.auth(http.HandlerFunc(a.getArticle)))
	mux.Handle("PATCH /v1/articles/{id}", a.auth(http.HandlerFunc(a.patchArticle)))
	mux.Handle("GET /v1/stories", a.auth(http.HandlerFunc(a.listStories)))
	mux.Handle("PATCH /v1/stories/{id}", a.auth(http.HandlerFunc(a.patchStory)))
	mux.Handle("GET /v1/folders", a.auth(http.HandlerFunc(a.listFolders)))
	mux.Handle("POST /v1/folders", a.auth(http.HandlerFunc(a.createFolder)))
	mux.Handle("DELETE /v1/folders/{id}", a.auth(http.HandlerFunc(a.deleteFolder)))
	mux.Handle("PUT /v1/folders/{id}/feeds/{feedId}", a.auth(http.HandlerFunc(a.assignFolder)))
	mux.Handle("DELETE /v1/folders/{id}/feeds/{feedId}", a.auth(http.HandlerFunc(a.unassignFolder)))
	mux.Handle("GET /v1/settings", a.auth(http.HandlerFunc(a.getSettings)))
	mux.Handle("PATCH /v1/settings", a.auth(http.HandlerFunc(a.patchSettings)))

	mux.Handle("GET /v1/read-later", a.auth(http.HandlerFunc(a.listReadLater)))
	mux.Handle("POST /v1/read-later", a.auth(http.HandlerFunc(a.addReadLater)))
	mux.Handle("GET /v1/read-later/{id}", a.auth(http.HandlerFunc(a.getReadLater)))
	mux.Handle("PATCH /v1/read-later/{id}", a.auth(http.HandlerFunc(a.patchReadLater)))
	mux.Handle("DELETE /v1/read-later/{id}", a.auth(http.HandlerFunc(a.deleteReadLater)))

	mux.Handle("GET /v1/sports/followed/{sport}", a.auth(http.HandlerFunc(a.getFollowedTeams)))
	mux.Handle("PUT /v1/sports/followed/{sport}", a.auth(http.HandlerFunc(a.putFollowedTeams)))
	mux.Handle("GET /v1/sports/mlb/teams", a.auth(http.HandlerFunc(a.mlbTeams)))
	mux.Handle("GET /v1/sports/mlb/seasons", a.auth(http.HandlerFunc(a.mlbSeasons)))
	mux.Handle("GET /v1/sports/mlb/schedule", a.auth(http.HandlerFunc(a.mlbSchedule)))
	mux.Handle("GET /v1/sports/mlb/daily", a.auth(http.HandlerFunc(a.mlbDaily)))
	mux.Handle("GET /v1/sports/mlb/games/{id}", a.auth(http.HandlerFunc(a.mlbGame)))
	mux.Handle("GET /v1/sports/mlb/standings", a.auth(http.HandlerFunc(a.mlbStandings)))
	mux.Handle("GET /v1/sports/mlb/roster", a.auth(http.HandlerFunc(a.mlbRoster)))
	mux.Handle("GET /v1/sports/f1/years", a.auth(http.HandlerFunc(a.f1Years)))
	mux.Handle("GET /v1/sports/f1/races", a.auth(http.HandlerFunc(a.f1Races)))
	mux.Handle("GET /v1/sports/f1/races/{id}", a.auth(http.HandlerFunc(a.f1Race)))
	mux.Handle("GET /v1/sports/f1/standings", a.auth(http.HandlerFunc(a.f1Standings)))
	mux.Handle("GET /v1/ai/status", a.auth(http.HandlerFunc(a.aiStatus)))
	if cfg.WebDir != "" {
		mux.Handle("GET /", a.webApp())
	}

	return requestLogger(log, securityHeaders(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": a.cfg.Version, "uptimeSeconds": int(time.Since(a.start).Seconds()),
	})
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthAttempt(r) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", errors.New("too many authentication attempts"))
		return
	}
	if !a.cfg.RegistrationEnabled {
		writeError(w, http.StatusForbidden, "REGISTRATION_DISABLED", errors.New("registration is disabled"))
		return
	}
	var body credentials
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.Register(r.Context(), body.Username, body.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthAttempt(r) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", errors.New("too many authentication attempts"))
		return
	}
	var body credentials
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", errors.New("invalid username or password"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) allowAuthAttempt(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}
	now := time.Now()
	a.limitMu.Lock()
	defer a.limitMu.Unlock()
	entry := a.attempts[host]
	if entry.Reset.IsZero() || now.After(entry.Reset) {
		entry = authAttempts{Reset: now.Add(time.Minute)}
	}
	entry.Count++
	a.attempts[host] = entry
	return entry.Count <= 10
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			if cookie, err := r.Cookie(webSessionCookie); err == nil {
				token = strings.TrimSpace(cookie.Value)
			}
		}
		if token == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", errors.New("session required"))
			return
		}
		user, err := a.store.Authenticate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
	})
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func currentUser(ctx context.Context) *serverstore.User {
	user, _ := ctx.Value(userContextKey{}).(*serverstore.User)
	return user
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r.Context()))
}

func (a *API) syncFeeds(w http.ResponseWriter, r *http.Request) {
	var body serverstore.SyncRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.SyncFeeds(r.Context(), currentUser(r.Context()).ID, body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) syncState(w http.ResponseWriter, r *http.Request) {
	var body serverstore.StateSyncRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.SyncState(r.Context(), currentUser(r.Context()).ID, body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) listFeeds(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.ListFeeds(r.Context(), currentUser(r.Context()).ID)
	respond(w, result, err)
}

func (a *API) listArticles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := a.store.ListArticles(r.Context(), currentUser(r.Context()).ID, q.Get("feedId"),
		parseBool(q.Get("unreadOnly")), parseBool(q.Get("starredOnly")), parseInt(q.Get("limit")))
	respond(w, result, err)
}

func (a *API) getArticle(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.GetArticle(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"))
	respond(w, result, err)
}

func (a *API) patchArticle(w http.ResponseWriter, r *http.Request) {
	var patch serverstore.ArticleStatePatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.SetArticleState(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"), patch)
	respond(w, result, err)
}

func (a *API) listStories(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.ListStories(r.Context(), currentUser(r.Context()).ID)
	respond(w, result, err)
}

func (a *API) patchStory(w http.ResponseWriter, r *http.Request) {
	var patch serverstore.ArticleStatePatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	err := a.store.SetStoryState(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"), patch)
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) listFolders(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.ListFolders(r.Context(), currentUser(r.Context()).ID)
	respond(w, result, err)
}

func (a *API) createFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.CreateFolder(r.Context(), currentUser(r.Context()).ID, body.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) deleteFolder(w http.ResponseWriter, r *http.Request) {
	err := a.store.DeleteFolder(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"))
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) assignFolder(w http.ResponseWriter, r *http.Request) {
	err := a.store.AssignFolder(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"), r.PathValue("feedId"), true)
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) unassignFolder(w http.ResponseWriter, r *http.Request) {
	err := a.store.AssignFolder(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"), r.PathValue("feedId"), false)
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) getSettings(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.GetSettings(r.Context(), currentUser(r.Context()).ID)
	respond(w, result, err)
}

func (a *API) patchSettings(w http.ResponseWriter, r *http.Request) {
	var patch map[string]any
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.UpdateSettings(r.Context(), currentUser(r.Context()).ID, patch)
	respond(w, result, err)
}

func (a *API) listReadLater(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := a.store.ListReadLater(r.Context(), currentUser(r.Context()).ID, q.Get("filter"), q.Get("search"), parseInt(q.Get("limit")))
	respond(w, result, err)
}

func (a *API) addReadLater(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.AddReadLater(r.Context(), currentUser(r.Context()).ID, body.URL)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) getReadLater(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.GetReadLater(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"))
	respond(w, result, err)
}

func (a *API) patchReadLater(w http.ResponseWriter, r *http.Request) {
	var patch serverstore.ReadLaterPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.SetReadLaterState(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"), patch)
	respond(w, result, err)
}

func (a *API) deleteReadLater(w http.ResponseWriter, r *http.Request) {
	err := a.store.DeleteReadLater(r.Context(), currentUser(r.Context()).ID, r.PathValue("id"))
	respond(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) getFollowedTeams(w http.ResponseWriter, r *http.Request) {
	result, err := a.store.FollowedTeams(r.Context(), currentUser(r.Context()).ID, r.PathValue("sport"))
	respond(w, result, err)
}

func (a *API) putFollowedTeams(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TeamIDs []string `json:"teamIds"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.SetFollowedTeams(r.Context(), currentUser(r.Context()).ID, r.PathValue("sport"), body.TeamIDs)
	respond(w, result, err)
}

func (a *API) mlbTeams(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsTeams(r.Context())
	respond(w, result, err)
}
func (a *API) mlbSeasons(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsSeasons(r.Context())
	respond(w, result, err)
}
func (a *API) mlbSchedule(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsSchedule(r.Context(), parseInt(r.URL.Query().Get("teamId")), parseInt(r.URL.Query().Get("season")))
	respond(w, result, err)
}
func (a *API) mlbDaily(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsDailySchedule(r.Context(), r.URL.Query().Get("date"))
	respond(w, result, err)
}
func (a *API) mlbGame(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsGameGet(r.Context(), parseInt(r.PathValue("id")))
	respond(w, result, err)
}
func (a *API) mlbStandings(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsStandings(r.Context(), parseInt(r.URL.Query().Get("season")))
	respond(w, result, err)
}
func (a *API) mlbRoster(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsRoster(r.Context(), parseInt(r.URL.Query().Get("teamId")), parseInt(r.URL.Query().Get("season")))
	respond(w, result, err)
}
func (a *API) f1Years(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsF1Years(r.Context())
	respond(w, result, err)
}
func (a *API) f1Races(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsF1Races(r.Context(), parseInt(r.URL.Query().Get("year")))
	respond(w, result, err)
}
func (a *API) f1Race(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsF1RaceGet(r.Context(), parseInt(r.PathValue("id")))
	respond(w, result, err)
}
func (a *API) f1Standings(w http.ResponseWriter, r *http.Request) {
	result, err := a.svc.SportsF1Standings(r.Context(), parseInt(r.URL.Query().Get("year")))
	respond(w, result, err)
}

func (a *API) aiStatus(w http.ResponseWriter, r *http.Request) {
	if a.svc.AI == nil {
		writeJSON(w, http.StatusOK, domain.AIStatus{})
		return
	}
	writeJSON(w, http.StatusOK, a.svc.AI.Status(r.Context()))
}

func respond(w http.ResponseWriter, result any, err error) {
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, serverstore.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "USERNAME_TAKEN", err)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err)
	case errors.Is(err, domain.ErrInvalidParams), errors.Is(err, domain.ErrInvalidURL), errors.Is(err, domain.ErrInvalidFeed):
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", errors.New("internal server error"))
	}
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": err.Error()}})
}

func parseBool(value string) bool { v, _ := strconv.ParseBool(value); return v }
func parseInt(value string) int   { v, _ := strconv.Atoi(value); return v }

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; frame-src 'none'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Debug("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}
