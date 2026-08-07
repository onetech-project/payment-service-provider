# Feature Specification: No-Bill VA — Transaction Created at Payment, Not at Create-VA

**Feature Branch**: `013-no-bill-payment-transaction`

**Created**: 2026-08-05

**Status**: Draft

**Input**: User description: "ada kesalahan flow untuk static dan dynamic no bill (01 dan 04), dimana seharusnya untuk flow no bill transaksi baru di insert ke db saat pembayaran, bukan saat create-va, create-va hanya membuat detail VA (partnerServiceId, customerNo, virtualAccountNo, customerName, etc)

dan create-va hanya cukup di panggil satu kali untuk satu nomor va

contohnya seperti topup e-wallet"

## Problem Statement

The current no-bill Virtual Account flow (VA types `01` static no bill and `04` dynamic no bill) treats VA creation and transaction creation as the same event. When a merchant calls `/create-va`, the system immediately records a *pending transaction* against that VA number. This produces three defects:

1. **A no-bill VA can only be paid once.** After the first payment settles the pending transaction, the VA number has no pending transaction left. Any further payment to the same VA number is rejected as "already paid or inactive".
2. **Merchants must re-register the VA before every payment.** To accept a second payment, the merchant has to call `/create-va` again for the same VA number — which contradicts the intent that a no-bill VA number is a durable, reusable payment address.
3. **Phantom transactions accumulate.** Every `/create-va` call creates a pending transaction that may never be paid, polluting transaction reporting and reconciliation with records that represent no real customer activity.

A no-bill VA has, by definition, no bill and no expected amount. It behaves like an e-wallet top-up address: the VA number is registered once, published to the customer, and the customer pays into it whenever they want, for whatever amount they want, as many times as they want. Each of those payments is one transaction.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Register a No-Bill VA Once (Priority: P1)

As a merchant, I want to call `/create-va` once for a no-bill VA and receive a durable VA number with its holder details, so that I can publish that number to my customer without ever having to re-register it.

**Why this priority**: This is the root correction. Everything else depends on `/create-va` no longer producing a transaction.

**Independent Test**: Send one `/create-va` request with `additionalInfo.vaType: 01` (or `04`), verify a success response carrying the VA identity fields, and verify no transaction/payment record exists for that VA number.

**Acceptance Scenarios**:

1. **Given** a merchant sends `/create-va` with `partnerServiceId: 15973`, `additionalInfo.vaType: 01`, a chosen `customerNo`, `virtualAccountName`, and `trxId`, **When** the system processes the request, **Then** the system registers the VA identity (partner service ID, customer number, virtual account number, holder name, holder email, holder phone, notification URL) and returns them in the response, and **no** transaction record is created.
2. **Given** a merchant sends `/create-va` with `partnerServiceId: 15973`, `additionalInfo.vaType: 04`, and an empty `customerNo`, **When** the system processes the request, **Then** the system generates the next sequential customer number, registers the VA identity under it, returns the generated customer number and derived virtual account number, and creates **no** transaction record.
3. **Given** a no-bill VA is already registered, **When** the merchant sends `/create-va` again for the same virtual account number with updated holder details, **Then** the system updates the registered holder details and returns success — it does **not** create a transaction and does **not** return a conflict.
4. **Given** a merchant sends `/create-va` for a no-bill VA type with `totalAmount` present, **When** the system processes the request, **Then** the system rejects it, because a no-bill VA carries no bill amount.

---

### User Story 2 - Customer Pays a No-Bill VA Repeatedly (Priority: P1)

As a customer, I want to pay into a registered no-bill VA number as many times as I like, each for an amount of my choosing, so that the VA behaves like an e-wallet top-up address.

**Why this priority**: This is the user-visible defect being fixed — today the second payment is rejected.

**Independent Test**: Register a no-bill VA once, then send three payment notifications for that VA number with different amounts and distinct payment request IDs, and verify three independent, successfully-settled transactions exist.

**Acceptance Scenarios**:

1. **Given** a no-bill VA is registered and has never been paid, **When** a payment notification arrives for that virtual account number with a paid amount and a unique payment request ID, **Then** the system creates a **new** transaction record for that payment, marks it settled, and returns a success response.
2. **Given** a no-bill VA has already received one settled payment, **When** a second payment notification arrives for the same virtual account number with a different payment request ID, **Then** the system creates a **second, independent** transaction record and returns success — it does **not** return "already paid or inactive".
3. **Given** a payment notification arrives for a no-bill virtual account number that has never been registered, **When** the system processes it, **Then** the system rejects the payment as an invalid bill / virtual account.
4. **Given** a payment notification is retried with a payment request ID that was already processed, **When** the system processes it, **Then** the system returns the original outcome without creating a duplicate transaction.
5. **Given** a payment arrives for a no-bill VA, **When** the transaction is created, **Then** it inherits the holder details (name, email, phone) and notification URL from the VA registration, and carries the paid amount as both the paid and total amount for that transaction.
6. **Given** a payment for a no-bill VA settles and the registered VA has a notification URL, **When** the transaction is created, **Then** the merchant receives exactly one payment callback for that payment.

---

### User Story 3 - Inquiry Against a Registered but Unpaid No-Bill VA (Priority: P1)

As a payment channel, I want an inquiry against a registered no-bill VA number to succeed and return the VA holder's name even when no payment has ever been made, so that the customer sees who they are paying before confirming.

**Why this priority**: With no transaction created at registration time, inquiry can no longer rely on a transaction row existing. Without this, every first-time payment attempt fails at the inquiry step.

**Independent Test**: Register a no-bill VA, send an inquiry for its virtual account number, and verify a success response carrying the registered holder name.

**Acceptance Scenarios**:

1. **Given** a no-bill VA is registered and unpaid, **When** an inquiry arrives for that virtual account number, **Then** the system returns inquiry status success with the registered holder name and the registered partner service ID / customer number, and it creates **no** transaction record.
2. **Given** a no-bill VA is registered and already has settled payments, **When** a new inquiry arrives, **Then** the system still returns success with the registered holder name — prior settled payments never block a new inquiry.
3. **Given** an inquiry arrives for a no-bill VA whose registration has been deactivated, **When** the system processes it, **Then** the system rejects it as an invalid bill / virtual account.
4. **Given** an inquiry arrives for a no-bill VA whose registration has passed its expiry date, **When** the system processes it, **Then** the system rejects it as expired and emits the merchant expiry callback exactly once.
5. **Given** the same inquiry request ID is retried, **When** the system processes it, **Then** the system returns the same result without creating additional records.

---

### User Story 4 - Query the Status of One No-Bill Payment (Priority: P2)

As a merchant, I want to query the status of a specific payment made into a no-bill VA, so that I can reconcile an individual top-up rather than a VA-level aggregate.

**Why this priority**: Reconciliation depends on it, but the money movement itself works without it.

**Independent Test**: Make two payments into the same no-bill VA, query the status of each by its own identifier, and verify each returns its own amount and timestamp.

**Acceptance Scenarios**:

1. **Given** a no-bill VA has two settled payments, **When** the merchant queries status for the first payment's identifier, **Then** the system returns that payment's own paid amount, reference number, and transaction timestamp — not the other payment's.
2. **Given** a no-bill VA is registered but has never been paid, **When** a status query arrives referencing an identifier with no matching payment, **Then** the system returns an invalid bill / virtual account response.

---

### User Story 5 - Bill-Bearing VA Types Keep Their Current Behavior (Priority: P2)

As a merchant using fixed-bill or variable-bill VAs (`02`, `03`, `05`, `06`), I want their existing behavior preserved, so that this correction does not regress the flows that are already right.

**Why this priority**: This is a regression guard. It delivers no new capability but protects existing revenue flows.

**Independent Test**: Run the existing acceptance scenarios for VA types 02, 03, 05, and 06 and verify unchanged outcomes.

**Acceptance Scenarios**:

1. **Given** a merchant creates a fixed-bill VA (`03` or `06`) with a total amount, **When** the system processes it, **Then** a pending transaction bound to that amount is created at create-VA time, exactly as today.
2. **Given** a merchant creates a variable-bill VA (`02` or `05`) with a total amount, **When** payments arrive, **Then** cumulative payments are tracked against the total amount and the VA is marked fully paid once the total is reached, exactly as today.
3. **Given** a bill-bearing VA has a pending transaction, **When** the merchant calls `/create-va` again for the same virtual account number, **Then** the system returns a conflict, exactly as today.

---

### User Story 6 - Deactivate a Registered No-Bill VA (Priority: P3)

As a merchant, I want to deactivate a no-bill VA number so that it stops accepting payments, since there is no pending transaction to cancel.

**Why this priority**: Needed for lifecycle completeness, but a registered VA that is simply never published is harmless in the interim.

**Independent Test**: Register a no-bill VA, deactivate it, then attempt an inquiry and a payment and verify both are rejected.

**Acceptance Scenarios**:

1. **Given** a no-bill VA is registered and active, **When** the merchant sends a delete-VA request for it, **Then** the system deactivates the registration and returns success.
2. **Given** a no-bill VA registration is deactivated, **When** a payment notification arrives for it, **Then** the system rejects the payment as an invalid bill / virtual account.
3. **Given** a no-bill VA registration is deactivated and had prior settled payments, **When** the merchant queries those payments' statuses, **Then** those historical payments remain readable and unchanged.
4. **Given** a no-bill VA registration is already deactivated, **When** the merchant sends delete-VA again, **Then** the system returns success without error.

---

### Edge Cases

- **Concurrent payments into the same no-bill VA**: two payment notifications with distinct payment request IDs arriving simultaneously must produce two independent settled transactions, with neither overwriting the other.
- **Registration race**: two concurrent `/create-va` calls for the same static no-bill customer number must not produce two conflicting registrations; one wins, the other either updates the same registration or is rejected consistently.
- **Payment amount is zero or negative**: rejected as an invalid field format; no transaction is created.
- **Payment arrives while the registration has an expiry date in the past**: rejected as expired, with the merchant expiry callback emitted exactly once, and no transaction created.
- **No-bill VA registration with no expiry date**: never expires; it remains payable indefinitely until explicitly deactivated.
- **Existing pending no-bill transactions created under the old flow**: must remain payable and queryable after the change; the correction must not strand in-flight VAs.
- **A virtual account number previously used for a bill-bearing VA type** is later registered as no-bill (or vice versa): the system must resolve the VA type unambiguously and apply the matching flow.
- **Payment notification carries holder details that differ from the registration**: the transaction records what the payment channel reported, while the registration remains the source of truth for the holder identity used in callbacks.
- **Notification URL absent on the registration**: the payment still settles; no callback is attempted.

## Requirements *(mandatory)*

### Functional Requirements

#### VA Registration (create-VA)

- **FR-001**: For no-bill VA types (`01`, `04`), the system MUST NOT create any transaction record when processing a `/create-va` request.
- **FR-002**: For no-bill VA types, `/create-va` MUST persist a VA registration containing at minimum: partner service ID, customer number, virtual account number, VA type, holder name, holder email, holder phone, merchant transaction reference, merchant notification URL, optional expiry date, and an active/inactive state.
- **FR-003**: For dynamic no-bill (`04`), the system MUST generate the customer number sequentially as it does today, and derive the virtual account number from partner service ID + customer number when the merchant does not supply one.
- **FR-004**: For static no-bill (`01`), the system MUST use the merchant-supplied customer number and MUST require the supplied virtual account number to equal partner service ID + customer number.
- **FR-005**: A repeat `/create-va` call for an already-registered no-bill virtual account number MUST update the registered holder details and return success, and MUST NOT return a conflict and MUST NOT create a transaction.
- **FR-006**: The system MUST reject a `/create-va` request for a no-bill VA type that carries a total amount.
- **FR-007**: The system MUST serialize concurrent registration attempts for the same partner service ID + customer number so that at most one registration exists per pair.

#### Payment

- **FR-008**: On a payment notification for a virtual account number registered as no-bill, the system MUST create a **new** transaction record for that payment, keyed by the payment's own unique payment request ID.
- **FR-009**: Each such transaction MUST record the paid amount as both its paid amount and its total amount, and MUST be recorded in a settled state.
- **FR-010**: The system MUST NOT reject a no-bill payment on the grounds that the virtual account number has prior settled transactions.
- **FR-011**: The system MUST reject a payment for a no-bill virtual account number that has no active registration.
- **FR-012**: The system MUST remain idempotent per payment request ID: a repeated payment request ID MUST return the original result and MUST NOT create a second transaction.
- **FR-013**: Each no-bill transaction MUST inherit the holder name, holder contact details, and notification URL from the VA registration when the payment notification does not supply them.
- **FR-014**: The system MUST emit exactly one merchant payment callback per newly settled no-bill payment, when the registration carries a notification URL.

#### Inquiry

- **FR-015**: An inquiry for a virtual account number registered as no-bill MUST resolve against the VA registration and MUST succeed regardless of how many prior payments exist.
- **FR-016**: An inquiry for a no-bill VA MUST return the registered holder name and registered identity fields, and MUST NOT create a transaction record.
- **FR-017**: An inquiry for a no-bill VA whose registration is inactive or past its expiry date MUST be rejected, and an expired registration MUST trigger the merchant expiry callback exactly once.

#### Status

- **FR-018**: A status query MUST resolve to a single payment transaction and return that transaction's own amount, reference, and timestamp.

#### Lifecycle and Compatibility

- **FR-019**: A delete-VA request for a no-bill VA MUST deactivate the registration rather than cancelling a pending transaction, and MUST be idempotent.
- **FR-020**: Deactivating or expiring a registration MUST leave historical settled transactions for that virtual account number readable and unmodified.
- **FR-021**: Bill-bearing VA types (`02`, `03`, `05`, `06`) MUST retain their current behavior: a transaction is created at create-VA time, and the existing single-settlement / cumulative-settlement rules continue to apply.
- **FR-022**: No-bill VA transactions created under the previous flow MUST remain payable, queryable, and reportable after this change.
- **FR-023**: Merchant VA listing and reporting MUST distinguish registered VA numbers from individual payment transactions, so a no-bill VA with many payments is not misreported as many VAs.

### Key Entities

- **VA Registration**: The durable identity of a virtual account number. Holds partner service ID, customer number, virtual account number, VA type, holder name, holder email, holder phone, merchant transaction reference, merchant notification URL, optional expiry date, active/inactive state, and creation/update timestamps. Created once per virtual account number. For no-bill VAs it carries no amount.
- **VA Transaction**: One payment event against a virtual account number. Holds the paid amount, payment request ID, reference number, transaction timestamp, settlement status, and the payment channel details reported by the vendor. For no-bill VAs, exactly one is created per payment; many can exist for a single VA Registration.
- **VA Type Rule** (existing): Master data classifying each VA type as dynamic/static and as no-bill / variable-bill / fixed-bill. Determines which of the two flows applies.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A merchant can register a no-bill VA number exactly once and that number accepts an unlimited number of payments — verified by 10 consecutive successful payments into a single registration with zero rejections.
- **SC-002**: Zero transaction records are produced by no-bill `/create-va` calls — verified by registering 100 no-bill VAs and observing a transaction count of 0 for those VA numbers.
- **SC-003**: A first-time customer can complete inquiry-then-pay on a freshly registered, never-paid no-bill VA with a 100% success rate.
- **SC-004**: Every settled no-bill payment produces exactly one merchant callback — no duplicates, no misses — across 100 payments.
- **SC-005**: All existing acceptance scenarios for VA types 02, 03, 05, and 06 continue to pass unchanged.
- **SC-006**: Every no-bill VA that had an in-flight (unpaid) transaction before the change remains payable after the change, with zero stranded VAs.
- **SC-007**: For a no-bill VA with N payments, merchant reporting shows 1 VA number and N transactions.

## Assumptions

These are informed defaults chosen where the feature description did not specify. Flagged here so they can be corrected before implementation.

- **A-001**: The correction applies **only** to no-bill VA types (`01`, `04`). Variable-bill and fixed-bill types keep create-VA-time transaction creation, since those flows carry a bill that legitimately exists before payment.
- **A-002**: A VA Registration is introduced for **all** managed VA types (`01`–`06`) as the single home of VA identity, but only no-bill types change *when* a transaction is created. This lets the existing "customer number already registered" uniqueness check move to the registration, and lets bill-bearing types create a fresh transaction on repeat `/create-va` calls without duplicating identity.
- **A-003**: Repeat `/create-va` for an already-registered no-bill VA is treated as an update of the holder details, not an error — the underlying banking specification models this endpoint as an upsert.
- **A-004**: A no-bill VA registration with no expiry date never expires. Expiry, when present, applies to the registration as a whole rather than to any individual payment.
- **A-005**: The inquiry response for a no-bill VA reports `totalAmount.value` as `"0.00"` always; it does not assert a bill amount, since no bill exists, and it does not echo the amount the customer entered at the payment channel — echoing it would be indistinguishable from asserting a bill for that figure.
- **A-006**: Existing no-bill transaction rows are left in place. A registration is backfilled from them so those VA numbers keep working, and any still-pending row remains payable.
- **A-007**: Individual payment transactions for a no-bill VA are retained indefinitely for reconciliation, following the retention already applied to VA transactions.
- **A-008**: The vendor-facing request and response field contracts are unchanged. This is a change to when records are written and what blocks a payment, not to the API surface.

## Dependencies

- **D-001**: The existing VA type master data (`master_va_type`, `master_partner_service_ids`) and its Redis-cached lookup, which classifies a request as no-bill vs bill-bearing.
- **D-002**: The existing sequential customer-number generator used by dynamic VA types.
- **D-003**: The existing merchant notification/callback pipeline, including the expiry-callback behavior from feature `007-merchant-expiry-callback`.
- **D-004**: The existing idempotency enforcement on vendor-facing endpoints.

## Out of Scope

- Changing the vendor-facing request/response schemas.
- Changing the behavior of variable-bill or fixed-bill VA types.
- Adding balance tracking, ledgers, or settlement/payout of accumulated no-bill payments.
- Adding a per-payment or per-VA maximum amount or velocity limit.
- Changes to authentication, signature, or token handling.
