-- Crawl + AI queue + Read Later

ALTER TABLE articles ADD COLUMN rss_content TEXT NOT NULL DEFAULT '';
ALTER TABLE articles ADD COLUMN crawled_content TEXT NOT NULL DEFAULT '';
ALTER TABLE articles ADD COLUMN crawl_status TEXT NOT NULL DEFAULT 'none';
ALTER TABLE articles ADD COLUMN crawl_error TEXT NOT NULL DEFAULT '';
ALTER TABLE articles ADD COLUMN crawl_unreliable INTEGER NOT NULL DEFAULT 0;
ALTER TABLE articles ADD COLUMN is_read_later INTEGER NOT NULL DEFAULT 0;
ALTER TABLE articles ADD COLUMN live_content TEXT NOT NULL DEFAULT '';

UPDATE articles SET rss_content = content WHERE rss_content = '' AND content != '';

ALTER TABLE feeds ADD COLUMN crawl_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN crawl_failures INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN is_read_later INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS ai_queue (
  article_id TEXT PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  enqueued_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_ai_queue_status ON ai_queue(status, enqueued_at);

CREATE TABLE IF NOT EXISTS ai_logs (
  id TEXT PRIMARY KEY,
  ts TEXT NOT NULL,
  level TEXT NOT NULL DEFAULT 'info',
  article_id TEXT,
  message TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_ai_logs_ts ON ai_logs(ts DESC);
