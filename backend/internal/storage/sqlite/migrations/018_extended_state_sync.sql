-- Append-only LWW synchronization for the remaining tenant-owned desktop data.
-- Feed membership keeps its dedicated log; these records cover article flags,
-- Read Later, folders, preferences, and followed sports teams.

CREATE TABLE IF NOT EXISTS state_sync_ops (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  op_id TEXT NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  object_key TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  present INTEGER NOT NULL DEFAULT 1,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  client_created_at TEXT,
  received_at TEXT NOT NULL,
  UNIQUE (user_id, op_id)
);

CREATE INDEX IF NOT EXISTS idx_state_sync_ops_cursor ON state_sync_ops(user_id, sequence);

CREATE TABLE IF NOT EXISTS user_sync_records (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  object_key TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  present INTEGER NOT NULL DEFAULT 1,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  op_id TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, kind, object_key)
);

CREATE TABLE IF NOT EXISTS local_state_sync_ops (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  op_id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL,
  object_key TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  present INTEGER NOT NULL DEFAULT 1,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  pushed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_local_state_sync_pending ON local_state_sync_ops(pushed_at, sequence);

CREATE TABLE IF NOT EXISTS local_state_versions (
  kind TEXT NOT NULL,
  object_key TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  present INTEGER NOT NULL DEFAULT 1,
  logical_clock INTEGER NOT NULL,
  device_id TEXT NOT NULL,
  op_id TEXT NOT NULL,
  PRIMARY KEY (kind, object_key)
);

CREATE TABLE IF NOT EXISTS local_state_clock (
  id INTEGER PRIMARY KEY CHECK (id=1),
  value INTEGER NOT NULL
);

INSERT OR IGNORE INTO local_state_clock(id, value)
VALUES (1, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER));

ALTER TABLE local_sync_accounts ADD COLUMN state_cursor INTEGER NOT NULL DEFAULT 0;

-- Feed membership uses the same Lamport clock as the extended state log. This
-- guarantees a local action wins after observing a remote clock and preserves
-- actual write order for rapid delete/re-add pairs.
CREATE TRIGGER IF NOT EXISTS local_feed_sync_advance_clock
AFTER INSERT ON local_feed_sync_ops
BEGIN
  UPDATE local_state_clock SET value=MAX(value, NEW.logical_clock) WHERE id=1;
END;

DROP TRIGGER IF EXISTS feeds_local_sync_insert;
CREATE TRIGGER feeds_local_sync_insert
AFTER INSERT ON feeds
WHEN NEW.is_read_later=0
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_feed_sync_ops(op_id, feed_url, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), NEW.url, 1,
    MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
        (SELECT value + 1 FROM local_state_clock WHERE id=1)),
    (SELECT device_id FROM local_sync_config WHERE id=1),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
  INSERT INTO local_feed_versions(feed_url, present, logical_clock, device_id, op_id)
  SELECT feed_url, present, logical_clock, device_id, op_id
  FROM local_feed_sync_ops WHERE sequence=last_insert_rowid()
  ON CONFLICT(feed_url) DO UPDATE SET
    present=excluded.present, logical_clock=excluded.logical_clock,
    device_id=excluded.device_id, op_id=excluded.op_id;
END;

DROP TRIGGER IF EXISTS feeds_local_sync_delete;
CREATE TRIGGER feeds_local_sync_delete
AFTER DELETE ON feeds
WHEN OLD.is_read_later=0
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_feed_sync_ops(op_id, feed_url, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), OLD.url, 0,
    MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
        (SELECT value + 1 FROM local_state_clock WHERE id=1)),
    (SELECT device_id FROM local_sync_config WHERE id=1),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
  INSERT INTO local_feed_versions(feed_url, present, logical_clock, device_id, op_id)
  SELECT feed_url, present, logical_clock, device_id, op_id
  FROM local_feed_sync_ops WHERE sequence=last_insert_rowid()
  ON CONFLICT(feed_url) DO UPDATE SET
    present=excluded.present, logical_clock=excluded.logical_clock,
    device_id=excluded.device_id, op_id=excluded.op_id;
END;

CREATE TRIGGER IF NOT EXISTS local_state_sync_materialize
AFTER INSERT ON local_state_sync_ops
BEGIN

  UPDATE local_state_clock SET value=MAX(value, NEW.logical_clock) WHERE id=1;

  INSERT INTO local_state_versions(
    kind, object_key, payload, present, logical_clock, device_id, op_id
  ) VALUES (
    NEW.kind, NEW.object_key, NEW.payload, NEW.present,
    NEW.logical_clock, NEW.device_id, NEW.op_id
  )
  ON CONFLICT(kind, object_key) DO UPDATE SET
    payload=excluded.payload,
    present=excluded.present,
    logical_clock=excluded.logical_clock,
    device_id=excluded.device_id,
    op_id=excluded.op_id
  WHERE excluded.logical_clock > local_state_versions.logical_clock
     OR (excluded.logical_clock = local_state_versions.logical_clock
         AND excluded.device_id > local_state_versions.device_id)
     OR (excluded.logical_clock = local_state_versions.logical_clock
         AND excluded.device_id = local_state_versions.device_id
         AND excluded.op_id > local_state_versions.op_id);
END;

-- Bootstrap the current local state before installing the live capture triggers.
INSERT INTO local_state_sync_ops(
  op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
)
SELECT lower(hex(randomblob(16))), 'article_state', f.url || char(10) || a.fingerprint,
       json_object('isRead', json(CASE WHEN a.is_read=1 THEN 'true' ELSE 'false' END),
                   'isStarred', json(CASE WHEN a.is_starred=1 THEN 'true' ELSE 'false' END)),
       1, MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
       (SELECT device_id FROM local_sync_config WHERE id=1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM articles a JOIN feeds f ON f.id=a.feed_id
WHERE a.is_read_later=0 AND (a.is_read=1 OR a.is_starred=1);

INSERT INTO local_state_sync_ops(
  op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
)
SELECT lower(hex(randomblob(16))), 'read_later', a.url,
       json_object('title', a.title, 'isRead', json(CASE WHEN a.is_read=1 THEN 'true' ELSE 'false' END),
                   'isStarred', json(CASE WHEN a.is_starred=1 THEN 'true' ELSE 'false' END),
                   'archivedAt', a.archived_at),
       1, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
       (SELECT device_id FROM local_sync_config WHERE id=1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM articles a WHERE a.is_read_later=1;

INSERT INTO local_state_sync_ops(
  op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
)
SELECT lower(hex(randomblob(16))), 'folder', id, json_object('name', name), 1,
       CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
       (SELECT device_id FROM local_sync_config WHERE id=1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM folders;

INSERT INTO local_state_sync_ops(
  op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
)
SELECT lower(hex(randomblob(16))), 'folder_feed', ff.folder_id || char(10) || f.url,
       json_object('folderId', ff.folder_id, 'feedUrl', f.url), 1,
       CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
       (SELECT device_id FROM local_sync_config WHERE id=1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM feed_folders ff JOIN feeds f ON f.id=ff.feed_id;

INSERT INTO local_state_sync_ops(
  op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
)
SELECT lower(hex(randomblob(16))), 'settings', 'singleton',
       json_object('defaultPollIntervalSeconds', default_poll_interval_seconds,
                   'theme', theme, 'articleDensity', article_density, 'defaultSort', default_sort,
                   'markReadOnOpen', json(CASE WHEN mark_read_on_open=1 THEN 'true' ELSE 'false' END),
                   'notificationsEnabled', json(CASE WHEN notifications_enabled=1 THEN 'true' ELSE 'false' END),
                   'readLaterChrome', read_later_chrome),
       1, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
       (SELECT device_id FROM local_sync_config WHERE id=1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM settings WHERE id=1;

INSERT INTO local_state_sync_ops(
  op_id, kind, object_key, payload, present, logical_clock, device_id, created_at
)
SELECT lower(hex(randomblob(16))), 'sports_team', 'mlb' || char(10) || CAST(team_id AS TEXT),
       json_object('sport', 'mlb', 'teamId', CAST(team_id AS TEXT)), 1,
       CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
       (SELECT device_id FROM local_sync_config WHERE id=1),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM sports_followed_teams;

CREATE TRIGGER IF NOT EXISTS articles_state_sync_update
AFTER UPDATE OF is_read, is_starred ON articles
WHEN NEW.is_read_later=0
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
 AND (NEW.is_read != OLD.is_read OR NEW.is_starred != OLD.is_starred)
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), 'article_state',
    (SELECT url FROM feeds WHERE id=NEW.feed_id) || char(10) || NEW.fingerprint,
    json_object('isRead', json(CASE WHEN NEW.is_read=1 THEN 'true' ELSE 'false' END),
                'isStarred', json(CASE WHEN NEW.is_starred=1 THEN 'true' ELSE 'false' END)),
    1, MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
           (SELECT value + 1 FROM local_state_clock WHERE id=1)),
    (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
END;

CREATE TRIGGER IF NOT EXISTS read_later_state_sync_insert
AFTER INSERT ON articles
WHEN NEW.is_read_later=1
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), 'read_later', NEW.url,
    json_object('title', NEW.title, 'isRead', json(CASE WHEN NEW.is_read=1 THEN 'true' ELSE 'false' END),
                'isStarred', json(CASE WHEN NEW.is_starred=1 THEN 'true' ELSE 'false' END),
                'archivedAt', NEW.archived_at),
    1, MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
           (SELECT value + 1 FROM local_state_clock WHERE id=1)),
    (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
END;

CREATE TRIGGER IF NOT EXISTS read_later_state_sync_update
AFTER UPDATE OF is_read, is_starred, archived_at, title, url ON articles
WHEN NEW.is_read_later=1
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
 AND (NEW.is_read != OLD.is_read OR NEW.is_starred != OLD.is_starred
      OR COALESCE(NEW.archived_at, '') != COALESCE(OLD.archived_at, '')
      OR NEW.title != OLD.title OR NEW.url != OLD.url)
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), 'read_later', NEW.url,
    json_object('title', NEW.title, 'isRead', json(CASE WHEN NEW.is_read=1 THEN 'true' ELSE 'false' END),
                'isStarred', json(CASE WHEN NEW.is_starred=1 THEN 'true' ELSE 'false' END),
                'archivedAt', NEW.archived_at),
    1, MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
           (SELECT value + 1 FROM local_state_clock WHERE id=1)),
    (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
END;

CREATE TRIGGER IF NOT EXISTS read_later_state_sync_delete
AFTER DELETE ON articles
WHEN OLD.is_read_later=1
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (
    lower(hex(randomblob(16))), 'read_later', OLD.url, '{}', 0,
    MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
        (SELECT value + 1 FROM local_state_clock WHERE id=1)),
    (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  );
END;

CREATE TRIGGER IF NOT EXISTS folders_state_sync_insert
AFTER INSERT ON folders
WHEN (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'folder', NEW.id, json_object('name', NEW.name), 1,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;

CREATE TRIGGER IF NOT EXISTS folders_state_sync_delete
AFTER DELETE ON folders
WHEN (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'folder', OLD.id, '{}', 0,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;

CREATE TRIGGER IF NOT EXISTS feed_folders_state_sync_insert
AFTER INSERT ON feed_folders
WHEN (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'folder_feed', NEW.folder_id || char(10) || (SELECT url FROM feeds WHERE id=NEW.feed_id),
          json_object('folderId', NEW.folder_id, 'feedUrl', (SELECT url FROM feeds WHERE id=NEW.feed_id)), 1,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;

CREATE TRIGGER IF NOT EXISTS feed_folders_state_sync_delete
AFTER DELETE ON feed_folders
WHEN (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
 AND EXISTS (SELECT 1 FROM feeds WHERE id=OLD.feed_id)
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'folder_feed', OLD.folder_id || char(10) || (SELECT url FROM feeds WHERE id=OLD.feed_id),
          json_object('folderId', OLD.folder_id, 'feedUrl', (SELECT url FROM feeds WHERE id=OLD.feed_id)), 0,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;

-- A feed cascade removes feed_folders after the feed row is no longer safe to
-- query from their AFTER DELETE trigger. Capture those tombstones while OLD.url
-- is still available; the guard suppresses this when applying a remote delete.
CREATE TRIGGER IF NOT EXISTS feeds_folder_state_sync_delete
BEFORE DELETE ON feeds
WHEN OLD.is_read_later=0
 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  SELECT lower(hex(randomblob(16))), 'folder_feed', ff.folder_id || char(10) || OLD.url,
         json_object('folderId', ff.folder_id, 'feedUrl', OLD.url), 0,
         MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
             (SELECT value + 1 FROM local_state_clock WHERE id=1)),
         (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  FROM feed_folders ff WHERE ff.feed_id=OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS settings_state_sync_update
AFTER UPDATE OF default_poll_interval_seconds, theme, article_density, default_sort, mark_read_on_open, notifications_enabled, read_later_chrome ON settings
WHEN NEW.id=1 AND (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'settings', 'singleton',
          json_object('defaultPollIntervalSeconds', NEW.default_poll_interval_seconds,
                      'theme', NEW.theme, 'articleDensity', NEW.article_density, 'defaultSort', NEW.default_sort,
                      'markReadOnOpen', json(CASE WHEN NEW.mark_read_on_open=1 THEN 'true' ELSE 'false' END),
                      'notificationsEnabled', json(CASE WHEN NEW.notifications_enabled=1 THEN 'true' ELSE 'false' END),
                      'readLaterChrome', NEW.read_later_chrome), 1,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;

CREATE TRIGGER IF NOT EXISTS sports_teams_state_sync_insert
AFTER INSERT ON sports_followed_teams
WHEN (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'sports_team', 'mlb' || char(10) || CAST(NEW.team_id AS TEXT),
          json_object('sport', 'mlb', 'teamId', CAST(NEW.team_id AS TEXT)), 1,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;

CREATE TRIGGER IF NOT EXISTS sports_teams_state_sync_delete
AFTER DELETE ON sports_followed_teams
WHEN (SELECT applying FROM local_sync_apply_guard WHERE id=1)=0
BEGIN
  INSERT INTO local_state_sync_ops(op_id, kind, object_key, payload, present, logical_clock, device_id, created_at)
  VALUES (lower(hex(randomblob(16))), 'sports_team', 'mlb' || char(10) || CAST(OLD.team_id AS TEXT),
          json_object('sport', 'mlb', 'teamId', CAST(OLD.team_id AS TEXT)), 0,
          MAX(CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
              (SELECT value + 1 FROM local_state_clock WHERE id=1)),
          (SELECT device_id FROM local_sync_config WHERE id=1), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
END;
