CREATE TABLE IF NOT EXISTS job_batches (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    delay_seconds INTEGER NOT NULL,
    status TEXT NOT NULL,
    current_item_index INTEGER NOT NULL DEFAULT 0,
    current_job_id TEXT,
    processed_items INTEGER NOT NULL DEFAULT 0,
    total_items INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    next_run_at TEXT,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS job_batch_items (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL,
    item_index INTEGER NOT NULL,
    source_type TEXT NOT NULL,
    source_url TEXT,
    source_path TEXT,
    title TEXT,
    section_id TEXT,
    main_model TEXT,
    job_id TEXT,
    status TEXT NOT NULL,
    error_message TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(batch_id) REFERENCES job_batches(id)
);

CREATE INDEX IF NOT EXISTS idx_job_batch_items_batch_idx
    ON job_batch_items(batch_id, item_index);
