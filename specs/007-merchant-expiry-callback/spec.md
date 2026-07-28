# Feature Specification: Merchant Callback on Transaction Expiry (with Resend Endpoint)

**Feature Branch**: `007-merchant-expiry-callback`

**Created**: 2026-07-28

**Status**: Draft

**Input**: User description: "perlu callback ke merchant ketika transaksi expired" + "dan perlu ada endpoint untuk resend callback" (merged from feature 008 per user request on 2026-07-28)

## Clarifications

### Session 2026-07-28

- Q: Should the "resend callback" capability be merged into this feature instead of tracked separately (previously spec 008-resend-callback-endpoint)? → A: Yes, merge into this single feature as a second user story; 008 is retired.
- Q: How should the system detect expiry — via a periodic background job (original assumption), or via checks performed at inquiry/payment-notification time? → A: Event-driven checks: expiry is evaluated inline when the SNAP inquiry endpoint or the payment-notification (vendor notify) endpoint is called for that VA, rather than a separate periodic scan. Each check compares against the current time and, if expired, updates the VA's status and triggers the merchant callback as a side effect of that request.
- Q: What is the correct direction for the expiry comparison? → A: A transaction is expired when the current time is past the expiry timestamp, i.e. `current_date > expired_date` (equivalently `expired_date < current_date`). The condition `expired_date > current_date` means the VA is still active/not yet expired.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Merchant is notified when a Virtual Account transaction expires (Priority: P1)

A merchant integrates a payment channel that generates a Virtual Account (VA) with an expiry time. If the customer does not pay before the VA expires, the merchant currently has no automatic way of knowing the transaction is no longer payable. The merchant needs to receive a callback notification when the transaction/VA expires, so it can update its own order status (e.g., mark an order as "expired"/"cancelled") without having to repeatedly poll for VA status.

**Why this priority**: Without this notification, merchants must poll or wait indefinitely, leading to stale orders, poor customer experience, and support tickets. This is the core, primary capability of the feature.

**Independent Test**: Create a VA with a short expiry window, let it pass without payment, and verify the merchant's registered callback URL receives an "expired" event with the correct transaction details within the expected delay.

**Acceptance Scenarios**:

1. **Given** an unpaid VA whose expiry time has passed, **When** the customer/vendor performs a bill inquiry on that VA, **Then** the system rejects the inquiry as expired (`responseCode: 4042419`, `responseMessage: "Invalid Bill/Virtual Account"`, `inquiryStatus: 01`, `inquiryReason.english: "expired transaction"`, `inquiryReason.indonesia: "transaksi kadaluarsa"`), marks the VA as expired in the database, and triggers the merchant expiry callback.
2. **Given** an unpaid VA whose expiry time has passed, **When** the vendor sends a payment notification for that VA, **Then** the system rejects the payment as expired (`responseCode: 4042519`, `responseMessage: "Invalid Bill/Virtual Account"`, `paymentFlagStatus: 01`, `paymentFlagReason.english: "expired transaction"`, `paymentFlagReason.indonesia: "transaksi kadaluarsa"`), marks the VA as expired in the database, and triggers the merchant expiry callback.
3. **Given** a VA that is paid before its expiry time, **When** the expiry time passes, **Then** no "expired" callback is sent for that VA, and subsequent inquiry/notify calls are handled per their normal (non-expiry) status.
4. **Given** a VA that has already been marked as expired and notified (via a prior inquiry or notify call), **When** a later inquiry or notify call is made for the same VA, **Then** the system returns the same expired response but does not send a duplicate "expired" callback.
5. **Given** the merchant's callback endpoint is unreachable or returns an error when the expiry notification is sent, **When** the delivery fails, **Then** the system retries delivery a bounded number of times following the existing retry behavior used for other merchant callbacks, without blocking or delaying the inquiry/notify response to the caller.

---

### User Story 2 - Operator manually resends a failed or missed merchant callback (Priority: P2)

An internal operator (support/ops staff) finds that a merchant did not receive a callback notification for a transaction event (e.g., payment received or transaction expired) — for example because the merchant's endpoint was down during the original delivery attempts, or the merchant asks for the notification to be resent. The operator needs a way to trigger re-delivery of that specific callback for a given transaction without waiting for or relying on the automatic retry mechanism.

**Why this priority**: This depends on callback delivery (User Story 1 and the existing payment-received callback) already existing, and is an operational safety net rather than the core notification behavior — hence P2.

**Independent Test**: Pick a transaction that already has a recorded callback/notification, call the resend endpoint for it, and verify the merchant's registered notification URL receives the same event payload again and the attempt is recorded.

**Acceptance Scenarios**:

1. **Given** a transaction that previously had a callback sent (successfully or after exhausting retries), **When** the resend endpoint is called for that transaction, **Then** the system sends the callback again to the merchant's registered notification URL with the same event data.
2. **Given** a transaction that has never had a callback generated for it, **When** the resend endpoint is called, **Then** the system returns a clear error indicating there is no callback to resend for that transaction.
3. **Given** a transaction that does not exist, **When** the resend endpoint is called with its identifier, **Then** the system returns a "not found" error.
4. **Given** a transaction with no merchant notification URL registered, **When** the resend endpoint is called, **Then** the system returns a clear error indicating there is no destination to deliver to.
5. **Given** a successful resend request, **When** the callback is redelivered, **Then** the system records the new delivery attempt (timestamp, outcome) separately from the original attempt history.
6. **Given** an unauthenticated or unauthorized caller, **When** the resend endpoint is called, **Then** the system rejects the request without sending any callback.

### Edge Cases

- What happens when a VA has no registered notification URL? System should skip sending a callback but still record the VA as expired internally.
- What happens when a payment arrives at (or near) the exact moment the VA expires (race condition)? The system must not send both a "payment received" callback and an "expired" callback for the same transaction — payment takes precedence if it was accepted before expiry.
- What happens when the merchant callback endpoint is permanently unreachable? After the existing retry limit is exhausted, the delivery is marked as failed and the expiry status change itself is not rolled back.
- What happens to VAs that expired before this feature was deployed? They are out of scope — only VAs expiring after the feature is active are eligible for the expiry callback.
- What happens if the resend endpoint is called multiple times in quick succession for the same transaction? Each call triggers its own delivery attempt; the system does not silently dedupe rapid repeated resend requests, since resending is an explicit operator action.
- What happens if the merchant's endpoint is unreachable during a manual resend? The attempt is recorded as failed; the existing automatic retry behavior is not implicitly restarted by a manual resend unless the operator resends again.
- What happens if the transaction's callback event type has changed since the original delivery (e.g., a transaction that was pending is now expired)? The resend delivers the most recent/current event and status for that transaction, not a stale snapshot, unless a specific historical delivery is being resent.
- Who is allowed to call the resend endpoint? Restricted to authenticated internal/operator users, not merchants or public callers.

## Requirements *(mandatory)*

### Functional Requirements

**Expiry detection & callback (User Story 1)**

- **FR-001**: System MUST evaluate whether a VA/transaction is expired at the moment it is accessed via the bill inquiry endpoint or the payment-notification (vendor notify) endpoint, by comparing the current time against the VA's expiry timestamp — a VA is expired when the current time is later than the expiry timestamp (`current_date > expired_date`). No separate periodic/background scan is required.
- **FR-001a**: On bill inquiry, when the VA is unpaid and expired, System MUST reject the inquiry with `responseCode: 4042419`, `responseMessage: "Invalid Bill/Virtual Account"`, `inquiryStatus: 01`, `inquiryReason.english: "expired transaction"`, `inquiryReason.indonesia: "transaksi kadaluarsa"`.
- **FR-001b**: On payment notification (vendor notify), when the VA is unpaid and expired, System MUST reject the payment with `responseCode: 4042519`, `responseMessage: "Invalid Bill/Virtual Account"`, `paymentFlagStatus: 01`, `paymentFlagReason.english: "expired transaction"`, `paymentFlagReason.indonesia: "transaksi kadaluarsa"`.
- **FR-002**: System MUST send exactly one callback notification per expired transaction to the merchant's registered notification URL, triggered as a side effect of the first inquiry or notify call (whichever occurs first) that detects the expiry.
- **FR-003**: The expiry callback payload MUST identify the event as an expiry event and include the transaction/VA identifying details (virtual account number, customer number, trx ID/payment request ID, reference number, and the expiry timestamp).
- **FR-004**: System MUST NOT send an expiry callback for a transaction that has already been successfully paid.
- **FR-005**: System MUST NOT send more than one expiry callback for the same transaction, even if inquiry and/or notify calls are made multiple times after the VA is already marked expired.
- **FR-006**: System MUST mark an expired, unpaid transaction with an "expired" status in the database at the moment expiry is first detected (via inquiry or notify), before or together with triggering the merchant callback.
- **FR-007**: System MUST retry callback delivery on failure, consistent with the retry behavior of other merchant callbacks in the system.
- **FR-008**: System MUST sign/authenticate the expiry callback request in the same manner as existing merchant callback notifications, so merchants can verify authenticity.
- **FR-009**: System MUST skip sending a callback (but still mark the transaction expired internally) when the transaction has no registered merchant notification URL.
- **FR-010**: System MUST resolve the race between a near-simultaneous payment and expiry in favor of the payment when the payment was accepted before the expiry time.
- **FR-010a**: Sending the expiry callback and updating the VA status MUST NOT block or add unacceptable latency to the inquiry/notify response returned to the caller.

**Manual resend endpoint (User Story 2)**

- **FR-011**: System MUST provide an endpoint that allows an authorized operator to request re-delivery of a merchant callback for a specific transaction.
- **FR-012**: System MUST identify the target transaction for the resend request using its existing transaction/VA identifier(s).
- **FR-013**: System MUST reject resend requests from callers who are not authenticated and authorized as internal operators.
- **FR-014**: System MUST return a "not found" response when the resend request references a transaction that does not exist.
- **FR-015**: System MUST return a clear error when the resend request references a transaction that has no prior callback event to resend.
- **FR-016**: System MUST return a clear error when the transaction has no merchant notification URL registered.
- **FR-017**: System MUST redeliver the callback using the same delivery, signing, and payload format as the original automatic callback for that event type (expiry or payment-received).
- **FR-018**: System MUST record each manual resend attempt (including outcome: success or failure) distinctly from prior delivery attempts, for audit purposes.
- **FR-019**: System MUST NOT alter the underlying transaction status or business state as a side effect of resending a callback.

### Key Entities

- **Transaction / Virtual Account (VA)**: An existing entity representing a payment request with an expiry time and a status (e.g., active, paid, expired). This feature adds transitions into and reporting of the "expired" status, and is also the entity referenced when requesting a manual resend.
- **Merchant Callback / Notification**: An existing outbound notification sent to a merchant's registered URL when transaction events occur (e.g., payment received). This feature adds a new event type for expiry and a manual resend trigger for any event type.
- **Callback / Notification Delivery Attempt**: An existing or extended record of an attempt to deliver an event to a merchant's notification URL, now including manually-triggered resend attempts alongside automatic ones.
- **Operator**: An authenticated internal user permitted to trigger administrative actions such as resending callbacks.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of unpaid transactions that pass their expiry time receive exactly one expiry callback attempt (excluding those without a registered notification URL).
- **SC-002**: For a VA that is inquired or has a payment notification attempted against it after its expiry time, the merchant is notified within 5 minutes of that inquiry/notify call being made (expiry detection is on-access, not on a fixed schedule).
- **SC-003**: No transaction ever receives both a "payment received" and an "expired" callback.
- **SC-004**: No duplicate expiry callbacks are sent for the same transaction across repeated detection cycles.
- **SC-005**: An authorized operator can trigger a callback resend for a known transaction in a single request.
- **SC-006**: 100% of resend requests for transactions without a valid destination or prior callback return a clear, actionable error instead of a silent no-op.
- **SC-007**: Every resend attempt is retrievable afterward as a distinct record from the transaction's delivery history.
- **SC-008**: Unauthorized resend attempts are rejected 100% of the time with no callback sent.

## Assumptions

- "Transaction expired" refers to a Virtual Account (VA) whose current time is later than its existing `ExpiredDate` (`current_date > expired_date`) without being paid, consistent with the current VA domain model.
- The merchant notification URL used for the expiry callback is the same one already registered for payment-received callbacks.
- Expiry is detected lazily/on-access, at the moment a bill inquiry or payment-notification (vendor notify) call is made for the VA — there is no periodic background scan. A VA that is never inquired or notified against after expiring will not trigger a callback until the next such call occurs. The 5-minute notification-delay target in SC-002 applies to VAs that are actively inquired/notified against near their expiry time; VAs with no further activity are notified upon next access rather than on a fixed schedule.
- The expiry callback reuses the existing merchant callback delivery, signing, and retry infrastructure rather than introducing a new mechanism.
- Historical VAs that expired before this feature ships are out of scope and will not be retroactively notified.
- The resend endpoint is an internal/administrative capability (used by ops/support tooling), not exposed to merchants directly, since merchants already receive automatic callbacks and retries.
- "Resend callback" means redelivering the most recent relevant event for a transaction (e.g., payment received or expired) rather than replaying an arbitrary historical delivery by ID; if per-attempt replay is needed later, that can be a follow-up refinement.
- The resend endpoint reuses the same callback delivery, signing, and retry-recording infrastructure as the automatic notification flow.
- Authentication/authorization for the resend endpoint reuses whatever internal auth mechanism already governs other internal/admin endpoints in the system.
- Resending does not change transaction state (e.g., does not un-expire or re-mark a transaction); it only re-triggers notification delivery.
