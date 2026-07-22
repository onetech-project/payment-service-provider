# Quickstart: BCA Virtual Account Integration

**Feature**: 002-bca-virtual-account
**Date**: 2026-07-22

## Prerequisites

1. Go 1.26.5+ installed
2. Docker & Docker Compose running
3. BCA sandbox credentials (from partnership team)
4. PostgreSQL and Redis running (via Docker Compose)

## Setup

### 1. Start Infrastructure

```bash
docker compose up postgres redis -d
```

### 2. Configure BCA Credentials

Edit `.env.bca.va` with your BCA sandbox credentials:

```bash
BCA_VA_CLIENT_ID=your_client_id
BCA_VA_CLIENT_SECRET=your_client_secret
BCA_VA_PRIVATE_KEY_PATH=./certs/private.pem
BCA_VA_PUBLIC_KEY_PATH=./certs/public.pem
BCA_VA_PARTNER_ID=your_partner_id
```

### 3. Run Database Migrations

```bash
go run ./cmd/api migrate
```

### 4. Start the Server

```bash
go run ./cmd/api
```

Server starts on `http://localhost:8080`

## Validation Scenarios

### Scenario 1: VA Inquiry (Bill Presentment)

**Request**:
```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/inquiry \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-22T10:00:00+07:00" \
  -H "X-CLIENT-KEY: test_client" \
  -H "X-SIGNATURE: test_signature" \
  -H "CHANNEL-ID: 95231" \
  -H "X-PARTNER-ID: 12345" \
  -H "X-EXTERNAL-ID: 202607221000001" \
  -H "Idempotency-Key: 202607221000001" \
  -d '{
    "partnerServiceId": " 12345",
    "customerNo": "123456789012345678",
    "virtualAccountNo": " 12345123456789012345678",
    "channelCode": 6011,
    "inquiryRequestId": "202607221000001234500001"
  }'
```

**Expected Response**:
```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "inquiryStatus": "00",
    "inquiryReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "partnerServiceId": " 12345",
    "customerNo": "123456789012345678",
    "virtualAccountNo": " 12345123456789012345678",
    "virtualAccountName": "Jokul Doe",
    "inquiryRequestId": "202607221000001234500001",
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [
      {
        "billNo": "123456789012345678",
        "billDescription": {
          "english": "Maintenance",
          "indonesia": "Pemeliharaan"
        },
        "billSubCompany": "00000",
        "billAmount": {
          "value": "100000.00",
          "currency": "IDR"
        }
      }
    ]
  }
}
```

**Validation**:
- Response code `2002400` indicates success
- `virtualAccountData.inquiryStatus` is `00`
- Bill details are returned correctly

---

### Scenario 2: VA Payment Notification

**Request**:
```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/payment \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-22T10:05:00+07:00" \
  -H "X-CLIENT-KEY: test_client" \
  -H "X-SIGNATURE: test_signature" \
  -H "CHANNEL-ID: 95231" \
  -H "X-PARTNER-ID: 12345" \
  -H "X-EXTERNAL-ID: 202607221000002" \
  -H "Idempotency-Key: 202607221000002" \
  -d '{
    "partnerServiceId": " 12345",
    "customerNo": "123456789012345678",
    "virtualAccountNo": " 12345123456789012345678",
    "inquiryRequestId": "202607221000001234500001",
    "paymentRequestId": "202607221000001234500001",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "transactionDate": "2026-07-22T10:05:00+07:00",
    "referenceNo": "12345678901"
  }'
```

**Expected Response**:
```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    }
  }
}
```

**Validation**:
- Payment is recorded in database
- `paymentFlagStatus` is `00` (success)
- Bill is marked as paid

---

### Scenario 3: VA Payment Status Inquiry

**Request**:
```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/status \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-22T10:10:00+07:00" \
  -H "X-CLIENT-KEY: test_client" \
  -H "X-SIGNATURE: test_signature" \
  -H "CHANNEL-ID: 95231" \
  -H "X-PARTNER-ID: 12345" \
  -H "X-EXTERNAL-ID: 202607221000003" \
  -d '{
    "partnerServiceId": " 12345",
    "customerNo": "123456789012345678",
    "virtualAccountNo": " 12345123456789012345678",
    "inquiryRequestId": "202607221000001234500001"
  }'
```

**Expected Response**:
```json
{
  "responseCode": "2002600",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "partnerServiceId": " 12345",
    "customerNo": "123456789012345678",
    "virtualAccountNo": " 12345123456789012345678",
    "inquiryRequestId": "202607221000001234500001",
    "paymentRequestId": "202607221000001234500001",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "transactionDate": "2026-07-22T10:05:00+07:00"
  }
}
```

**Validation**:
- Status inquiry returns correct payment flag
- Transaction details match the original payment

---

### Scenario 4: Invalid VA Number

**Request**:
```bash
curl -X POST http://localhost:8080/v1.0/transfer-va/inquiry \
  -H "Content-Type: application/json" \
  -H "X-TIMESTAMP: 2026-07-22T10:15:00+07:00" \
  -H "X-CLIENT-KEY: test_client" \
  -H "X-SIGNATURE: test_signature" \
  -H "CHANNEL-ID: 95231" \
  -H "X-PARTNER-ID: 12345" \
  -H "X-EXTERNAL-ID: 202607221000004" \
  -H "Idempotency-Key: 202607221000004" \
  -d '{
    "partnerServiceId": " 12345",
    "customerNo": "999999999999999999",
    "virtualAccountNo": " 12345999999999999999999",
    "channelCode": 6011,
    "inquiryRequestId": "202607221000005234500001"
  }'
```

**Expected Response**:
```json
{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account"
}
```

**Validation**:
- Returns appropriate error code
- `virtualAccountData` is not present

---

### Scenario 5: Idempotency Check

**Request** (send same X-EXTERNAL-ID twice):
```bash
# First request (should succeed)
curl -X POST http://localhost:8080/v1.0/transfer-va/inquiry \
  -H "X-EXTERNAL-ID: 202607221000010" \
  -H "Idempotency-Key: 202607221000010" \
  ...

# Second request (should return cached response)
curl -X POST http://localhost:8080/v1.0/transfer-va/inquiry \
  -H "X-EXTERNAL-ID: 202607221000010" \
  -H "Idempotency-Key: 202607221000010" \
  ...
```

**Expected**:
- First request: processed normally
- Second request: returns cached response with `X-Cache-Replay: true` header

---

## Running Tests

```bash
# Unit tests
go test -race -v ./internal/usecase/... -run VA
go test -race -v ./internal/adapter/... -run VA
go test -race -v ./internal/infrastructure/... -run VA

# Coverage check
go test -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out | grep total

# Lint
golangci-lint run
```

## Troubleshooting

### Common Issues

1. **BCA Signature Mismatch**
   - Verify `BCA_VA_CLIENT_SECRET` is correct
   - Check timestamp format: ISO-8601 with timezone
   - Ensure request body is minified (no whitespace)

2. **Idempotency Key Conflict**
   - X-EXTERNAL-ID must be unique per day
   - Use format: `YYYYMMDDHHMMSS` + sequence number

3. **Timeout Errors**
   - Check network connectivity to BCA sandbox
   - Verify `BCA_VA_REQUEST_TIMEOUT` is sufficient (default 30s)

4. **Database Connection**
   - Ensure PostgreSQL is running: `docker compose ps postgres`
   - Check migration status
