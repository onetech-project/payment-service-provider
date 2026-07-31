# Feature Specification: Merchant HMAC Signature Verification (ASPI-Compliant Two-Factor Auth)

**Feature Branch**: `010-merchant-hmac-signature`

**Created**: 2026-07-30

**Status**: Draft

**Input**: User description: "Tambahkan verifikasi HMAC signature (X-SIGNATURE) pada endpoint merchant-facing (create-va, list, delete-va), sebagai pelengkap dari accessToken Bearer yang sudah diwajibkan di feature 009-transfer-va-auth, supaya endpoint ini benar-benar sesuai pola standar ASPI/SNAP untuk endpoint transaksional (bearer token + HMAC signature berjalan bersamaan). Perlu mekanisme penyimpanan clientSecret per merchant (terasosiasi dengan client_id yang sama dipakai untuk accessToken B2B). Verifikasi HMAC identik dengan sisi vendor (HMAC-SHA512, freshness window ±5 menit) tapi komponen AccessToken di stringToSign diisi token asli (bukan string kosong seperti sisi vendor), karena accessToken benar-benar dikirim lewat header Authorization di endpoint merchant. Berjalan bersamaan dengan bearer token check yang sudah ada — kedua lapis harus lolos. Tidak ada toggle enable/disable, fail closed kalau clientSecret belum terdaftar. Endpoint vendor tidak terpengaruh."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Merchant requests are rejected when their signature doesn't match (Priority: P1)

A merchant calls create-VA, list-VA, or delete-VA with a valid `accessToken` (satisfying feature 009's bearer-token check), but the request is not signed, or is signed with an incorrect shared secret. Today (post-009) this request would succeed as long as the bearer token is valid — this story closes that remaining gap: the request must also carry a verifiable `X-SIGNATURE`, computed the same way ASPI/SNAP requires for any transactional endpoint.

**Why this priority**: This is the core value of the feature — without it, merchant endpoints remain one layer short of the standard's transactional-endpoint pattern, and a stolen/leaked bearer token alone (without the shared secret) would be enough to act as the merchant.

**Independent Test**: Can be fully tested by sending a request with a valid bearer token and a correctly-computed signature (using the merchant's real shared secret) and confirming it succeeds exactly as before; then sending the same request with the same valid bearer token but an incorrect or missing signature and confirming it is rejected before any business logic runs.

**Acceptance Scenarios**:

1. **Given** a merchant has a valid bearer token and knows its correct shared secret, **When** it sends a create-VA/list-VA/delete-VA request with a correctly-computed signature, **Then** the request is processed normally (unchanged behavior from feature 009's happy path).
2. **Given** a merchant request carries a valid bearer token but an `X-SIGNATURE` that does not match what the shared secret would produce, **When** the request is received, **Then** it is rejected as unauthorized and no business logic executes.
3. **Given** a merchant request carries a valid bearer token but no `X-SIGNATURE` header at all, **When** the request is received, **Then** it is rejected as unauthorized (same outcome as an invalid signature).

---

### User Story 2 - Merchant requests are rejected when their timestamp is stale or too far in the future (Priority: P2)

Mirroring the vendor-side protection from feature 009, a merchant request's `X-TIMESTAMP` is checked for freshness, not just format — closing the same replay-attack window on the merchant side that was already closed on the vendor side.

**Why this priority**: Directly extends the vendor-side pattern that already proved valuable; secondary to Story 1 because a stale-but-correctly-signed request is a narrower risk than an entirely unverified one.

**Independent Test**: Can be fully tested by sending a validly-signed, validly-token-bearing request with a timestamp far outside the allowed window (e.g. an hour old, or an hour in the future) and confirming it is rejected, while a request with a timestamp within the window and a correct signature still succeeds.

**Acceptance Scenarios**:

1. **Given** a validly-signed, valid-token request with a timestamp within the allowed freshness window, **When** it is received, **Then** it is processed normally.
2. **Given** a validly-signed, valid-token request with a timestamp older than the allowed window, **When** it is received, **Then** it is rejected as unauthorized.
3. **Given** a validly-signed, valid-token request with a timestamp further in the future than the allowed window, **When** it is received, **Then** it is rejected as unauthorized.

---

### User Story 3 - Operators can provision and manage a merchant's shared secret (Priority: P1)

Before a merchant can be verified via signature, the system needs to know that merchant's shared secret. An operator (or an onboarding process) needs a way to register a shared secret for a merchant, associated with the same client identity already used for that merchant's `accessToken` issuance.

**Why this priority**: Without this, Stories 1 and 2 have nothing to verify against for any merchant — this is a hard prerequisite, not an optional nicety, so it shares top priority with Story 1.

**Independent Test**: Can be fully tested by provisioning a shared secret for a test merchant client identity, then confirming that merchant's correctly-signed requests succeed and confirming a *different*, unprovisioned merchant's requests are rejected (fail closed) regardless of signature correctness.

**Acceptance Scenarios**:

1. **Given** an operator has provisioned a shared secret for a merchant's client identity, **When** that merchant sends a correctly-signed request, **Then** it is accepted.
2. **Given** a merchant's client identity has no shared secret provisioned, **When** that merchant sends any request (regardless of what signature it carries), **Then** it is rejected as unauthorized — the system fails closed rather than treating "no secret" as "no verification needed."

---

### User Story 4 - Bearer token and signature checks both apply, independently enforced (Priority: P1)

A merchant request must satisfy *both* the existing bearer-token check (feature 009) and the new signature check (this feature) — passing one does not exempt it from the other.

**Why this priority**: This is what makes the feature actually deliver ASPI-compliant two-factor verification rather than just swapping one check for another; without enforcing both simultaneously, the feature would silently regress one of the two protections.

**Independent Test**: Can be fully tested with four combinations: (valid token, valid signature) → success; (valid token, invalid/missing signature) → rejected; (invalid/missing token, valid signature) → rejected; (invalid/missing token, invalid/missing signature) → rejected.

**Acceptance Scenarios**:

1. **Given** a request has a valid bearer token AND a valid signature, **When** it is received, **Then** it succeeds.
2. **Given** a request has a valid bearer token but an invalid or missing signature, **When** it is received, **Then** it is rejected.
3. **Given** a request has an invalid or missing bearer token but a valid signature, **When** it is received, **Then** it is rejected.
4. **Given** a request has neither a valid bearer token nor a valid signature, **When** it is received, **Then** it is rejected.

---

### User Story 5 - Vendor-facing endpoints remain unaffected (Priority: P2)

The vendor-facing endpoints (inquiry/payment/status) and their existing signature-verification behavior (feature 009, including the convention that the AccessToken component of their `stringToSign` is always empty) are untouched by this feature.

> **Note (feature 011-vendor-access-token-signature)**: the "always empty" AccessToken convention described above was later made conditional — vendors migrated to `ClientID`-based onboarding now bind a real bearer token into the vendor-side `stringToSign` too. This statement remains accurate as a description of feature 010's own scope (it changed nothing here); see `specs/011-vendor-access-token-signature/` for the follow-up change.

**Why this priority**: Regression-safety confirmation rather than new capability — this feature is scoped entirely to the merchant-facing endpoint group.

**Independent Test**: Can be fully tested by re-running the existing vendor-side signature/timestamp test suite and end-to-end flows from feature 009 and confirming zero behavior change.

**Acceptance Scenarios**:

1. **Given** the existing vendor-facing inquiry/payment/status test suite and e2e flows from feature 009, **When** they are re-run after this feature ships, **Then** all outcomes are identical to before this feature.

### Edge Cases

- What happens when a merchant's shared secret is provisioned but empty/blank? Treated the same as "not provisioned" — fails closed (System Story 3, Scenario 2).
- What happens when the `Authorization` bearer token used in the signature computation doesn't match the actual token used for the bearer-token check (e.g. a client that signs with one token but sends a different one in the header)? The signature verification recomputes using the token that was **actually presented** in the `Authorization` header for that same request — a mismatch between what a client intended to sign and what it actually sent is not distinguishable from an incorrect signature, and is rejected the same way.
- What happens to a merchant whose shared secret was provisioned after this feature is already live, versus one who was never provisioned? Both are treated identically at request time — no special first-time grace period; a merchant either has a valid, provisioned secret at the moment of the request or its requests are rejected.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST store a shared secret per merchant client identity, associated with the same client identity used for that merchant's `accessToken` issuance.
- **FR-002**: System MUST provide a way for an operator to provision (create) a merchant's shared secret.
- **FR-003**: System MUST recompute the expected signature for create-VA/list-VA/delete-VA requests using the shared secret associated with the request's authenticated client identity, with the `accessToken` actually presented in the request's `Authorization` header used as the AccessToken component of the signing input — and compare it against the `X-SIGNATURE` header.
- **FR-004**: System MUST reject a create-VA/list-VA/delete-VA request whose signature does not match the recomputed value, before any business logic executes.
- **FR-005**: System MUST reject a create-VA/list-VA/delete-VA request whose `X-TIMESTAMP` falls outside an allowed freshness window (too old or too far in the future), independent of whether its signature is valid.
- **FR-006**: System MUST fail closed — reject all requests — for a merchant client identity that has no shared secret provisioned (including an empty/blank one).
- **FR-007**: System MUST enforce the signature check (FR-003/FR-004) and the freshness check (FR-005) in addition to, not instead of, the existing bearer-token check from feature 009 — both must pass for a request to be admitted.
- **FR-008**: System MUST NOT change the request/response contract, business logic, or outcomes of a successfully-admitted create-VA/list-VA/delete-VA request — this feature only changes whether a request is admitted.
- **FR-009**: System MUST NOT change vendor-facing (inquiry/payment/status) signature verification behavior established by feature 009, including that endpoint group's convention of an empty AccessToken component in its own signing input.
- **FR-010**: Enforcement (FR-003–FR-007) MUST apply unconditionally to every merchant from the moment this feature is deployed — no configuration exists to enable/disable it per merchant or globally. **Amended**: the timestamp-freshness check specifically (FR-005) later gained a single global (not per-merchant), `APP_ENV=dev`/`uat`-derived exception, off in prod — see `specs/009-transfer-va-auth/research.md` Decision 4 Amendment. Signature verification (FR-003/FR-004/FR-006/FR-007) remains unconditional with no toggle.

### Key Entities

- **Merchant shared secret**: A secret value associated one-to-one with a merchant's client identity (the same identity used for `accessToken` issuance), used to verify that identity's `X-SIGNATURE` on merchant-facing requests.
- **Merchant client identity**: The existing identity (already used for B2B `accessToken` issuance) that a merchant shared secret is provisioned against.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of merchant requests with a valid bearer token but an incorrect or missing signature are rejected — measured by sending known-bad-signature requests with otherwise-valid tokens and confirming zero reach business logic.
- **SC-002**: 100% of merchant requests with a valid bearer token and valid signature but a stale/future-dated timestamp outside the allowed window are rejected.
- **SC-003**: 100% of requests from a merchant with no provisioned shared secret are rejected, regardless of what signature they carry.
- **SC-004**: Merchant integrations that already send a valid bearer token AND a correctly-computed signature see zero behavior change on the happy path.
- **SC-005**: 100% of feature 009's existing vendor-side test/e2e outcomes remain unchanged after this feature ships.

## Assumptions

- The signature algorithm, `stringToSign` format, and freshness-window duration mirror feature 009's vendor-side implementation exactly (HMAC-SHA512, `HTTPMethod:EndpointUrl:AccessToken:SHA256Hash(Body):Timestamp`, ±5 minutes) — the only substantive difference is that the AccessToken component is the real token (since it is genuinely transmitted via the `Authorization` header on merchant endpoints), not an empty string.
- The exact technical mechanism for storing and provisioning merchant shared secrets (new table vs. extending an existing one, migration/seed vs. an admin API) is a plan-level decision, not fixed by this spec — the spec only requires that such a mechanism exists and is keyed to the same client identity used for `accessToken` issuance.
- This feature does not introduce a rotation, expiry, or revocation policy for merchant shared secrets beyond what already exists for other credentials in this system; that is out of scope unless a future feature requires it.
- This feature does not change how merchants obtain their `accessToken` (still via the existing B2B token endpoint) — it only adds a requirement on top of already-obtained tokens.
- Fail-closed behavior for an unprovisioned/empty shared secret (FR-006) is preferred over allowing the request through, consistent with the same design decision already made in feature 009 for the vendor side.
