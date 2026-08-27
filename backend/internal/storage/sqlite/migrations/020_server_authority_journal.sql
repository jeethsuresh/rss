-- Desktop writes made while connected to a server are journaled locally before
-- they are sent. A failed server request is explicitly rolled back in this
-- journal; successful requests are followed by normal CRDT synchronization.
CREATE TABLE IF NOT EXISTS local_authority_mutations (
  id TEXT PRIMARY KEY,
  server_url TEXT NOT NULL,
  username TEXT NOT NULL COLLATE NOCASE,
  method TEXT NOT NULL,
  params TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL CHECK (status IN ('pending', 'committed', 'rolled_back')),
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_local_authority_mutations_status
  ON local_authority_mutations(status, created_at);
