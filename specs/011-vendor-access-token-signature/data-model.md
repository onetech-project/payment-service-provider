# Data Model: Vendor Access Token in Symmetric Signature

No new database tables or migrations. This feature extends two existing entities.

## VendorConfig (reused as-is)

Location: `internal/infrastructure/config/vendor_config.go`

`VendorConfig.ClientID` **already exists** (loaded from `VENDOR_CLIENT_ID` env var, `vendor_config.go:126,178`) but is currently unused by `SNAPAuthMiddleware` — no code/schema change needed to add it. This feature is the first consumer of that field for signature purposes.

| Field | Type | Notes |
|---|---|---|
| `ClientID` | `string` | **Newly consumed.** Links this vendor's static config entry to a `client_apps` row (the same `client_id` used for JWT issuance via `/access-token/b2b`). Empty by default in existing deployments (vendor not yet migrated — see Rollout below). |
| ...existing fields (`PartnerID`, `ChannelID`, `ClientSecret`, `SignatureAlgorithm`, `RequiredHeaders`, etc.) | — | Unchanged. |

**Validation rule (new consumption)**: `ClientID` MAY be empty (vendor not yet migrated — see Rollout below). If non-empty, it MUST correspond to an active `client_apps` row with a registered `client_keys` entry (enforced at token-issuance time by the existing `/access-token/b2b` flow, not by `VendorConfig` itself).

**Rollout semantics**: `SNAPAuthMiddleware` enforces the new `Authorization`/access-token requirement **only for vendor configs where `ClientID` is set**. A vendor config with an empty `ClientID` continues to use today's behavior (empty-string access-token component, no `Authorization` required). This gives per-vendor migration control without a separate feature flag (see [research.md](./research.md) Decision 4).

## TokenClaims (existing, reused as-is)

Location: `internal/domain/token.go`

| Field | Type | Notes |
|---|---|---|
| `ClientID` | `string` | Used by the new vendor check to confirm the token presented in `Authorization` was issued for the same `client_id` as the resolved `VendorConfig.ClientID` — prevents a valid token from one vendor being replayed against another vendor's signature verification. |
| `JTI`, `IssuedAt`, `Expires` | — | Unchanged; existing expiry/validity checks in `jwtIssuer.ValidateToken` apply unmodified. |

No new fields added to `TokenClaims`.

## Request/Response shapes

No changes to any vendor endpoint's request or response body. The only wire-format additions are:
- New required request header: `Authorization: Bearer <accessToken>` (for migrated vendors, i.e. those with `ClientID` configured).
- `stringToSign` composition changes from `HTTPMethod:RelativeURL::SHA256(Body):Timestamp` (empty AccessToken component) to `HTTPMethod:RelativeURL:AccessToken:SHA256(Body):Timestamp` (real token) — for migrated vendors only.

## State transitions

None — this is a stateless per-request validation change. No new entity lifecycle is introduced.
