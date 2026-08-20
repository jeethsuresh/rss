package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type FolderRepo struct{ db *DB }

func NewFolderRepo(db *DB) *FolderRepo { return &FolderRepo{db: db} }

func (r *FolderRepo) List(ctx context.Context) ([]domain.Folder, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT id, name, created_at FROM folders ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Folder{}
	for rows.Next() {
		var f domain.Folder
		var created string
		if err := rows.Scan(&f.ID, &f.Name, &created); err != nil {
			return nil, err
		}
		f.CreatedAt = mustParseTime(created)
		f.FeedIDs = []string{}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachFeedIDs(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *FolderRepo) attachFeedIDs(ctx context.Context, folders []domain.Folder) error {
	if len(folders) == 0 {
		return nil
	}
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT folder_id, feed_id FROM feed_folders`)
	if err != nil {
		return err
	}
	defer rows.Close()
	byFolder := make(map[string][]string, len(folders))
	for rows.Next() {
		var folderID, feedID string
		if err := rows.Scan(&folderID, &feedID); err != nil {
			return err
		}
		byFolder[folderID] = append(byFolder[folderID], feedID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range folders {
		if ids, ok := byFolder[folders[i].ID]; ok {
			folders[i].FeedIDs = ids
		}
	}
	return nil
}

func (r *FolderRepo) Get(ctx context.Context, id string) (*domain.Folder, error) {
	var f domain.Folder
	var created string
	err := r.db.SQL.QueryRowContext(ctx, `SELECT id, name, created_at FROM folders WHERE id = ?`, id).Scan(&f.ID, &f.Name, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.CreatedAt = mustParseTime(created)
	return &f, nil
}

func (r *FolderRepo) Create(ctx context.Context, folder *domain.Folder) error {
	_, err := r.db.SQL.ExecContext(ctx, `INSERT INTO folders(id, name, created_at) VALUES (?, ?, ?)`,
		folder.ID, folder.Name, folder.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (r *FolderRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.SQL.ExecContext(ctx, `DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FolderRepo) AssignFeed(ctx context.Context, folderID, feedID string) error {
	_, err := r.db.SQL.ExecContext(ctx, `INSERT OR IGNORE INTO feed_folders(folder_id, feed_id) VALUES (?, ?)`, folderID, feedID)
	return err
}

func (r *FolderRepo) UnassignFeed(ctx context.Context, folderID, feedID string) error {
	_, err := r.db.SQL.ExecContext(ctx, `DELETE FROM feed_folders WHERE folder_id = ? AND feed_id = ?`, folderID, feedID)
	return err
}

func (r *FolderRepo) FeedIDsInFolder(ctx context.Context, folderID string) ([]string, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT feed_id FROM feed_folders WHERE folder_id = ?`, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
