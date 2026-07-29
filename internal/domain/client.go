package domain

import (
	"context"
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

// ClientSecret is a merchant's shared secret used to verify the symmetric
// X-SIGNATURE on merchant-facing transfer-va requests (feature
// 010-merchant-hmac-signature) — the HMAC analog of ClientKey's RSA public
// key, associated with the same client_id used for accessToken issuance.
type ClientSecret struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	SecretID    string    `json:"secret_id"`
	SecretValue string    `json:"-"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ClientRepository interface {
	GetClientByID(ctx context.Context, clientID string) (*ClientApp, error)
	GetActiveClientPublicKey(ctx context.Context, clientID string) (string, error)
	CreateClient(ctx context.Context, client *ClientApp) error
	CreateClientKey(ctx context.Context, key *ClientKey) error
	RevokeClientKey(ctx context.Context, clientID, keyID string) error
	GetActiveClientSecret(ctx context.Context, clientID string) (string, error)
	CreateClientSecret(ctx context.Context, secret *ClientSecret) error
	RevokeClientSecret(ctx context.Context, clientID, secretID string) error
}

// RegisterClientRequest registers a new B2B client_app (and, optionally, its
// first client_key) allowed to request SNAP access tokens.
type RegisterClientRequest struct {
	ClientID     string `json:"clientId"`
	ClientName   string `json:"clientName"`
	KeyID        string `json:"keyId,omitempty"`
	PublicKeyPEM string `json:"publicKeyPem,omitempty"`
	Algorithm    string `json:"algorithm,omitempty"`
}

// AddClientKeyRequest registers an additional public key (SNAP asymmetric
// signature verification) under an existing client_app, e.g. for key
// rotation.
type AddClientKeyRequest struct {
	KeyID        string `json:"keyId"`
	PublicKeyPEM string `json:"publicKeyPem"`
	Algorithm    string `json:"algorithm,omitempty"`
}

// AddClientSecretRequest provisions a merchant shared secret (feature
// 010-merchant-hmac-signature), used to verify the symmetric X-SIGNATURE on
// merchant-facing transfer-va requests, e.g. for secret rotation.
type AddClientSecretRequest struct {
	SecretID    string `json:"secretId"`
	SecretValue string `json:"secretValue"`
}

// ClientUsecase implements client onboarding: registering client_apps and
// their client_keys used to verify the asymmetric X-SIGNATURE on
// /openapi/v1.0/access-token/b2b, plus client_secrets used to verify the
// symmetric X-SIGNATURE on merchant-facing transfer-va requests.
type ClientUsecase interface {
	RegisterClient(ctx context.Context, client *ClientApp, key *ClientKey) error
	AddClientKey(ctx context.Context, key *ClientKey) error
	RevokeClientKey(ctx context.Context, clientID, keyID string) error
	AddClientSecret(ctx context.Context, secret *ClientSecret) error
	RevokeClientSecret(ctx context.Context, clientID, secretID string) error
}
