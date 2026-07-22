# Data Model & Schema Definitions: SNAP Token Management

**Feature Branch**: `001-snap-token-management`
**Date**: 2026-07-22

---

## 1. Domain Entities & Database Schemas

### Entity: `ClientApp`
Represents an onboarded partner client or internal consumer system.

**Database Table**: `client_apps`

```sql
CREATE TABLE client_apps (
    id VARCHAR(36) PRIMARY KEY,              -- UUID v4
    client_id VARCHAR(64) UNIQUE NOT NULL,    -- Public Client ID / X-CLIENT-KEY
    client_name VARCHAR(128) NOT NULL,        -- Human-readable partner name
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, REVOKED, SUSPENDED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_client_apps_client_id ON client_apps(client_id);
```

---

### Entity: `ClientKey`
Represents cryptographic public keys assigned to clients for SNAP signature verification.

**Database Table**: `client_keys`

```sql
CREATE TABLE client_keys (
    id VARCHAR(36) PRIMARY KEY,              -- UUID v4
    client_id VARCHAR(64) NOT NULL REFERENCES client_apps(client_id) ON DELETE CASCADE,
    key_id VARCHAR(64) NOT NULL,             -- Unique Key ID
    public_key_pem TEXT NOT NULL,            -- RSA Public Key in PEM format
    algorithm VARCHAR(32) NOT NULL DEFAULT 'SHA256withRSA',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_client_key UNIQUE(client_id, key_id)
);

CREATE INDEX idx_client_keys_lookup ON client_keys(client_id, is_active);
```

---

### Entity: `TokenSession`
Represents active or revoked JWT B2B access token sessions.

**Database Table**: `token_sessions`

```sql
CREATE TABLE token_sessions (
    id VARCHAR(36) PRIMARY KEY,              -- UUID v4 (JTI)
    client_id VARCHAR(64) NOT NULL,          -- Client ID
    token_type VARCHAR(20) NOT NULL DEFAULT 'Bearer',
    expires_in_seconds INT NOT NULL DEFAULT 900,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    ip_address VARCHAR(45) EMPTY NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_token_sessions_jti ON token_sessions(id);
CREATE INDEX idx_token_sessions_lookup ON token_sessions(client_id, is_revoked, expires_at);
```

---

## 2. Redis Key Schemas & Caching

| Key Pattern | Data Type | TTL | Purpose |
|-------------|-----------|-----|---------|
| `client:key:{client_id}` | String (PEM format) | 1 hour | Caches client public key for sub-millisecond signature verification |
| `token:session:{jti}` | Hash / JSON | 900 seconds | Active JWT session cache for rapid token validation |
| `idempotency:lock:{key}` | String (`1`) | 30 seconds | In-flight atomic processing lock (`SETNX`) |
| `idempotency:val:{key}` | JSON String | 24 hours | Replay cache for idempotency key responses |

---

## 3. Go Structural Models

```go
package domain

import (
	"time"
)

type ClientStatus string

const (
	ClientStatusActive    ClientStatus = "ACTIVE"
	ClientStatusRevoked   ClientStatus = "REVOKED"
	ClientStatusSuspended ClientStatus = "SUSPENDED"
)

type ClientApp struct {
	ID         string       `json:"id"`
	ClientID   string       `json:"client_id"`
	ClientName string       `json:"client_name"`
	Status     ClientStatus `json:"status"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

type ClientKey struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	KeyID        string    `json:"key_id"`
	PublicKeyPEM string    `json:"public_key_pem"`
	Algorithm    string    `json:"algorithm"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SNAPTokenRequest struct {
	GrantType      string                 `json:"grantType" validate:"required,eq=client_credentials"`
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

type SNAPTokenResponse struct {
	ResponseCode    string                 `json:"responseCode"`
	ResponseMessage string                 `json:"responseMessage"`
	AccessToken     string                 `json:"accessToken,omitempty"`
	TokenType       string                 `json:"tokenType,omitempty"`
	ExpiresIn       string                 `json:"expiresIn,omitempty"`
	AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
}
```
