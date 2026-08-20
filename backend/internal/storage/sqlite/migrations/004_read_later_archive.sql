ALTER TABLE articles ADD COLUMN archived_at TEXT;
ALTER TABLE settings ADD COLUMN read_later_chrome TEXT NOT NULL DEFAULT 'tabs';
