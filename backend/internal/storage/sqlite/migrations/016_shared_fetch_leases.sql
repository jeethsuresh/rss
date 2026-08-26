-- Cross-process single-flight leases for globally cached external resources
-- such as sports schedules, standings, rosters, games, and race data.
CREATE TABLE IF NOT EXISTS shared_fetch_leases (
  cache_key TEXT PRIMARY KEY,
  lease_owner TEXT NOT NULL,
  lease_until TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_shared_fetch_leases_expiry ON shared_fetch_leases(lease_until);
