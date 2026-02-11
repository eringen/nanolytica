-- Schema for sqlc type inference only. Not executed at runtime.
-- Runtime DDL is managed by ensureSchema() and migrate() in store.go.

CREATE TABLE visits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    ip_hash TEXT NOT NULL,
    browser TEXT NOT NULL,
    os TEXT NOT NULL,
    device TEXT NOT NULL,
    path TEXT NOT NULL,
    referrer TEXT,
    screen_size TEXT,
    timestamp DATETIME NOT NULL,
    duration_sec INTEGER DEFAULT 0
);

CREATE TABLE bot_visits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bot_name TEXT NOT NULL,
    ip_hash TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    path TEXT NOT NULL,
    timestamp DATETIME NOT NULL
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
