CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    url TEXT NOT NULL,
    domain TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    summary TEXT,
    file_path TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_user_created ON tasks (user_id, created_at DESC);
