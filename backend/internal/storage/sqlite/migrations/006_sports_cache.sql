CREATE TABLE IF NOT EXISTS sports_cache (
  cache_key TEXT PRIMARY KEY NOT NULL,
  payload TEXT NOT NULL,
  fetched_at TEXT NOT NULL
);
