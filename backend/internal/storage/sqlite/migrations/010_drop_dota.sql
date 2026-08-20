-- Hard-remove Dota 2 state (follows, pins, tokens, provider, cache).

DROP TABLE IF EXISTS sports_dota_followed_teams;
DROP TABLE IF EXISTS sports_dota_pinned_events;

DELETE FROM sports_cache WHERE cache_key LIKE 'dota.%';

CREATE TABLE settings_no_dota (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  default_poll_interval_seconds INTEGER NOT NULL DEFAULT 3600,
  theme TEXT NOT NULL DEFAULT 'system',
  article_density TEXT NOT NULL DEFAULT 'comfortable',
  default_sort TEXT NOT NULL DEFAULT 'newest',
  mark_read_on_open INTEGER NOT NULL DEFAULT 1,
  notifications_enabled INTEGER NOT NULL DEFAULT 0,
  ai_enabled INTEGER NOT NULL DEFAULT 0,
  ai_base_url TEXT NOT NULL DEFAULT 'http://127.0.0.1:1234/v1',
  ai_model TEXT NOT NULL DEFAULT '',
  read_later_chrome TEXT NOT NULL DEFAULT 'tabs'
);

INSERT INTO settings_no_dota (
  id, default_poll_interval_seconds, theme, article_density, default_sort,
  mark_read_on_open, notifications_enabled, ai_enabled, ai_base_url, ai_model, read_later_chrome
)
SELECT
  id, default_poll_interval_seconds, theme, article_density, default_sort,
  mark_read_on_open, notifications_enabled, ai_enabled, ai_base_url, ai_model,
  COALESCE(read_later_chrome, 'tabs')
FROM settings;

DROP TABLE settings;
ALTER TABLE settings_no_dota RENAME TO settings;
INSERT OR IGNORE INTO settings (id) VALUES (1);
