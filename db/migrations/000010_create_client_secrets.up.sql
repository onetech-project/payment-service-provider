CREATE TABLE IF NOT EXISTS client_secrets (
    id VARCHAR(36) PRIMARY KEY,
    client_id VARCHAR(64) NOT NULL REFERENCES client_apps(client_id) ON DELETE CASCADE,
    secret_id VARCHAR(64) NOT NULL,
    secret_value TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_client_secret UNIQUE(client_id, secret_id)
);

CREATE INDEX IF NOT EXISTS idx_client_secrets_lookup ON client_secrets(client_id, is_active);
