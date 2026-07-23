# Merchant VA API Contracts (ASPI OpenAPI-Aligned)

**Date**: 2026-07-23 | **Feature**: 003-merchant-va-dashboard

Reference: `aspi-open-api.yaml` (ASPI SNAP Transfer Kredit Virtual Account API)

All endpoints use unified SNAP routes under `/v1.0/transfer-va/*`.

---

## 1. Create VA Transaction (SNAP Service Code 27)

**POST** `/v1.0/transfer-va/create-va`

Schema: `VAUpsertRequest` / `VAUpsertResponse`

### Request Body

```json
{
  "partnerServiceId": "088899",
  "customerNo": "12345678901234567890",
  "virtualAccountName": "Jokul Doe",
  "trxId": "abcdefgh1234",
  "virtualAccountEmail": "jokul@email.com",
  "virtualAccountPhone": "6281828384858",
  "totalAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "billDetails": [
    {
      "billCode": "01",
      "billNo": "INV-2026-001",
      "billName": "Invoice Januari",
      "billShortName": "Inv Jan",
      "billDescription": {
        "english": "Product Purchase",
        "indonesia": "Pembelian Produk"
      },
      "billSubCompany": "00001",
      "billAmount": {
        "value": "150000.00",
        "currency": "IDR"
      },
      "billAmountLabel": "Amount",
      "billAmountValue": "Rp150.000"
    }
  ],
  "freeTexts": [
    {
      "english": "Thank you",
      "indonesia": "Terima kasih"
    }
  ],
  "virtualAccountTrxType": "C",
  "feeAmount": {
    "value": "0.00",
    "currency": "IDR"
  },
  "expiredDate": "2026-07-30T23:59:59+07:00",
  "additionalInfo": {
    "deviceId": "12345679237",
    "channel": "mobilephone",
    "dbUrlProcess":"https://website.hook"
  }
}
```

**Required fields**: `partnerServiceId`, `customerNo`, `virtualAccountName`, `trxId`
**Optional fields**: `virtualAccountEmail`, `virtualAccountPhone`, `totalAmount`, `billDetails`, `freeTexts`, `virtualAccountTrxType`, `feeAmount`, `expiredDate`, `additionalInfo`

### Success Response (200)

```json
{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "088899",
    "customerNo": "12345678901234567890",
    "virtualAccountNo": "08889912345678901234567890",
    "virtualAccountName": "Jokul Doe",
    "virtualAccountEmail": "jokul@email.com",
    "virtualAccountPhone": "6281828384858",
    "trxId": "abcdefgh1234",
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "billDetails": [...],
    "freeTexts": [...],
    "virtualAccountTrxType": "C",
    "feeAmount": {
      "value": "0.00",
      "currency": "IDR"
    },
    "expiredDate": "2026-07-30T23:59:59+07:00",
    "lastUpdateDate": "2026-07-23T10:00:00+07:00",
    "paymentDate": null,
    "additionalInfo": {
      "dbUrlProcess":"https://website.hook"
    }
  }
}
```

### virtualAccountTrxType Values

| Code | Description |
|------|-------------|
| C | Closed Payment |
| O | Open Payment |
| I | Partial |
| M | Minimum |
| L | Maximum |
| N | Open Minimum |
| X | Open Maximum |

---

## 2. List VA Transactions (Merchant Dashboard Convenience API)

**POST** `/v1.0/transfer-va/list`

**Note**: Not a SNAP standard endpoint. Convenience API for merchant dashboard.

### Request Body

```json
{
  "partnerServiceId": "088899",
  "fromDate": "2026-07-01T00:00:00+07:00",
  "toDate": "2026-07-31T23:59:59+07:00",
  "status": "00",
  "virtualAccountNo": "",
  "page": 1,
  "pageSize": 20
}
```

### Success Response (200)

```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "data": [...],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "totalRows": 150,
    "totalPages": 8
  }
}
```

---

## 3. Delete VA Transaction (SNAP Service Code 31)

**DELETE** `/v1.0/transfer-va/delete-va`

Schema: `DeleteVARequest` / `DeleteVAResponse`

### Request Body

```json
{
  "partnerServiceId": "088899",
  "customerNo": "12345678901234567890",
  "virtualAccountNo": "08889912345678901234567890",
  "trxId": "abcdefgh1234",
  "additionalInfo": {}
}
```

**Required fields**: `partnerServiceId`, `customerNo`, `virtualAccountNo`
**Optional fields**: `trxId`, `additionalInfo`

### Success Response (200)

```json
{
  "responseCode": "2003100",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "088899",
    "customerNo": "12345678901234567890",
    "virtualAccountNo": "08889912345678901234567890",
    "trxId": "abcdefgh1234",
    "additionalInfo": {}
  }
}
```

---

## 4. Payment Notification (SNAP Service Code 25)

**POST** `/v1.0/transfer-va/payment`

Schema: `PaymentRequest` / `PaymentResponse`

### Request Body

```json
{
  "partnerServiceId": "088899",
  "customerNo": "12345678901234567890",
  "virtualAccountNo": "08889912345678901234567890",
  "trxId": "abcdefgh1234",
  "paymentRequestId": "PAY-20260723001",
  "paidAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "cumulativePaymentAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-07-23T14:30:00+07:00",
  "referenceNo": "BNK123456789",
  "journalNum": "000001",
  "paymentType": "0",
  "billDetails": [...],
  "additionalInfo": {}
}
```

**Required fields**: `paymentRequestId`, `paidAmount`
**Optional fields**: `virtualAccountName`, `virtualAccountEmail`, `virtualAccountPhone`, `trxId`, `channelCode`, `hashedSourceAccountNo`, `sourceBankCode`, `cumulativePaymentAmount`, `paidBills`, `totalAmount`, `trxDateTime`, `referenceNo`, `journalNum`, `paymentType`, `flagAdvise`, `subCompany`, `billDetails`, `freeTexts`, `additionalInfo`

### Success Response (200)

```json
{
  "responseCode": "2002500",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "088899",
    "customerNo": "12345678901234567890",
    "virtualAccountNo": "08889912345678901234567890",
    "trxId": "abcdefgh1234",
    "paymentRequestId": "PAY-20260723001",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "paidBills": "01",
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-07-23T14:30:00+07:00",
    "referenceNo": "BNK123456789",
    "journalNum": "000001",
    "paymentType": "0",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [...],
    "freeTexts": [...],
    "additionalInfo": {}
  }
}
```

---

## 5. Merchant Webhook Notification (Outbound)

When a payment is received, the system sends a POST notification to the `notificationUrl` provided during VA creation.

### Webhook Payload

```json
{
  "eventType": "payment.received",
  "timestamp": "2026-07-23T14:30:05+07:00",
  "data": {
    "virtualAccountNo": "08889912345678901234567890",
    "customerNo": "12345678901234567890",
    "trxId": "abcdefgh1234",
    "paymentRequestId": "PAY-20260723001",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-07-23T14:30:00+07:00",
    "referenceNo": "BNK123456789",
    "status": "00"
  }
}
```

### Webhook Headers

| Header | Description |
|--------|-------------|
| X-Timestamp | Notification timestamp |
| X-Signature | HMAC-SHA512 signature for verification |
| Content-Type | application/json |

### Expected Response

Merchant webhook must respond with HTTP 2xx within 5 seconds. Retries follow Asynq exponential backoff (3 retries, 10s/30s/60s delays).
