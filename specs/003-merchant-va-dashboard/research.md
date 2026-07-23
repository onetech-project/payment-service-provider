# Research: Merchant VA Dashboard

**Date**: 2026-07-23 | **Feature**: 003-merchant-va-dashboard

## R1: SNAP VA Create VA API Format (Merchant-Side)

**Decision**: Use the existing SNAP VA domain types as the canonical format. The merchant create VA endpoint will accept a request body matching `domain.VAInquiryRequest` extended with merchant-specific fields (bill details, amount, expiry).

**Rationale**: The existing `domain/va.go` already defines SNAP-compliant types for VA inquiry, payment, and status. The merchant dashboard creates VA transactions from the internal side, so the request/response format mirrors the SNAP standard but originates from the merchant rather than a vendor/bank. The existing `VAAccountData`, `BillDetail`, and `Amount` value objects are reused.

**Alternatives considered**:
- Separate merchant-specific DTOs with mapping layer — rejected for YAGNI; the SNAP types already capture the required fields.
- GraphQL or REST-only internal API — rejected; spec requires SNAP VA API format compliance.

## R2: Asynq Integration for Background Processing

**Decision**: Add `github.com/hibiken/asynq` as a dependency and create a thin wrapper in `infrastructure/queue/asynq.go`. Workers register in `adapter/delivery/worker/`.

**Rationale**: The constitution (Principle IX) requires non-blocking operations (notifications, retries) to use Asynq. The existing Redis client (`infrastructure/redis/redis.go`) can serve as the Asynq broker. The Asynq client wraps the same Redis connection. Tasks are enqueued after payment notification receipt and processed asynchronously to avoid blocking the webhook response.

**Alternatives considered**:
- Channel-based goroutine workers — rejected; no persistence, retry, or monitoring (violates Principle IX).
- Asynqmon dashboard — will be available once Asynq is integrated; no separate setup needed.

## R3: Payment Notification Flow

**Decision**: Banks send payment notifications to the existing `/transfer-va/payment` endpoint. The payment usecase processes the payment, then enqueues an Asynq task to notify the merchant via configured channels (webhook, email, SMS).

**Rationale**: The spec requires receiving payment notifications from banks (FR-003) and notifying merchants (FR-007). The existing `VAPayment` handler receives the bank webhook. After successful payment recording, an Asynq task is enqueued to deliver the notification. This decouples the bank-facing response from merchant notification delivery.

**Flow**:
1. Bank → POST `/v1.0/{vendor}/{channel}/transfer-va/payment` (existing endpoint)
2. `VAUsecase.Payment()` records payment, returns SNAP response to bank
3. On success, enqueue `merchant:payment:notify` Asynq task with payment details
4. Worker picks up task, queries merchant notification config, delivers via configured channel
5. Delivery result logged; retry on failure per Asynq retry policy

**Alternatives considered**:
- Synchronous notification in the payment handler — rejected; blocks bank response, violates ≤5s notification SLA if notification is slow.
- Separate notification endpoint polled by merchants — rejected; spec requires push notifications.

## R4: VA Number Generation (Merchant Create)

**Decision**: Generate VA number as `partnerServiceId (8 digits) + customerNo (up to 20 digits)`, per SNAP standard. The merchant provides both values; the system concatenates and validates uniqueness.

**Rationale**: FR-005 requires SNAP-format VA numbers. The `partnerServiceId` identifies the merchant's service, and `customerNo` identifies the customer. The combined VA number is unique per the existing `virtual_account_no` column (VARCHAR(28)).

**Alternatives considered**:
- Auto-generated sequential numbers — rejected; violates SNAP format requirement.
- UUID-based — rejected; not SNAP-compliant.

## R5: Merchant Notification Configuration

**Decision**: Merchant notification preferences stored in a new `merchant_notifications` table with channel type (webhook URL, email, phone) and enabled flag. Configurable per merchant.

**Rationale**: FR-007 requires configurable notification channels per merchant. The table maps merchant ID to notification targets. The Asynq worker reads this config when processing notification tasks.

**Alternatives considered**:
- Environment variable configuration — rejected; not per-merchant configurable.
- Vendor config YAML files — rejected; notification config is per-merchant, not per-vendor.

## R6: Database Schema for Notifications

**Decision**: New migration `000004_add_merchant_notifications.up.sql` creates `merchant_notifications` table. No changes to existing `va_transactions` or `va_bill_details` tables.

**Rationale**: The existing VA tables already store all transaction data needed for the merchant dashboard. The only new table is for notification channel configuration. The `va_bill_details` table already supports multi-bill scenarios (FR-006).

**Schema**:
```sql
CREATE TABLE IF NOT EXISTS merchant_notifications (
    id VARCHAR(36) PRIMARY KEY,
    merchant_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(20) NOT NULL,  -- 'webhook', 'email', 'sms'
    channel_config JSONB NOT NULL,       -- {"url": "...", "email": "...", "phone": "..."}
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_merchant_notifications_merchant ON merchant_notifications(merchant_id);
```
