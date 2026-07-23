# Quickstart Validation: Merchant VA Dashboard

**Date**: 2026-07-23 | **Feature**: 003-merchant-va-dashboard

## Prerequisites

- Go 1.26.5+
- PostgreSQL running locally (port 5432)
- Redis running locally (port 6379)
- Environment variables set: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `REDIS_ADDR`

## Setup

```bash
# 1. Run migrations
psql -U postgres -d payment_gateway -f db/migrations/000003_create_va_transactions.up.sql
psql -U postgres -d payment_gateway -f db/migrations/000004_add_va_expiry_and_notification.up.sql

# 2. Build and run
go build -o server ./cmd/api && ./server

# 3. Verify health
curl http://localhost:8080/health
# Expected: {"service":"payment-integration-gateway","status":"UP"}
```

## Validation Scenarios

### Scenario 1: Create VA Transaction (SNAP Service Code 27)

```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/create-va \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-23T10:00:00+07:00" \
  -H "X-CLIENT-KEY: test-client-key" \
  -H "X-SIGNATURE: <generated-signature>" \
  -H "X-PARTNER-ID: 82150823919040624621823174737537" \
  -H "X-EXTERNAL-ID: 41807553358950093184162180797837" \
  -H "CHANNEL-ID: 95221" \
  -H "Idempotency-Key: create-va-001" \
  -d '{
    "partnerServiceId": "088899",
    "customerNo": "12345678901234567890",
    "virtualAccountName": "Jokul Doe",
    "virtualAccountEmail": "jokul@email.com",
    "virtualAccountPhone": "6281828384858",
    "trxId": "abcdefgh1234",
    "totalAmount": {"value": "150000.00", "currency": "IDR"},
    "billDetails": [{
      "billCode": "01",
      "billNo": "INV-2026-001",
      "billName": "Invoice Januari",
      "billShortName": "Inv Jan",
      "billDescription": {"english": "Product Purchase", "indonesia": "Pembelian Produk"},
      "billSubCompany": "00001",
      "billAmount": {"value": "150000.00", "currency": "IDR"}
    }],
    "virtualAccountTrxType": "C",
    "expiredDate": "2026-07-30T23:59:59+07:00",
    "notificationUrl": "https://merchant.example.com/webhook/va-notification"
  }'
```

**Expected**: `200 OK` with `responseCode: "2002700"`, `virtualAccountNo` = `"08889912345678901234567890"`

### Scenario 2: List VA Transactions

```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/list \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-23T10:00:00+07:00" \
  -H "X-CLIENT-KEY: test-client-key" \
  -H "X-SIGNATURE: <generated-signature>" \
  -H "Idempotency-Key: list-va-001" \
  -d '{
    "partnerServiceId": "088899",
    "page": 1,
    "pageSize": 20
  }'
```

**Expected**: `200 OK` with paginated list of VA transactions (including `expiredDate` field)

### Scenario 3: Delete VA Transaction (SNAP Service Code 31)

```bash
curl -X DELETE http://localhost:8080/v1.0/transfer-va/delete-va \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-23T10:00:00+07:00" \
  -H "X-CLIENT-KEY: test-client-key" \
  -H "X-SIGNATURE: <generated-signature>" \
  -H "X-PARTNER-ID: 82150823919040624621823174737537" \
  -H "X-EXTERNAL-ID: 41807553358950093184162180797837" \
  -H "CHANNEL-ID: 95221" \
  -H "Idempotency-Key: delete-va-001" \
  -d '{
    "partnerServiceId": "088899",
    "customerNo": "12345678901234567890",
    "virtualAccountNo": "08889912345678901234567890",
    "trxId": "abcdefgh1234"
  }'
```

**Expected**: `200 OK` with `responseCode: "2003100"`. Verify status changed to "04" (deleted) in database.

### Scenario 4: Receive Payment Notification (SNAP Service Code 25)

```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/payment \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: payment-001" \
  -d '{
    "partnerServiceId": "088899",
    "customerNo": "12345678901234567890",
    "virtualAccountNo": "08889912345678901234567890",
    "trxId": "abcdefgh1234",
    "paymentRequestId": "PAY-20260723001",
    "paidAmount": {"value": "150000.00", "currency": "IDR"},
    "totalAmount": {"value": "150000.00", "currency": "IDR"},
    "transactionDate": "2026-07-23T14:30:00+07:00"
  }'
```

**Expected**: `200 OK` with `responseCode: "2002500"`. Asynq worker sends webhook POST to `notificationUrl`.

### Scenario 5: Verify Idempotency

```bash
# Send same create request again with same Idempotency-Key
curl -X POST http://localhost:8080/v1.0/transfer-va/create-va \
  -H "Idempotency-Key: create-va-001" \
  ... (same request body)
```

**Expected**: `200 OK` with same response (cached replay), `X-Cache-Replay: true` header

### Scenario 6: Verify Multi-Bill

```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/create-va \
  -H "Idempotency-Key: create-va-002" \
  -d '{
    "partnerServiceId": "088899",
    "customerNo": "999999999999999999",
    "virtualAccountName": "Jane Doe",
    "totalAmount": {"value": "300000.00", "currency": "IDR"},
    "billDetails": [
      {"billNo": "INV-001", "billAmount": {"value": "150000.00", "currency": "IDR"}},
      {"billNo": "INV-002", "billAmount": {"value": "150000.00", "currency": "IDR"}}
    ],
    "virtualAccountTrxType": "C",
    "expiredDate": "2026-07-30T23:59:59+07:00",
    "notificationUrl": "https://merchant.example.com/webhook/va-notification"
  }'
```

**Expected**: `200 OK` with both bills stored in `va_bill_details`

### Scenario 7: Verify Expiry Enforcement

```bash
# Try to pay an expired VA
curl -X POST http://localhost:8080/v1.0/transfer-va/payment \
  -H "Idempotency-Key: payment-expired-001" \
  -d '{
    "partnerServiceId": "088899",
    "customerNo": "<expired-va-customer-no>",
    "virtualAccountNo": "<expired-va-number>",
    "trxId": "<expired-trx-id>",
    "paymentRequestId": "PAY-EXPIRED-001",
    "paidAmount": {"value": "100000.00", "currency": "IDR"},
    "totalAmount": {"value": "100000.00", "currency": "IDR"},
    "transactionDate": "2026-07-23T14:30:00+07:00"
  }'
```

**Expected**: Error response indicating VA is expired (SNAP 404/19)

## Test Commands

```bash
# Run all tests
go test -race -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

# Run specific test suites
go test -v ./internal/usecase/ -run TestMerchantVA
go test -v ./internal/adapter/delivery/http/handler/ -run TestMerchantVA
go test -v ./internal/infrastructure/database/ -run TestVARepository

# Lint check
golangci-lint run
```

## Key Validation Points

| Check | How to Verify |
|-------|---------------|
| VA number format | `virtualAccountNo` = `partnerServiceId(8) + customerNo(up to 20)` |
| SNAP response codes | Create: `2002700`, Delete: `2003100`, Payment: `2002500` |
| Delete method | HTTP DELETE on `/v1.0/transfer-va/delete-va` |
| Multi-bill support | Query `va_bill_details` for transaction with multiple bills |
| Idempotency | Same `Idempotency-Key` + same payload → cached response |
| Payment status update | After payment, `va_transactions.status` changes from `03` to `00` |
| Delete status update | After delete, `va_transactions.status` changes from `03` to `04` |
| Expiry enforcement | Expired VA (past `expiredDate`) rejects payment, status `02` |
| Merchant webhook | Asynq worker POSTs to `notificationUrl` after payment received |
| Pagination | List endpoint returns correct `totalRows` and `totalPages` |
| Context propagation | OTel traces span from HTTP handler through usecase to repository |
