package serverstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"time"
)

// LoginFederated binds an IAM identity by issuer and immutable subject. It never
// links an existing local account by a matching display name or username.
func (s *Store) LoginFederated(ctx context.Context, issuer, subject, displayName string) (*AuthResult, error) {
	if issuer == "" || subject == "" || displayName == "" {
		return nil, ErrInvalidCredentials
	}
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var user User
	var created string
	err = tx.QueryRowContext(ctx, `SELECT u.id,u.username,u.created_at FROM federated_identities f JOIN users u ON u.id=f.user_id WHERE f.issuer=? AND f.subject=?`, issuer, subject).Scan(&user.ID, &user.Username, &created)
	if errors.Is(err, sql.ErrNoRows) {
		digest := sha256.Sum256([]byte(issuer + "\x00" + subject))
		created = formatTime(s.now())
		user = User{ID: uuid.NewString(), Username: "iam." + hex.EncodeToString(digest[:16])}
		if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,username,password_hash,created_at,updated_at) VALUES (?,?,'',?,?)`, user.ID, user.Username, created, created); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO federated_identities(issuer,subject,user_id) VALUES (?,?,?)`, issuer, subject, user.ID); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	user.Created = parseTime(created)
	user.DisplayName = displayName
	if _, err = tx.ExecContext(ctx, `UPDATE users SET display_name=?,updated_at=? WHERE id=?`, displayName, formatTime(s.now()), user.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.createSessionFrom(ctx, user, issuer, 8*time.Hour)
}
func (s *Store) AuthenticateFederated(ctx context.Context, token, issuer string) (*User, error) {
	digest := sha256.Sum256([]byte(token))
	var source string
	if err := s.db.SQL.QueryRowContext(ctx, `SELECT auth_source FROM auth_sessions WHERE token_hash=?`, hex.EncodeToString(digest[:])).Scan(&source); err != nil || source != issuer {
		return nil, ErrInvalidCredentials
	}
	return s.Authenticate(ctx, token)
}
