CREATE TABLE IF NOT EXISTS client_apps (
    id VARCHAR(36) PRIMARY KEY,
    client_id VARCHAR(64) UNIQUE NOT NULL,
    client_name VARCHAR(128) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_client_apps_client_id ON client_apps(client_id);
