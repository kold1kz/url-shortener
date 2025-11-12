-- migrations/000001_create_urls_table.up.sql
CREATE TABLE urls (
                      id VARCHAR(10) PRIMARY KEY,
                      original_url TEXT NOT NULL UNIQUE,
                      short_url TEXT NOT NULL,
                      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_urls_original ON urls(original_url);
CREATE INDEX idx_urls_short ON urls(short_url);
CREATE INDEX idx_urls_created ON urls(created_at);