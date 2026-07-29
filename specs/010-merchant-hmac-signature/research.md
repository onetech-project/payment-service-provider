# Phase 0 Research: Merchant HMAC Signature Verification

No `NEEDS CLARIFICATION` markers remain. The items below record the concrete decisions made while confirming the approach against the current codebase.

## Decision 1: Storage schema for merchant shared secrets

- **Decision**: New table `client_secrets`, structurally identical to the existing `client_keys` table (`db/migrations/000002_create_client_keys.up.sql`): `client_id` (FK to `client_apps.client_id`, cascade delete), a `secret_id` analogous to `key_id` (supports multiple provisioned secrets per client, only one `is_active`), the secret value itself, `is_active`, timestamps, unique constraint on `(client_id, secret_id)`.
- **Rationale**: `client_keys` is the proven, already-shipped pattern for "a client_app has zero-or-more associated credential records, exactly one (or zero) active at a time, supporting rotation via add-new/revoke-old." Reusing it exactly minimizes new design surface and review risk.
- **Alternatives considered**: A single `client_secret` column directly on `client_apps`. Rejected — would make rotation (issue a new secret, revoke the old one atomically) awkward compared to the add/revoke-key pattern already proven, and breaks symmetry with how `client_keys` already solves the identical shape of problem for RSA keys.

## Decision 2: Repository interface shape

- **Decision**: Extend `domain.ClientRepository` (`internal/domain/client.go:36-42`) with `GetActiveClientSecret(ctx, clientID) (string, error)`, `CreateClientSecret(ctx, secret *ClientSecret) error`, `RevokeClientSecret(ctx, clientID, secretID string) error` — mirroring `GetActiveClientPublicKey`/`CreateClientKey`/`RevokeClientKey` exactly (same method shapes, same interface).
- **Rationale**: `MerchantAuthMiddleware` needs a single, simple lookup (`GetActiveClientSecret`) analogous to how the RSA verification flow already looks up `GetActiveClientPublicKey`. Consistency with the existing interface style keeps the codebase's mental model uniform (Principle I: consumer-defined, minimal interfaces).
- **Alternatives considered**: A separate `SecretRepository` interface. Rejected — `client_keys` and `client_secrets` are both sub-resources of the same `client_apps` aggregate; splitting them into different repository interfaces would fragment a single client-management concern without a clear benefit, and `ClientRepository` is already the established home for this kind of per-client credential.

## Decision 3: Admin provisioning endpoints

- **Decision**: `POST /admin/clients/:clientId/secret` (create/rotate — creates a new active secret) and `DELETE /admin/clients/:clientId/secret/:secretId` (revoke), added to `client_handler.go` and wired in `main.go`'s existing `adminGroup` (already `ADMIN_API_KEY`-protected, per `main.go:368`'s sibling `/clients/:clientId/keys` route).
- **Rationale**: Directly mirrors `AddClientKey`/`RevokeClientKey` (`client_handler.go:93-148`), which already exists, is already tested, and is already protected by the same admin auth the constitution expects for this class of operation. No new auth mechanism, no new route group.
- **Alternatives considered**: A one-off CLI/seed-script-only provisioning path (as was used ad hoc during feature 009's manual testing). Rejected as the *sole* mechanism — spec FR-002 requires "a way for an operator to provision," and an HTTP admin endpoint is consistent with how `client_keys` (the direct analog) is already provisioned in this codebase; a script-only path would be an inconsistent, harder-to-audit exception. (A seed/CLI path may still be useful for local dev — that's an implementation detail, not a spec-level alternative.)

## Decision 4: Where signature verification is wired into `merchant_auth.go`

- **Decision**: Extend `MerchantAuthMiddleware` (`internal/adapter/delivery/http/middleware/merchant_auth.go`, feature 009) in place: after the existing `jwtIssuer.ValidateToken(token)` call succeeds, look up `GetActiveClientSecret(ctx, claims.ClientID)` and verify the signature before calling `next(c)`. The constructor gains a `clientRepo domain.ClientRepository` parameter alongside the existing `jwtIssuer domain.JWTIssuer`.
- **Rationale**: Keeps both merchant-side checks in one middleware (matches how `SNAPAuthMiddleware` already keeps header/timestamp/signature checks together for the vendor side) and naturally sequences "who are you" (token) before "prove you know the secret" (signature) — if the token itself is invalid, there is no `ClientID` to look up a secret for.
- **Alternatives considered**: A second, separate middleware chained after `MerchantAuthMiddleware`. Rejected — would require passing `TokenClaims` between two middlewares (via Echo context values) for no structural benefit, and diverges from the single-middleware-per-concern-group pattern already established for the vendor side.

## Decision 5: `stringToSign` construction — the merchant/vendor AccessToken difference

- **Decision**: Reuse `crypto.BuildStringToSign(method, path, accessToken, bodyHash, timestamp)` exactly as `SNAPAuthMiddleware` already does, but pass the **actual bearer token** (stripped of the `"Bearer "` prefix, the same string already extracted for `jwtIssuer.ValidateToken`) as the `accessToken` argument — versus `SNAPAuthMiddleware`'s hardcoded `""` for vendor requests (feature 009 Decision 2, since no header ever carries it there).
- **Rationale**: This is the one substantive difference from feature 009's vendor implementation, and it's mechanical: merchant endpoints already have the real token in hand (it's how the bearer-token check itself works), so there's no ambiguity about what value to sign with — unlike the vendor side, where no such value is ever transmitted.
- **Alternatives considered**: None — this follows directly from spec FR-003 and the Assumptions section, which already settled this question during specification.

## Decision 6: Body re-buffering

- **Decision**: Same pattern as `SNAPAuthMiddleware` (feature 009 Decision 2) and `IdempotencyMiddleware`: read `c.Request().Body` via `io.ReadAll`, hash it, then restore it via `io.NopCloser(bytes.NewBuffer(bodyBytes))` before calling `next(c)`, so `merchantVAHandler`'s existing `c.Bind(&req)` calls are unaffected.
- **Rationale**: Proven, already-shipped pattern in this exact codebase for exactly this problem (a middleware needing to both hash and forward a request body). No reason to deviate.
- **Alternatives considered**: None — this is a direct reuse of an established pattern, not a new design question.
