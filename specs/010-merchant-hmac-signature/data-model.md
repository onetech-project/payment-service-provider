# Phase 1 Data Model: Merchant HMAC Signature Verification

## Entity: ClientSecret (new)

New table `client_secrets`, mirroring `client_keys` structurally:

| Field | Type | Notes |
|---|---|---|
| `id` | VARCHAR(36), PK | Row identifier (matches `client_keys.id` convention). |
| `client_id` | VARCHAR(64), FK → `client_apps.client_id`, `ON DELETE CASCADE` | Same client identity used for `accessToken` issuance (feature 001) and RSA key verification. |
| `secret_id` | VARCHAR(64) | Analogous to `client_keys.key_id` — allows multiple provisioned secrets per client (for rotation), only one active at a time. |
| `secret_value` | TEXT | The shared secret itself. Stored as-is (plaintext at rest), matching the existing precedent for `VENDOR_CLIENT_SECRET` and `client_keys.public_key_pem` — see plan.md Complexity Tracking. |
| `is_active` | BOOLEAN, default `TRUE` | Only one active secret should be relied upon at verification time; `GetActiveClientSecret` returns the active one. |
| `created_at` / `updated_at` | TIMESTAMPTZ | Standard audit columns. |

Unique constraint: `(client_id, secret_id)`. Index: `(client_id, is_active)` — mirrors `idx_client_keys_lookup`.

**Domain type** (`internal/domain/client.go`):

```go
type ClientSecret struct {
    ID          string    `json:"id"`
    ClientID    string    `json:"client_id"`
    SecretID    string    `json:"secret_id"`
    SecretValue string    `json:"-"` // never serialized in API responses
    IsActive    bool      `json:"is_active"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type AddClientSecretRequest struct {
    SecretID    string `json:"secretId"`
    SecretValue string `json:"secretValue"`
}
```

## Repository Interface Extension

`domain.ClientRepository` (existing interface, `internal/domain/client.go:36-42`) gains:

```go
GetActiveClientSecret(ctx context.Context, clientID string) (string, error)
CreateClientSecret(ctx context.Context, secret *ClientSecret) error
RevokeClientSecret(ctx context.Context, clientID, secretID string) error
```

## Validation Rules (new, added to the merchant request-admission path)

Extends `MerchantAuthMiddleware` (feature 009), which today only validates the bearer token. New steps run **after** a valid token is confirmed:

1. Extract `claims.ClientID` from the already-validated token.
2. `secret, err := clientRepo.GetActiveClientSecret(ctx, claims.ClientID)`. If `err != nil` or `secret == ""`: reject 401 (fail closed — FR-006), regardless of what `X-SIGNATURE` the request carries.
3. Validate `X-TIMESTAMP` freshness (±5 minutes) — reject 401 if outside window (FR-005), independent of signature.
4. Read and re-buffer the request body; compute `bodyHash := crypto.HashSHA256Hex(bodyBytes)`.
5. Build `stringToSign := crypto.BuildStringToSign(method, path, token /* the actual bearer token, not "" */, bodyHash, timestamp)`.
6. `crypto.NewHMACSigner(secret, "HMAC-SHA512").Verify(stringToSign, X-SIGNATURE header)` — reject 401 on mismatch (FR-003/FR-004).
7. Only if all of: token valid AND secret provisioned AND timestamp fresh AND signature valid → `next(c)`.

## State / Flow Summary

```
Request to /transfer-va/create-va|/list|DELETE /delete-va  (merchant-facing)
  MerchantAuthMiddleware(jwtIssuer, clientRepo):
    IF no "Authorization: Bearer <token>" → 401                          (feature 009, unchanged)
    claims, err := jwtIssuer.ValidateToken(token)
    IF err != nil → 401                                                  (feature 009, unchanged)
    secret, err := clientRepo.GetActiveClientSecret(ctx, claims.ClientID)
    IF err != nil OR secret == "" → 401                                  (NEW, fail closed)
    IF X-TIMESTAMP outside ±5min window → 401                            (NEW)
    recompute stringToSign (accessToken = actual token), HMACSigner.Verify(...) → 401 on mismatch  (NEW)
    → pass through to merchantVAHandler.CreateVA/ListVA/DeleteVA

Admin provisioning (new):
  POST /admin/clients/:clientId/secret     (ADMIN_API_KEY-protected, mirrors /keys)
    → CreateClientSecret (new row, is_active=true)
  DELETE /admin/clients/:clientId/secret/:secretId
    → RevokeClientSecret (is_active=false)
```
