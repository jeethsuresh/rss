package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type StoryRepo struct{ db *DB }

func NewStoryRepo(db *DB) *StoryRepo { return &StoryRepo{db: db} }

func (r *StoryRepo) List(ctx context.Context) ([]domain.Story, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT s.id, s.title, s.summary, s.is_read, s.is_starred, s.created_at, s.updated_at,
		       COALESCE((SELECT COUNT(1) FROM story_articles sa WHERE sa.story_id = s.id), 0)
		FROM stories s
		ORDER BY s.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Story{}
	for rows.Next() {
		s, err := scanStory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *StoryRepo) Get(ctx context.Context, id string) (*domain.Story, error) {
	row := r.db.SQL.QueryRowContext(ctx, `
		SELECT s.id, s.title, s.summary, s.is_read, s.is_starred, s.created_at, s.updated_at,
		       COALESCE((SELECT COUNT(1) FROM story_articles sa WHERE sa.story_id = s.id), 0)
		FROM stories s WHERE s.id = ?`, id)
	s, err := scanStory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ids, err := r.memberIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	s.ArticleIDs = ids
	arts := make([]domain.Article, 0, len(ids))
	ar := NewArticleRepo(r.db)
	for _, aid := range ids {
		a, err := ar.Get(ctx, aid)
		if err != nil {
			continue
		}
		arts = append(arts, *a)
	}
	s.Articles = arts
	return &s, nil
}

func (r *StoryRepo) memberIDs(ctx context.Context, storyID string) ([]string, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT article_id FROM story_articles WHERE story_id = ?`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *StoryRepo) Create(ctx context.Context, story *domain.Story) error {
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO stories(id, title, summary, is_read, is_starred, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		story.ID, story.Title, story.Summary, boolToInt(story.IsRead), boolToInt(story.IsStarred),
		story.CreatedAt.UTC().Format(time.RFC3339Nano), story.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (r *StoryRepo) Update(ctx context.Context, story *domain.Story) error {
	res, err := r.db.SQL.ExecContext(ctx, `
		UPDATE stories SET title=?, summary=?, is_read=?, is_starred=?, updated_at=? WHERE id=?`,
		story.Title, story.Summary, boolToInt(story.IsRead), boolToInt(story.IsStarred),
		story.UpdatedAt.UTC().Format(time.RFC3339Nano), story.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *StoryRepo) SetMembers(ctx context.Context, storyID string, articleIDs []string) error {
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET story_id=NULL WHERE story_id=?`, storyID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM story_articles WHERE story_id=?`, storyID); err != nil {
		return err
	}
	for _, aid := range articleIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM story_articles WHERE article_id=?`, aid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO story_articles(story_id, article_id) VALUES (?, ?)`, storyID, aid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE articles SET story_id=? WHERE id=?`, storyID, aid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *StoryRepo) AddMember(ctx context.Context, storyID, articleID string) error {
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM story_articles WHERE article_id=?`, articleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO story_articles(story_id, article_id) VALUES (?, ?)`, storyID, articleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET story_id=? WHERE id=?`, storyID, articleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE stories SET updated_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), storyID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *StoryRepo) FindStoryForArticle(ctx context.Context, articleID string) (*domain.Story, error) {
	var storyID string
	err := r.db.SQL.QueryRowContext(ctx, `SELECT story_id FROM story_articles WHERE article_id=?`, articleID).Scan(&storyID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, storyID)
}

func (r *StoryRepo) CascadeFlags(ctx context.Context, storyID string, isRead *bool, isStarred *bool) error {
	story, err := r.Get(ctx, storyID)
	if err != nil {
		return err
	}
	if isRead != nil {
		story.IsRead = *isRead
	}
	if isStarred != nil {
		story.IsStarred = *isStarred
	}
	story.UpdatedAt = time.Now().UTC()
	if err := r.Update(ctx, story); err != nil {
		return err
	}
	ids, err := r.memberIDs(ctx, storyID)
	if err != nil {
		return err
	}
	ar := NewArticleRepo(r.db)
	for _, id := range ids {
		a, err := ar.Get(ctx, id)
		if err != nil {
			continue
		}
		if isRead != nil {
			a.IsRead = *isRead
		}
		if isStarred != nil {
			a.IsStarred = *isStarred
		}
		_ = ar.Update(ctx, a)
	}
	return nil
}

func scanStory(row rowScanner) (domain.Story, error) {
	var s domain.Story
	var isRead, isStarred int
	var created, updated string
	err := row.Scan(&s.ID, &s.Title, &s.Summary, &isRead, &isStarred, &created, &updated, &s.MemberCount)
	if err != nil {
		return s, err
	}
	s.IsRead = isRead == 1
	s.IsStarred = isStarred == 1
	s.CreatedAt = mustParseTime(created)
	s.UpdatedAt = mustParseTime(updated)
	return s, nil
}
