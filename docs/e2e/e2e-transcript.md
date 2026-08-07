# SNAP Virtual Account — end-to-end request/response transcript

Every request this suite puts on the wire and every response it got back,
captured from an actual run of `test/e2e`. The suite drives the production
router, idempotency middleware, SNAP auth middleware, handler and usecase
against an in-memory repository, so the headers, `stringToSign` inputs,
service codes and JSON envelopes below are the real ones.

Regenerate with:

```sh
E2E_TRANSCRIPT=docs/e2e/e2e-transcript.md go test ./test/e2e/...
```

- Generated: `2026-08-08T00:30:48+07:00`
- Commit: `25b9dab`
- Exchanges: 108 across 79 scenarios

Signatures and timestamps are genuine but computed over the suite's own
throwaway vendor secret, so they change on every run.

## Contents

- [Transaction flows — fixed bill, variable bill, no bill](#transaction-flows--fixed-bill-variable-bill-no-bill)
- [Negative cases — auth, headers, payload, business rejections](#negative-cases--auth-headers-payload-business-rejections)
- [Multi-vendor / multi-merchant isolation](#multi-vendor--multi-merchant-isolation)
- [BCA conformance regressions](#bca-conformance-regressions)

---

## Transaction flows — fixed bill, variable bill, no bill

### TestE2E_FixedBill_InquiryPaymentStatus

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848393665361
X-PARTNER-ID: 12345
X-SIGNATURE: SNwbZfnf024lrKT8VkaSnwuUhkAa2LfjKe9gVVhQIRN7HknPItGp3vwn+lsaIcSJodUCqVCgN5FLpBLLE7kQBw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890123",
  "inquiryRequestId": "INQ-FIXED-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890123"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890123",
    "virtualAccountNo": "12345678901234567890123",
    "virtualAccountName": "Budi Manjo",
    "inquiryRequestId": "INQ-FIXED-1",
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "00",
    "inquiryReason": {
      "english": "Success",
      "indonesia": "Sukses"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848393820800
X-PARTNER-ID: 12345
X-SIGNATURE: pTH10Tje7YMh/b8ZFqvoHSdXsoPwPlkSENSNFHvx42ll5pCBmTOEzf11rNGIB4JoCQ/oHz0NGaCopVs0ZIf+9Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890123",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FIXED-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890123"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890123",
    "virtualAccountNo": "12345678901234567890123",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890123",
    "paymentRequestId": "PAY-FIXED-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 3. `POST /openapi/v2.0/transfer-va/status` → 200

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848393951397
X-PARTNER-ID: 12345
X-SIGNATURE: Fb4GKNyVeZbMfnZ7uFcacz6W8R3zF784kWM/l2DFmUg3N6QjwqY+CF3V+UljDgNQkXALjZwZfcNXWIAicujgZw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890123",
  "inquiryRequestId": "PAY-FIXED-1",
  "partnerServiceId": "12345",
  "virtualAccountNo": "12345678901234567890123"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002600",
  "responseMessage": "Success",
  "virtualAccountData": {
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "partnerServiceId": "12345",
    "customerNo": "678901234567890123",
    "virtualAccountNo": "12345678901234567890123",
    "inquiryRequestId": "INQ-FIXED-1",
    "paymentRequestId": "PAY-FIXED-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "transactionDate": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N"
  },
  "additionalInfo": {}
}
```

### TestE2E_FixedBill_AmountComparedAgainstStoredBill

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848394093175
X-PARTNER-ID: 12345
X-SIGNATURE: W+zXYpxyo6oAa9Ycw5YRn1zYUl6AN19IfSNnwtPAIejwFZZdbZNkd/iF41RQS3neecAN1kGjp/jCi2VeSprpkg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890124",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-UNDER-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890124"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042513",
  "responseMessage": "Invalid amount",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890124",
    "virtualAccountNo": "12345678901234567890124",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-UNDER-1",
    "paidAmount": {
      "value": "1.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Invalid Amount",
      "indonesia": "Nominal pembayaran tidak sesuai"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_FixedBill_TrailingZerosAccepted

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848394289974
X-PARTNER-ID: 12345
X-SIGNATURE: rI5U4m20hFWRFR2+oDuRjn+wcEYP3uN2386F7hbtNl9poGWyetk8pdoDN9pzTTL8Qp5GwXsbW8iA7uu34p3HoQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890125",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NUM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890125"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890125",
    "virtualAccountNo": "12345678901234567890125",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890125",
    "paymentRequestId": "PAY-NUM-1",
    "paidAmount": {
      "value": "250000",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_FixedBill_PaidBillRejectedOnSecondPayment

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848394477358
X-PARTNER-ID: 12345
X-SIGNATURE: LWzFNqJ3w2bKSHUjGhiDPgnMRyxzoi+9ukrQQQS16EnIH/xrePf0yaCvKsVtRn/M/mxDrllIj1X5DaFm1fjSwA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890126",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FIRST",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890126"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890126",
    "virtualAccountNo": "12345678901234567890126",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890126",
    "paymentRequestId": "PAY-FIRST",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848394606424
X-PARTNER-ID: 12345
X-SIGNATURE: J86MQT2YSmpt17oQbD7IPzkfxdfYpjyuPV/nrBmfR/2p0hQVHWLGXZV7ZAhbgIGgtAXBqbUwc1dz7qTI4B+shg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890126",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-SECOND",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890126"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042514",
  "responseMessage": "Paid Bill",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890126",
    "virtualAccountNo": "12345678901234567890126",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890126",
    "paymentRequestId": "PAY-SECOND",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Bill has been paid",
      "indonesia": "Tagihan telah dibayar"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_FixedBill_InquiryAfterPaymentReportsPaidBill

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848394793492
X-PARTNER-ID: 12345
X-SIGNATURE: 4ZP/NO7bEqd/sSR++DzYqQcrifSz4reusI/zf/rpKbVYl+j4/AR4r4BSOh/maRDzJ6q8tD4CUxfCTuwaxLvhPQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890127",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-PAID-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890127"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890127",
    "virtualAccountNo": "12345678901234567890127",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890127",
    "paymentRequestId": "PAY-PAID-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848394948145
X-PARTNER-ID: 12345
X-SIGNATURE: HFAgpvH09OywxV6L2ctounK4nq8Efbcxs26DBrwB0Kx1NebtvPHXP81AdP8xjzyHTxkcNPDTuGNPKxTo1RZwyw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890127",
  "inquiryRequestId": "INQ-PAID-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890127"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042414",
  "responseMessage": "Paid Bill",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890127",
    "virtualAccountNo": "12345678901234567890127",
    "virtualAccountName": "Budi Manjo",
    "inquiryRequestId": "INQ-PAID-1",
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Bill has been paid",
      "indonesia": "Tagihan telah dibayar"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_VariableBill_InstalmentsFlagSuccessNotPending

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848395120621
X-PARTNER-ID: 12345
X-SIGNATURE: Km/EVF+DJB8UYjAViaaBvqlBcUUsvnzVW7jozd0ODayqsBdH5yxmXjmAp0PvATHEsv5ATu7sR+tBZOUg1Wy4Zw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890130",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890130"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890130",
    "virtualAccountNo": "12345678901234567890130",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890130",
    "paymentRequestId": "PAY-VAR-1",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848395322807
X-PARTNER-ID: 12345
X-SIGNATURE: uuZjvKBzsHZUQk14KW2OXwSBDiGbQBLVDfSThRX1ULLCKc+X+ZWqK3jmm1Ukis5fDcduCrHSVrub2J3Ea2b+cA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890130",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "40000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "40000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890130"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890130",
    "virtualAccountNo": "12345678901234567890130",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890130",
    "paymentRequestId": "PAY-VAR-2",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "40000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_VariableBill_NoExactAmountCheck

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848395529694
X-PARTNER-ID: 12345
X-SIGNATURE: Tfx/MqJjzpQ9xCvrFy6V+PX0YZxK6NhzVyTnC/0A5qbeTJCBs5WE3RgtXXJpTqO3DZ9UuJphGROTHqa+b4GXuQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890131",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-PART",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890131"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890131",
    "virtualAccountNo": "12345678901234567890131",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890131",
    "paymentRequestId": "PAY-VAR-PART",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_NoBill_PayableRepeatedly

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848395732442
X-PARTNER-ID: 12345
X-SIGNATURE: 32e6v5lEiPB1RTOe9M7WLypvPgjUI2Rvy2Kikvrh+GFAIK38taefnMPeKTR3jWm3GzfC6t0rAueoFUbr1AHwZA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "inquiryRequestId": "INQ-NOBILL-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890140"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "12345678901234567890140",
    "virtualAccountName": "Budi NoBill",
    "inquiryRequestId": "INQ-NOBILL-1",
    "totalAmount": {
      "value": "0.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "00",
    "inquiryReason": {
      "english": "Success",
      "indonesia": "Sukses"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848396556916
X-PARTNER-ID: 12345
X-SIGNATURE: NUj2hLNeUfJ02tZAhUKScj88QyJaNp8KLf3sikOfgVOWuVaMVBHcCuL8UkHEX9AFm8LKj2NfVBgwbxXct4PW5Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NOBILL-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890140"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "12345678901234567890140",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890140",
    "paymentRequestId": "PAY-NOBILL-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 3. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848397507305
X-PARTNER-ID: 12345
X-SIGNATURE: 7Vrq9X3L6hDr5ZWVnYHipGKI9k0vaPa9OmSnOZnq3xbBmxPUkK1AcbF2kaMxGdGn1dwMqXIsauWLdRPfHCl91A==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "35000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NOBILL-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "35000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890140"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "12345678901234567890140",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890140",
    "paymentRequestId": "PAY-NOBILL-2",
    "paidAmount": {
      "value": "35000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "35000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 4. `POST /openapi/v1.0/transfer-va/inquiry` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848397768484
X-PARTNER-ID: 12345
X-SIGNATURE: aV+GcFZA0TnwtgcCH8BbATcuEmTeDN1zPldTqsBACunB0FawNgZ5pIuGIWB6Y75IyqTOJYr6Zh8xwQkqpR2Urg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "inquiryRequestId": "INQ-NOBILL-2",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890140"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "12345678901234567890140",
    "virtualAccountName": "Budi NoBill",
    "inquiryRequestId": "INQ-NOBILL-2",
    "totalAmount": {
      "value": "0.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "00",
    "inquiryReason": {
      "english": "Success",
      "indonesia": "Sukses"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_NoBill_ExpiredRegistrationRejected

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848398129939
X-PARTNER-ID: 12345
X-SIGNATURE: dZ7sa3D1ycOf2fd36bYKTZzXWHTKbzFSCWVYIuXftY7fAQYFLo5XtrKa2oENoznLxZWRD4139PH2CwPFJRwv2w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890141",
  "inquiryRequestId": "INQ-EXP-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890141"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890141",
    "virtualAccountNo": "12345678901234567890141",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-EXP-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "expired transaction",
      "indonesia": "transaksi kadaluarsa"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848398440575
X-PARTNER-ID: 12345
X-SIGNATURE: A9XN929oQhBttjFvcykOLbn6lhJqVEE1knMhMeNR5iui4XIg0CneWAvoN70qoCAx/sgbUDJUqh5TCO1XwpENig==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890141",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-EXP-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890141"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042519",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890141",
    "virtualAccountNo": "12345678901234567890141",
    "virtualAccountName": "Budi NoBill",
    "trxId": "trx-678901234567890141",
    "paymentRequestId": "PAY-EXP-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "expired transaction",
      "indonesia": "transaksi kadaluarsa"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_DoubleFlagging_ReturnsInconsistentRequest

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848398741435
X-PARTNER-ID: 12345
X-SIGNATURE: BUV5og2HWazdjAXMbe+4X5W4pXm7CEu1+TLKjZZpivK6IxLQkjjavSVva6o5lcVOc93hEN3eijti+XY27Mg+/w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890150",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-DF-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890150"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890150",
    "virtualAccountNo": "12345678901234567890150",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890150",
    "paymentRequestId": "PAY-DF-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848398969310
X-PARTNER-ID: 12345
X-SIGNATURE: BUV5og2HWazdjAXMbe+4X5W4pXm7CEu1+TLKjZZpivK6IxLQkjjavSVva6o5lcVOc93hEN3eijti+XY27Mg+/w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890150",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-DF-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890150"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042518",
  "responseMessage": "Inconsistent Request",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890150",
    "virtualAccountNo": "12345678901234567890150",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890150",
    "paymentRequestId": "PAY-DF-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_AdviceRetry_ReplaysOriginalSuccess

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848399247752
X-PARTNER-ID: 12345
X-SIGNATURE: aoNXPKj+tekXB6j4Bh4+HxM6aQtqxrDMfj5EbjfsN6FkVWYUzmn/kxGd4HqY7nlEljJyoE/gQWVv92dABfnoNQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890151",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ADV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890151"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890151",
    "virtualAccountNo": "12345678901234567890151",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890151",
    "paymentRequestId": "PAY-ADV-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848399516849
X-PARTNER-ID: 12345
X-SIGNATURE: mMPCsQo2ZVDDq6y2mHMz92jTz8O7ECrQSXysaB1YQApukHg8oQ1CCFq8N6gR3JKece64A0KW+9kc2wpsvuT8qw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890151",
  "flagAdvise": "Y",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ADV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890151"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890151",
    "virtualAccountNo": "12345678901234567890151",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890151",
    "paymentRequestId": "PAY-ADV-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_SameExternalIDSamePayload_ReplaysCachedResponse

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000002
X-PARTNER-ID: 12345
X-SIGNATURE: o682t4DoycS2ljDDRftk5j96JxKf0uyI0zF31eR5+0awHk8IKX36Nl2aMDyTtbaA6U3EluKbXR8pZu2E4ZNiaw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890160",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-IDEM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890160"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890160",
    "virtualAccountNo": "12345678901234567890160",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890160",
    "paymentRequestId": "PAY-IDEM-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000002
X-PARTNER-ID: 12345
X-SIGNATURE: o682t4DoycS2ljDDRftk5j96JxKf0uyI0zF31eR5+0awHk8IKX36Nl2aMDyTtbaA6U3EluKbXR8pZu2E4ZNiaw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890160",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-IDEM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890160"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json
X-Cache-Replay: true

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890160",
    "virtualAccountNo": "12345678901234567890160",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890160",
    "paymentRequestId": "PAY-IDEM-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_VariableBill_RepeatedInstalmentNotCreditedTwice

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848400147177
X-PARTNER-ID: 12345
X-SIGNATURE: LFGnhRz2ih+CVij1HLRpalcDV3OD9G/W9nFNapKrQ10OKarwYucarqB0+2p3rGwvI2l50DjaMSE+Zv2NW8TQQg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890132",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-DUP",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890132"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890132",
    "virtualAccountNo": "12345678901234567890132",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890132",
    "paymentRequestId": "PAY-VAR-DUP",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848400569893
X-PARTNER-ID: 12345
X-SIGNATURE: LFGnhRz2ih+CVij1HLRpalcDV3OD9G/W9nFNapKrQ10OKarwYucarqB0+2p3rGwvI2l50DjaMSE+Zv2NW8TQQg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890132",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-DUP",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890132"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042518",
  "responseMessage": "Inconsistent Request",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890132",
    "virtualAccountNo": "12345678901234567890132",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890132",
    "paymentRequestId": "PAY-VAR-DUP",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_VariableBill_AdviceRetryReplaysWithoutDoubleCrediting

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848400875762
X-PARTNER-ID: 12345
X-SIGNATURE: 84FJY/4CFQh4nIcOTv93HZIvaU3HvIhoQyCJQbWtrhF326A8Uah8aJ+vGlOd3aOPAYET0oh5JV2aZY0Nx0R9PQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890133",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-ADV",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890133"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890133",
    "virtualAccountNo": "12345678901234567890133",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890133",
    "paymentRequestId": "PAY-VAR-ADV",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848401137137
X-PARTNER-ID: 12345
X-SIGNATURE: 4YAmi2RzaGbz6c4dH4sa6/EmSZBpFIyw//KCnnOcHbJ8BaIrdp7uDGSebZO3zfpKpwCVOCi0Zkt4/H0lO08tbw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890133",
  "flagAdvise": "Y",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-VAR-ADV",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890133"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890133",
    "virtualAccountNo": "12345678901234567890133",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890133",
    "paymentRequestId": "PAY-VAR-ADV",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "Y",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Status_ResolvesByPaymentRequestID

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848401361435
X-PARTNER-ID: 12345
X-SIGNATURE: IA093V4jyoHC2/UuHL6hqqio6d1ZCXHq/S6OhManL+qOMQ/cNlQbpC3kvZwbFShx0WG151ghJJoGhXMRUi1m+g==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890170",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ST-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890170"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890170",
    "virtualAccountNo": "12345678901234567890170",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890170",
    "paymentRequestId": "PAY-ST-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v2.0/transfer-va/status` → 200

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848401519862
X-PARTNER-ID: 12345
X-SIGNATURE: Un+fVtuW6MVlLodd05PAAVtHCx4Xd1i5Yay55neEBt8Ee0xIuNvbnOinEbA6mlLeosb0axE9CHT5WwDSDR7FzQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "customerNo": "678901234567890170",
  "inquiryRequestId": "INQ-NEVER-PERSISTED",
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ST-1",
  "virtualAccountNo": "12345678901234567890170"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002600",
  "responseMessage": "Success",
  "virtualAccountData": {
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "partnerServiceId": "12345",
    "customerNo": "678901234567890170",
    "virtualAccountNo": "12345678901234567890170",
    "inquiryRequestId": "PAY-ST-1",
    "paymentRequestId": "PAY-ST-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "transactionDate": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N"
  },
  "additionalInfo": {}
}
```

### TestE2E_Status_UnknownIDsStillNotFound

#### 1. `POST /openapi/v2.0/transfer-va/status` → 404

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848401711751
X-PARTNER-ID: 12345
X-SIGNATURE: 4Uc/4Wiyd8vvlPZnYjpvuPudPqfQdga0/lsjDrdVSynyceOyyhVz30hZlTx+KklPORkhhYTyvHrCY9oBL9YuDA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "customerNo": "678901234567890171",
  "inquiryRequestId": "INQ-UNKNOWN",
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-UNKNOWN",
  "virtualAccountNo": "12345678901234567890171"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042601",
  "responseMessage": "Transaction Not Found",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Transaction Not Found",
      "indonesia": "Transaksi Tidak Ditemukan"
    },
    "partnerServiceId": "12345",
    "customerNo": "678901234567890171",
    "virtualAccountNo": "12345678901234567890171",
    "inquiryRequestId": "INQ-UNKNOWN",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "transactionDate": null
  },
  "additionalInfo": {}
}
```

---

## Negative cases — auth, headers, payload, business rejections

### TestE2E_Negative_Auth/signature_signed_with_the_wrong_secret

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848376348400
X-PARTNER-ID: 12345
X-SIGNATURE: 5FWurm4QOKynnfS2q9VO0ca2DUV7LPJz+4aahWi9+X/jYV6Lyx6Tn6pEEqMtiIo9sToduV1tkcQM2UKQaa1iKg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012400",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_Negative_Auth/garbage_signature

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848376620001
X-PARTNER-ID: 12345
X-SIGNATURE: not-a-signature
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_Negative_Auth/empty_signature_is_a_missing_mandatory_header

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848376841895
X-PARTNER-ID: 12345
X-SIGNATURE:  
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_Negative_Auth/unknown_CHANNEL-ID

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 00000
Content-Type: application/json
X-EXTERNAL-ID: 1786123848377048427
X-PARTNER-ID: 12345
X-SIGNATURE: 4ARJUL3YLUI8AGM3qcHq+naPYL3qgI3nEnrK+cDnT1xrf1zLnySKaE670ysWVckChX+/BKapH2KUToUiwqk0Kg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012400",
  "responseMessage": "Unauthorized. [Unknown channel]",
  "data": {}
}
```

### TestE2E_Negative_Auth/unknown_X-PARTNER-ID

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848377240890
X-PARTNER-ID: 99999
X-SIGNATURE: Egxg4HL7XVuEP4OIIDlDfz9mMiZPGfoT3VKm32qbhhJae1MDMONxsT5qeudMucIxfL0A4YOa944ynWC+4au7Ew==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Unknown client]",
  "data": {}
}
```

### TestE2E_Negative_Auth/missing_CHANNEL-ID

#### 1. `POST /openapi/v2.0/transfer-va/status` → 400

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
Content-Type: application/json
X-EXTERNAL-ID: 1786123848377392441
X-PARTNER-ID: 12345
X-SIGNATURE: U/yvMnS/I82jKSmCY3gLdMl5jJ278/aPl4vqhM7T7gaOkhPbltwQ9EEH8ZUcMra5qkLX5HS7gC2/4tPDlBEPMw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "12345",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002602",
  "responseMessage": "Invalid Mandatory Field [CHANNEL-ID]",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "inquiryRequestId": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "transactionDate": null
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_Auth/missing_X-PARTNER-ID

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848377772265
X-SIGNATURE: 4ARJUL3YLUI8AGM3qcHq+naPYL3qgI3nEnrK+cDnT1xrf1zLnySKaE670ysWVckChX+/BKapH2KUToUiwqk0Kg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [X-PARTNER-ID]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_Auth/X-TIMESTAMP_not_parseable

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848378153438
X-PARTNER-ID: 12345
X-SIGNATURE: iMYjgGusfSI96FDsTScbCmkwuDIhcHJDXLHpWgNwMXMi375i9TMYLJpN3dpvhMT8CXvpmj28fNMqIlOWTBb5KA==
X-TIMESTAMP: 2026-08-06 10:00:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002401",
  "responseMessage": "Invalid Field Format [X-TIMESTAMP]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_Auth/X-TIMESTAMP_without_a_timezone_designator

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848378382091
X-PARTNER-ID: 12345
X-SIGNATURE: UnoHWRjr8tmFgPNvAgF3wJXvypiIn1vjmB7TufpWT65SzdBttJrt8L0osp5a9naJbyk+t97S8WnfyRDeGDG0Dg==
X-TIMESTAMP: 2026-08-06T10:00:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [X-TIMESTAMP]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_WrongBodyHashEncodingRejected

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848378648285
X-PARTNER-ID: 12345
X-SIGNATURE: 090cWKPgVhfeWGjl1llxg1/wINAFRHkrCNgVhFr+WU1p6dIH+caMwl7fCCHeNQ8/KrTR22lpag4XSb16vC7SbQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890201",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ENC-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890201"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_SignatureCoversTheBody

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848378888444
X-PARTNER-ID: 12345
X-SIGNATURE: ppjiXzJIThodrJLi2rbfLteKgUdbh1F4euUmtQzfcZS+T2eIi2ODfMqsRF6hIxJaWychB/RcVZ0nammWp1gP3g==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890202",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "9999.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-TAMPER-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890202"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_PrettyPrintedBodyStillVerifies

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848379088200
X-PARTNER-ID: 12345
X-SIGNATURE: a69ieQ2eCR3LFK1v7DvDOxT0U1uPFuY2oAuvblNBXSV6Ukuxnox4+3OXlisPuwnNLJV3xqDvyh2CQy1aLyw84w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "partnerServiceId": "12345",
  "customerNo": "678901234567890203",
  "virtualAccountNo": "12345678901234567890203",
  "virtualAccountName": "Budi Manjo",
  "paymentRequestId": "PAY-PRETTY-1",
  "channelCode": 6011,
  "paidAmount": {
    "value": "1000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "1000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "flagAdvise": "N"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890203",
    "virtualAccountNo": "12345678901234567890203",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890203",
    "paymentRequestId": "PAY-PRETTY-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_ExternalID/missing

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-PARTNER-ID: 12345
X-SIGNATURE: j3vqhRAu6Rx5fcP4YmmujvMyqSllhjyxI9rt8Nv+jE3CH9ns5IpDfk0VnibMoYrN3JyGCZrdaayqyaRpXQwdkA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890210",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-EXT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890210"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [X-EXTERNAL-ID]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_ExternalID/longer_than_36_characters

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 9999999999999999999999999999999999999
X-PARTNER-ID: 12345
X-SIGNATURE: GZ8lAVZPIvFMeAJtH/khHIEfTs8f2MgIpgO6WTupLBlykeiyt44d9vphbkG7I5zuYXtON7viJr9zdfeeS7TP6g==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890211",
  "inquiryRequestId": "INQ-EXT-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890211"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002401",
  "responseMessage": "Invalid Field Format [X-EXTERNAL-ID]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_ExternalID/reused_with_a_different_payload_is_a_Conflict,_not_a_422

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000001
X-PARTNER-ID: 12345
X-SIGNATURE: WzShuBXn+sr5S2//9Ev9tx2g3/LulazyMh/49uefRtazVXt0iJ8mI5/z/MI14vuxI8W6eWd9zlwYliP+sBRJrA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890212",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-EXT-A",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890212"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890212",
    "virtualAccountNo": "12345678901234567890212",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890212",
    "paymentRequestId": "PAY-EXT-A",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 409

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000001
X-PARTNER-ID: 12345
X-SIGNATURE: hO5zSH/rwbH3I6qiksns4j5cZVaYdhU+n5Le9S4z76VrVk+BOSscNHVkwDMtfRAq6+NRSxZGSPhF8rNGi9wc/w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890212",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-EXT-B",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890212"
}
```

**Response**

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{
  "responseCode": "4092500",
  "responseMessage": "Conflict",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Cannot use the same X-EXTERNAL-ID",
      "indonesia": "Tidak bisa menggunakan X-EXTERNAL-ID yang sama"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_MalformedBody//openapi/v1.0/transfer-va/inquiry

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848380487851
X-PARTNER-ID: 12345
X-SIGNATURE: 3xTDyiiTl6X8G/hFzkQuBWbZrXE2PfWj02npqRCVh/rlDDtQ6/gxfC9EjaoWvXt+JRzERvIb0sXT3JaJ6ODyjw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{"partnerServiceId":
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002400",
  "responseMessage": "Bad Request",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_MalformedBody//openapi/v1.0/transfer-va/payment

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848380731985
X-PARTNER-ID: 12345
X-SIGNATURE: td028kSEna45TqdPi5exhpIc6UehCqeJev6lSwjPvcV4FohsOEZk07Tu2wtvGP7KhtfYqwX37UkWkX6JKOF2TQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{"partnerServiceId":
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002500",
  "responseMessage": "Bad Request",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_MalformedBody//openapi/v2.0/transfer-va/status

#### 1. `POST /openapi/v2.0/transfer-va/status` → 400

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848380969003
X-PARTNER-ID: 12345
X-SIGNATURE: PZNkyc9eE+1q8R522tB7/hZdJYsaj5eL2wm2wr5xVU3aPIWTqtz22d6fZ2FLyPX4wSE59O3AsQVSOmoHqt469w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{"partnerServiceId":
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002600",
  "responseMessage": "Bad Request",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "inquiryRequestId": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "transactionDate": null
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_InquiryMandatoryFields/partnerServiceId

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848381327499
X-PARTNER-ID: 12345
X-SIGNATURE: O8BbNMYS/OvRZVJECNAurBHngwOeUDIsDyE/YM4S88y5LEKYg+LtHBOtjlMU/FC2nY0/qvBBYDxm9j+2V6RYkg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890220",
  "inquiryRequestId": "INQ-MAND-1",
  "partnerServiceId": "",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890220"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [partnerServiceId]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "678901234567890220",
    "virtualAccountNo": "12345678901234567890220",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-MAND-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_InquiryMandatoryFields/customerNo

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848382531079
X-PARTNER-ID: 12345
X-SIGNATURE: zNgTO8nkaF0Kbv8PNtEWqOSbFwNij/EX2T4jBB8wG1v/ycr6Qews3BIXN7CvI76xxAQge0Ev8qX8Yf31j4zbwQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "",
  "inquiryRequestId": "INQ-MAND-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890220"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [customerNo]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "",
    "virtualAccountNo": "12345678901234567890220",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-MAND-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_InquiryMandatoryFields/virtualAccountNo

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848384341161
X-PARTNER-ID: 12345
X-SIGNATURE: Lr4tEPZSqNsZo943FQmtTjqsSUmeqo2gvTfZdZz/rnz0gHCruLJ1DuPcSSsl/z4njB1UA70UOpgWAI6zne840Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890220",
  "inquiryRequestId": "INQ-MAND-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": ""
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [virtualAccountNo]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890220",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-MAND-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_InquiryMandatoryFields/inquiryRequestId

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848384794788
X-PARTNER-ID: 12345
X-SIGNATURE: cPOVJmwOFL1PZDSk9sYiWKBLYHiD5Zx0WogvXSqbU9knygiMKN9YoNmO1xNf6eoTB6rRHPPB6/mUzM8zWC+tgw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890220",
  "inquiryRequestId": "",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890220"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [inquiryRequestId]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890220",
    "virtualAccountNo": "12345678901234567890220",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/paymentRequestId

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848385100849
X-PARTNER-ID: 12345
X-SIGNATURE: ixRasHtvt1QAcneLM8NgCPnULs/GdbaKjh6eUDtn/w7AFxSxYz9cZvMz+pfbEB6y0EMsu9ooLCuSxTEXd+ixKA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [paymentRequestId]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/paidAmount

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848385464637
X-PARTNER-ID: 12345
X-SIGNATURE: bU4cIYh1xdp4MywK/N8wi3+OBYHVrbVuLD3XdT67MPOzPV12iXm9ovKRpfrYKuMc7ZzRQa1Kt+v6/Y6TVCXCVg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [paidAmount]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MAND-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/virtualAccountName

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848385742606
X-PARTNER-ID: 12345
X-SIGNATURE: M5pafPBEwVXRwtcTWs89lCl5l4/4hXs6lwDGve3TP1oSvch+hyTfdTCYpg4jq/iW8tDmuANdhT9ECQ5TqKs77w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [virtualAccountName]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MAND-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/channelCode

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848386024639
X-PARTNER-ID: 12345
X-SIGNATURE: gmhSwX3MznnTHnV+d81Dskn4ltVTYH5yG93UL/SwQsOQC6RKB9jBaFi3jXalTZp65+aUmKEad2y0mHqcWYckWg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [channelCode]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MAND-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/totalAmount

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848386298092
X-PARTNER-ID: 12345
X-SIGNATURE: hJhMyDdVPdo0jVbuwB6+3yAC6nWgAJNmDAofJlgFH2k0nr+KdQfxr+XNR9y3yv25Q3iPOsVm5sN2EH7S+5cZ2Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [totalAmount]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MAND-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/trxDateTime

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848386557384
X-PARTNER-ID: 12345
X-SIGNATURE: yAn7vUr1RD7zyq4mXAUUynuSE0dUJgivPP+E4lcO7yWaeknepPBWHF/DaX3fqezXJ3zgF7+qxNtL3bb9pKrfaQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [trxDateTime]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MAND-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentMandatoryFields/flagAdvise

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848386844766
X-PARTNER-ID: 12345
X-SIGNATURE: e9bMKCZjyU7rLpiq7th1KxhMxm9aS1gQa3xjs08UlvwOAd65Qr1zrY06mqipVN2m/84gOwLCbwDEMVce1zC2Cw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890221"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [flagAdvise]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "12345678901234567890221",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MAND-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Payment_TrxIDStaysOptional

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848387133926
X-PARTNER-ID: 12345
X-SIGNATURE: KVGfNH3ZSXs/DuFv4z33xbEKTiI4rf7CLhOYpgc8hVfFVDrHw2KKSab7Kai9JrGtfcZPPhaMHFjrkyThnRSXfQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890222",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-NOTRX-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890222"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890222",
    "virtualAccountNo": "12345678901234567890222",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890222",
    "paymentRequestId": "PAY-NOTRX-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/partnerServiceId_over_8

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848387438594
X-PARTNER-ID: 12345
X-SIGNATURE: o02Ku3z/fF8Cmjg2b3OrHuPI1uPHdlRNgo2echfAKlbDJxDJSdQMIuob+ejRhvQwgLVPPsATjLnPd//05YXgmQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "123456789",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [partnerServiceId]",
  "virtualAccountData": {
    "partnerServiceId": "123456789",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/paymentRequestId_over_30

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848387692737
X-PARTNER-ID: 12345
X-SIGNATURE: mK9/JhvK44cotyueEGUjPe9X9uS4d3L6SqRuu7NUzGKbBGn7R/tlwIjtV4h8X+TV+xZ7Khjfr2jV2Fy38KvcxQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "9999999999999999999999999999999",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [paymentRequestId]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "9999999999999999999999999999999",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/virtualAccountName_over_30

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848387960668
X-PARTNER-ID: 12345
X-SIGNATURE: +x+qBim4H64FeGQCFVRzxHjs93fSBjjZMXHKcQuIzb4kmAaLJQ+x6CMiflGEAGcEZqkK0/MwA972NeNsnlPX7w==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [virtualAccountName]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/referenceNo_over_11

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848388256493
X-PARTNER-ID: 12345
X-SIGNATURE: rVcOW+b+w5U+aDsxm+UO1jDtUgIQ7espH3D0GFsYbymdzvDekJ7njMO0OKu7uBaCt4XSc+kgM1PTIxNfTWrZ6Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "999999999999",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [referenceNo]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/unsupported_currency

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848388538054
X-PARTNER-ID: 12345
X-SIGNATURE: wZchl1sudf55p7jMaLOeB3wZj3L7+TzWGKffod2AeqneynnxqGeasdxIUbey9My0+fgl/fBJ6MrB44hTafT+TA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "EUR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "EUR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [paidAmount.currency]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/currency_disagrees_between_amounts

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848388790081
X-PARTNER-ID: 12345
X-SIGNATURE: mhmoIFm5hagjAXktiANxP5u20XQsK+AG+Yeo3UK0zck4lHwATzTENbd5vU7YR6xocglTOH84gnWZAAQhkZubBQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "USD",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [totalAmount.currency]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/amount_over_13_integer_digits

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848389054028
X-PARTNER-ID: 12345
X-SIGNATURE: NXg/8kauyNkWqcLajXDPpUhwU7q/j1OH4Q1kkZKTD04Q4h9KkDpS2YMR96TBbhBO4hP10sYv/HvFCP43LndiMQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12345678901234.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12345678901234.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [paidAmount.value]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/amount_is_not_numeric

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848389344645
X-PARTNER-ID: 12345
X-SIGNATURE: 4YKCh5rDvtrb6F2dO6dUuDm7lynCRqcJmlpnvEXYV57rHPv3NG5dXuNCh4vJhB8+aI7yNCkJGkOe9X1Lxmb+Yg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "abc"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "abc"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [paidAmount.value]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/flagAdvise_outside_N/Y

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848389662476
X-PARTNER-ID: 12345
X-SIGNATURE: bqMY5zOqDrCPTgUeVxujXvMi4Rmhj/WG0WtftO3PlJG8Lj3G8LHCz8G92+G6O9Y73khYGRSt4On//6caUmGX3A==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "X",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [flagAdvise]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_PaymentFieldFormats/customerNo_is_not_the_VA_suffix

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848389989720
X-PARTNER-ID: 12345
X-SIGNATURE: ViiHrM70oc6S8z+1qBm3SDxJozNHp8CN2/2nTZdUQqvR46jLFw5QYyTINZcnzW3to5/gvGGOfL+pSg5VItV4Dg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "999999999999999999",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890223"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [virtualAccountNo]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "999999999999999999",
    "virtualAccountNo": "12345678901234567890223",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-FMT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_UnregisteredVA

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848390308588
X-PARTNER-ID: 12345
X-SIGNATURE: LdFIiHl53xetmZw2QI/9VYNw9SpNhp7rn8YIwfZ7sNMsy6kcXq7PyqNAXMJg806tECAK8FOPW2cMcraWx1rL6Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890230",
  "inquiryRequestId": "INQ-404-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890230"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890230",
    "virtualAccountNo": "12345678901234567890230",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-404-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848390764290
X-PARTNER-ID: 12345
X-SIGNATURE: Qgrkb6L1V06kY8JSxtMrqhIB3NHK144rDKhIA9OdlN6x8iBOfEkp/D4GZqgvFKjdag6YUY2iuOFKr19a4b/FOQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890230",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-404-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890230"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042512",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890230",
    "virtualAccountNo": "12345678901234567890230",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-404-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_ExpiredBill

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848391070592
X-PARTNER-ID: 12345
X-SIGNATURE: dqlWszyY1S53Md28/FpAYD9t7Kh6OjvnrOwKwJwaEps1JV6Qb/v/VqCvMDQdribtMp63zNHtPcgKgMbMaAKIHQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890231",
  "inquiryRequestId": "INQ-EXPIRED-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890231"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890231",
    "virtualAccountNo": "12345678901234567890231",
    "virtualAccountName": "Budi Manjo",
    "inquiryRequestId": "INQ-EXPIRED-1",
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "expired transaction",
      "indonesia": "transaksi kadaluarsa"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848391248055
X-PARTNER-ID: 12345
X-SIGNATURE: RLbA1zleU41TD+VS8ooVtNYOFglb/ikPp2fktIHFb1VgYSxLKyB5NBwm1zNIkidV8jhQHwE0fg8iF5HvZkKXuQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890231",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-EXPIRED-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890231"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042519",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890231",
    "virtualAccountNo": "12345678901234567890231",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890231",
    "paymentRequestId": "PAY-EXPIRED-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "expired transaction",
      "indonesia": "transaksi kadaluarsa"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_Negative_StatusTransactionNotFound

#### 1. `POST /openapi/v2.0/transfer-va/status` → 404

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848391623919
X-PARTNER-ID: 12345
X-SIGNATURE: wwHq7pAkDGZ6+EsutpmJ8+1mC2zwMgASO3bbLfeH/m00t8x2nLzNBt0+koUqpq0vAQubbdQ46lGwtACq+wiR/g==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890232",
  "inquiryRequestId": "UNKNOWN-REQ",
  "partnerServiceId": "12345",
  "virtualAccountNo": "12345678901234567890232"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042601",
  "responseMessage": "Transaction Not Found",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Transaction Not Found",
      "indonesia": "Transaksi Tidak Ditemukan"
    },
    "partnerServiceId": "12345",
    "customerNo": "678901234567890232",
    "virtualAccountNo": "12345678901234567890232",
    "inquiryRequestId": "UNKNOWN-REQ",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "transactionDate": null
  },
  "additionalInfo": {}
}
```

### TestE2E_EveryRejectionCarriesStatusAndReason/inquiry_not_found

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848391841053
X-PARTNER-ID: 12345
X-SIGNATURE: fHs9XosoKwu3VEF/xG4wmmSfapc6r+H1fVz/pTGTfetiInj4tRAfOHE0YPQA9uFX0T5gUBEOUK3eMywJMqlkfg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890240",
  "inquiryRequestId": "INQ-ENV-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890240"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "12345678901234567890240",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-ENV-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_EveryRejectionCarriesStatusAndReason/inquiry_mandatory_field

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848392007461
X-PARTNER-ID: 12345
X-SIGNATURE: 3J1+g/v6dgtceQxSjAk2I6Ux1iLfupcOP7DnpDFV5hwTMSfyLZWaL+MQ/xIsZpbmeQ9ZiIeGmEQNAlKLaNSm1Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "partnerServiceId": "12345"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [customerNo]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

### TestE2E_EveryRejectionCarriesStatusAndReason/payment_not_found

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848392153997
X-PARTNER-ID: 12345
X-SIGNATURE: dtK9Xn1EmqG95/y9lOJ7FIYl9/xNiI9eiueds+iMX8hqvlO3e01//XSCXcLidQDa5JWTUXV1GFu7IV9k/H1JFg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890240",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ENV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890240"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042512",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "12345678901234567890240",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-ENV-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_EveryRejectionCarriesStatusAndReason/payment_mandatory_field

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848392342115
X-PARTNER-ID: 12345
X-SIGNATURE: 9EFGNLj/0GQYn9cjvbhR8teIt6s7bHAUfRZgLCjv3lsL0eiUNqnsliZGtlqzlvJrwlpF1SDCgikc7Nj4vkdrzw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "partnerServiceId": "12345"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [customerNo]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_EveryRejectionCarriesStatusAndReason/status_not_found

#### 1. `POST /openapi/v2.0/transfer-va/status` → 404

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848392461240
X-PARTNER-ID: 12345
X-SIGNATURE: hYcGu1AS80cAkSa1wlYSHKBPtPgZ4x5EoFlJAShk2cbIBcdkxh0dptlClTNvEEdehseLAx8k7gbXQsBmgHDFVw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890240",
  "inquiryRequestId": "ST-ENV-1",
  "partnerServiceId": "12345",
  "virtualAccountNo": "12345678901234567890240"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042601",
  "responseMessage": "Transaction Not Found",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Transaction Not Found",
      "indonesia": "Transaksi Tidak Ditemukan"
    },
    "partnerServiceId": "12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "12345678901234567890240",
    "inquiryRequestId": "ST-ENV-1",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "transactionDate": null
  },
  "additionalInfo": {}
}
```

### TestE2E_EveryRejectionCarriesStatusAndReason/conflict

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000003
X-PARTNER-ID: 12345
X-SIGNATURE: kp1xQu8YvOr8fRn54+N4pw3Qfh0CyDsJIty8tf81yQ+8UAjlecBZddNpE1f8oEUlAXrcOxMIXJ5WmjQYbHgLSw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890240",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-ENV-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890240"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042512",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "12345678901234567890240",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-ENV-2",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_AuthFailuresCarryBareBody

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848392775845
X-PARTNER-ID: 12345
X-SIGNATURE: al5X1xpKmlnSSYZbpGPyFIUMmUT/KU4yszJ0pG020KVL8Gvj612z/L5mRthxaIUpZIx/Goprj6au9pMIRN7CGQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890241",
  "inquiryRequestId": "INQ-AUTH-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890241"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012400",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_ResponseCodesCarryTheCalledServiceCode

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848392919342
X-PARTNER-ID: 12345
X-SIGNATURE: 6yCPPSYLO2Y12WqhfC3kWeO+2EF6Ix3aNNAUvpNX97K7KoX4K/oIyfF/KIH/x7L8cyXlpsyK4uNh2WUWPrVrkQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002400",
  "responseMessage": "Bad Request",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "inquiryRequestId": "",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848393022301
X-PARTNER-ID: 12345
X-SIGNATURE: 7zGNoXZ1Go4jOpUhmRaa6XkJYgklF0BfmZHQgHIlh7mLYGnauX4xqyG9Gyep330CBtnG/IjveUna7IXW+7lnGw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002500",
  "responseMessage": "Bad Request",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "virtualAccountName": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 3. `POST /openapi/v2.0/transfer-va/status` → 400

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848393081938
X-PARTNER-ID: 12345
X-SIGNATURE: 6pq8OrXRSwYHdBkOHHtLrTQgI+9wDYebeF9icLEf3ljjEmqsZ9AbWGlYohQVGAwpIEUpqT++MX/8qd5JSV6Rlg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002600",
  "responseMessage": "Bad Request",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "partnerServiceId": "",
    "customerNo": "",
    "virtualAccountNo": "",
    "inquiryRequestId": "",
    "paymentRequestId": "",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "transactionDate": null
  },
  "additionalInfo": {}
}
```

### TestE2E_UndocumentedHeadersAreIgnored

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
Idempotency-Key: d3b07384-d9a0-4c9b-9b0e-1f2a3b4c5d6e
X-CLIENT-KEY: some-client-key
X-Device-Id: device-1
X-EXTERNAL-ID: 1786123848393196908
X-PARTNER-ID: 12345
X-SIGNATURE: QD3LP92MOkVG5+G4uYgRiGT8EVZw3UNU59A2W3oRTPrH0aQFU3+rMGq/ydGJ4s/3Lzgxffhsk7gSav8ZdxcMOg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890260",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-HDR-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890260"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890260",
    "virtualAccountNo": "12345678901234567890260",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890260",
    "paymentRequestId": "PAY-HDR-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_UndocumentedHeadersDoNotBreakTheSignature

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
Idempotency-Key: irrelevant
X-EXTERNAL-ID: 1786123848393458724
X-PARTNER-ID: 12345
X-SIGNATURE: y3SCbMYxjnUmiacg3niiOA2i7aJFjsazxaU5cPziazvYYKqjMRIcHNO3xxoHF0ojnxTg8PdgSvOM7/CbW7CgSA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890261",
  "inquiryRequestId": "INQ-HDR-1",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890261"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890261",
    "virtualAccountNo": "12345678901234567890261",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-HDR-1",
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "01",
    "inquiryReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    }
  },
  "additionalInfo": {}
}
```

---

## Multi-vendor / multi-merchant isolation

### TestE2E_MultiVendor_EachVendorUsesItsOwnCredentials

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848371550717
X-PARTNER-ID: 12345
X-SIGNATURE: 0mcxpgMwf0277QAyDa9XRkeC3ttDHB8WFn1i4wCPy16nGz9rTBiPbMkOqR2jjrB2iTl/kHlCEnBxJed+yJLTdQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890300",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890300"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890300",
    "virtualAccountNo": "12345678901234567890300",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890300",
    "paymentRequestId": "PAY-MV-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786123848372301722
X-PARTNER-ID: 67890
X-SIGNATURE: xZqr4zhmEhKR54ZxOZvkv+xxnLkSlywXjWLuRvDgx77G1m7X8q9Xot3cI3v9va7TSzyovEyCzXEE/WWRChpR/Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890301",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-MV-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "67890678901234567890301"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "67890",
    "customerNo": "678901234567890301",
    "virtualAccountNo": "67890678901234567890301",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890301",
    "paymentRequestId": "PAY-MV-2",
    "paidAmount": {
      "value": "2000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "2000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_MultiVendor_CredentialsDoNotCrossOver

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848372603934
X-PARTNER-ID: 12345
X-SIGNATURE: kl6yph3td2Ykg/JGun1U8XcMl4D0T7WuSQxjzJsJP/dYHywKzL5ZOkrkFCFVXoFArAZwhe40D7QGc5OEGBsleA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890302",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MV-3",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890302"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_MultiVendor_PartnerAndChannelPinnedPerVendor

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786123848372862951
X-PARTNER-ID: 12345
X-SIGNATURE: X3XuClbhCbSNauBo50rQwdxltPrMCT0ry13QFneM7Y7V8E5GfuG33G506kRd2HuyJ6TSus38ktp6ftsth/eq5Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890303",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MV-4",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890303"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Unknown channel]",
  "data": {}
}
```

### TestE2E_MultiVendor_BodyHashEncodingIsPerVendor

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786123848373123742
X-PARTNER-ID: 67890
X-SIGNATURE: xPz7isEbM5kllU59RTNKR0AheMRB+cPJp+PC4Swo+tsONSlB0qBDW0KLId99tOhW0Si72YEcaOEDo4LqboBIeg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890304",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-ENC-OK",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "67890678901234567890304"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "67890",
    "customerNo": "678901234567890304",
    "virtualAccountNo": "67890678901234567890304",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890304",
    "paymentRequestId": "PAY-ENC-OK",
    "paidAmount": {
      "value": "5000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "5000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786123848373438479
X-PARTNER-ID: 67890
X-SIGNATURE: BngMVfuTg5Sr2HpjtnLE62BnystRiz+9B+5Cxfmsrt4EhmPzcXYWT7tNfzCnwI+YRMHFObO0VBj5bf63/8+NkA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890304",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-ENC-BAD",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "67890678901234567890304"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### TestE2E_MultiVendor_FieldStrictnessIsPerVendor

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848373627345
X-PARTNER-ID: 12345
X-SIGNATURE: ZapdI+K7Sxq43s0677flTQtDty7ABRjDzUi/Dpevdu/gjEyXTV39GZzPkjSwJyngkBOhCxUaUB5WkPfzuH1GgA==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890305",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-STRICT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "virtualAccountNo": "12345678901234567890305"
}
```

**Response**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [virtualAccountName]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890305",
    "virtualAccountNo": "12345678901234567890305",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-STRICT-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Rejected",
      "indonesia": "Ditolak"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786123848373821732
X-PARTNER-ID: 67890
X-SIGNATURE: A8AVMf3Ej1ndHXssVFTvb7rMOyjmY1aBbKX1+d9sTJti/lZq7L07jym6B5h2VBt89jexuDtM+M3wzOmcTG3vrQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890306",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-LAX-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "virtualAccountNo": "67890678901234567890306"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "67890",
    "customerNo": "678901234567890306",
    "virtualAccountNo": "67890678901234567890306",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890306",
    "paymentRequestId": "PAY-LAX-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48.373936069+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_MultiMerchant_VAsAreIsolatedByPartnerServiceID

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848374135684
X-PARTNER-ID: 12345
X-SIGNATURE: Y8Y862GOgtb8b/hKJd4r2V7zdttiaqtH8W0jXWZVTPC2OJqNLEonJnppCn8LJeNeU3jKjyb/FhXWERcb/ydrKw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890310",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890310"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890310",
    "virtualAccountNo": "12345678901234567890310",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890310",
    "paymentRequestId": "PAY-MM-1",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848374362717
X-PARTNER-ID: 67890
X-SIGNATURE: VaGQ5rDARNpCY6mqBu6eiPXXhGViYgdI9K93PW5iFSQb5tek4iu7amczOXs2N3ZqS7Dc/hEFo+iSBMNsvKVEyQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890311",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-MM-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "67890678901234567890311"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "67890",
    "customerNo": "678901234567890311",
    "virtualAccountNo": "67890678901234567890311",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890311",
    "paymentRequestId": "PAY-MM-2",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 3. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848374523158
X-PARTNER-ID: 67890
X-SIGNATURE: 1/bO0huKmKWXstXjdIfD07hFNXA2dzFxIRqU11J3ap3gWDlt7LcCno4pdpFiZbcDLEBS1yR3DcfivJQ13vt9Uw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890312",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-MM-3",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "67890678901234567890312"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042513",
  "responseMessage": "Invalid amount",
  "virtualAccountData": {
    "partnerServiceId": "67890",
    "customerNo": "678901234567890312",
    "virtualAccountNo": "67890678901234567890312",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-MM-3",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Invalid Amount",
      "indonesia": "Nominal pembayaran tidak sesuai"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_MultiMerchant_UnknownVAUnderKnownPartnerRejected

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848374765553
X-PARTNER-ID: 12345
X-SIGNATURE: H8EZaPgoDAHoR3KIU1yjnyPzxE0wcfNERKc6x5uERrNRr+UjBIt36in449vl0OAwHo/XCX73wNKCkadJzPfJqg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890399",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-MM-4",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890399"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042512",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890399",
    "virtualAccountNo": "12345678901234567890399",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-MM-4",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Virtual Account Not Found",
      "indonesia": "Virtual Account Tidak Ditemukan"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_MultiMerchant_CallbacksGoToTheOwningMerchant

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848375045785
X-PARTNER-ID: 12345
X-SIGNATURE: jq4RPfR69HUhxT35HWsLNWmPdSMvIYtWfwg70ZI+VLLdxraDuNPbnckVVY1Xp5HHDfnR9LtOfmQHW6ShnLy+mw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890320",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "PAY-CB-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890320"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890320",
    "virtualAccountNo": "12345678901234567890320",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890320",
    "paymentRequestId": "PAY-CB-1",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848375304577
X-PARTNER-ID: 67890
X-SIGNATURE: 5czzHX2igWN0KBLGIbrDKR/ht+hJVq5Hl/JefuSBT16+y3HQAJ9SoZfM6frScQYiuJIWTHibOWn+mdJzmfnbww==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890321",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "67890",
  "paymentRequestId": "PAY-CB-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "67890678901234567890321"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "67890",
    "customerNo": "678901234567890321",
    "virtualAccountNo": "67890678901234567890321",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890321",
    "paymentRequestId": "PAY-CB-2",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

### TestE2E_MultiVendor_CommaSeparatedPartnerAllowList

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848375602970
X-PARTNER-ID: 11111
X-SIGNATURE: P2txWLLhSvhFbwRT9fY/qEDv/0omdJhBBqm+pGizUtHya+V+dU2E6wqTPd/d3lFa6r3mzs0x+g4NrF9sTGeufw==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890330",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "11111",
  "paymentRequestId": "PAY-ALLOW-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "11111678901234567890330"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "11111",
    "customerNo": "678901234567890330",
    "virtualAccountNo": "11111678901234567890330",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890330",
    "paymentRequestId": "PAY-ALLOW-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848375822259
X-PARTNER-ID: 22222
X-SIGNATURE: UADK7DlrI7zAyt3XxZgfnSIBKGcOaqvC/axOO+Q2KREvcMU6c+YNneWnS6KDeCOEZf46cF/n0c6x5FIjaDIlGg==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890330",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "11111",
  "paymentRequestId": "PAY-ALLOW-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "11111678901234567890330"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012500",
  "responseMessage": "Unauthorized. [Unknown client]",
  "data": {}
}
```

---

## BCA conformance regressions

### TestE2E_PaymentReusingInquiryRequestID_IsAFirstPaymentNotADoubleFlag

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848401857742
X-PARTNER-ID: 12345
X-SIGNATURE: 2RpOEdHgsM0akSYbIecke2YK25ahFeqjfVSP+6oV033IRqt1uhb91xiN7irWIWZxJubu4CJFEbl0pb+/UdwE9Q==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890900",
  "inquiryRequestId": "202202111031031234500900",
  "partnerServiceId": "12345",
  "trxDateInit": "2026-08-08T00:30:48+07:00",
  "virtualAccountNo": "12345678901234567890900"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890900",
    "virtualAccountNo": "12345678901234567890900",
    "virtualAccountName": "Budi Manjo",
    "inquiryRequestId": "202202111031031234500900",
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "subCompany": "00000",
    "billDetails": [],
    "freeTexts": [],
    "inquiryStatus": "00",
    "inquiryReason": {
      "english": "Success",
      "indonesia": "Sukses"
    }
  },
  "additionalInfo": {}
}
```

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848401968577
X-PARTNER-ID: 12345
X-SIGNATURE: urx//5XwkpmfdiKjL1iMt88IgGdUpugxT0Rfch/44GX2YPBcTNl73Ng5O18qDG4sWho4hrnlkeT+vhx1smM2WQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890900",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "202202111031031234500900",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890900"
}
```

**Response**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890900",
    "virtualAccountNo": "12345678901234567890900",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890900",
    "paymentRequestId": "202202111031031234500900",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

#### 3. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786123848402090990
X-PARTNER-ID: 12345
X-SIGNATURE: urx//5XwkpmfdiKjL1iMt88IgGdUpugxT0Rfch/44GX2YPBcTNl73Ng5O18qDG4sWho4hrnlkeT+vhx1smM2WQ==
X-TIMESTAMP: 2026-08-08T00:30:48+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890900",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "partnerServiceId": "12345",
  "paymentRequestId": "202202111031031234500900",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "trxDateTime": "2026-08-08T00:30:48+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "12345678901234567890900"
}
```

**Response**

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "responseCode": "4042518",
  "responseMessage": "Inconsistent Request",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "678901234567890900",
    "virtualAccountNo": "12345678901234567890900",
    "virtualAccountName": "Budi Manjo",
    "trxId": "trx-678901234567890900",
    "paymentRequestId": "202202111031031234500900",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:48+07:00",
    "referenceNo": "12345678901",
    "flagAdvise": "N",
    "paymentFlagStatus": "00",
    "paymentFlagReason": {
      "english": "Success",
      "indonesia": "Sukses"
    },
    "billDetails": [],
    "freeTexts": []
  },
  "additionalInfo": {}
}
```

