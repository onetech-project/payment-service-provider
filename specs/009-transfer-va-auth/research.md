# Phase 0 Research: Enforce Signature & Token Verification on Transfer-VA Endpoints

No `NEEDS CLARIFICATION` markers remain in the Technical Context. The items below record the concrete decisions made while confirming the approach against the current codebase.

## Decision 1: Where HMAC verification is wired in

- **Decision**: Extend `SNAPAuthMiddleware` (`internal/adapter/delivery/http/middleware/snap_auth.go:13-82`) in place, rather than adding a separate middleware. After the existing header-presence and timestamp-format checks, add: (a) a timestamp-freshness check, and (b) an HMAC recomputation + `HMACSigner.Verify` call — both gated behind a new `vendorConfig.SignatureEnforcementEnabled` flag.
- **Rationale**: `SNAPAuthMiddleware` already receives the `*config.VendorConfig` for the exact vendor/channel a route was registered under (`main.go:391-392`: `vendorGroup.Use(customMiddleware.SNAPAuthMiddleware(vc))`), so the shared secret needed for verification is already in scope — no new lookup mechanism is needed. Splitting verification into a second middleware would require passing the same `vc` through again for no benefit.
- **Alternatives considered**: A `usecase`-layer check. Rejected — Clean Architecture (Principle I) places request authentication in the delivery/middleware layer, and the usecase layer has no HTTP concept of headers.

## Decision 2: Reconstructing `stringToSign` for verification

- **Decision**: The middleware recomputes `stringToSign = HTTPMethod:EndpointUrl:AccessToken:Lowercase(HexEncode(SHA-256(minify(body)))):Timestamp` — the exact format already used client-side (confirmed in `scripts/vendor-inquiry-va.sh:109`, `scripts/vendor-payment-va.sh:121`) — then calls `HMACSigner.Verify(secret, stringToSign, signatureHeader)` (`internal/infrastructure/crypto/hmac.go:54`). Because Echo middleware runs before the handler binds the body, the middleware reads and re-buffers the raw request body (`c.Request().Body`) so the handler can still read it afterward.
- **Rationale**: Matches exactly what the client already computes — no new signing convention is introduced. Re-buffering the body is the standard Echo pattern for middleware that needs to both hash and forward the request body.
- **Alternatives considered**: Bind the body to a struct first, then re-marshal it to recompute the hash. Rejected — re-marshaling is not guaranteed to byte-for-byte match the original client-sent JSON (key ordering, whitespace), which would break every request's signature regardless of validity. Raw-bytes hashing is required for correctness.

## Decision 3: Timestamp freshness tolerance

- **Decision**: Reuse the same ±5-minute window already implemented for the B2B token endpoint (`internal/usecase/token_usecase.go:39-47`: `time.Since(parsedTime) > 5*time.Minute || time.Until(parsedTime) > 5*time.Minute`), applied to `X-TIMESTAMP` on inquiry/payment/status requests once enforcement is enabled for that vendor/channel.
- **Rationale**: Spec Assumptions explicitly calls for reusing the existing tolerance rather than inventing a new value; keeps the two time-based checks in the codebase consistent.
- **Alternatives considered**: A per-vendor-configurable window. Rejected as unnecessary scope (YAGNI) — the spec only asks for the enable/disable toggle (FR-004), not a tunable tolerance; a fixed, proven value is simpler and matches the existing pattern.

## Decision 4: No enforcement toggle — verification is unconditional

- **Decision**: Signature verification (FR-001/FR-002) and timestamp-freshness checking (FR-003) are always active for every vendor/channel from the moment this feature ships — no new config field, no env var, no on/off setting anywhere. `config.VendorConfig` (`internal/infrastructure/config/vendor_config.go`) is left unchanged.
- **Rationale**: Explicit user correction during planning: a per-vendor/channel enable/disable toggle was originally proposed to de-risk rollout, but the user pointed out this defeats the purpose of the fix — enforcing signature validation correctly IS the goal, not an optional mode. Any vendor/channel whose client-side signing is currently incorrect (which is only possible because it was never checked) needs to be fixed before this deploys, not accommodated by a permanent bypass switch left in the codebase.
- **Alternatives considered**: The originally-planned `SignatureEnforcementEnabled` toggle (see prior revision of this document). Rejected per the above — a lingering "verification off" switch is itself a security liability (it can be flipped off and forgotten, or misconfigured to "off" by default in a new environment) and contradicts the spec's own goal.

### Amendment (2026-07-31): scoped, non-prod-only exception added for the timestamp-freshness sub-check

- **What did NOT change**: HMAC signature verification (FR-001/FR-002) remains always-on and
  unconditional in every environment. There is still no per-vendor/channel config toggle —
  `config.VendorConfig` is unchanged, and this is deliberately NOT the previously-rejected
  `SignatureEnforcementEnabled` toggle from the "Alternatives considered" above (that was
  per-vendor/channel, persisted in config, and could silently vary by environment or onboarding
  mistake).
- **What changed**: a single process-wide `skipSkewCheck bool`, computed once in `main()` as
  `appEnv == "dev" || appEnv == "uat"` (`cmd/api/main.go`, `APP_ENV` env var — already used
  elsewhere in `main()` to decide extra SNAP base-path mirrors) and threaded into
  `SNAPAuthMiddleware`, `MerchantAuthMiddleware`, and `TokenUsecase.GenerateB2BToken`, narrows only
  the ±5-minute `X-TIMESTAMP` freshness sub-check (Decision 3) specifically — never the HMAC/JWT
  verification itself. It is global (not per-vendor), derived from server-wide `APP_ENV` (not a
  persisted config field anyone could accidentally leave "off" in prod), and off by default in
  `prod`.
- **Rationale**: replaying a fixed/stale ASPI sample timestamp (e.g. from vendor documentation)
  during local dev/UAT testing was otherwise rejected by the same check meant to catch genuine
  replay/clock-skew issues in production traffic — the check's value is specifically in catching
  stale *production* requests, not in blocking developers from testing with canned timestamps.
- **Affected files**: `internal/usecase/token_usecase.go`, `internal/adapter/delivery/http/middleware/snap_auth.go`, `internal/adapter/delivery/http/middleware/merchant_auth.go`, `cmd/api/main.go`.

## Decision 5: Fail-closed on missing secret

- **Decision**: If `ClientSecret` is empty for a vendor/channel, the middleware rejects all requests for that vendor/channel (401), rather than silently skipping verification.
- **Rationale**: Directly required by FR-004 and the corresponding Edge Case; mirrors how the B2B token flow already treats missing credentials as a hard failure, not a soft pass-through. Since there is no longer an enforcement toggle (Decision 4), this is now the primary safety net against a misconfigured/incomplete vendor onboarding.
- **Alternatives considered**: Log a warning and skip verification. Rejected — would silently reintroduce today's exact vulnerability the moment a config typo or missing secret occurs, defeating the purpose of the feature.

## Decision 6: Merchant bearer-token middleware

- **Decision**: New middleware `internal/adapter/delivery/http/middleware/merchant_auth.go`, applied only to the merchant route group in `main.go` (the `transferVAGroup.POST("/create-va", ...)` / `/list` / `DELETE /delete-va` registrations at `main.go:408-410`, wrapped in a new `Group` with `.Use(merchantAuthMiddleware)` rather than attached to `transferVAGroup` itself, keeping the vendor sub-groups registered on the same `transferVAGroup` unaffected). It extracts `Authorization: Bearer <token>`, calls the existing `JWTIssuer.ValidateToken` (`jwt_issuer.go:85-104`), and rejects with 401 on a missing header, malformed bearer scheme, or a token that fails `ValidateToken` (which already covers both invalid-signature and expired-token cases via `jwt.Parse`'s built-in `exp` validation — confirmed in the earlier investigation).
- **Rationale**: Directly implements the user's confirmed decision to reuse the existing SNAP B2B JWT mechanism rather than introduce a new merchant secret scheme (spec Input, Decision already made). `ValidateToken` already does everything FR-007/FR-008 require — no new crypto code needed.
- **Alternatives considered**: Extending `SNAPAuthMiddleware` to also handle bearer tokens via a mode flag. Rejected — the two mechanisms check fundamentally different things (shared-secret HMAC vs. RSA-signed JWT) and conflating them into one middleware with a mode switch would violate Interface Segregation (Principle I) and make US5's isolation guarantee harder to verify by inspection.

## Decision 7: Route wiring keeps the two mechanisms isolated (US5)

- **Decision**: The merchant bearer-token middleware is `.Use()`'d only on a new sub-group wrapping `create-va`/`list`/`delete-va`; `SNAPAuthMiddleware` continues to be `.Use()`'d only on the per-vendor `vendorGroup` sub-groups wrapping `inquiry`/`payment`/`status`. Both sub-groups hang off the same parent `transferVAGroup` (which only carries `IdempotencyMiddleware`), so neither middleware is applied to the other group's routes.
- **Rationale**: This is exactly the existing structural pattern (`main.go:386-410`) — vendor routes already live in their own `vendorGroup.Group("")`. Adding one more sibling sub-group for merchant routes requires no restructuring, just wrapping three existing route registrations in `.Group("").Use(...)`.
- **Alternatives considered**: Applying both middlewares to `transferVAGroup` itself with internal short-circuit logic per-path. Rejected — more complex, harder to test in isolation, and against the existing convention already established for the vendor side.
