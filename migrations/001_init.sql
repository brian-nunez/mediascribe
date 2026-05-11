CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_url TEXT,
    source_path TEXT,
    title TEXT,

    status TEXT NOT NULL,
    current_stage TEXT,
    error_message TEXT,

    artifact_dir TEXT NOT NULL,

    main_model TEXT NOT NULL DEFAULT 'gpt-oss',
    main_model_base_url TEXT NOT NULL,

    embedding_model TEXT NOT NULL DEFAULT 'embeddinggemma',
    embedding_model_base_url TEXT NOT NULL,

    translation_model TEXT NOT NULL DEFAULT 'translategemma',
    translation_model_base_url TEXT NOT NULL,

    translation_enabled INTEGER NOT NULL DEFAULT 0,
    translation_language TEXT,

    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE IF NOT EXISTS transcript_chunks (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,

    start_time_seconds REAL,
    end_time_seconds REAL,

    content TEXT NOT NULL,
    token_count INTEGER,

    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chunk_embeddings (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    chunk_id TEXT NOT NULL,

    embedding BLOB NOT NULL,
    embedding_dimensions INTEGER NOT NULL,

    embedding_model TEXT NOT NULL,
    embedding_model_base_url TEXT NOT NULL,

    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chunk_analysis (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    chunk_id TEXT NOT NULL,

    analysis_json TEXT NOT NULL,

    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS blog_outputs (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL UNIQUE,

    title TEXT NOT NULL,
    slug TEXT NOT NULL,

    outline_path TEXT,
    draft_path TEXT,
    final_markdown_path TEXT,

    translated_markdown_path TEXT,
    translation_language TEXT,

    status TEXT NOT NULL,

    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_transcript_chunks_job_id ON transcript_chunks(job_id);
CREATE INDEX IF NOT EXISTS idx_chunk_embeddings_job_id ON chunk_embeddings(job_id);
CREATE INDEX IF NOT EXISTS idx_chunk_analysis_job_id ON chunk_analysis(job_id);
CREATE INDEX IF NOT EXISTS idx_blog_outputs_job_id ON blog_outputs(job_id);
