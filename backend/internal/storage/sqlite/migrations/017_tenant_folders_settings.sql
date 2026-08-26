-- Remaining desktop-owned organizational/preferences data becomes tenant state.
-- AI connection settings stay server-global and environment-controlled.
CREATE TABLE IF NOT EXISTS user_folders (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (user_id, name)
);

CREATE TABLE IF NOT EXISTS user_feed_folders (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  folder_id TEXT NOT NULL REFERENCES user_folders(id) ON DELETE CASCADE,
  feed_id TEXT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, folder_id, feed_id)
);

CREATE INDEX IF NOT EXISTS idx_user_feed_folders_feed ON user_feed_folders(user_id, feed_id);

CREATE TABLE IF NOT EXISTS user_settings (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  theme TEXT NOT NULL DEFAULT 'system',
  article_density TEXT NOT NULL DEFAULT 'comfortable',
  default_sort TEXT NOT NULL DEFAULT 'newest',
  mark_read_on_open INTEGER NOT NULL DEFAULT 1,
  notifications_enabled INTEGER NOT NULL DEFAULT 0,
  read_later_chrome TEXT NOT NULL DEFAULT 'tabs',
  updated_at TEXT NOT NULL
);
