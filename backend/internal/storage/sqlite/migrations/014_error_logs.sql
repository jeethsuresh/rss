CREATE TABLE IF NOT EXISTS error_logs (
  id TEXT PRIMARY KEY,
  occurred_at TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  operation TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_error_logs_occurred_at ON error_logs(occurred_at DESC);
