package serverstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{2,63}$`)

var (
	ErrUsernameTaken      = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid username or password")
)

type Store struct {
	db       *sqlite.DB
	feeds    *sqlite.FeedRepo
	articles *sqlite.ArticleRepo
	stories  *sqlite.StoryRepo
	now      func() time.Time
}

func New(db *sqlite.DB) *Store {
	return &Store{
		db:       db,
		feeds:    sqlite.NewFeedRepo(db),
		articles: sqlite.NewArticleRepo(db),
		stories:  sqlite.NewStoryRepo(db),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s *Store) Register(ctx context.Context, username, password string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return nil, fmt.Errorf("%w: username must be 3-64 letters, digits, dots, dashes, or underscores", domain.ErrInvalidParams)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidParams, err)
	}
	now := s.now()
	user := User{ID: uuid.NewString(), Username: username, Created: now}
	_, err = s.db.SQL.ExecContext(ctx, `
		INSERT INTO users(id, username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, user.ID, user.Username, hash, formatTime(now), formatTime(now))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	return s.createSession(ctx, user)
}

func (s *Store) Login(ctx context.Context, username, password string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) || validatePassword(password) != nil {
		return nil, ErrInvalidCredentials
	}
	var user User
	var created, hash string
	err := s.db.SQL.QueryRowContext(ctx, `
		SELECT id, username, created_at, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &created, &hash)
	if err != nil {
		_ = verifyPassword(dummyPasswordHash, password)
		return nil, ErrInvalidCredentials
	}
	if !verifyPassword(hash, password) {
		return nil, ErrInvalidCredentials
	}
	user.Created = parseTime(created)
	return s.createSession(ctx, user)
}

func (s *Store) createSession(ctx context.Context, user User) (*AuthResult, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := sha256.Sum256([]byte(token))
	now := s.now()
	expires := now.Add(30 * 24 * time.Hour)
	_, _ = s.db.SQL.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at <= ?`, formatTime(now))
	_, err := s.db.SQL.ExecContext(ctx, `
		INSERT INTO auth_sessions(id, user_id, token_hash, created_at, expires_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), user.ID, hex.EncodeToString(tokenHash[:]),
		formatTime(now), formatTime(expires), formatTime(now))
	if err != nil {
		return nil, err
	}
	_, _ = s.db.SQL.ExecContext(ctx, `
		DELETE FROM auth_sessions
		WHERE user_id=? AND id NOT IN (
		  SELECT id FROM auth_sessions WHERE user_id=? ORDER BY created_at DESC LIMIT 20
		)`, user.ID, user.ID)
	return &AuthResult{Token: token, ExpiresAt: expires, User: user}, nil
}

func (s *Store) Authenticate(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, errors.New("missing bearer token")
	}
	h := sha256.Sum256([]byte(token))
	now := s.now()
	var user User
	var created string
	err := s.db.SQL.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.created_at
		FROM auth_sessions sess
		JOIN users u ON u.id = sess.user_id
		WHERE sess.token_hash = ? AND sess.expires_at > ?`, hex.EncodeToString(h[:]), formatTime(now)).
		Scan(&user.ID, &user.Username, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("invalid or expired session")
	}
	if err != nil {
		return nil, err
	}
	user.Created = parseTime(created)
	_, _ = s.db.SQL.ExecContext(ctx, `UPDATE auth_sessions SET last_used_at=? WHERE token_hash=?`, formatTime(now), hex.EncodeToString(h[:]))
	return &user, nil
}

func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", domain.ErrInvalidURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", domain.ErrInvalidURL
	}
	if u.User != nil {
		return "", domain.ErrInvalidURL
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" && !((u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443")) {
		host = net.JoinHostPort(host, port)
	}
	u.Host = host
	u.Fragment = ""
	return u.String(), nil
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, raw)
	}
	return t
}

func parseNullTime(raw sql.NullString) *time.Time {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	t := parseTime(raw.String)
	return &t
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
