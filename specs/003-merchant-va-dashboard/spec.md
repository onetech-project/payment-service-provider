# Feature Specification: Merchant VA Dashboard

**Feature Branch**: `003-merchant-va-dashboard`

**Created**: 2026-07-22

**Status**: Draft

**Input**: User description: "Endpoint untuk create VA transactions untuk dashboard internal merchant, inquiry untuk list transaksi, dan notifikasi ketika status transaksi sudah dibayar. Format endpoint, request, dan response sesuai SNAP VA API."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create VA Transaction (Priority: P1)

As a merchant admin, I want to create Virtual Account transactions through the internal dashboard so that customers can pay their bills via VA.

**Why this priority**: Creating VA transactions is the core function that enables the entire payment flow. Without this, merchants cannot generate VA numbers for customers.

**Independent Test**: Can be fully tested by sending a create VA request with valid customer and bill details, and verifying the VA number is generated and stored in the database.

**Acceptance Scenarios**:

1. **Given** a merchant admin enters customer and bill details, **When** the system receives a create VA request, **Then** the system generates a VA number, stores the transaction, and returns success with VA details.
2. **Given** a merchant admin enters duplicate customer number, **When** the system processes the request, **Then** the system returns the existing VA transaction without creating a duplicate.
3. **Given** a merchant admin enters invalid bill details, **When** the system validates the request, **Then** the system returns an appropriate error message.

---

### User Story 2 - Inquiry VA Transactions (Priority: P2)

As a merchant admin, I want to view a list of VA transactions in the dashboard so that I can monitor payment status and track transactions.

**Why this priority**: Merchants need visibility into their VA transactions to reconcile payments and handle customer inquiries.

**Independent Test**: Can be fully tested by querying transactions with filters (date range, status, VA number) and verifying the list returns correct results.

**Acceptance Scenarios**:

1. **Given** a merchant admin requests a list of transactions, **When** the system processes the request, **Then** the system returns a paginated list of VA transactions with status.
2. **Given** a merchant admin filters by date range, **When** the system processes the request, **Then** the system returns only transactions within the specified period.
3. **Given** a merchant admin filters by payment status, **When** the system processes the request, **Then** the system returns only transactions matching the status.

---

### User Story 3 - Payment Notification (Priority: P3)

As a merchant admin, I want to receive notifications when a VA transaction is paid so that I can update the customer's billing status.

**Why this priority**: Real-time payment notifications enable merchants to process orders, deliver services, and reconcile accounts immediately.

**Independent Test**: Can be fully tested by simulating a payment notification and verifying the merchant receives the notification with correct transaction details.

**Acceptance Scenarios**:

1. **Given** a customer makes a payment at the bank, **When** the system receives the payment notification, **Then** the system updates the transaction status and notifies the merchant.
2. **Given** a payment notification is received for a transaction, **When** the system processes the notification, **Then** the transaction status changes from "pending" to "paid".
3. **Given** multiple payments for the same VA number, **When** the system processes the notifications, **Then** each payment is recorded separately with correct amounts.

---

### Edge Cases

- What happens when a VA number is expired before payment?
- How does the system handle partial payments for multi-bill transactions?
- What happens when the same VA number receives concurrent payment notifications?
- How does the system handle network timeouts when calling external bank APIs?
- What happens when a merchant tries to create a VA with an amount that exceeds the maximum allowed?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose an endpoint to create VA transactions following SNAP VA Create VA API format.
- **FR-002**: System MUST expose an endpoint to query VA transactions following SNAP VA Inquiry VA API format.
- **FR-003**: System MUST receive and process payment notifications from banks following SNAP VA Payment API format.
- **FR-004**: System MUST store VA transactions in the database with all required fields.
- **FR-005**: System MUST generate unique VA numbers following the SNAP format (partnerServiceId + customerNo).
- **FR-006**: System MUST support both single bill and multi-bill transactions.
- **FR-007**: System MUST notify merchants when payments are received via configurable notification channels.
- **FR-008**: System MUST validate all SNAP headers (X-TIMESTAMP, X-CLIENT-KEY, X-SIGNATURE) on incoming requests.
- **FR-009**: System MUST enforce idempotency using X-EXTERNAL-ID header.
- **FR-010**: System MUST return SNAP-compliant response codes for all success and error scenarios.

### Key Entities

- **VA Transaction**: Represents a Virtual Account payment request with customer details, bill information, and payment status.
- **VA Bill Detail**: Individual bill items within a VA transaction, supporting multi-bill scenarios.
- **Payment Notification**: Record of a payment received for a VA transaction.
- **Merchant Notification**: Notification sent to merchant when payment is received.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Create VA requests are processed and responded to within 3 seconds on average.
- **SC-002**: Transaction list queries return results within 2 seconds for up to 10,000 records.
- **SC-003**: Payment notifications are delivered to merchants within 5 seconds of receipt.
- **SC-004**: System handles 500 concurrent create VA requests without degradation.
- **SC-005**: 100% of SNAP API responses follow the standard response code format.

## Assumptions

- The existing VA transaction and bill detail tables will be used to store merchant-created transactions.
- Merchant authentication will use the existing SNAP B2B token system.
- Payment notifications will be received from banks via webhooks.
- The notification system will support configurable channels (webhook, email, SMS) per merchant.
- VA numbers follow the SNAP standard format: partnerServiceId (8 digits) + customerNo (up to 20 digits).
- Multi-bill transactions support up to 24 bill items per transaction.
