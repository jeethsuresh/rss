package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

type StoryRepo struct{ db *DB }

func NewStoryRepo(db *DB) *StoryRepo { return &StoryRepo{db: db} }

const rssMemberCountSQL = `COALESCE((SELECT COUNT(1) FROM story_articles sa
		JOIN articles a ON a.id = sa.article_id
		WHERE sa.story_id = s.id AND a.is_read_later = 0), 0)`

func (r *StoryRepo) List(ctx context.Context) ([]domain.Story, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT s.id, s.title, s.summary, s.is_read, s.is_starred, s.created_at, s.updated_at,
		       `+rssMemberCountSQL+`, s.source
		FROM stories s
		WHERE `+rssMemberCountSQL+` >= 2
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
		       `+rssMemberCountSQL+`, s.source
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
	votes, err := r.ListArticleVotes(ctx, id)
	if err != nil {
		return nil, err
	}
	s.ArticleVotes = votes
	sv, err := r.GetStoryVote(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	s.Vote = sv
	return &s, nil
}

func (r *StoryRepo) memberIDs(ctx context.Context, storyID string) ([]string, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `
		SELECT sa.article_id FROM story_articles sa
		JOIN articles a ON a.id = sa.article_id
		WHERE sa.story_id = ? AND a.is_read_later = 0`, storyID)
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
	source := story.Source
	if source == "" {
		source = domain.StorySourceAI
	}
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO stories(id, title, summary, is_read, is_starred, created_at, updated_at, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		story.ID, story.Title, story.Summary, boolToInt(story.IsRead), boolToInt(story.IsStarred),
		story.CreatedAt.UTC().Format(time.RFC3339Nano), story.UpdatedAt.UTC().Format(time.RFC3339Nano),
		source,
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

func (r *StoryRepo) rssArticleIDs(ctx context.Context, articleIDs []string) ([]string, error) {
	ar := NewArticleRepo(r.db)
	out := make([]string, 0, len(articleIDs))
	seen := map[string]bool{}
	for _, aid := range articleIDs {
		if aid == "" || seen[aid] {
			continue
		}
		a, err := ar.Get(ctx, aid)
		if err != nil || a.IsReadLater {
			continue
		}
		seen[aid] = true
		out = append(out, aid)
	}
	return out, nil
}

func (r *StoryRepo) SetMembers(ctx context.Context, storyID string, articleIDs []string) error {
	ids, err := r.rssArticleIDs(ctx, articleIDs)
	if err != nil {
		return err
	}
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
	for _, aid := range ids {
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
	ids, err := r.rssArticleIDs(ctx, []string{articleID})
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
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

func (r *StoryRepo) RemoveMember(ctx context.Context, storyID, articleID string) error {
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM story_articles WHERE story_id=? AND article_id=?`, storyID, articleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET story_id=NULL WHERE id=? AND story_id=?`, articleID, storyID); err != nil {
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
	var created, updated, source string
	err := row.Scan(&s.ID, &s.Title, &s.Summary, &isRead, &isStarred, &created, &updated, &s.MemberCount, &source)
	if err != nil {
		return s, err
	}
	s.IsRead = isRead == 1
	s.IsStarred = isStarred == 1
	s.Source = source
	s.CreatedAt = mustParseTime(created)
	s.UpdatedAt = mustParseTime(updated)
	return s, nil
}

func (r *StoryRepo) SetSource(ctx context.Context, storyID, source string) error {
	res, err := r.db.SQL.ExecContext(ctx, `UPDATE stories SET source=? WHERE id=?`, source, storyID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *StoryRepo) GetTokenWeights(ctx context.Context) (map[string]domain.TokenFeedback, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT token, up_count, down_count FROM story_token_weights`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.TokenFeedback{}
	for rows.Next() {
		var token string
		var fb domain.TokenFeedback
		if err := rows.Scan(&token, &fb.Up, &fb.Down); err != nil {
			return nil, err
		}
		out[token] = fb
	}
	return out, rows.Err()
}

func (r *StoryRepo) AdjustTokenWeights(ctx context.Context, tokens []string, upDelta, downDelta int) error {
	if len(tokens) == 0 || (upDelta == 0 && downDelta == 0) {
		return nil
	}
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO story_token_weights(token, up_count, down_count) VALUES (?, ?, ?)
			ON CONFLICT(token) DO UPDATE SET
				up_count = MAX(0, story_token_weights.up_count + excluded.up_count),
				down_count = MAX(0, story_token_weights.down_count + excluded.down_count)`,
			token, upDelta, downDelta); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *StoryRepo) GetArticleVote(ctx context.Context, storyID, articleID string) (domain.ArticleVoteRecord, error) {
	var vote, snapshot string
	err := r.db.SQL.QueryRowContext(ctx, `
		SELECT vote, member_snapshot FROM story_article_votes WHERE story_id=? AND article_id=?`,
		storyID, articleID).Scan(&vote, &snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ArticleVoteRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ArticleVoteRecord{}, err
	}
	rec := domain.ArticleVoteRecord{Vote: domain.StoryVote(vote)}
	if snapshot != "" {
		_ = json.Unmarshal([]byte(snapshot), &rec.Snapshot)
	}
	return rec, nil
}

func (r *StoryRepo) SetArticleVote(ctx context.Context, storyID, articleID string, vote domain.StoryVote, snapshot []string) error {
	raw := "[]"
	if snapshot != nil {
		b, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO story_article_votes(story_id, article_id, vote, member_snapshot, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(story_id, article_id) DO UPDATE SET vote=excluded.vote, member_snapshot=excluded.member_snapshot, created_at=excluded.created_at`,
		storyID, articleID, string(vote), raw, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (r *StoryRepo) ClearArticleVote(ctx context.Context, storyID, articleID string) error {
	_, err := r.db.SQL.ExecContext(ctx, `DELETE FROM story_article_votes WHERE story_id=? AND article_id=?`, storyID, articleID)
	return err
}

func (r *StoryRepo) ListArticleVotes(ctx context.Context, storyID string) (map[string]domain.StoryVote, error) {
	rows, err := r.db.SQL.QueryContext(ctx, `SELECT article_id, vote FROM story_article_votes WHERE story_id=?`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.StoryVote{}
	for rows.Next() {
		var id, vote string
		if err := rows.Scan(&id, &vote); err != nil {
			return nil, err
		}
		out[id] = domain.StoryVote(vote)
	}
	return out, rows.Err()
}

func (r *StoryRepo) GetStoryVote(ctx context.Context, storyID string) (domain.StoryVote, error) {
	var vote string
	err := r.db.SQL.QueryRowContext(ctx, `SELECT vote FROM story_votes WHERE story_id=?`, storyID).Scan(&vote)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VoteNone, domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return domain.StoryVote(vote), nil
}

func (r *StoryRepo) SetStoryVote(ctx context.Context, storyID string, vote domain.StoryVote) error {
	_, err := r.db.SQL.ExecContext(ctx, `
		INSERT INTO story_votes(story_id, vote, created_at) VALUES (?, ?, ?)
		ON CONFLICT(story_id) DO UPDATE SET vote=excluded.vote, created_at=excluded.created_at`,
		storyID, string(vote), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (r *StoryRepo) ClearStoryVote(ctx context.Context, storyID string) error {
	_, err := r.db.SQL.ExecContext(ctx, `DELETE FROM story_votes WHERE story_id=?`, storyID)
	return err
}

func (r *StoryRepo) ClearAllMemberships(ctx context.Context) error {
	tx, err := r.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET story_id=NULL`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM stories`); err != nil {
		return err
	}
	return tx.Commit()
}
