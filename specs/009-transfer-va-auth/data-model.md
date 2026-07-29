# Phase 1 Data Model: Enforce Signature & Token Verification on Transfer-VA Endpoints

No new persistent entities, migrations, or config fields. This feature formalizes two already-existing runtime concepts (vendor shared secret, merchant JWT) as unconditionally-enforced preconditions.

## Entity: Vendor/Channel Signature Configuration

Existing entity — `config.VendorConfig` (`internal/infrastructure/config/vendor_config.go:10-47`). No new fields; one existing field takes on a stricter meaning:

| Field | Type | Notes |
|---|---|---|
| `ClientSecret` | string | Existing (`VENDOR_CLIENT_SECRET`). Now load-bearing for every request: MUST be non-empty or all requests for that vendor/channel are rejected (fail-closed, Decision 5). Verification is unconditional — there is no setting to disable it (Decision 4). |

No config or schema changes: this feature only reads the config value that already exists.

## Entity: Merchant Access Token (existing, now enforced)

No new entity — the existing JWT issued by `JWTIssuer.GenerateB2BToken` (`internal/infrastructure/crypto/jwt_issuer.go:63-81`) and validated by `ValidateToken` (`jwt_issuer.go:85-104`). Claims already present and now relied upon as a precondition:

| Claim | Type | Notes |
|---|---|---|
| `client_id` | string | Identifies the calling merchant/client. Not newly used by this feature — validation of `client_id` against a known client already happens at issuance time; this feature only adds "is this token present and valid" as a gate on merchant routes, it does not add new per-client authorization logic. |
| `exp` | unix timestamp | Already enforced by `jwt.Parse`'s built-in validation (confirmed: `token.Valid == false` on expiry). This feature relies on that existing behavior rather than re-implementing expiry checking. |

## Validation Rules (new, added to the request-admission path)

### Vendor-facing (inquiry/payment/status)

Unconditional from deployment — no toggle:

1. If `vendorConfig.ClientSecret == ""`: reject all requests (401) — fail closed.
2. Otherwise, recompute `stringToSign` from the raw request body + timestamp per the existing client-side convention, and reject (401) if `HMACSigner.Verify(secret, stringToSign, X-SIGNATURE header) == false`.
3. Reject (401) if `X-TIMESTAMP` is older than 5 minutes or more than 5 minutes in the future relative to server time.

### Merchant-facing (create-VA/list-VA/delete-VA)

1. Reject (401) if the `Authorization` header is absent or does not start with `Bearer `.
2. Reject (401) if `JWTIssuer.ValidateToken(token)` returns an error (covers malformed token, invalid signature, and expired token — all via the existing `jwt.Parse` behavior).
3. Otherwise, allow the request through to the existing handler/usecase logic, unchanged.

## State / Flow Summary

```
Request to /transfer-va/inquiry|/payment|/status  (vendor-facing)
  SNAPAuthMiddleware(vc):
    header presence + timestamp format checks (unchanged, existing)
    IF vc.ClientSecret == "" → 401 (fail closed)
    IF X-TIMESTAMP outside ±5min window → 401
    recompute stringToSign, HMACSigner.Verify(...) → 401 on mismatch
    → pass through to vaHandler.Inquiry/Payment/Status

Request to /transfer-va/create-va|/list|DELETE /delete-va  (merchant-facing)
  MerchantAuthMiddleware:
    IF no "Authorization: Bearer <token>" → 401
    IF JWTIssuer.ValidateToken(token) fails (invalid sig OR expired) → 401
    → pass through to merchantVAHandler.CreateVA/ListVA/DeleteVA
```
