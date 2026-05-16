CREATE TABLE IF NOT EXISTS blog_content_cache (
    blog_id TEXT NOT NULL,
    language TEXT NOT NULL,
    markdown TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (blog_id, language),
    FOREIGN KEY(blog_id) REFERENCES blog_catalog(id)
);
