CREATE TABLE IF NOT EXISTS blog_catalog_derived (
    blog_id TEXT PRIMARY KEY,
    preview_text TEXT,
    languages_json TEXT,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(blog_id) REFERENCES blog_catalog(id)
);
