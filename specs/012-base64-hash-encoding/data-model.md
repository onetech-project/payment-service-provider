# Data Model: Base64 Hash/Signature Encoding Standardization

No new entities, tables, or migrations. This feature changes the **encoding** of existing derived values; the values' meaning and lifecycle are unchanged.

## Signed Request String (stringToSign)

Location: built by `crypto.BuildStringToSign` (format string itself unchanged: `HTTPMethod:RelativeURL:AccessToken:RequestBodyHash:Timestamp`), consumed by `snap_auth.go`, `merchant_auth.go`, `signature_usecase.go`, `snap/client.go`.

| Component | Before | After |
|---|---|---|
| `RequestBodyHash` | `hex.EncodeToString(sha256.Sum(...))` — lowercase hex, 64 chars | `base64.StdEncoding.EncodeToString(sha256.Sum(...))` — standard base64, 44 chars (with `=` padding) |
| `X-SIGNATURE` (HMAC-SHA512 over the above string) | `hex.EncodeToString(hmac.Sum(...))` — lowercase hex, 128 chars | `base64.StdEncoding.EncodeToString(hmac.Sum(...))` — standard base64, 88 chars (with `=` padding) |

**Validation rule (unchanged)**: fails closed exactly as today — missing/empty secret, or `Verify` mismatch, both still reject with the existing `401 invalid signature` response; only the value being compared changes shape.

## Webhook/Callback Signature

Location: `payment_notification_worker.go` — `X-Signature` header on outbound merchant payment-notification callbacks.

| Field | Before | After |
|---|---|---|
| `X-Signature` | `hex.EncodeToString(hmac.Sum(...))` (HMAC-SHA512 over JSON body) | `base64.StdEncoding.EncodeToString(hmac.Sum(...))` |

No change to `X-Timestamp` or any other callback header/body field.

## Idempotency Payload Hash

Location: `idempotency.go` — `CachedResponse.PayloadHash`, stored in the Redis-cached JSON envelope keyed by `X-EXTERNAL-ID`.

| Field | Before | After |
|---|---|---|
| `PayloadHash` | `hex.EncodeToString(sha256.Sum256(bodyBytes))` | `base64.StdEncoding.EncodeToString(sha256.Sum256(bodyBytes))` |

**Lifecycle note**: this value only ever needs to match another value computed by this same process within one idempotency cache entry's TTL (short-lived, per spec Edge Cases) — no stored value outlives a single deploy of this change in a way that requires migration.

## Unaffected: RSA-Signed B2B Token-Request Signature

`rsa_signer.go`/`rsa_verifier.go` (`X-SIGNATURE` on `POST /access-token/b2b`) already produces/consumes `base64.StdEncoding` — explicitly out of scope (spec Assumptions), no change.
