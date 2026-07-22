# Research: BCA Virtual Account Integration

**Feature**: 002-bca-virtual-account
**Date**: 2026-07-22

## Research Areas

### 1. BCA SNAP VA API Authentication

**Decision**: Use HMAC-SHA256 signature for outbound API calls (not RSA asymmetric)

**Rationale**: BCA SNAP VA API uses symmetric signature (HMAC-SHA512 with Client Secret) for API requests, while RSA asymmetric signature is only used for B2B access token requests. The existing `RSAVerifier` handles token requests; new `HMACSigner` will handle API calls.

**Alternatives Considered**:
- RSA asymmetric for all calls: Rejected - BCA VA API only accepts HMAC for API endpoints
- No signature: Rejected - violates BCA security requirements

**Implementation**:
- New `HMACSigner` in `infrastructure/crypto/hmac.go`
- StringToSign format: `HTTPMethod:RelativeURL:AccessToken:SHA256(RequestBody):Timestamp`
- Signature = HMAC-SHA512(ClientSecret, StringToSign), base64 encoded

---

### 2. Configuration-Driven Provider Pattern

**Decision**: Load BCA VA config from `.env.bca.va` file with structured parsing

**Rationale**: Constitution Principle II requires zero-code modification for provider integration. The `.env.bca.va` format allows environment-specific overrides while keeping defaults in the file.

**Alternatives Considered**:
- YAML/JSON config: Rejected - user explicitly requested `.env` format
- Hardcoded constants: Rejected - violates Principle II
- Database-stored config: Rejected - adds unnecessary complexity for initial integration

**Implementation**:
- New `BCAVAConfig` struct in `infrastructure/config/bca_va_config.go`
- Parse `.env.bca.va` file at startup
- Support environment variable overrides (standard `.env` behavior)
- Struct fields map to BCA API requirements (endpoints, credentials, channel IDs)

---

### 3. VA Domain Model Design

**Decision**: Create separate domain entities for VA operations (Inquiry, Payment, Status)

**Rationale**: Each operation has distinct request/response structures per BCA API spec. Separate entities improve type safety and make the code self-documenting.

**Alternatives Considered**:
- Single generic VA entity: Rejected - loses type safety, harder to validate
- Map directly to BCA JSON: Rejected - violates Clean Architecture (domain shouldn't depend on external format)

**Implementation**:
- `domain/va.go` with:
  - `VAInquiryRequest/Response` - bill presentment
  - `VAPaymentRequest/Response` - payment notification
  - `VAStatusRequest/Response` - status inquiry
  - `VAConfig` interface - provider configuration port
  - `VARepository` interface - persistence port
  - `VAGateway` interface - external API port

---

### 4. BCA Response Code Mapping

**Decision**: Create a dedicated response code mapper

**Rationale**: BCA uses a 7-character response code format (AAABBCC) where AAA=HTTP code, BB=service code, CC=case code. A mapper isolates this logic and makes it reusable.

**Alternatives Considered**:
- Inline switch statements: Rejected - scattered, hard to maintain
- Constants only: Rejected - doesn't handle mapping logic

**Implementation**:
- `BCAResponseCode` type with constants
- `MapBCAResponseCode(code string) (httpStatus int, domainError error)` function
- Tests verify all known BCA response codes

---

### 5. Idempotency for BCA Requests

**Decision**: Use X-EXTERNAL-ID header as idempotency key

**Rationale**: BCA requires X-EXTERNAL-ID to be unique per day. This naturally serves as an idempotency key. The existing idempotency middleware can be extended or a new BCA-specific middleware created.

**Alternatives Considered**:
- Generate our own idempotency key: Rejected - BCA requires specific format
- No idempotency: Rejected - violates Constitution Principle X

**Implementation**:
- New `BCAIdempotencyMiddleware` in `adapter/delivery/http/middleware/bca_idempotency.go`
- Validates X-EXTERNAL-ID format (numeric, max 36 chars)
- Stores in Redis with daily expiry
- Returns cached response if duplicate detected

---

### 6. Multi-Bills Transaction Support

**Decision**: Support both single settlement and multi-bills/multi-settlement

**Rationale**: BCA API supports both patterns. Multi-bills allow splitting a single VA payment into multiple sub-company settlements.

**Alternatives Considered**:
- Single settlement only: Rejected - limits functionality
- Multi-bills only: Rejected - breaks single settlement compatibility

**Implementation**:
- `BillDetail` struct with optional `billSubCompany` and `billAmount`
- Validation logic checks:
  - Single settlement: `totalAmount` must equal sum of `billDetails` amounts
  - Multi-bills: `billSubCompany` values must be unique
  - `subCompany` and `billSubCompany` must be registered in BCA

---

### 7. Error Handling Pattern

**Decision**: Extend existing `DomainError` pattern for BCA-specific errors

**Rationale**: The existing `DomainError` with `SNAPCode` field already maps domain errors to SNAP response codes. BCA uses similar codes (2400 series for VA).

**Alternatives Considered**:
- New error type: Rejected - adds complexity without benefit
- Generic errors: Rejected - loses BCA-specific error context

**Implementation**:
- Add BCA VA error constants to `domain/errors.go`:
  - `ErrVAInvalidBill` (4042419)
  - `ErrVAPaidBill` (4042414)
  - `ErrVAInvalidPartner` (4042412)
  - `ErrVADuplicateExternalID` (4092400)
- Handler maps these to appropriate HTTP status codes

---

### 8. Testing Strategy

**Decision**: Mock-based unit tests + integration tests with BCA sandbox

**Rationale**: Constitution Principle III requires TDD. External dependencies (BCA API, PostgreSQL, Redis) must be mocked for unit tests. Integration tests validate against BCA sandbox.

**Alternatives Considered**:
- Only unit tests: Rejected - doesn't validate real API behavior
- Only integration tests: Rejected - slow, flaky, requires network

**Implementation**:
- Unit tests: Mock `VAGateway`, `VARepository`, `VAConfig`
- Integration tests: BCA sandbox with test credentials
- Coverage target: 90%+ per Constitution Principle XI
- Test files: `va_usecase_test.go`, `va_handler_test.go`, `va_client_test.go`
