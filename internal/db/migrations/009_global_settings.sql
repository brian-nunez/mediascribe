CREATE TABLE IF NOT EXISTS global_settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME NOT NULL
);

INSERT OR IGNORE INTO global_settings (key, value, updated_at) 
VALUES ('dashboard_message', '', CURRENT_TIMESTAMP);
