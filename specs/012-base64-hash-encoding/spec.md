# Feature Specification: Base64 Hash/Signature Encoding Standardization

**Feature Branch**: `012-base64-hash-encoding`

**Created**: 2026-07-30

**Status**: Draft

**Input**: User description: "ubah cara endoded hash menggunakan base64 instead of hex"

## Clarifications

### Session 2026-07-30

- Q: Which hash/signature encoding is in scope for this change? → A: All of them — standardize every hex-encoded hash/signature in the system to base64: the SNAP/ASPI `stringToSign` body hash (signature verification, both inbound and outbound), the internal idempotency payload hash, and the outbound merchant webhook signature.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Vendors and merchants sign requests with base64-encoded body hashes (Priority: P1)

As a vendor or merchant integrating with this payment gateway, I compute a hash of my request body and include it in the string I sign to authenticate my request. Today that hash must be hex-encoded; going forward it must be base64-encoded, so my signing logic (and the system verifying it) both need to agree on the same encoding.

**Why this priority**: This is the encoding used in the core inbound/outbound signature verification flow (SNAP/ASPI-style symmetric signatures) that every vendor and merchant transaction depends on — without this, no other part of the change matters, and getting it wrong breaks every signed request.

**Independent Test**: Can be fully tested by computing a request body hash both ways (hex and base64), building the signed string with the base64 form, sending the request, and confirming the system accepts it — while a request whose hash uses the old hex form is rejected.

**Acceptance Scenarios**:

1. **Given** a vendor computes a base64-encoded SHA-256 hash of its request body and builds its signed string using that hash, **When** it sends a transactional request, **Then** the system verifies the signature successfully using the same base64 encoding and accepts the request.
2. **Given** a merchant computes a base64-encoded SHA-256 hash of its request body and signs accordingly, **When** it calls a signature-protected endpoint, **Then** the request is accepted.
3. **Given** a vendor or merchant still sends a request signed using the old hex-encoded hash convention, **When** the system verifies the signature, **Then** the request is rejected, since the two sides no longer agree on hash encoding.
4. **Given** this system calls out to a vendor's API as a client (outbound signature generation), **When** it builds its own signed request, **Then** it computes and encodes the body hash as base64, matching what it now expects to receive.

---

### User Story 2 - Merchants verify webhook callback signatures encoded as base64 (Priority: P2)

As a merchant receiving payment notification callbacks from this gateway, I verify the authenticity of each callback using a signature the gateway attaches to the request. Today that signature is hex-encoded; going forward it is base64-encoded, so my callback receiver needs to expect the new format.

**Why this priority**: Important for end-to-end consistency and to avoid leaving one signature type on the old encoding while everything else moves to base64, but it's a separate, less transaction-critical path (asynchronous notification, not the synchronous request path), so it can follow the core signature change.

**Independent Test**: Can be tested by triggering a payment notification callback and confirming the delivered signature is valid base64 and verifies correctly using a base64-based verification routine.

**Acceptance Scenarios**:

1. **Given** a payment event triggers an outbound webhook callback to a merchant, **When** the callback is sent, **Then** its signature is encoded as base64 rather than hex.
2. **Given** a merchant's callback receiver expects the old hex-encoded signature format, **When** it receives a base64-encoded signature after this change, **Then** the merchant is expected to update their verification logic accordingly (this is a breaking change requiring merchant-side migration, not a silent compatibility shim).

---

### User Story 3 - Internal payload-mismatch detection continues to work unaffected by user-facing encoding (Priority: P3)

As the system itself, I use a hash of incoming request payloads purely internally to detect whether a repeated request (same idempotency key) has a different body than the original — this hash is never sent to or computed by any external party. For consistency, this internal hash also moves to base64 encoding, but since no external client depends on its format, this carries no compatibility risk.

**Why this priority**: Lowest priority since it's an internal-only implementation detail with no external contract — included for consistency across the codebase, not because any user-facing behavior depends on it.

**Independent Test**: Can be tested by sending two requests with the same idempotency key and different bodies, and confirming the system still correctly detects and rejects the mismatch after the encoding change.

**Acceptance Scenarios**:

1. **Given** two requests share the same idempotency key but have different bodies, **When** the second request is processed, **Then** the system still detects the payload mismatch and rejects it, regardless of the underlying hash encoding used internally.

---

### Edge Cases

- What happens when a vendor/merchant sends a request whose hash was computed correctly but encoded as hex instead of base64 (i.e., they haven't migrated yet)? The system must reject it with a signature-verification failure, not silently accept or silently attempt both encodings.
- How does the system handle a request where the hash string itself is not valid base64 (malformed encoding, stray characters)? It should be treated as a signature/verification failure, not a server error.
- What happens to in-flight requests or cached idempotency records that were created before this change, using the old hex-encoded internal hash? They should not need to be migrated, since the internal hash is only compared within the lifetime of a single idempotency cache entry (short TTL), not persisted long-term across the change.
- How does the change interact with existing automated tests, sample scripts, and onboarding documentation that hardcode the hex convention? All of these must be updated in lockstep with the production code change, or they will fail against the new behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST compute and encode the request-body hash used in signature verification (for both vendor-facing and merchant-facing signed endpoints) as base64 instead of hex.
- **FR-002**: The system MUST, when acting as a client calling an external vendor's API, compute and encode its own outbound request-body hash as base64, consistent with FR-001.
- **FR-003**: The system MUST reject any signed request whose signature was computed using the old hex-encoded hash convention, once this change is active — there is no dual-encoding compatibility mode.
- **FR-004**: The system MUST encode the outbound merchant webhook/callback signature as base64 instead of hex.
- **FR-005**: The system MUST update its internal idempotency payload-mismatch hash to use base64 encoding as well, for consistency, even though this hash has no external contract.
- **FR-006**: The system's own test suite, sample integration scripts, and onboarding documentation MUST be updated to reflect base64 encoding everywhere the old hex convention was previously documented or hardcoded.
- **FR-007**: The system MUST NOT change the underlying hash algorithm (SHA-256) or signature algorithm (HMAC-SHA512/RSA, as applicable) — only the textual encoding of their output changes.
- **FR-008**: The system MUST treat a malformed (invalid) base64 hash/signature value the same way it treats a signature mismatch — as an authentication/verification failure, not a server error.

### Key Entities

- **Signed Request String (stringToSign)**: The colon-delimited string vendors, merchants, and this system (as an outbound client) build and sign; one of its components — the request-body hash — changes from hex to base64 encoding.
- **Webhook/Callback Signature**: The signature this system attaches to outbound payment-notification callbacks to merchants; changes from hex to base64 encoding.
- **Idempotency Payload Hash**: An internal-only hash used to detect payload mismatches on repeated requests sharing an idempotency key; changes from hex to base64 encoding with no external impact.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of vendor and merchant transactional requests signed using the new base64 hash convention are accepted with no change in success rate compared to today's hex convention.
- **SC-002**: 100% of requests signed using the old hex-encoded hash convention are rejected once this change is active, with no unintended acceptance of stale-format requests.
- **SC-003**: All existing sample integration scripts and onboarding documentation are updated and produce accepted requests end-to-end with zero manual correction needed by a new integrator following them.
- **SC-004**: Merchants relying on webhook callback signature verification can migrate to the new base64 format and successfully verify 100% of callbacks post-migration.

## Assumptions

- This is an intentional breaking change for all existing vendor/merchant integrations that sign requests or verify webhook callbacks — there is no requirement to support both hex and base64 encodings simultaneously.
- The hashing algorithm (SHA-256) and signing algorithm (HMAC-SHA512 for symmetric signatures, RSA for asymmetric token-request signing) remain unchanged; only the textual encoding of hash/signature output changes from hex to base64.
- Coordinated rollout communication to vendors and merchants (so they update their signing/verification logic before or alongside this change going live) is assumed to happen outside the scope of this spec, similar to prior breaking-change features in this system.
- The RSA-signed B2B token-request signature (already base64-encoded today, per existing token-issuance flow) is unaffected by this change since it's already in the target encoding.
