# Contract: SNAPAuthMiddleware (extended)

Applies to vendor endpoints currently gated by `SNAPAuthMiddleware`:
`POST {snapBasePath}/transfer-va/inquiry`, `POST {snapBasePath}/transfer-va/payment`, `POST {snapBasePath}/transfer-va/status`
(`snapBasePath` ∈ `/openapi/v1.0` always, plus `/api/v1.0`, `/v1.0` in dev/uat).

## Request headers

| Header | Required | Notes |
|---|---|---|
| `X-TIMESTAMP` | Yes (unchanged) | ISO8601, ±5min skew enforced. |
| `X-SIGNATURE` | Yes (unchanged) | HMAC over `stringToSign`. |
| `CHANNEL-ID` | Conditional (unchanged) | Required if `VendorConfig.ChannelID` set. |
| `X-PARTNER-ID` | Conditional (unchanged) | Required if `VendorConfig.PartnerID` set. |
| `X-EXTERNAL-ID` | Conditional, non-GET (unchanged) | |
| `Authorization` | **New — conditional** | Required as `Bearer <accessToken>` if the resolved `VendorConfig.ClientID` is non-empty (i.e., vendor has been migrated). Not required for vendors without `ClientID` configured (legacy behavior preserved). |

## String-to-sign composition

- **Legacy vendor (no `ClientID` configured)**: `HTTPMethod:RelativeURL::SHA256Hex(Body):Timestamp` — unchanged, AccessToken component empty.
- **Migrated vendor (`ClientID` configured)**: `HTTPMethod:RelativeURL:{AccessToken}:SHA256Hex(Body):Timestamp` — `{AccessToken}` is the exact bearer token string from `Authorization`.

## Validation order (migrated vendors)

1. Existing header presence + `X-TIMESTAMP` skew checks (unchanged).
2. Parse `Authorization: Bearer <token>`. Missing or malformed → `401`, error identifies missing/invalid `Authorization` header (distinguishable from generic signature-mismatch error, per spec FR-007).
3. `jwtIssuer.ValidateToken(token)` → `*domain.TokenClaims`. Invalid/expired → `401`, generic unauthorized (same shape as `MerchantAuthMiddleware`'s existing invalid-token response).
4. Confirm `claims.ClientID == VendorConfig.ClientID` for the resolved vendor. Mismatch → `401 invalid signature` (same response as a normal signature mismatch — no extra information disclosed about *why*).
5. Compute `stringToSign` using the raw bearer token as the AccessToken component; verify HMAC against `VendorConfig.ClientSecret` (unchanged verification mechanism). Mismatch → existing `401 invalid signature` response, unchanged shape.

## Response contract

No change to success response bodies/codes for any vendor endpoint. All new rejection paths reuse the existing `401` error response envelope already used by `SNAPAuthMiddleware` and `MerchantAuthMiddleware`, varying only the error message text per step 2 above.

## Backward compatibility

A vendor request built with the legacy signing convention (no `Authorization`, empty AccessToken component in `stringToSign`) against a **migrated** vendor config (`ClientID` set) is rejected at step 2 with the missing-`Authorization` error — this is the intended breaking-change signal described in spec User Story 3. Against a **non-migrated** vendor config, it continues to succeed unchanged.
