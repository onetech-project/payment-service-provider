# Research: Vendor Access Token in Symmetric Signature

## Decision 1: How does a vendor "have" an access token to put in the signature?

**Decision**: Add a `ClientID` field to `config.VendorConfig`. Each configured vendor's `client_id` must correspond to an existing `client_apps` row (with an RSA key registered in `client_keys`, provisioned the same way merchants are onboarded per feature 010). The vendor obtains a JWT via the existing `POST {snapBasePath}/access-token/b2b` endpoint using that `client_id`, exactly as merchants already do.

**Rationale**: The spec (Assumption) requires reusing the existing access-token issuance mechanism rather than inventing a new one. Investigation of `internal/domain/client.go` and `cmd/api/main.go` confirmed `/access-token/b2b` is already a single shared endpoint, keyed only by `client_id`/RSA key — nothing merchant-specific about it. The only gap is that `config.VendorConfig` (used by `SNAPAuthMiddleware`) has no `client_id` field today, so there's no way to tie a vendor's static config entry to the `client_id` embedded in its JWT claims. Adding `ClientID` to `VendorConfig` closes that gap with a one-field, backward-compatible config addition.

**Alternatives considered**:
- *New vendor-specific token-issuance endpoint*: rejected — spec explicitly assumes reuse of the existing mechanism (YAGNI, avoids a second, parallel auth surface).
- *Skip ClientID-claim matching, just validate the JWT is well-formed and issued by this system*: rejected — without confirming the token's `ClientID` claim matches the vendor config entry that's about to verify the signature, any vendor's valid token could be replayed against any other vendor's `ClientSecret`-signed request (violates spec FR-004/FR-005's intent of binding token to signer identity).

## Decision 2: Where in the request pipeline does Authorization/JWT validation happen relative to existing SNAP checks?

**Decision**: Insert `Authorization` header parsing and `jwtIssuer.ValidateToken` immediately after the existing header-presence and timestamp-skew checks in `SNAPAuthMiddleware`, before body-hash/signature computation — mirroring `MerchantAuthMiddleware`'s existing check ordering (token first, then signature, both must pass). The validated token's raw string becomes the `AccessToken` component passed into `crypto.BuildStringToSign`.

**Rationale**: `MerchantAuthMiddleware` (`merchant_auth.go:26-94`) already establishes this exact ordering and is the direct precedent this feature is asked to mirror. Keeping the same order (cheap header/timestamp checks fail fast, then token validation, then expensive body-hash+HMAC verification) avoids wasted work on malformed requests and keeps the two middlewares structurally consistent for future maintainers.

**Alternatives considered**:
- *Validate signature first, then token*: rejected — would require computing body hash before knowing whether to bother with a token at all; also diverges from the reviewed merchant precedent for no benefit.

## Decision 3: Error responses for the three rejection classes (missing header, invalid token, ClientID mismatch)

**Decision**: Missing/malformed `Authorization` header → distinguishable error (e.g. `401` with a message identifying the missing/invalid `Authorization` header, matching spec FR-007 / User Story 3). Invalid/expired token → same class of error as `MerchantAuthMiddleware` produces for invalid tokens today (`401`, generic "unauthorized" body — no additional information disclosure about *why* the token is invalid, consistent with the merchant precedent's error handling). Valid token but signature mismatch (including `ClientID`-claim mismatch) → the existing SNAP `401 invalid signature` response, unchanged.

**Rationale**: Spec FR-007 only requires the *missing-Authorization* case be distinguishable from generic signature mismatch (this is the case a legacy vendor integration will hit during migration); it does not require distinguishing every other failure mode, and `MerchantAuthMiddleware` already establishes the precedent of not leaking granular token-validation detail. Keeping the other error paths aligned with existing SNAP/merchant response shapes avoids introducing a new error-response convention.

**Alternatives considered**:
- *Distinct error code/body for every failure mode (missing header vs. invalid token vs. expired vs. ClientID mismatch)*: rejected as over-engineering relative to spec scope (SC-004 only calls out the missing-header case) and inconsistent with the merchant precedent's coarser error granularity.

## Decision 4: Rollout mechanism (feature flag vs. unconditional)

**Decision**: Unconditional — once a vendor's `VendorConfig` has a `ClientID` configured, `SNAPAuthMiddleware` requires `Authorization` for that vendor's requests. No global enable/disable flag.

**Rationale**: Matches the precedent set by feature 009 (SNAP HMAC itself) and feature 010 (merchant HMAC), both of which shipped unconditionally per their plans ("No enable/disable configuration — unconditional from deploy"). The spec frames this as a breaking change requiring vendor migration, not a togglable feature.

**Alternatives considered**:
- *Per-vendor opt-in flag in `VendorConfig`*: reconsidered but rejected as unnecessary complexity — the presence/absence of `ClientID` in a vendor's config already acts as the natural rollout gate (a vendor without `ClientID` configured cannot be issued a token and so cannot be cut over yet; once `ClientID` is set for a vendor, the middleware enforces the new requirement for that vendor specifically). This gives per-vendor rollout control without a separate boolean.
