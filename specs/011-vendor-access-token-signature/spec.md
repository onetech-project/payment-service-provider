# Feature Specification: Vendor Access Token in Symmetric Signature

**Feature Branch**: `011-vendor-access-token-signature`

**Created**: 2026-07-30

**Status**: Draft

**Input**: User description: "disisi vendor, keadaan sekarang tidak menggunakan access_token untuk symmetric signature, tambah header Authorization juga untuk sisi vendor dan masukan ke dalam symmetric signature"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Vendor authenticates transactional requests with Authorization header (Priority: P1)

As a vendor integrating with the payment service, I want to include an `Authorization` header (bearer access token) on my transactional requests (e.g. VA inquiry, VA payment) and have that token be part of the symmetric signature, so that my requests are authenticated the same way merchant requests are — proving both that I hold a valid access token and that the request content has not been tampered with.

**Why this priority**: This is the core of the requested change — without it, vendor requests remain signed with an empty access-token placeholder, which is weaker than the merchant flow and inconsistent with the SNAP-style standard the system otherwise follows.

**Independent Test**: Can be fully tested by issuing a vendor access token, sending a signed transactional request with a valid `Authorization: Bearer <token>` header where the token is included in the string-to-sign, and confirming the request is accepted when the signature is valid and rejected when it is not.

**Acceptance Scenarios**:

1. **Given** a vendor has obtained a valid access token, **When** it sends a transactional request with `Authorization: Bearer <token>` and a signature computed using that token as the access-token component of the string-to-sign, **Then** the request is accepted.
2. **Given** a vendor sends a transactional request with a valid `Authorization` header but a signature computed as if the access-token component were empty (old behavior), **When** the request is validated, **Then** the request is rejected as an invalid signature.
3. **Given** a vendor sends a transactional request without an `Authorization` header at all, **When** the request is validated, **Then** the request is rejected as unauthenticated.

---

### User Story 2 - Vendor requests are rejected on token/signature mismatch (Priority: P2)

As the payment service, I want to detect and reject vendor requests where the access token in the `Authorization` header does not match the token that was actually used to compute the signature, or where the token itself is invalid/expired, so that a stolen or reused signature cannot be replayed with a different or invalid token.

**Why this priority**: Prevents downgrade/replay attacks and ensures the new access-token binding actually strengthens security rather than just adding an unused header.

**Independent Test**: Can be tested by sending a request with a valid signature computed from token A but an `Authorization` header presenting token B, and confirming rejection; and by sending a request with an expired/invalid access token, confirming rejection regardless of signature validity.

**Acceptance Scenarios**:

1. **Given** a signature was computed using access token A, **When** the request is sent with `Authorization: Bearer B` (a different, otherwise-valid token), **Then** the request is rejected.
2. **Given** an expired or invalid access token, **When** a request is sent with that token in `Authorization` and a signature computed against it, **Then** the request is rejected.

---

### User Story 3 - Existing vendor integrations are migrated without silent breakage (Priority: P3)

As an operator of this service, I want existing vendor clients that currently sign requests without an `Authorization` header (empty access-token component) to receive a clear rejection and migration path once this change is active, rather than intermittent or confusing failures, so vendors can update their integration in a predictable way.

**Why this priority**: Important for rollout safety but secondary to the core security behavior itself; the affected vendor scripts and integrations are known and limited in number.

**Independent Test**: Can be tested by sending a legacy-format request (no `Authorization`, empty access-token component in signature) after the change is active and confirming it is rejected with an error that clearly indicates a missing/invalid `Authorization` header, distinct from a generic signature-mismatch error.

**Acceptance Scenarios**:

1. **Given** the new requirement is active, **When** a vendor sends a request using the old (no-`Authorization`, empty access-token) signing convention, **Then** the system rejects it with an error message indicating the `Authorization` header is required.

---

### Edge Cases

- What happens when the `Authorization` header is present but malformed (e.g. missing `Bearer` prefix, empty token value)?
- How does the system handle an access token that is syntactically valid but was never issued by this service?
- What happens when the access token is valid and correctly bound in the signature, but has expired between issuance and request time?
- How does the system handle case differences or extra whitespace in the `Authorization` header value when reconstructing the string-to-sign?
- What happens if a vendor sends multiple `Authorization` headers in one request?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST require vendor transactional requests (VA inquiry, VA payment, and other symmetric-signature-protected vendor endpoints) to include an `Authorization` header carrying a valid access token, in addition to the existing `X-SIGNATURE` and `X-TIMESTAMP` headers.
- **FR-002**: The system MUST include the vendor's access token as the access-token component of the string-to-sign when validating the symmetric signature for vendor requests, replacing the current fixed empty-string component.
- **FR-003**: The system MUST reject vendor requests that omit the `Authorization` header on endpoints that require symmetric-signature authentication.
- **FR-004**: The system MUST reject vendor requests where the signature does not match a string-to-sign built from the access token actually presented in the `Authorization` header (i.e., the token cannot be swapped after signing).
- **FR-005**: The system MUST reject vendor requests presenting an invalid, unrecognized, or expired access token, independent of whether the signature is otherwise well-formed.
- **FR-006**: The system MUST issue access tokens to vendors through the existing access-token issuance mechanism used elsewhere in the system, so vendors can obtain a token before signing requests.
- **FR-007**: The system MUST return a distinguishable error response when a request fails due to a missing/invalid `Authorization` header versus a generic signature mismatch, so vendors can diagnose integration issues during migration.
- **FR-008**: The system MUST apply this requirement consistently across all existing vendor-facing symmetric-signature-protected endpoints (not just a subset).

### Key Entities

- **Vendor Access Token**: A credential issued to a vendor that proves its identity/authorization; now becomes a required input to both the `Authorization` header and the signature computation for vendor transactional requests.
- **Vendor Symmetric Signature Request**: An inbound vendor request carrying `X-TIMESTAMP`, `X-SIGNATURE`, and now `Authorization`, whose signature is validated against a string-to-sign that incorporates all three plus method, path, and body hash.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of vendor transactional requests missing a valid `Authorization` header are rejected before reaching business logic.
- **SC-002**: 100% of vendor transactional requests whose signature was computed with a different access token than the one presented are rejected.
- **SC-003**: Existing vendor integrations that adopt the new signing convention (token included in `Authorization` and in the signature) see no change in success rate compared to before this feature, once migrated.
- **SC-004**: Vendors attempting to use the old (no-`Authorization`) signing convention receive an error response that clearly identifies the missing/invalid `Authorization` header as the cause, enabling self-service migration without support intervention.

## Assumptions

- Vendors already have (or can obtain) access tokens via the existing access-token issuance endpoint used by merchants, so no new token-issuance mechanism is required — only its adoption by vendor clients.
- "Vendor transactional requests" refers to the endpoints currently protected by the vendor symmetric-signature middleware (e.g. VA inquiry, VA payment, and related transfer-VA endpoints), not the token-issuance endpoint itself.
- This change is a breaking change for existing vendor integrations that do not yet send an `Authorization` header; a coordinated rollout/migration communication to vendors is assumed to happen outside the scope of this spec.
- The access-token format and validation rules (expiry, issuer, etc.) reuse the same mechanism already used for merchant `Authorization` validation, rather than introducing a separate vendor-specific token type.
