CREATE TABLE IF NOT EXISTS blog_output_embeddings (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    language TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    embedding BLOB NOT NULL,
    embedding_dimensions INTEGER NOT NULL,
    embedding_model TEXT NOT NULL,
    embedding_model_base_url TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(job_id, language)
);

CREATE INDEX IF NOT EXISTS idx_blog_output_embeddings_job_id ON blog_output_embeddings(job_id);
CREATE INDEX IF NOT EXISTS idx_blog_output_embeddings_language ON blog_output_embeddings(language);
