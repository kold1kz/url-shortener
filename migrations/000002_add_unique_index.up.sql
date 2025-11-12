-- migrations/000002_add_unique_index.up.sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_original_unique ON urls(original_url);
