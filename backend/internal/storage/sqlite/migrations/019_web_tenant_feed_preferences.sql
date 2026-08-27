-- Per-tenant feed presentation controls used by the hosted web client. The
-- adaptive server scheduler remains authoritative; poll_interval_seconds is a
-- reader preference and never bypasses the shared fetch heuristic.

ALTER TABLE user_feeds ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE user_feeds ADD COLUMN poll_interval_seconds INTEGER NOT NULL DEFAULT 3600;

ALTER TABLE user_settings ADD COLUMN default_poll_interval_seconds INTEGER NOT NULL DEFAULT 3600;

CREATE INDEX IF NOT EXISTS idx_user_feeds_fetchable
ON user_feeds(feed_id, present, enabled);
