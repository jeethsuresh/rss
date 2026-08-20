ALTER TABLE articles ADD COLUMN priority TEXT NOT NULL DEFAULT 'none';
ALTER TABLE articles ADD COLUMN story_id TEXT;

CREATE TABLE IF NOT EXISTS stories (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  is_read INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS story_articles (
  story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
  article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  PRIMARY KEY (story_id, article_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_story_articles_article ON story_articles(article_id);
CREATE INDEX IF NOT EXISTS idx_articles_priority ON articles(priority);
CREATE INDEX IF NOT EXISTS idx_stories_updated ON stories(updated_at DESC);

ALTER TABLE settings ADD COLUMN ai_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settings ADD COLUMN ai_base_url TEXT NOT NULL DEFAULT 'http://127.0.0.1:1234/v1';
ALTER TABLE settings ADD COLUMN ai_model TEXT NOT NULL DEFAULT '';
