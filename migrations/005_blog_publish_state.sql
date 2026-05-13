CREATE TABLE IF NOT EXISTS blog_publish_state (
    blog_id TEXT PRIMARY KEY,
    published INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_blog_publish_state_published ON blog_publish_state(published);
