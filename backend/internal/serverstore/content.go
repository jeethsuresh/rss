package serverstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jeeth/rss-reader/backend/internal/domain"
)

func (s *Store) ListArticles(ctx context.Context, userID, feedID string, unreadOnly, starredOnly bool, limit int) ([]domain.Article, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := []string{"uf.user_id=?", "uf.present=1", "a.is_read_later=0"}
	args := []any{userID}
	if feedID != "" {
		where = append(where, "a.feed_id=?")
		args = append(args, feedID)
	}
	if unreadOnly {
		where = append(where, "COALESCE(uas.is_read, 0)=0")
	}
	if starredOnly {
		where = append(where, "COALESCE(uas.is_starred, 0)=1")
	}
	args = append(args, limit)
	rows, err := s.db.SQL.QueryContext(ctx, fmt.Sprintf(`
		SELECT a.id, COALESCE(uas.is_read, 0), COALESCE(uas.is_starred, 0)
		FROM articles a
		JOIN user_feeds uf ON uf.feed_id=a.feed_id
		LEFT JOIN user_article_state uas ON uas.user_id=uf.user_id AND uas.article_id=a.id
		WHERE %s
		ORDER BY COALESCE(a.published_at, a.discovered_at) DESC, a.id DESC
		LIMIT ?`, strings.Join(where, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type itemState struct {
		id      string
		read    int
		starred int
	}
	states := []itemState{}
	for rows.Next() {
		var state itemState
		if err := rows.Scan(&state.id, &state.read, &state.starred); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.Article, 0, len(states))
	for _, state := range states {
		article, err := s.articles.Get(ctx, state.id)
		if err != nil {
			return nil, err
		}
		article.IsRead = state.read == 1
		article.IsStarred = state.starred == 1
		out = append(out, *article)
	}
	return out, nil
}

func (s *Store) GetArticle(ctx context.Context, userID, articleID string) (*domain.Article, error) {
	var read, starred int
	err := s.db.SQL.QueryRowContext(ctx, `
		SELECT COALESCE(uas.is_read, 0), COALESCE(uas.is_starred, 0)
		FROM articles a
		JOIN user_feeds uf ON uf.feed_id=a.feed_id AND uf.user_id=? AND uf.present=1
		LEFT JOIN user_article_state uas ON uas.user_id=uf.user_id AND uas.article_id=a.id
		WHERE a.id=?`, userID, articleID).Scan(&read, &starred)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	article, err := s.articles.Get(ctx, articleID)
	if err != nil {
		return nil, err
	}
	article.IsRead = read == 1
	article.IsStarred = starred == 1
	return article, nil
}

func (s *Store) SetArticleState(ctx context.Context, userID, articleID string, patch ArticleStatePatch) (*domain.Article, error) {
	article, err := s.GetArticle(ctx, userID, articleID)
	if err != nil {
		return nil, err
	}
	if patch.IsRead != nil {
		article.IsRead = *patch.IsRead
	}
	if patch.IsStarred != nil {
		article.IsStarred = *patch.IsStarred
	}
	now := s.now()
	_, err = s.db.SQL.ExecContext(ctx, `
		INSERT INTO user_article_state(user_id, article_id, is_read, is_starred, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, article_id) DO UPDATE SET
			is_read=excluded.is_read, is_starred=excluded.is_starred, updated_at=excluded.updated_at`,
		userID, articleID, boolInt(article.IsRead), boolInt(article.IsStarred), formatTime(now))
	if err != nil {
		return nil, err
	}
	result, err := s.GetArticle(ctx, userID, articleID)
	if err != nil {
		return nil, err
	}
	var feedURL, fingerprint string
	if err := s.db.SQL.QueryRowContext(ctx, `
		SELECT f.url, a.fingerprint FROM articles a JOIN feeds f ON f.id=a.feed_id
		WHERE a.id=?`, articleID).Scan(&feedURL, &fingerprint); err != nil {
		return nil, err
	}
	if err := s.AppendState(ctx, userID, "article_state", articleStateKey(feedURL, fingerprint), map[string]any{
		"isRead": result.IsRead, "isStarred": result.IsStarred,
	}, true); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ListStories(ctx context.Context, userID string) ([]domain.Story, error) {
	rows, err := s.db.SQL.QueryContext(ctx, `
		SELECT s.id, COUNT(DISTINCT a.id)
		FROM stories s
		JOIN story_articles sa ON sa.story_id=s.id
		JOIN articles a ON a.id=sa.article_id
		JOIN user_feeds uf ON uf.feed_id=a.feed_id AND uf.user_id=? AND uf.present=1
		WHERE a.is_read_later=0
		GROUP BY s.id
		HAVING COUNT(DISTINCT a.id) >= 2
		ORDER BY s.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type storyCount struct {
		id    string
		count int
	}
	ids := []storyCount{}
	for rows.Next() {
		var item storyCount
		if err := rows.Scan(&item.id, &item.count); err != nil {
			return nil, err
		}
		ids = append(ids, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.Story, 0, len(ids))
	for _, item := range ids {
		story, err := s.stories.Get(ctx, item.id)
		if err != nil {
			return nil, err
		}
		story.Articles = nil
		story.ArticleIDs = nil
		story.MemberCount = item.count
		var read, starred int
		err = s.db.SQL.QueryRowContext(ctx, `
			SELECT CASE WHEN EXISTS (
			  SELECT 1 FROM story_articles sa
			  JOIN articles a ON a.id=sa.article_id
			  JOIN user_feeds uf ON uf.feed_id=a.feed_id AND uf.user_id=? AND uf.present=1
			  LEFT JOIN user_article_state uas ON uas.user_id=uf.user_id AND uas.article_id=a.id
			  WHERE sa.story_id=? AND COALESCE(uas.is_read, 0)=0
			) THEN 0 ELSE 1 END,
			COALESCE((SELECT is_starred FROM user_story_state WHERE user_id=? AND story_id=?), 0)`,
			userID, item.id, userID, item.id).Scan(&read, &starred)
		if err != nil {
			return nil, err
		}
		story.IsRead = read == 1
		story.IsStarred = starred == 1
		out = append(out, *story)
	}
	return out, nil
}

func (s *Store) SetStoryState(ctx context.Context, userID, storyID string, patch ArticleStatePatch) error {
	var exists int
	if err := s.db.SQL.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM story_articles sa
		JOIN articles a ON a.id=sa.article_id
		JOIN user_feeds uf ON uf.feed_id=a.feed_id AND uf.user_id=? AND uf.present=1
		WHERE sa.story_id=?`, userID, storyID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return domain.ErrNotFound
	}
	var read, starred int
	err := s.db.SQL.QueryRowContext(ctx, `SELECT is_read, is_starred FROM user_story_state WHERE user_id=? AND story_id=?`, userID, storyID).
		Scan(&read, &starred)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if patch.IsRead != nil {
		read = boolInt(*patch.IsRead)
	}
	if patch.IsStarred != nil {
		starred = boolInt(*patch.IsStarred)
	}
	tx, err := s.db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_story_state(user_id, story_id, is_read, is_starred, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, story_id) DO UPDATE SET
			is_read=excluded.is_read, is_starred=excluded.is_starred, updated_at=excluded.updated_at`,
		userID, storyID, read, starred, formatTime(s.now()))
	if err != nil {
		return err
	}
	if patch.IsRead != nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_article_state(user_id, article_id, is_read, is_starred, updated_at)
			SELECT ?, a.id, ?, 0, ?
			FROM story_articles sa
			JOIN articles a ON a.id=sa.article_id
			JOIN user_feeds uf ON uf.feed_id=a.feed_id AND uf.user_id=? AND uf.present=1
			WHERE sa.story_id=?
			ON CONFLICT(user_id, article_id) DO UPDATE SET
			  is_read=excluded.is_read, updated_at=excluded.updated_at`,
			userID, boolInt(*patch.IsRead), formatTime(s.now()), userID, storyID)
		if err != nil {
			return err
		}
	}
	if patch.IsStarred != nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_article_state(user_id, article_id, is_read, is_starred, updated_at)
			SELECT ?, a.id, 0, ?, ?
			FROM story_articles sa
			JOIN articles a ON a.id=sa.article_id
			JOIN user_feeds uf ON uf.feed_id=a.feed_id AND uf.user_id=? AND uf.present=1
			WHERE sa.story_id=?
			ON CONFLICT(user_id, article_id) DO UPDATE SET
			  is_starred=excluded.is_starred, updated_at=excluded.updated_at`,
			userID, boolInt(*patch.IsStarred), formatTime(s.now()), userID, storyID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
