CREATE TABLE IF NOT EXISTS client_keys (
    id VARCHAR(36) PRIMARY KEY,
    client_id VARCHAR(64) NOT NULL REFERENCES client_apps(client_id) ON DELETE CASCADE,
    key_id VARCHAR(64) NOT NULL,
    public_key_pem TEXT NOT NULL,
    algorithm VARCHAR(32) NOT NULL DEFAULT 'SHA256withRSA',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_client_key UNIQUE(client_id, key_id)
);

CREATE INDEX IF NOT EXISTS idx_client_keys_lookup ON client_keys(client_id, is_active);
