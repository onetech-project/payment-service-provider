# Feature Specification: BCA Virtual Account Integration

**Feature Branch**: `002-bca-virtual-account`

**Created**: 2026-07-22

**Status**: Draft

**Input**: User description: "Integrasi ke layanan API BCA untuk layanan transaksi Virtual Account untuk biller. Konfigurasi environment, payload, dan response disimpan dalam file `.env.bca.va`"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - VA Bill Inquiry (Priority: P1)

As a biller system, I want to receive and process Virtual Account inquiry requests from BCA so that customers can view their bill details before making a payment.

**Why this priority**: This is the first step in the payment flow. Without inquiry, customers cannot see their bills or proceed to payment. This enables the core value proposition of the integration.

**Independent Test**: Can be fully tested by sending a mock inquiry request with valid VA number and verifying the response contains correct bill details, amounts, and customer information.

**Acceptance Scenarios**:

1. **Given** a customer enters a valid Virtual Account number at BCA channel, **When** the system receives an inquiry request from BCA, **Then** the system responds with bill details including customer name, total amount, and bill breakdown.
2. **Given** a customer enters an invalid or expired Virtual Account number, **When** the system receives an inquiry request, **Then** the system responds with appropriate error code and message indicating the VA is invalid or expired.
3. **Given** a customer's bill has already been paid, **When** the system receives an inquiry request, **Then** the system responds with "Paid Bill" status.

---

### User Story 2 - VA Payment Notification (Priority: P2)

As a biller system, I want to receive and process payment notifications from BCA so that customer payments are recorded and bills are marked as paid.

**Why this priority**: Payment notification is the critical step that completes the transaction. Without this, the biller cannot reconcile payments.

**Independent Test**: Can be fully tested by sending a mock payment notification with valid transaction data and verifying the system acknowledges the payment correctly.

**Acceptance Scenarios**:

1. **Given** a customer completes payment at BCA channel, **When** the system receives a payment notification, **Then** the system responds with success acknowledgment and records the transaction.
2. **Given** a payment notification is received for an already-paid bill, **When** the system processes the notification, **Then** the system responds with appropriate status and does not duplicate the payment record.
3. **Given** a payment notification contains mismatched amounts, **When** the system validates the notification, **Then** the system responds with rejection and logs the discrepancy.

---

### User Story 3 - VA Payment Status Inquiry (Priority: P3)

As a biller system, I want to check the payment status of a Virtual Account transaction so that reconciliation can be performed.

**Why this priority**: Status inquiry supports reconciliation and dispute resolution. It's important but not critical for the initial payment flow.

**Independent Test**: Can be fully tested by sending a mock status inquiry request and verifying the response contains the correct payment flag status.

**Acceptance Scenarios**:

1. **Given** a payment has been completed, **When** the system receives a status inquiry, **Then** the system responds with payment flag status "00" (success).
2. **Given** a payment is still pending, **When** the system receives a status inquiry, **Then** the system responds with payment flag status "03" (pending).
3. **Given** a payment was rejected by the partner, **When** the system receives a status inquiry, **Then** the system responds with payment flag status "01" (reject).

---

### Edge Cases

- What happens when BCA sends an inquiry with an expired timestamp (X-TIMESTAMP older than allowed threshold)?
- How does the system handle concurrent inquiries for the same VA number?
- What happens when the payment notification arrives but the inquiry data is not in the system?
- How does the system handle malformed JSON payloads from BCA?
- What happens when the idempotency key (X-EXTERNAL-ID) is duplicated within the same day?
- How does the system handle network timeouts when calling BCA APIs?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose endpoints to receive BCA SNAP VA inquiry, payment notification, and status inquiry requests.
- **FR-002**: System MUST validate BCA SNAP authentication headers (X-TIMESTAMP, X-CLIENT-KEY, X-SIGNATURE) on all incoming requests.
- **FR-003**: System MUST store BCA API configuration (credentials, endpoints, channel IDs) in environment-specific `.env.bca.va` files.
- **FR-004**: System MUST generate HMAC-SHA256 signatures for outbound requests to BCA APIs.
- **FR-005**: System MUST handle BCA response codes and map them to appropriate internal status values.
- **FR-006**: System MUST enforce idempotency using X-EXTERNAL-ID header for duplicate request prevention.
- **FR-007**: System MUST support both single settlement and multi-bills/multi-settlement transaction types.
- **FR-008**: System MUST log all inbound and outbound BCA API calls with correlation IDs for traceability.
- **FR-009**: System MUST validate virtual account numbers against registered partnerServiceId and customerNo formats.
- **FR-010**: System MUST respond to BCA within the required timeout window (typically 30 seconds).

### Key Entities

- **Virtual Account Inquiry Request**: Contains partnerServiceId, customerNo, virtualAccountNo, channelCode, and inquiryRequestId from BCA.
- **Virtual Account Inquiry Response**: Contains virtualAccountData with customer name, total amount, bill details, and inquiry status.
- **Payment Notification Request**: Contains payment details including paid amount, bill details, and transaction date from BCA.
- **Payment Notification Response**: Contains payment flag status (00=success, 01=reject, 02=timeout, 03=pending) and acknowledgment.
- **Payment Status Inquiry Request**: Contains inquiry details for checking payment flag status of a previous transaction.
- **Payment Status Inquiry Response**: Contains payment flag status and transaction details.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: VA inquiry requests are processed and responded to within 5 seconds on average.
- **SC-002**: Payment notifications are acknowledged within 10 seconds with 99.9% success rate.
- **SC-003**: System handles 100 concurrent VA inquiry requests without degradation.
- **SC-004**: All BCA API integrations can be configured via environment files without code changes.
- **SC-005**: 100% of inbound BCA requests are logged with correlation IDs for audit trail.

## Assumptions

- BCA API credentials (client_id, client_secret, API key, API secret) will be provided by the BCA partnership team.
- The existing SNAP token management system (feature 001) will be reused for B2B access token generation.
- BCA sandbox environment will be used for development and testing before production deployment.
- The biller system already has a database to store VA master data and transaction records.
- BCA will provide the CHANNEL-ID (95231) and partner company code for integration.
- The system will use SNAP authentication (RSA asymmetric signature) for access token requests and HMAC-SHA256 for API calls.
- Multi-bills transactions with multiple settlements are supported by default.
- Currency is primarily IDR, with potential support for USD and SGD.
