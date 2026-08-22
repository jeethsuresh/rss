ALTER TABLE articles ADD COLUMN reader_content TEXT NOT NULL DEFAULT '';
ALTER TABLE articles ADD COLUMN extract_status TEXT NOT NULL DEFAULT 'none';
ALTER TABLE articles ADD COLUMN extract_source TEXT NOT NULL DEFAULT '';
ALTER TABLE articles ADD COLUMN crawl_retryable INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_articles_extract_status ON articles(extract_status);
CREATE INDEX IF NOT EXISTS idx_articles_crawl_retry ON articles(crawl_status, crawl_retryable);
