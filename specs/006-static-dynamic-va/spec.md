# Feature Specification: Static and Dynamic Virtual Account Creation

**Feature Branch**: `006-static-dynamic-va`

**Created**: 2026-07-28

**Status**: Draft

**Input**: User description: "fitur baru dibutuhkan untuk Static dan Dynamic VA dengan rule seperti berikut:
static no bill -> partnerServiceId: 15973, additionalInfo.vaType: 01
static variable bill -> partnerServiceId: 15974, additionalInfo.vaType: 02
static fixed bill -> partnerServiceId: 15975, additionalInfo.vaType: 03
dynamic no bill -> partnerServiceId: 15973, additionalInfo.vaType: 04
dynamic variable bill -> partnerServiceId: 15974, additionalInfo.vaType: 05
dynamic fixed bill -> partnerServiceId: 15975, additionalInfo.vaType: 06

semua request dynamic akan generate customerNo otomatis secara sequential, merchant akan mengirimkan request ke endpoint /create-va dengan customerNo kosong dan partnerServiceId dan additionalInfo.vaType sesuai dengan jenis request yang diinginkan. Sistem akan mengembalikan response dengan customerNo yang sudah terisi secara otomatis.

sedangkan static request, merchant harus mengirimkan request ke endpoint /create-va dengan customerNo yang sudah ditentukan sebelumnya, partnerServiceId dan additionalInfo.vaType sesuai dengan jenis request yang diinginkan. Sistem akan mengembalikan response dengan customerNo yang sama seperti yang dikirimkan oleh merchant."

## Clarifications

### Session 2026-07-28

- Q: For "variable bill" VAs, how should repeated payments toward the same VA be handled? → A: The same VA number can receive multiple payments over time; each payment is individually recorded, and the VA is considered fully paid ("lunas") once cumulative payments reach the target amount.
- Q: What is the maximum length and structure of `customerNo`, especially for dynamic (auto-generated) VAs? → A: `customerNo` has a maximum length of 20 digits. For dynamic VAs, it is composed of a 2-digit `vaType` code followed by an 18-digit sequential number (`vaType` + sequence, zero-padded to 18 digits).
- Q: Where does the bill amount live for "fixed bill" and "variable bill" requests? → A: The bill amount is carried in a top-level `totalAmount` field on the request/response, not nested inside `additionalInfo`.
- Q: How should the system prevent duplicate `customerNo` assignment for dynamic VAs under concurrent requests? → A: The system MUST use a locking or queuing mechanism (e.g., a per-`partnerServiceId` lock or serialized sequence-increment queue) around sequence generation so that no two concurrent requests can ever receive the same generated `customerNo`.
- Q: What should the error response contain when the sequence generator is unavailable? → A: The system MUST return a system-unavailable error response that includes a specific reason/cause describing why the sequence generator could not be reached or used, instead of a generic failure.

### Session 2026-07-28 (amendment)

- Q: The six VA type rules and the reserved `partnerServiceId` set were originally hardcoded. Should they instead be operator-configurable master data? → A: Yes. Move both into database-backed master data tables (`master_va_type`: id, va_type, dynamic, billing, description; `master_partner_service_ids`: id, partner_service_id, bankCode) so operators can add/adjust VA types and partner service IDs without a code deployment, per Constitution Principle II (configuration-driven integrations).
- Q: How should the system avoid hitting PostgreSQL for this master data on every `/create-va` call under high load? → A: Cache both tables in Redis, refreshed on a 5-minute interval; when a change is made to either table, the cache MUST be refreshed immediately (not wait for the next interval) so operators see their change take effect right away.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create Dynamic Virtual Account with Auto-Generated Customer Number (Priority: P1)

As a merchant, I want to request a dynamic Virtual Account (no bill, variable bill, or fixed bill) without specifying a customer number, so that the system automatically assigns the next available sequential customer number and I don't have to manage numbering myself.

**Why this priority**: Dynamic VA creation with auto-numbering is the primary new capability requested and unblocks merchants who need on-demand VA issuance without pre-coordinating customer numbers.

**Independent Test**: Can be fully tested by sending a `/create-va` request with an empty `customerNo`, a valid `partnerServiceId`, and a dynamic `additionalInfo.vaType` (04, 05, or 06), and verifying the response contains a system-generated `customerNo` that is unique and strictly greater than (or otherwise sequentially follows) the last one issued for that `partnerServiceId`/`vaType` combination.

**Acceptance Scenarios**:

1. **Given** a merchant sends a `/create-va` request with `customerNo` empty, `partnerServiceId: 15973`, `additionalInfo.vaType: 04` (dynamic no bill), **When** the system processes the request, **Then** the system generates the next sequential `customerNo` for that combination and returns it in the response.
2. **Given** a merchant sends a `/create-va` request with `customerNo` empty, `partnerServiceId: 15974`, `additionalInfo.vaType: 05` (dynamic variable bill), `totalAmount` set to the cumulative target, **When** the system processes the request, **Then** the system generates a sequential `customerNo` and returns a VA that accepts one or more payments toward `totalAmount`.
3. **Given** a merchant sends a `/create-va` request with `customerNo` empty, `partnerServiceId: 15975`, `additionalInfo.vaType: 06` (dynamic fixed bill), `totalAmount` set to the fixed bill amount, **When** the system processes the request, **Then** the system generates a sequential `customerNo` and returns a VA bound to that fixed `totalAmount`.
4. **Given** two dynamic `/create-va` requests for the same `partnerServiceId`/`vaType` arrive concurrently, **When** both are processed, **Then** each receives a distinct, non-colliding `customerNo`.

---

### User Story 2 - Create Static Virtual Account with Merchant-Supplied Customer Number (Priority: P2)

As a merchant, I want to request a static Virtual Account (no bill, variable bill, or fixed bill) by supplying my own pre-determined customer number, so that the resulting VA number matches the customer identifier I already track in my own system.

**Why this priority**: Static VA creation is required for merchants who pre-assign customer numbers (e.g. existing customer IDs) and must be supported alongside dynamic VA, but it depends on the same `/create-va` endpoint and validation logic established for dynamic VA, making it a close second priority.

**Independent Test**: Can be fully tested by sending a `/create-va` request with a specific non-empty `customerNo`, a valid `partnerServiceId`, and a static `additionalInfo.vaType` (01, 02, or 03), and verifying the response echoes back the exact same `customerNo` that was submitted.

**Acceptance Scenarios**:

1. **Given** a merchant sends a `/create-va` request with `customerNo: "0001234567"`, `partnerServiceId: 15973`, `additionalInfo.vaType: 01` (static no bill), **When** the system processes the request, **Then** the response returns the identical `customerNo: "0001234567"`.
2. **Given** a merchant sends a `/create-va` request with a specific `customerNo`, `partnerServiceId: 15974`, `additionalInfo.vaType: 02` (static variable bill), `totalAmount` set to the cumulative target, **When** the system processes the request, **Then** the response returns the identical `customerNo` and the VA accepts one or more payments toward `totalAmount`.
3. **Given** a merchant sends a `/create-va` request with a specific `customerNo`, `partnerServiceId: 15975`, `additionalInfo.vaType: 03` (static fixed bill), `totalAmount` set to the fixed bill amount, **When** the system processes the request, **Then** the response returns the identical `customerNo` and the VA is bound to that fixed `totalAmount`.
4. **Given** a merchant sends a static `/create-va` request with a `customerNo` that already exists for that `partnerServiceId`, **When** the system processes the request, **Then** the system rejects the request with an error indicating the customer number is already in use.

---

### User Story 3 - Reject Invalid VA Type and Service Combinations (Priority: P3)

As a merchant, I want the system to validate that my request's `partnerServiceId` and `additionalInfo.vaType` combination is one of the six defined VA types, and that `customerNo` is supplied or omitted correctly for the chosen type, so that I get clear, immediate feedback instead of a malformed or inconsistent VA being created.

**Why this priority**: Validation is important for data integrity and merchant experience but is a supporting/guardrail capability rather than core new value; it can be layered onto the flows in User Story 1 and 2.

**Independent Test**: Can be fully tested by submitting requests with mismatched `partnerServiceId`/`vaType` pairs, unknown `vaType` values, a non-empty `customerNo` on a dynamic request, or an empty `customerNo` on a static request, and verifying each is rejected with a descriptive error.

**Acceptance Scenarios**:

1. **Given** a merchant sends a `/create-va` request with `partnerServiceId: 15973` and `additionalInfo.vaType: 02` (a mismatched pair), **When** the system validates the request, **Then** the system rejects it with an error indicating the `partnerServiceId`/`vaType` combination is invalid.
2. **Given** a merchant sends a `/create-va` request with an `additionalInfo.vaType` outside the defined set (01-06), **When** the system validates the request, **Then** the system rejects it with an error indicating the VA type is unrecognized.
3. **Given** a merchant sends a `/create-va` request with a dynamic `vaType` (04, 05, or 06) but a non-empty `customerNo`, **When** the system validates the request, **Then** the system rejects it with an error indicating `customerNo` must be empty for dynamic VA types.
4. **Given** a merchant sends a `/create-va` request with a static `vaType` (01, 02, or 03) but an empty `customerNo`, **When** the system validates the request, **Then** the system rejects it with an error indicating `customerNo` is required for static VA types.

---

### User Story 4 - Manage VA Type and Partner Service ID Master Data Without a Deployment (Priority: P4)

As an operator, I want the six VA type rules and the reserved `partnerServiceId`/bank code set to live in database tables instead of hardcoded application logic, so that adding or adjusting a VA type or partner service ID mapping is a data change, not a code deployment, while `/create-va` traffic still reads this data from a fast in-memory cache rather than hitting the database on every request.

**Why this priority**: This is an internal operability improvement, not a new merchant-facing capability — the six VA type combinations and their validation behavior (User Stories 1-3) are unaffected from the merchant's point of view. It is lower priority than the merchant-facing flows but still valuable for reducing future maintenance cost and database load.

**Independent Test**: Can be fully tested by inserting/updating a row in `master_va_type` or `master_partner_service_ids` and verifying that (a) a subsequent `/create-va` request reflects the change without restarting the service, and (b) repeated `/create-va` calls under load do not produce a proportional increase in PostgreSQL queries for this master data (the cache is doing the work).

**Acceptance Scenarios**:

1. **Given** the `master_va_type` and `master_partner_service_ids` tables are seeded with the six existing VA type rules and three reserved partner service IDs, **When** the service starts and serves `/create-va` requests, **Then** validation and routing behave identically to the previously hardcoded rules (no regression).
2. **Given** an operator updates a row in `master_va_type` (e.g. changes a `description` or `billing` value) or `master_partner_service_ids`, **When** the change is committed to the database, **Then** the in-memory/Redis cache used by `/create-va` reflects the change without waiting for the next scheduled refresh.
3. **Given** no changes have been made to either table, **When** the scheduled refresh interval (5 minutes) elapses, **Then** the cache is refreshed from the database on schedule regardless.
4. **Given** Redis is temporarily unavailable, **When** a `/create-va` request arrives, **Then** the system falls back to the last known-good in-memory copy of the master data (or reads PostgreSQL directly as a last resort) rather than failing every request outright.

---

### Edge Cases

- What happens when the sequential number component for a given `vaType` exhausts its 18-digit range? This is an operational limit far beyond expected volume and is out of scope for initial validation, but the system MUST NOT silently wrap or collide — it MUST surface a system-unavailable error (see below) if generation cannot produce a valid next value.
- How does the system handle a "fixed bill" request (static or dynamic) that omits the required `totalAmount`? The request MUST be rejected.
- How does the system handle a "variable bill" request that includes a `totalAmount` at creation time when only a target/cumulative amount (not a fixed one-time amount) is expected? The system records `totalAmount` as the cumulative target the VA must reach across one or more payments before being considered fully paid.
- What happens if two static VA creation requests for the same `customerNo` and `partnerServiceId` arrive at nearly the same time (race to register the same identifier)? The same locking/queuing mechanism used for dynamic sequence generation MUST also guard static `customerNo` registration so only one of the two requests succeeds and the other is rejected as duplicate.
- How does the system behave if the underlying sequence generator is temporarily unavailable when a dynamic VA request arrives? The request MUST be rejected with a system-unavailable error response that includes the specific reason the sequence generator could not be used.
- How are multiple payments toward a single "variable bill" VA tracked? Each payment MUST be recorded individually against the VA, and the VA's paid status transitions to fully paid ("lunas") only once the sum of recorded payments reaches the `totalAmount` target.
- What happens if two operators update `master_va_type` or `master_partner_service_ids` at nearly the same time? The last write to PostgreSQL wins (standard row update semantics); the cache-refresh-on-change mechanism MUST still pick up whichever state is currently in the database, not an intermediate one.
- What happens to in-flight `/create-va` requests while a master-data cache refresh is in progress? In-flight requests MUST continue to use a consistent snapshot (either the pre-refresh or post-refresh data, never a partially-applied mix) — the refresh MUST NOT be visible as a torn/partial read.
- What happens if `master_va_type` or `master_partner_service_ids` is emptied out or a row referenced by an in-flight request is deleted? `/create-va` requests referencing a no-longer-present combination MUST be rejected the same way an always-invalid combination is (FR-003), not crash or fall back to stale hardcoded defaults.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose a `/create-va` endpoint that accepts `partnerServiceId`, `customerNo`, `additionalInfo.vaType`, and `totalAmount` along with other existing VA creation fields.
- **FR-002**: System MUST recognize exactly six valid `partnerServiceId` / `additionalInfo.vaType` combinations: (15973, 01) static no bill, (15974, 02) static variable bill, (15975, 03) static fixed bill, (15973, 04) dynamic no bill, (15974, 05) dynamic variable bill, (15975, 06) dynamic fixed bill.
- **FR-003**: System MUST reject `/create-va` requests whose `partnerServiceId` and `additionalInfo.vaType` do not match one of the six defined combinations.
- **FR-004**: System MUST, for dynamic `vaType` values (04, 05, 06), require the request's `customerNo` field to be empty and automatically generate a sequential `customerNo` for the response.
- **FR-005**: System MUST generate dynamic `customerNo` values sequentially and uniquely, using a locking or queuing mechanism around sequence generation, such that no two dynamic VAs ever receive the same `customerNo` for the same `vaType`, including under concurrent request load.
- **FR-005a**: System MUST format dynamic `customerNo` values as a 20-digit string composed of the 2-digit `additionalInfo.vaType` code followed by an 18-digit zero-padded sequential number.
- **FR-006**: System MUST, for static `vaType` values (01, 02, 03), require the request's `customerNo` field to be non-empty and use it as-is for the created VA.
- **FR-007**: System MUST return the merchant-supplied `customerNo` unchanged in the response for static VA creation requests.
- **FR-008**: System MUST reject static `/create-va` requests where the supplied `customerNo` is already registered for the same `partnerServiceId`, using the same locking/queuing mechanism to prevent race conditions between concurrent static registrations.
- **FR-009**: System MUST reject `/create-va` requests where `customerNo` is non-empty for a dynamic `vaType`, and requests where `customerNo` is empty for a static `vaType`.
- **FR-010**: System MUST persist the VA type classification (static/dynamic, no bill/variable/fixed) as part of the created VA record so subsequent inquiry and payment operations can apply the correct bill-amount behavior.
- **FR-011**: System MUST reject `/create-va` requests for "fixed bill" `vaType` values (03, 06) that do not include a `totalAmount`.
- **FR-012**: System MUST NOT require (and MUST ignore or reject, per validation policy) a `totalAmount` on "no bill" `vaType` requests (01, 04).
- **FR-013**: System MUST accept a `totalAmount` on "variable bill" `vaType` requests (02, 05) as the cumulative target amount, and MUST allow multiple payments against the same VA number, individually recording each payment, until the sum of recorded payments reaches `totalAmount`, at which point the VA is marked fully paid.
- **FR-014**: System MUST reject `/create-va` requests with a system-unavailable error, including a specific reason describing the cause, when the sequence generator required for dynamic `customerNo` generation cannot be reached or used.
- **FR-015**: System MUST persist the six VA type rules (partnerServiceId/vaType mode/billing/description) in a `master_va_type` database table, replacing the previously hardcoded rule table, and MUST persist the reserved partner service ID set (with each entry's associated bank code) in a `master_partner_service_ids` database table.
- **FR-016**: System MUST cache both master data tables in Redis and serve `/create-va` VA-type-rule lookups from that cache rather than querying PostgreSQL on every request.
- **FR-017**: System MUST refresh the master-data cache on a fixed 5-minute interval, and MUST also refresh it immediately whenever a row in `master_va_type` or `master_partner_service_ids` is created, updated, or deleted, so operator changes take effect without waiting for the next scheduled interval.
- **FR-018**: System MUST continue serving `/create-va` requests using the last known-good cached master data if Redis is temporarily unavailable, rather than failing all such requests.
- **FR-019**: System MUST reject `/create-va` requests whose `partnerServiceId`/`additionalInfo.vaType` combination is not present in the current master data, using the same invalid-combination error as FR-003.

### Key Entities

- **Virtual Account Type**: Represents the classification of a VA by mode (static/dynamic) and billing behavior (no bill/variable bill/fixed bill), identified by a `partnerServiceId` + `additionalInfo.vaType` pair.
- **Virtual Account Creation Request**: The `/create-va` payload containing `partnerServiceId`, `customerNo` (empty for dynamic, pre-set for static), `additionalInfo.vaType`, and `totalAmount` (when applicable).
- **Virtual Account Creation Response**: The `/create-va` reply containing the resolved `customerNo` (system-generated for dynamic, echoed for static) and the created VA details.
- **Customer Number Sequence**: The per-`vaType` counter/state, guarded by a locking or queuing mechanism, that produces the next unique 18-digit sequential number used to build a dynamic VA's 20-digit `customerNo` (2-digit `vaType` + 18-digit sequence).
- **Virtual Account Payment Record**: An individual payment made against a VA number, recorded separately for each payment; for "variable bill" VAs, the sum of these records is compared against `totalAmount` to determine fully-paid status.
- **VA Type Master Data**: The database-backed definition of a VA type rule — `id`, `va_type` (unique, max 2 chars), `dynamic` (boolean), `billing` (`none`/`variable`/`fixed`), `description` — replacing the previously hardcoded six-row rule table.
- **Partner Service ID Master Data**: The database-backed definition of a reserved partner service ID — `id`, `partner_service_id` (unique, numeric, max 8 digits), `bankCode` (unique) — replacing the previously hardcoded reserved-ID set.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Merchants can create a dynamic VA without ever needing to compute or track customer numbers themselves — 100% of dynamic requests receive a valid, unique `customerNo` in the response.
- **SC-002**: 100% of static VA creation responses return the exact `customerNo` the merchant submitted.
- **SC-003**: 0% of dynamic VA creation requests under concurrent load result in duplicate `customerNo` assignment.
- **SC-004**: 100% of requests with an invalid `partnerServiceId`/`vaType` combination, or an incorrectly empty/non-empty `customerNo` for the given mode, are rejected with a clear, actionable error message rather than creating a malformed VA.
- **SC-005**: All six VA type combinations (static/dynamic × no bill/variable/fixed) are creatable and distinguishable end-to-end within the same release.
- **SC-006**: 100% of payments made against a "variable bill" VA are individually recorded, and the VA's paid status correctly reflects "lunas" only once cumulative payments reach `totalAmount`.
- **SC-007**: 100% of sequence-generator-unavailable errors returned to merchants include a specific, actionable reason for the failure.
- **SC-008**: An operator can add or modify a VA type or partner service ID mapping and see it take effect in `/create-va` behavior within seconds, without a code deployment or service restart.
- **SC-009**: Under sustained `/create-va` load, PostgreSQL query volume attributable to VA-type-rule/partner-service-ID lookups does not scale linearly with request volume (i.e., the cache is absorbing the read load).

## Assumptions

- The `/create-va` endpoint already exists (per prior VA integration features) and is being extended, not created from scratch, to support these six VA type combinations.
- "No bill" VAs carry no `totalAmount` at creation time; the payable amount is determined entirely at inquiry/payment time in a manner consistent with existing VA inquiry behavior.
- "Variable bill" VAs carry a `totalAmount` representing the cumulative target amount; the payer can make one or more payments against the same VA number, each individually recorded, until the cumulative total reaches `totalAmount` and the VA becomes fully paid.
- "Fixed bill" VAs require `totalAmount` to be supplied at creation time as a top-level field, and that amount is fixed for the life of the VA.
- Sequential number generation for dynamic `customerNo` is scoped per `vaType` (04, 05, 06 each maintain their own independent 18-digit sequence), since the `customerNo` itself encodes the `vaType` as its first 2 digits, making per-`vaType` sequences sufficient to guarantee overall uniqueness.
- `customerNo` is a maximum of 20 digits. Static `customerNo` values are merchant-defined (subject to the 20-digit maximum) and are not required to follow the `vaType`-prefix convention used for dynamic VAs.
- The same locking/queuing mechanism protects both dynamic sequence generation and static `customerNo` registration, and is treated as an implementation-level detail to be finalized during planning (e.g., database-level locking vs. an application-level queue).
- Existing authentication, authorization, and channel validation on `/create-va` remain unchanged by this feature.
- Master data management (creating/editing `master_va_type` and `master_partner_service_ids` rows) is performed directly against the database or via an existing/future admin surface; a dedicated merchant- or operator-facing CRUD API for this master data is out of scope for this feature.
- "Immediate refresh on change" is achieved via an application-level mechanism (e.g. a publish/subscribe notification or a write-path cache invalidation call) triggered when master data is modified through the application's own data-access layer; changes made by directly editing the database outside the application (e.g. a raw `UPDATE` from a DB console) are picked up on the next scheduled 5-minute refresh rather than instantly, since the application cannot observe changes it wasn't involved in.
- `bankCode` on `master_partner_service_ids` is descriptive/reference data (which bank the partner service ID belongs to) and is not itself validated against `/create-va` request fields in this feature.
