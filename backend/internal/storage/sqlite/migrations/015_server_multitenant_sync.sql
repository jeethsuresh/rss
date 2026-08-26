-- Multi-tenant server overlays and local feed-list synchronization.
--
-- Feeds, articles, crawls, stories, and sports_cache remain canonical/global.
-- Tenant-owned state lives in user_* tables. Feed membership is materialized
-- from the append-only feed_sync_ops log with a last-write-wins tuple of
-- (logical_clock, device_id, op_id).

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  last_used_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS user_feeds (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  present INTEGER NOT NULL DEFAULT 1,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  op_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, feed_id)
);

CREATE INDEX IF NOT EXISTS idx_user_feeds_present ON user_feeds(user_id, present, feed_id);

CREATE TABLE IF NOT EXISTS feed_sync_ops (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  op_id TEXT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  feed_url TEXT NOT NULL,
  present INTEGER NOT NULL,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  client_created_at TEXT,
  received_at TEXT NOT NULL,
  UNIQUE (user_id, op_id)
);

CREATE INDEX IF NOT EXISTS idx_feed_sync_ops_cursor ON feed_sync_ops(user_id, sequence);

CREATE TABLE IF NOT EXISTS user_article_state (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  is_read INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_user_article_state_unread ON user_article_state(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_user_article_state_starred ON user_article_state(user_id, is_starred);

CREATE TABLE IF NOT EXISTS user_story_state (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
  is_read INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, story_id)
);

-- Only one server worker may fetch a canonical feed at a time. The adaptive
-- state survives restarts and also coordinates multiple server processes.
CREATE TABLE IF NOT EXISTS feed_fetch_state (
  feed_id TEXT PRIMARY KEY REFERENCES feeds(id) ON DELETE CASCADE,
  next_fetch_at TEXT NOT NULL,
  lease_until TEXT,
  lease_owner TEXT NOT NULL DEFAULT '',
  average_item_interval_seconds REAL NOT NULL DEFAULT 3600,
  last_changed_at TEXT,
  consecutive_unchanged INTEGER NOT NULL DEFAULT 0,
  consecutive_failures INTEGER NOT NULL DEFAULT 0,
  last_new_item_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_feed_fetch_due ON feed_fetch_state(next_fetch_at, lease_until);

CREATE TABLE IF NOT EXISTS feed_fetch_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  fetched_at TEXT NOT NULL,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  new_item_count INTEGER NOT NULL DEFAULT 0,
  not_modified INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_feed_fetch_log_feed ON feed_fetch_log(feed_id, fetched_at DESC);

INSERT OR IGNORE INTO feed_fetch_state(feed_id, next_fetch_at, updated_at)
SELECT id, COALESCE(last_attempt_at, created_at), updated_at
FROM feeds
WHERE is_read_later = 0;

-- Read-later membership is tenant-owned, while the fetched page and extracted
-- content are globally cached by normalized URL.
CREATE TABLE IF NOT EXISTS web_documents (
  id TEXT PRIMARY KEY,
  normalized_url TEXT NOT NULL UNIQUE,
  url TEXT NOT NULL,
  final_url TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  crawled_content TEXT NOT NULL DEFAULT '',
  reader_content TEXT NOT NULL DEFAULT '',
  crawl_status TEXT NOT NULL DEFAULT 'pending',
  crawl_error TEXT NOT NULL DEFAULT '',
  fetched_at TEXT,
  lease_until TEXT,
  lease_owner TEXT NOT NULL DEFAULT '',
  next_fetch_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_web_documents_due ON web_documents(crawl_status, next_fetch_at, lease_until);

CREATE TABLE IF NOT EXISTS user_read_later (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES web_documents(id) ON DELETE CASCADE,
  is_read INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (user_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_user_read_later_list ON user_read_later(user_id, archived_at, created_at DESC);

CREATE TABLE IF NOT EXISTS user_sports_followed_teams (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  sport TEXT NOT NULL,
  team_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (user_id, sport, team_id)
);

-- Desktop-side append-only log. Triggers capture normal local creates/deletes;
-- sync application temporarily raises the guard so pulled events do not echo.
CREATE TABLE IF NOT EXISTS local_sync_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  device_id TEXT NOT NULL
);

INSERT OR IGNORE INTO local_sync_config(id, device_id)
VALUES (1, lower(hex(randomblob(16))));

CREATE TABLE IF NOT EXISTS local_sync_apply_guard (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  applying INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO local_sync_apply_guard(id, applying) VALUES (1, 0);

CREATE TABLE IF NOT EXISTS local_feed_sync_ops (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  op_id TEXT NOT NULL UNIQUE,
  feed_url TEXT NOT NULL,
  present INTEGER NOT NULL,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  pushed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_local_feed_sync_pending ON local_feed_sync_ops(pushed_at, sequence);

CREATE TABLE IF NOT EXISTS local_feed_versions (
  feed_url TEXT PRIMARY KEY,
  present INTEGER NOT NULL,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  op_id TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS local_sync_accounts (
  server_url TEXT NOT NULL,
  username TEXT NOT NULL COLLATE NOCASE,
  cursor INTEGER NOT NULL DEFAULT 0,
  last_sync_at TEXT,
  last_error TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (server_url, username)
);

INSERT INTO local_feed_sync_ops(op_id, feed_url, present, logical_clock, device_id, created_at)
SELECT lower(hex(randomblob(16))), url, 1,
       CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
       (SELECT device_id FROM local_sync_config WHERE id = 1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM feeds
WHERE is_read_later = 0
  AND NOT EXISTS (SELECT 1 FROM local_feed_versions v WHERE v.feed_url = feeds.url);

INSERT OR IGNORE INTO local_feed_versions(feed_url, present, logical_clock, device_id, op_id)
SELECT feed_url, present, logical_clock, device_id, op_id
FROM local_feed_sync_ops;

CREATE TRIGGER IF NOT EXISTS feeds_local_sync_insert
AFTER INSERT ON feeds
WHEN NEW.is_read_later = 0
 AND (SELECT applying FROM local_sync_apply_guard WHERE id = 1) = 0
BEGIN
  INSERT INTO local_feed_sync_ops(op_id, feed_url, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), NEW.url, 1,
    CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
    (SELECT device_id FROM local_sync_config WHERE id = 1),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
  INSERT INTO local_feed_versions(feed_url, present, logical_clock, device_id, op_id)
  SELECT feed_url, present, logical_clock, device_id, op_id
  FROM local_feed_sync_ops WHERE sequence = last_insert_rowid()
  ON CONFLICT(feed_url) DO UPDATE SET
    present=excluded.present,
    logical_clock=excluded.logical_clock,
    device_id=excluded.device_id,
    op_id=excluded.op_id;
END;

CREATE TRIGGER IF NOT EXISTS feeds_local_sync_delete
AFTER DELETE ON feeds
WHEN OLD.is_read_later = 0
 AND (SELECT applying FROM local_sync_apply_guard WHERE id = 1) = 0
BEGIN
  INSERT INTO local_feed_sync_ops(op_id, feed_url, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), OLD.url, 0,
    CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
    (SELECT device_id FROM local_sync_config WHERE id = 1),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
  INSERT INTO local_feed_versions(feed_url, present, logical_clock, device_id, op_id)
  SELECT feed_url, present, logical_clock, device_id, op_id
  FROM local_feed_sync_ops WHERE sequence = last_insert_rowid()
  ON CONFLICT(feed_url) DO UPDATE SET
    present=excluded.present,
    logical_clock=excluded.logical_clock,
    device_id=excluded.device_id,
    op_id=excluded.op_id;
END;
