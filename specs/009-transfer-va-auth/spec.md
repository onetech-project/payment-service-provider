# Feature Specification: Enforce Signature & Token Verification on Transfer-VA Endpoints

**Feature Branch**: `009-transfer-va-auth`

**Created**: 2026-07-29

**Status**: Draft

**Input**: User description: "Perbaikan implementasi keamanan verifikasi signature/autentikasi pada endpoint transfer-va, mencakup dua sisi sekaligus (dikerjakan berbarengan): (A) sisi vendor/bank — SNAPAuthMiddleware hanya mengecek header ada isinya dan format timestamp, tidak pernah benar-benar menghitung ulang dan membandingkan HMAC signature terhadap VENDOR_CLIENT_SECRET; tidak ada pengecekan freshness/window pada X-TIMESTAMP. Signature verification MUST simply be enforced directly — no per-vendor/channel enable/disable toggle, since enforcing it correctly IS the point of this fix. (B) sisi merchant — endpoint create-va/list/delete-va tidak dilindungi middleware apapun; harus mewajibkan header Authorization Bearer berisi accessToken JWT valid yang diterbitkan lewat access-token/b2b, reuse mekanisme SNAP B2B yang sudah ada, bukan skema HMAC baru. (C) kedua perbaikan dikerjakan dan dirilis berbarengan dalam satu feature karena menutup celah keamanan serupa pada modul yang sama."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Vendor/bank requests are rejected when their signature doesn't match (Priority: P1)

A vendor (bank) calls this service's inquiry, payment, or status endpoints as part of the SNAP virtual-account flow. Today, any request with a non-empty `X-SIGNATURE` header and a plausibly-formatted `X-TIMESTAMP` is accepted regardless of whether the signature is actually valid — meaning a caller who doesn't know the shared `clientSecret` can still successfully call these endpoints. This story closes that gap: only requests whose signature can be verified against the shared secret for that vendor/channel are accepted.

**Why this priority**: This is the highest-value fix — it protects the transactional endpoints (inquiry/payment/status) that move money-adjacent state, and the verification logic itself already mostly exists (only the wiring is missing), so it delivers the most security value for the least new code.

**Independent Test**: Can be fully tested by sending a request with a correctly-computed HMAC signature (using the vendor's real shared secret) and confirming it succeeds exactly as before, then sending the same request with an incorrect or missing signature and confirming it is rejected before any business logic runs.

**Acceptance Scenarios**:

1. **Given** a vendor has the correct shared secret for its channel, **When** it sends an inquiry/payment/status request with a correctly-computed signature, **Then** the request is processed normally (unchanged behavior from today for the happy path).
2. **Given** a request is sent to an inquiry/payment/status endpoint with a signature that does not match what the shared secret would produce, **When** the request is received, **Then** it is rejected as unauthorized and no business logic (VA lookup, payment recording, etc.) executes.
3. **Given** a request is sent with the `X-SIGNATURE` header present but empty or malformed, **When** the request is received, **Then** it is rejected as unauthorized (same outcome as scenario 2, not treated as a separate/softer case).

---

### User Story 2 - Vendor/bank requests are rejected when their timestamp is stale or too far in the future (Priority: P2)

Building on Story 1, a request's `X-TIMESTAMP` header is checked for freshness, not just format. A request replayed long after it was originally signed, or one bearing a timestamp far in the future, is rejected — closing a replay-attack window that exists today because only the timestamp's string shape is checked.

**Why this priority**: Meaningfully raises the bar against replay attacks, but is secondary to Story 1 (a valid-but-stale signed request is a narrower risk than an entirely unverified one) and reuses an existing, already-proven tolerance pattern from the B2B token endpoint.

**Independent Test**: Can be fully tested by sending a request with a correctly-computed signature but a timestamp far outside the allowed window (e.g. an hour old, or an hour in the future) and confirming it is rejected, while a request with a timestamp within the window and a correct signature still succeeds.

**Acceptance Scenarios**:

1. **Given** a correctly-signed request with a timestamp within the allowed freshness window, **When** it is received, **Then** it is processed normally.
2. **Given** a correctly-signed request with a timestamp older than the allowed window, **When** it is received, **Then** it is rejected as unauthorized.
3. **Given** a correctly-signed request with a timestamp further in the future than the allowed window, **When** it is received, **Then** it is rejected as unauthorized.

---

### User Story 3 - Merchant requests require a valid access token (Priority: P1)

A merchant calls create-VA, list-VA, or delete-VA. Today these endpoints run with no authentication check at all — any caller who can reach the network can create, list, or delete virtual accounts for any partner. This story requires a valid `accessToken` (obtained via the existing B2B token endpoint) on every merchant request, closing that gap using the authentication mechanism the system already has.

**Why this priority**: Equal top priority to Story 1 — this endpoint group is currently completely unauthenticated, which is at least as severe a gap as the vendor side's weak-but-present checks, and it protects state-mutating operations (create/delete) directly.

**Independent Test**: Can be fully tested by calling create-VA/list-VA/delete-VA with a valid, unexpired `accessToken` obtained from the token endpoint and confirming success identical to today's behavior, then calling the same endpoints with no token, a malformed token, or an expired token and confirming each is rejected before business logic runs.

**Acceptance Scenarios**:

1. **Given** a merchant has obtained a valid, unexpired `accessToken` from the B2B token endpoint, **When** it calls create-VA, list-VA, or delete-VA with that token, **Then** the request is processed normally (unchanged from today's behavior for the happy path).
2. **Given** a request to create-VA, list-VA, or delete-VA has no `Authorization` header at all, **When** it is received, **Then** it is rejected as unauthorized before any business logic executes.
3. **Given** a request carries an `Authorization` header with a malformed or otherwise invalid token, **When** it is received, **Then** it is rejected as unauthorized.
4. **Given** a request carries a syntactically valid token that has expired, **When** it is received, **Then** it is rejected as unauthorized.

---

### User Story 4 - Vendor and merchant authentication mechanisms coexist without interference (Priority: P2)

Since the two endpoint groups (vendor-facing inquiry/payment/status, and merchant-facing create-VA/list-VA/delete-VA) use different authentication mechanisms (shared-secret signature vs. bearer token), applying the fix to one group must not affect or leak into the other.

**Why this priority**: A correctness/regression-safety story rather than new capability — confirms Stories 1-3 don't interfere with each other. Lower priority because it's validation of the other stories' isolation, not standalone value.

**Independent Test**: Can be fully tested by confirming a valid vendor-signed request to inquiry/payment/status succeeds without needing an `Authorization` bearer token, and a valid merchant bearer-token request to create-VA/list-VA/delete-VA succeeds without needing an `X-SIGNATURE` header — i.e. neither mechanism is accidentally required on the other group's endpoints.

**Acceptance Scenarios**:

1. **Given** a correctly-signed vendor request to an inquiry/payment/status endpoint with no `Authorization` header at all, **When** it is received, **Then** it is processed normally (bearer token is not required on vendor-facing endpoints).
2. **Given** a merchant request to create-VA/list-VA/delete-VA with a valid bearer token but no `X-SIGNATURE` header, **When** it is received, **Then** it is processed normally (signature is not required on merchant-facing endpoints).

### Edge Cases

- What happens when a vendor/channel's shared secret is missing or empty in configuration? The system must fail closed (reject all requests for that vendor/channel) rather than silently skipping verification.
- What happens when the merchant `accessToken` was issued for a different, now-deactivated or unknown client? Rejected as unauthorized, consistent with how the token issuance/validation already treats unknown clients today.
- What happens to in-flight requests or already-issued access tokens at the moment this feature is deployed? Any request arriving after deployment is evaluated under the new rule; this is a one-time cutover for both the vendor side and the merchant side — there is no opt-out or gradual rollout, since enforcing these checks correctly is the entire point of the fix.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST recompute the expected signature for inquiry/payment/status requests using the shared secret configured for the request's vendor/channel, and compare it against the `X-SIGNATURE` header.
- **FR-002**: System MUST reject an inquiry/payment/status request whose signature does not match the recomputed value, before any business logic (VA lookup, payment recording, status change) executes.
- **FR-003**: System MUST reject an inquiry/payment/status request whose `X-TIMESTAMP` falls outside an allowed freshness window (too old or too far in the future), independent of whether its signature is valid.
- **FR-004**: System MUST fail closed — reject all requests — for a vendor/channel that is missing the shared secret needed to verify signatures.
- **FR-005**: System MUST require a valid, unexpired `accessToken` (bearer token issued by the existing B2B token endpoint) on every create-VA, list-VA, and delete-VA request.
- **FR-006**: System MUST reject a create-VA, list-VA, or delete-VA request that has no `Authorization` header, an invalid/malformed token, or an expired token, before any business logic executes.
- **FR-007**: System MUST NOT require a bearer `accessToken` on vendor-facing inquiry/payment/status endpoints, and MUST NOT require an `X-SIGNATURE` on merchant-facing create-VA/list-VA/delete-VA endpoints — the two mechanisms apply exclusively to their respective endpoint groups.
- **FR-008**: Both the vendor-side enforcement (FR-001–FR-004) and the merchant-side enforcement (FR-005–FR-007) MUST ship together in the same release, since they close related gaps in the same endpoint family, and both take effect immediately and unconditionally upon deployment — there is no configuration to enable/disable either check. **Amended**: the timestamp-freshness sub-check specifically (not signature verification) later gained a global, non-per-vendor `APP_ENV=dev`/`uat` exception — see `research.md` Decision 4 Amendment.

### Key Entities

- **Vendor/channel signature configuration**: Represents the per-vendor/channel settings governing signature verification — specifically, the shared secret used to verify `X-SIGNATURE`. Verification itself is always active; there is no on/off setting.
- **Merchant access token**: The existing bearer token issued by the B2B token endpoint, now additionally required as a precondition for merchant-facing VA management requests.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of inquiry/payment/status requests with an incorrect or missing signature are rejected, across all vendors/channels — measured by sending known-bad-signature requests and confirming zero reach business logic.
- **SC-002**: 100% of inquiry/payment/status requests with a stale or future-dated timestamp outside the allowed window are rejected, across all vendors/channels.
- **SC-003**: 100% of create-VA/list-VA/delete-VA requests lacking a valid, unexpired access token are rejected before reaching business logic.
- **SC-004**: Existing integrations that already send a correctly-computed signature and/or a valid access token (as the current tooling does) see zero behavior change on the happy path, for both the vendor and merchant sides.

## Assumptions

- The allowed timestamp freshness window follows the same tolerance already used by the B2B token endpoint (a small number of minutes in each direction), rather than introducing a new, differently-tuned value.
- "Fail closed" for a misconfigured vendor/channel (FR-004) is preferred over silently allowing unverified requests, even though it means a configuration mistake shows up as an outage rather than a silent security gap — this matches how the B2B token flow already treats missing/invalid credentials.
- Enforcement is unconditional and applies identically to every vendor/channel and every merchant from the moment this feature is deployed — there is intentionally no per-vendor or global enable/disable toggle, since enforcing these checks correctly is the entire purpose of this fix, not an optional behavior. Any vendor/channel whose client-side signing was never actually correct (because it was never checked) is expected to be fixed before this deploys, not accommodated by a bypass. **Amended**: signature verification still has no such toggle; the timestamp-freshness sub-check alone gained a single global (not per-vendor) `APP_ENV=dev`/`uat` exception, off in prod — see `research.md` Decision 4 Amendment.
- The merchant-side access token requirement applies to all current and future endpoints under the merchant-facing group (create-VA, list-VA, delete-VA), not just the three named today.
- No new secret-provisioning flow is introduced for merchants — the existing B2B token issuance process is the sole source of the credential merchants use to satisfy FR-005.
- This feature does not change what happens after a token/signature is validated (i.e. no changes to business logic, response shapes, or the VA number consistency rules from feature 008); it only changes whether a request is admitted in the first place.
