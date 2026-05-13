CREATE TABLE IF NOT EXISTS sections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS blog_catalog (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL UNIQUE,
    section_id TEXT,
    title TEXT NOT NULL,
    deleted INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS blog_content_overrides (
    id TEXT PRIMARY KEY,
    blog_id TEXT NOT NULL,
    language TEXT NOT NULL,
    markdown TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(blog_id, language)
);

CREATE TABLE IF NOT EXISTS admin_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    revoked_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_sections_sort_order ON sections(sort_order);
CREATE INDEX IF NOT EXISTS idx_blog_catalog_section_id ON blog_catalog(section_id);
CREATE INDEX IF NOT EXISTS idx_blog_catalog_deleted ON blog_catalog(deleted);
CREATE INDEX IF NOT EXISTS idx_blog_content_overrides_blog_id ON blog_content_overrides(blog_id);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_token_hash ON admin_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_user_id ON admin_sessions(user_id);
