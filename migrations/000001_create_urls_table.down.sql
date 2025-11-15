-- migrations/000001_create_urls_table.down.sql
DROP INDEX IF EXISTS idx_urls_created;
DROP INDEX IF EXISTS idx_urls_short;
DROP INDEX IF EXISTS idx_urls_original;
DROP TABLE IF EXISTS urls;