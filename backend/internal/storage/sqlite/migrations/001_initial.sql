PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS feeds (
  id TEXT PRIMARY KEY,
  url TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  site_url TEXT NOT NULL DEFAULT '',
  icon_url TEXT NOT NULL DEFAULT '',
  last_success_at TEXT,
  last_attempt_at TEXT,
  last_error TEXT NOT NULL DEFAULT '',
  etag TEXT NOT NULL DEFAULT '',
  last_modified TEXT NOT NULL DEFAULT '',
  poll_interval_seconds INTEGER NOT NULL DEFAULT 3600,
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS articles (
  id TEXT PRIMARY KEY,
  feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  title TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  published_at TEXT,
  updated_at TEXT,
  external_id TEXT NOT NULL DEFAULT '',
  fingerprint TEXT NOT NULL DEFAULT '',
  is_read INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  discovered_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_feed_fingerprint ON articles(feed_id, fingerprint);
CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_feed_external ON articles(feed_id, external_id) WHERE external_id != '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_feed_url ON articles(feed_id, url) WHERE url != '';
CREATE INDEX IF NOT EXISTS idx_articles_feed_id ON articles(feed_id);
CREATE INDEX IF NOT EXISTS idx_articles_unread ON articles(is_read, published_at);
CREATE INDEX IF NOT EXISTS idx_articles_starred ON articles(is_starred, published_at);
CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at DESC);

CREATE TABLE IF NOT EXISTS folders (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS feed_folders (
  folder_id TEXT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  PRIMARY KEY (folder_id, feed_id)
);

CREATE TABLE IF NOT EXISTS settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  default_poll_interval_seconds INTEGER NOT NULL DEFAULT 3600,
  theme TEXT NOT NULL DEFAULT 'system',
  article_density TEXT NOT NULL DEFAULT 'comfortable',
  default_sort TEXT NOT NULL DEFAULT 'newest',
  mark_read_on_open INTEGER NOT NULL DEFAULT 1,
  notifications_enabled INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO settings (id) VALUES (1);

CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
  title,
  author,
  content,
  summary,
  content='articles',
  content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS articles_ai AFTER INSERT ON articles BEGIN
  INSERT INTO articles_fts(rowid, title, author, content, summary)
  VALUES (new.rowid, new.title, new.author, new.content, new.summary);
END;

CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
  INSERT INTO articles_fts(articles_fts, rowid, title, author, content, summary)
  VALUES('delete', old.rowid, old.title, old.author, old.content, old.summary);
END;

CREATE TRIGGER IF NOT EXISTS articles_au AFTER UPDATE ON articles BEGIN
  INSERT INTO articles_fts(articles_fts, rowid, title, author, content, summary)
  VALUES('delete', old.rowid, old.title, old.author, old.content, old.summary);
  INSERT INTO articles_fts(rowid, title, author, content, summary)
  VALUES (new.rowid, new.title, new.author, new.content, new.summary);
END;
