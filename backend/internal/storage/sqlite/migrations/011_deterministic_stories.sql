ALTER TABLE stories ADD COLUMN source TEXT NOT NULL DEFAULT 'ai';

CREATE TABLE IF NOT EXISTS story_token_weights (
  token TEXT PRIMARY KEY,
  up_count INTEGER NOT NULL DEFAULT 0,
  down_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS story_article_votes (
  story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
  article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  vote TEXT NOT NULL,
  member_snapshot TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  PRIMARY KEY (story_id, article_id)
);

CREATE TABLE IF NOT EXISTS story_votes (
  story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
  vote TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (story_id)
);
