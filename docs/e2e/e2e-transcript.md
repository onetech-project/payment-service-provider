# SNAP Virtual Account — end-to-end request/response transcript

Every request this suite puts on the wire and every response it got back,
captured from an actual run of `test/e2e`.

The suite drives the production router, idempotency middleware, SNAP auth
middleware, handler and usecase against an in-memory repository, so the
headers, `stringToSign` inputs, service codes and JSON envelopes below are
the real ones.

Regenerate with:

```sh
E2E_TRANSCRIPT=docs/e2e/e2e-transcript.md go test ./test/e2e/...
```

- Generated: `2026-08-10T09:35:27+07:00`
- Commit: `f73c636`
- Exchanges: 110 across 81 scenarios

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810432663
X-PARTNER-ID: 12345
X-SIGNATURE: yBOfL/W9KOjMayJ7jFOvznEnQ321fGKyqZX4dcs1RwPWgUYq36l2nFFzZfa06M5a7KMzitCAveNBQYEm9bTbWg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890123",
  "inquiryRequestId": "INQ-FIXED-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890123"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890123",
    "virtualAccountNo": "   12345678901234567890123",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810518303
X-PARTNER-ID: 12345
X-SIGNATURE: ncc+V4vxYuZwU/6TzeoX5dTMJh1DEAvhTsWpqJgYrS4oRtDqhshGRpRNSQh42zGWC2sYJQQj2tKSK9gM03bBEw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890123",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FIXED-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890123"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890123",
    "virtualAccountNo": "   12345678901234567890123",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-FIXED-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 3. `POST /openapi/v2.0/transfer-va/status` → 200

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810603423
X-PARTNER-ID: 12345
X-SIGNATURE: LJ2iNCdpJv8J/6AYsL7xHkPNtyeGFopa+CPoxgIxqtRp/suh0KJ6Y+9qAL1tVAR+htnOSKQJr657iUXsDwsclA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890123",
  "inquiryRequestId": "PAY-FIXED-1",
  "partnerServiceId": "   12345",
  "virtualAccountNo": "   12345678901234567890123"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890123",
    "virtualAccountNo": "   12345678901234567890123",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
    "transactionDate": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810718111
X-PARTNER-ID: 12345
X-SIGNATURE: V5TpdTCwhUScG7irLByfDT5fJBE3SLbQT62JuYDr7lkbDDYYoZZZtpXs3w28/0u1RErH5T+sqtrRB/KQc1Wp9g==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890124",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-UNDER-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890124"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890124",
    "virtualAccountNo": "   12345678901234567890124",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810860560
X-PARTNER-ID: 12345
X-SIGNATURE: YboEkHRPQVFzKMsKH5Dtnl9VyXk/o4oO5JabK43mosM/zLQWP4PvNaDbDbWNPrITt+tWpqeVqcFs1gRqOHYpag==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890125",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NUM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890125"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890125",
    "virtualAccountNo": "   12345678901234567890125",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-NUM-1",
    "paidAmount": {
      "value": "250000",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_FixedBill_PaidBillRejectedOnSecondPayment

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810992651
X-PARTNER-ID: 12345
X-SIGNATURE: ABNrZ/lXVotftj6ydfVQ0s5TubDYj71akQpy+K2J7jJIarpmcwrqxFGrTsCbMhyjGfc7aQ/m7GQ28SdF1BiEVQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890126",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FIRST",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890126"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890126",
    "virtualAccountNo": "   12345678901234567890126",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-FIRST",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327811096556
X-PARTNER-ID: 12345
X-SIGNATURE: sN9sZaUtzo/Z+wvYTN/byNlObf3VV3m0BeFMqpahkyD0PQxxgkRijqUwaP3FHYvShBWhGQd39tYPcw+93cUVAw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890126",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-SECOND",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890126"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890126",
    "virtualAccountNo": "   12345678901234567890126",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-SECOND",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327811225802
X-PARTNER-ID: 12345
X-SIGNATURE: Fzgj6wTdf+tdPHWSS8ZCR0mhuiA/S2iKGGHvt0SigXZbAuXMg88MYJymEojR2kA9uiQ+Mv3MoMySzlQQ8QiPTQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890127",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-PAID-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890127"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890127",
    "virtualAccountNo": "   12345678901234567890127",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-PAID-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327811308205
X-PARTNER-ID: 12345
X-SIGNATURE: xByPsNgNxz51BJPBLZyCiKzfOp4gyRGBU7dk3FeEc6viwunA7AA+KMcBlzEX5Ytg1iYzijkNCphjI5DmopcFSQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890127",
  "inquiryRequestId": "INQ-PAID-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890127"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890127",
    "virtualAccountNo": "   12345678901234567890127",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327811481192
X-PARTNER-ID: 12345
X-SIGNATURE: dmka45pgpcFuPYv11/wh97eLtfbnRuAzEK6F9TvaXvBxPiY4tQLbF7edkn2B71RroJ5AAP0lNNf9tOUj/0sh1w==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890130",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890130"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890130",
    "virtualAccountNo": "   12345678901234567890130",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-1",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327811559231
X-PARTNER-ID: 12345
X-SIGNATURE: HpJLlGdmJcpu2G2xtBo4Z0JyvlK72Yp8ukABV6bwHGqAbvtEVEOyHs2X844TTQRXjqpOJCa+nLjGP5Qk79aRyQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890130",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "40000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "40000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890130"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890130",
    "virtualAccountNo": "   12345678901234567890130",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-2",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "40000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_VariableBill_NoExactAmountCheck

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327811682955
X-PARTNER-ID: 12345
X-SIGNATURE: bWnNW4WSuNyTfwQdKT6XvJqNndNR2kNyeuA0w6E8owdqlGSvOK0xOLX4gkFCGcpwRgcPHwd3osCqni//4FH9wA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890131",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-PART",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890131"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890131",
    "virtualAccountNo": "   12345678901234567890131",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-PART",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_NoBill_PayableRepeatedly

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812248566
X-PARTNER-ID: 12345
X-SIGNATURE: +eaKrkZz2/myoYEs1iCOsr4yBeFkDJO/r0b0ZTjWHyzR1qw1/nCr1wBWYN+gdgEwYRUuLXSAq8koH/IfnLjSGw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "inquiryRequestId": "INQ-NOBILL-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890140"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "   12345678901234567890140",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812317316
X-PARTNER-ID: 12345
X-SIGNATURE: xqLYFHnxc0ZK1ZIxpm+8o3min7ytn3BkuxK4l+curNyYlSh6gZY94g7EvH7+rxpouqRG8pic9Z8Z9VPHyxMOcw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NOBILL-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890140"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "   12345678901234567890140",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-NOBILL-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 3. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812385035
X-PARTNER-ID: 12345
X-SIGNATURE: 3PhFOW+eFKQRcuWZHs02d4KklTtuoYJ1gGFFthLw17jXsRMyCHztq44nSKYYhFo3s2BWd+vBnNaCrkT25b4r4Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "35000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NOBILL-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "35000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890140"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "   12345678901234567890140",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-NOBILL-2",
    "paidAmount": {
      "value": "35000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "35000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 4. `POST /openapi/v1.0/transfer-va/inquiry` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812435068
X-PARTNER-ID: 12345
X-SIGNATURE: ykTnp8oSbYDW+h/Q7rOZh1b0B8O4y8K1TS+UKDSToYdOWRKZY1M8iXE0huCjGpPyn7qXtCmHrYS7eAChtW9fkw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890140",
  "inquiryRequestId": "INQ-NOBILL-2",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890140"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890140",
    "virtualAccountNo": "   12345678901234567890140",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812506271
X-PARTNER-ID: 12345
X-SIGNATURE: Xy5MniXcXm/Tw+DCt4RoyvzD4TXKh5LMBrRpZqkTIGm9cTPK+M4QSqOUEYsgqNEo6u+dKpbn4CxpLn+G3hR/Qg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890141",
  "inquiryRequestId": "INQ-EXP-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890141"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890141",
    "virtualAccountNo": "   12345678901234567890141",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812567285
X-PARTNER-ID: 12345
X-SIGNATURE: Ttpn9sqbJpLp3GjcX/T8+QsZ9kRteT7Y9rsmxVQx0WAOdVlg9FFXxEdrgvLgcefFbdpt2CM3eN/4dmeSXgODAA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890141",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-EXP-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890141"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890141",
    "virtualAccountNo": "   12345678901234567890141",
    "virtualAccountName": "Budi NoBill",
    "paymentRequestId": "PAY-EXP-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812644053
X-PARTNER-ID: 12345
X-SIGNATURE: AwXbqjNwiWFr/z8CTQK7XFQBvPLXaG6t1x6Pak9QC0UDqt4WIravAFnw9zEjp8kgvAuvZtPXcOhDYJ0ij3pzpw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890150",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-DF-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890150"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890150",
    "virtualAccountNo": "   12345678901234567890150",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-DF-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812702548
X-PARTNER-ID: 12345
X-SIGNATURE: AwXbqjNwiWFr/z8CTQK7XFQBvPLXaG6t1x6Pak9QC0UDqt4WIravAFnw9zEjp8kgvAuvZtPXcOhDYJ0ij3pzpw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890150",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-DF-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890150"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890150",
    "virtualAccountNo": "   12345678901234567890150",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-DF-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_AdviceRetry_ReplaysOriginalSuccess

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812773322
X-PARTNER-ID: 12345
X-SIGNATURE: LiSmJKVhBOFAPBV3f5mp9cdY2Y5m6aZYeUKDqsoTFfMrzj9mCb+dD21vJnHt/Tt5pPRNyRHft4Q2vBnpt2GROA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890151",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ADV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890151"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890151",
    "virtualAccountNo": "   12345678901234567890151",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-ADV-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812817597
X-PARTNER-ID: 12345
X-SIGNATURE: 2oGpOubuPxgTkU9EcyhujvBCr/9MTr6OK5W54t0puae5biEHrURdCErEkxqnkPvKJmK6zOkiwWpxuI6uUhPYyw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890151",
  "flagAdvise": "Y",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ADV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890151"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890151",
    "virtualAccountNo": "   12345678901234567890151",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-ADV-1",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_SameExternalIDSamePayload_ReplaysCachedResponse

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000002
X-PARTNER-ID: 12345
X-SIGNATURE: wJ8/uE7O6tOIXapRGEaI4iJd3Ap6vJ1vmCtSIcMWiaaStCW3VpKe9jFnY6pWHrZb/6ctcPDLPC5hw/hGMOF+XA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890160",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-IDEM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890160"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890160",
    "virtualAccountNo": "   12345678901234567890160",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-IDEM-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000002
X-PARTNER-ID: 12345
X-SIGNATURE: wJ8/uE7O6tOIXapRGEaI4iJd3Ap6vJ1vmCtSIcMWiaaStCW3VpKe9jFnY6pWHrZb/6ctcPDLPC5hw/hGMOF+XA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890160",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-IDEM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890160"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890160",
    "virtualAccountNo": "   12345678901234567890160",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-IDEM-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_VariableBill_RepeatedInstalmentNotCreditedTwice

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327812972413
X-PARTNER-ID: 12345
X-SIGNATURE: 4BnI1+/XbVQlhEi6YrB/RoLR/P1ynSSf6f4gXTpiev6Z/0C5yi/M6YBFb0wgHWLYNDOfeRaMrmyx5mCAhIxOag==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890132",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-DUP",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890132"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890132",
    "virtualAccountNo": "   12345678901234567890132",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-DUP",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813017706
X-PARTNER-ID: 12345
X-SIGNATURE: 4BnI1+/XbVQlhEi6YrB/RoLR/P1ynSSf6f4gXTpiev6Z/0C5yi/M6YBFb0wgHWLYNDOfeRaMrmyx5mCAhIxOag==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890132",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-DUP",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890132"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890132",
    "virtualAccountNo": "   12345678901234567890132",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-DUP",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_VariableBill_AdviceRetryReplaysWithoutDoubleCrediting

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813079347
X-PARTNER-ID: 12345
X-SIGNATURE: gfq9ihuw35UYPKMyKgSkMMdCWHjrEsRVnRFFIHiym08SMVc3HcrVX4MNy+xdcbeFsHRxK1D407qwyl7d4z7l/g==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890133",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-ADV",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890133"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890133",
    "virtualAccountNo": "   12345678901234567890133",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-ADV",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813119670
X-PARTNER-ID: 12345
X-SIGNATURE: bSXij4vad1Az0vE4waPxS0hQh2B5UAzhPA0gUCb7yYFu7DpYCD6STwNjPotwOkdMDojP+DT2meeBWYdcVgnUgA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890133",
  "flagAdvise": "Y",
  "paidAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-VAR-ADV",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "60000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890133"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890133",
    "virtualAccountNo": "   12345678901234567890133",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-VAR-ADV",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_Status_ResolvesByPaymentRequestID

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813175301
X-PARTNER-ID: 12345
X-SIGNATURE: Mmz6T64/btQ/+YaJEUhZuYxG9WtMM3oUD8qxosVFliIfwPsBN/lMqtm84ZYxcCKIOtuGIYPJm7sR8zrR2bybvg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890170",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ST-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890170"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890170",
    "virtualAccountNo": "   12345678901234567890170",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-ST-1",
    "paidAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "12000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v2.0/transfer-va/status` → 200

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813214911
X-PARTNER-ID: 12345
X-SIGNATURE: 39e6Y7W9R3QrCJA+QrYaCc9EeAhNhbgCwuZhWyCbcnpua7UQZwr5Ig2lAn8gGLTCCfsKm/WiEZTplifTc+oGYw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "customerNo": "678901234567890170",
  "inquiryRequestId": "INQ-NEVER-PERSISTED",
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ST-1",
  "virtualAccountNo": "   12345678901234567890170"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890170",
    "virtualAccountNo": "   12345678901234567890170",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
    "transactionDate": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813276580
X-PARTNER-ID: 12345
X-SIGNATURE: fkm9NOrFeJf/bldUmc6gUD4gLMRfSvaUAJfYK9DdgpM/FJcmiKoslYRD6eTg4TGi7/9mYTPZbIeVedA6WmZrPw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "customerNo": "678901234567890171",
  "inquiryRequestId": "INQ-UNKNOWN",
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-UNKNOWN",
  "virtualAccountNo": "   12345678901234567890171"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890171",
    "virtualAccountNo": "   12345678901234567890171",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802911535
X-PARTNER-ID: 12345
X-SIGNATURE: IUfDlhoA/DvEa0PW/XBEK4NzKtWgiAByDEx1xAxn/6pxb6Wz7ldTcYITSW4DX1eWogGcYu42LXGcVVGfpmzDhA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803010018
X-PARTNER-ID: 12345
X-SIGNATURE: not-a-signature
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803063127
X-PARTNER-ID: 12345
X-SIGNATURE:  
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 00000
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803099164
X-PARTNER-ID: 12345
X-SIGNATURE: ZHgXFonpeKzGKHpC3SFOKSePRMMKh2TiVj6ZZ7b2BH/D1d6hhlor8O30XyI2d2vWP1NZZ9jPdbrnEuYwjlrNdw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803150119
X-PARTNER-ID: 99999
X-SIGNATURE: AEnzYe8OcwKLrsPE+IeTl4B9DYPDM5ue0UaGMGbNmPZRmn5J5XW2WjuRIsPZ8jgoK9W0as1ihpmHI9nK8h06JA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803200449
X-PARTNER-ID: 12345
X-SIGNATURE: FqwaQlRqF739GfHwTijWIoh8xWoGFi20Ah9u58m9/VOe0J9UIxC+ITxTfe9CgyM7hHJePeCI3osqQ3m3z/BQ3A==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "   12345",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803304815
X-SIGNATURE: ZHgXFonpeKzGKHpC3SFOKSePRMMKh2TiVj6ZZ7b2BH/D1d6hhlor8O30XyI2d2vWP1NZZ9jPdbrnEuYwjlrNdw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890200"
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

### TestE2E_Negative_Auth/missing_Authorization_on_a_ClientID-onboarded_vendor

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803421429
X-PARTNER-ID: 12345
X-SIGNATURE: JzMHoRxVIxJGyjVE16u7bjSQFa5VUxkKunCf1lH6QHz7KJd7gpXgyU3bi5Vg4N2NPiPDwjhdeH81vzz3z3CvTA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012401",
  "responseMessage": "Invalid Token (B2B)",
  "data": {}
}
```

### TestE2E_Negative_Auth/accessToken_this_issuer_never_minted

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer forged-token
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803466049
X-PARTNER-ID: 12345
X-SIGNATURE: n0O+SW5v1M96ACwwR4DTLrGSDsLyYSVyiiN4hkz2xM0pGfNGva05Wz8Hok5ZJLYKAuBG9c5Ld6PTljFm3TXQsA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890200"
}
```

**Response**

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "responseCode": "4012501",
  "responseMessage": "Invalid Token (B2B)",
  "data": {}
}
```

### TestE2E_Negative_Auth/X-TIMESTAMP_not_parseable

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803515645
X-PARTNER-ID: 12345
X-SIGNATURE: 4ROB68ZcVv5N8y4KC28SHAGaedMI/C5tQJ7zzbUTVc5VBZCMr/NSMacJtXxiAeo8b5zmpCWvy0AtxCGbXtnOdQ==
X-TIMESTAMP: 2026-08-06 10:00:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890200",
  "inquiryRequestId": "INQ-NEG-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803573145
X-PARTNER-ID: 12345
X-SIGNATURE: 6Fg30L84/RTYLdM9OMMnj76VrlF2BP1BejrRRd/MK9q0rh1MB0iooXFStQEOSMARh1HIz7pvui3WF9FcIwJ7eA==
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
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NEG-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890200"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803636225
X-PARTNER-ID: 12345
X-SIGNATURE: Nmeh+exLifcgApLV6j4nbooPyd79Kg6wQE2pd2udbAJ+rBkJeHiEuPUQ+h8jEpZhGz+CL/SbndQlDCCd/tcO7w==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890201",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ENC-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890201"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803696334
X-PARTNER-ID: 12345
X-SIGNATURE: NNTEQERVAKUxqMF8h2FDuxqYNFqVHQQkwjE8ElLjzMY4aG2zzgY7NqhpoNZzZ+vdXHpf3sh79GMFF/c+VwJvpg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890202",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "9999.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-TAMPER-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890202"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327803758586
X-PARTNER-ID: 12345
X-SIGNATURE: KynoNz5NmktNCNRESsztFSQF8C4XAel/D/ju9B5WySS7n77FZmQxZXostvqehuK+XKq3XPyI7aHj2CkrQ3/NeA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "partnerServiceId": "   12345",
  "customerNo": "678901234567890203",
  "virtualAccountNo": "   12345678901234567890203",
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
  "trxDateTime": "2026-08-10T09:35:27+07:00",
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890203",
    "virtualAccountNo": "   12345678901234567890203",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-PRETTY-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-PARTNER-ID: 12345
X-SIGNATURE: h5KC/bRimjDpPu63LuQYEAprlwFwA/tnhLcNKd4noBV1aQebWPdgjjJfNu4fY1bXo9MmzbjPcioNnD0sfmGR0Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890210",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-EXT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890210"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 9999999999999999999999999999999999999
X-PARTNER-ID: 12345
X-SIGNATURE: 9FjqT64bfYHmoUF5MUYWvVZYL62i0CMEz277Fa1jM0Ji12ZXTy5zLI5Fxa0HGcQ4H3kG3giySa/fK4WS4rE3Ig==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890211",
  "inquiryRequestId": "INQ-EXT-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890211"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000001
X-PARTNER-ID: 12345
X-SIGNATURE: pDUdrxDRqFqfEvYKdsjRJnKtKjbZ8YOo7el0M+E3dDmI93pIL2F9qaMjNLlwgbMuoNaADu0F+InItBqXVspGKg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890212",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-EXT-A",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890212"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890212",
    "virtualAccountNo": "   12345678901234567890212",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-EXT-A",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 409

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000001
X-PARTNER-ID: 12345
X-SIGNATURE: gxhVBuiOU28a2t4YVLVB+d+o9qsaYroCJxHp++GfpYNRGB8NR94pCWen98X12vUkrahR55GGWeExjR8dkmJRkw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890212",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-EXT-B",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890212"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327804085518
X-PARTNER-ID: 12345
X-SIGNATURE: jTIIGJixbUewsojGnuh/c3rasZW+wI4SsTf7WeMTiWjNDYFSUOVZqA4uugePbbUDTvPtr1EeV0uaYAxcZjVWfw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327804144943
X-PARTNER-ID: 12345
X-SIGNATURE: 0DdBGdBMSfyiRHRMuXFXxQ1Y5hnDnwkctbnGoQxQF76Jwbq3/gZrtBGdQuSxL1YvknOAqmrapmFF0cXMjEf+7Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327804201941
X-PARTNER-ID: 12345
X-SIGNATURE: QhAvOyRwhZdd4Xk0KKEhNDNEg7n3cIQ5Udsl1nwV53e4bpsEJ383dFejOYF+ifJeXfJhq/mzcR5llBTDYR37+A==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327804522563
X-PARTNER-ID: 12345
X-SIGNATURE: tU7iL7yThfh9EypWRR8H46/vl+MnGCXvW6+3GDTnQhT/5MnS8UMuAxSGAlT/PzjQKc2gTgWuwp5Ue2BZceOe/Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890220",
  "inquiryRequestId": "INQ-MAND-1",
  "partnerServiceId": "",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890220"
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
    "virtualAccountNo": "   12345678901234567890220",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327804868564
X-PARTNER-ID: 12345
X-SIGNATURE: BtJW/10s6/7o+Q+KzH7iW3MnfWJH+x/jyBcjHBN/2nb04SnV8osCdMgzF2xy1dkOZHFUYxZAfQcNM0uIfd8unw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "",
  "inquiryRequestId": "INQ-MAND-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890220"
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
    "partnerServiceId": "   12345",
    "customerNo": "",
    "virtualAccountNo": "   12345678901234567890220",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327805454164
X-PARTNER-ID: 12345
X-SIGNATURE: vkC2BJXteIh4aQp2v5MNoyt4pXZjwtY2q//WtWerJwDcM/dF+LcLjoSBFHicLP74f4zxKKluk80QUEI0BadsAg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890220",
  "inquiryRequestId": "INQ-MAND-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
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
    "partnerServiceId": "   12345",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327805591831
X-PARTNER-ID: 12345
X-SIGNATURE: kK1CA+ShqgkH7crh6AMlXQnObemb5djdqt1ytg7IQNaEwOJfGPRXaDteZJUIk7bWOTINB5JveSgkuajZB7fnHQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890220",
  "inquiryRequestId": "",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890220"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890220",
    "virtualAccountNo": "   12345678901234567890220",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327805762906
X-PARTNER-ID: 12345
X-SIGNATURE: /+1vOr0AzWw+MtYCyaTaL1TFj43mdxNN27Y2Yf9vt83l+rAxu7Fp5CtugomAlYi6WLn8Ja5Nt3+nu5OizHKSyw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327805922938
X-PARTNER-ID: 12345
X-SIGNATURE: fUuXcoW8eF4NwSgucPZK0SarcbKo/wpEPjon2I07xOqEwamu4ZXpeN7+4lbvt704ZuqSY1Gb0mOMtioemWbM6Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327806069346
X-PARTNER-ID: 12345
X-SIGNATURE: QlBfxuAxJPcGyPK3b7sRjjRtfGic8RaVhi47Beg8Fw/zIycfpIjMY+4Sxs9nruwyM5nOrSkPRDbJy6OwmSqQDA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327806220989
X-PARTNER-ID: 12345
X-SIGNATURE: AAh5OTpzkkUgb5mmQNnmiaCj/L0ByUhlxjALV9UjgMRSUpf3KNSdq78Mg+1saHawdEk9hIMhExXHFH5SofPe1A==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327806364029
X-PARTNER-ID: 12345
X-SIGNATURE: zdNG8f1oUhxUo2CwMKOHsh7G9ky6OfFgtEcDmOv+3Glt+O0eJZROSsvCqQhJTxB8IE5BGL69u5j5/VuNqkplCA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327806547119
X-PARTNER-ID: 12345
X-SIGNATURE: K1khlw8kKBlQL96Lt7xF0PJkC0MC5yXTXn8N3ELmWkEfhoQl+asMhDmWquWQKTOmM9BcBXFoxRqHkOshIgGdDw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327806713622
X-PARTNER-ID: 12345
X-SIGNATURE: kLIxHzimbZTcL0cpCbszSTbQcUgSWq3gRPUEjIjjP8iwcIkw3sVc0U+dozj3HaEBuElqBOxcPT2BDC9LNL2vYg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890221",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MAND-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890221"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890221",
    "virtualAccountNo": "   12345678901234567890221",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327806866149
X-PARTNER-ID: 12345
X-SIGNATURE: VH2rWRD1dFe5yVnhQdBKlljJWn4IKNBqJwN7CnFGxc/cGDkjlloje0KYuRJ+D+QVb+QjaxLoNCzcoQlNp+3jCg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890222",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-NOTRX-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890222"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890222",
    "virtualAccountNo": "   12345678901234567890222",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-NOTRX-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_Negative_PaymentFieldFormats/partnerServiceId_over_8

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 400

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807021783
X-PARTNER-ID: 12345
X-SIGNATURE: dv0n5wbk+zzQ2PwSq0QiIxLqAznksk+YB0Xtxh8xuQnHsfNDUiWtaFVIgJdWgRjQc3rLiHLl5EGWOUb1CMOeUA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807163148
X-PARTNER-ID: 12345
X-SIGNATURE: GckL6J9dBwBhXMw4seSvAzdApzHgDybiYSgDC81sjO4MUWXxm6mcys/6c8Qef+d93hdcCTDpR0jgf7axoSOA+A==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "9999999999999999999999999999999",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807326637
X-PARTNER-ID: 12345
X-SIGNATURE: eeRHPT6aJcEx9xhsrleRTbP972Nw6jxDATH0QvPSTCs0qWIfyZ5XC67TfeAzTqUidtRuKab0S7nsaq0ZNFyObw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807463445
X-PARTNER-ID: 12345
X-SIGNATURE: EFm1iI0hWKLyanASnYIxOR7bB0s6hr84UnILBCd0dA+06D4LKusGj2BK5Bf4eVoUmc76bu6K7gbvRk5SbxpOUg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "999999999999",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807608983
X-PARTNER-ID: 12345
X-SIGNATURE: z24IGGwt7SNnsEWom+D1jHYIZtQZ11Wt0MTAfp3Wqxmh0M6w1DpwuM+gjaEEI/y7L3oteRvPxTQ59KeMZKKcjg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "EUR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "EUR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807741303
X-PARTNER-ID: 12345
X-SIGNATURE: xs9GTH3gsgloDncL54+7hHQwxiARaeYOD1nsQTTzFtfoqqQmYlMV/L62/no/KJnR14htADDH0RfbgMTcYd9whw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "USD",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327807870546
X-PARTNER-ID: 12345
X-SIGNATURE: d4Z2MBJLeDvAP02z3EBN0e0CbqUygZHcZaXRrSikb07BNbQ01fje/eJV3/ivh91fxxMLTa0dub7N0JyJPcCLrA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "12345678901234.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "12345678901234.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808002631
X-PARTNER-ID: 12345
X-SIGNATURE: y2YWMyaoyUMhG5vDRqtIBaTeVjj2/w4ehN5WdQjBTBieMmCgnfgnB03Fpsyip+D8BYb/CXkDrV3qQK25M7cZSw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "abc"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "abc"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808135357
X-PARTNER-ID: 12345
X-SIGNATURE: 9dTsOrcF67n1bWUGvCuPuH/Nf/NxoHNkswfatwg+fREU2BSAW5nCfZybna/5Xxd5v8Ww/XG6NKDkJLeEgV411w==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890223",
  "flagAdvise": "X",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890223",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808269095
X-PARTNER-ID: 12345
X-SIGNATURE: Kygu/GqZ1IALc+ZLB0Kg7wbrXrvCKYl2IFQ+WN37YmZX2hencJyn7Ukc1nWIxZlP2JgysTL7Z0QyDH1ea22/DQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "999999999999999999",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-FMT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890223"
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
    "partnerServiceId": "   12345",
    "customerNo": "999999999999999999",
    "virtualAccountNo": "   12345678901234567890223",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808432954
X-PARTNER-ID: 12345
X-SIGNATURE: 8UmIldusR3YhbcOv2wN7TvhaPYfWbNP9E/NWftmHFhm8sd2zSiQL0wGf++r3oCSoGKIre3fKCBJsinnVhbVsUw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890230",
  "inquiryRequestId": "INQ-404-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890230"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890230",
    "virtualAccountNo": "   12345678901234567890230",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808549326
X-PARTNER-ID: 12345
X-SIGNATURE: ulAhKzWBeB1+X8s14Sk18JKIw83d/4p9diNHLHjJ307qSaQldgza3kQz2unoUzFO45DssDta29V9cHs/wkeTFw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890230",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-404-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890230"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890230",
    "virtualAccountNo": "   12345678901234567890230",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808689141
X-PARTNER-ID: 12345
X-SIGNATURE: WXyZ83Hzz+NI3vek1JScl8gf6/X0PkkYb9cr6NtsC8uKnxwqe8fBothdwJAF1uLZuo1vRV4qIsmIBxBe7svC6g==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890231",
  "inquiryRequestId": "INQ-EXPIRED-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890231"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890231",
    "virtualAccountNo": "   12345678901234567890231",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808805058
X-PARTNER-ID: 12345
X-SIGNATURE: hySCGWdTcUcJSL3le8B+gDon8FW9naN1RtdJTx+qyqErGqK1wzFPiQFVBXAN21z+qz4d2nR1XJpL8y1bdYusBw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890231",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-EXPIRED-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890231"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890231",
    "virtualAccountNo": "   12345678901234567890231",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-EXPIRED-1",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327808943445
X-PARTNER-ID: 12345
X-SIGNATURE: K9lQHJldxLRDHbtKrARo0X1sS2RdAMNSAT196teAMopsB3xcXsjr35qlTVdmjPXENFNNrEsWJ+odoqeB9usykg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890232",
  "inquiryRequestId": "UNKNOWN-REQ",
  "partnerServiceId": "   12345",
  "virtualAccountNo": "   12345678901234567890232"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890232",
    "virtualAccountNo": "   12345678901234567890232",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809115402
X-PARTNER-ID: 12345
X-SIGNATURE: g4SeNALM1ZIypmcRdFdlUQivc3YueXzEJnQlrgzB3AJrXLJnzG+TYSNP//IDejtw3HiAkmiZr2UzalUu0tYPjA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890240",
  "inquiryRequestId": "INQ-ENV-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890240"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "   12345678901234567890240",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809233739
X-PARTNER-ID: 12345
X-SIGNATURE: sg3HmcpgiLQxYYKzcYqe3dEYINCYxJS5h1E1Z4W/Xkl+8WZqzXCnw0OA8E5D6sbPMyYFCcy+bw2LFfQvRjDe+Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "partnerServiceId": "   12345"
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
    "partnerServiceId": "   12345",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809335220
X-PARTNER-ID: 12345
X-SIGNATURE: uuu3A8GeTjrCPjiP9ynMcSqFod2Vvn04KYcCJbowLEmivvUh9goGrFjD8ItthfYxUybLL6Vb/5V+wQeks23OFg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890240",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ENV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890240"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "   12345678901234567890240",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809470043
X-PARTNER-ID: 12345
X-SIGNATURE: QYP35rFX7uOlTRbLDvWjPMBiddzIiOw7Jm7fuWysWeJR7B2mpeIku5cz3lupqhCi2xi25W5a9ZFH5hLKVrR8fQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "partnerServiceId": "   12345"
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
    "partnerServiceId": "   12345",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809553452
X-PARTNER-ID: 12345
X-SIGNATURE: SvmoWD56ef20fsuzJ5UB2FIuIq5RbzHV9+5u283YjL1/plRPhuhfNUgWK+KRo596pfpGOSq5LWwxohFJVLgmeA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890240",
  "inquiryRequestId": "ST-ENV-1",
  "partnerServiceId": "   12345",
  "virtualAccountNo": "   12345678901234567890240"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "   12345678901234567890240",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 900000000000003
X-PARTNER-ID: 12345
X-SIGNATURE: fyeCgy5l97AGb2RXWFgCE4UxLPeJEa1GmNW/5nkyKnOpedK97v2ZsiCvOKzd9KLIBOPpMB2EUKPpnhjRhU3SEA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890240",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-ENV-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890240"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890240",
    "virtualAccountNo": "   12345678901234567890240",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809795027
X-PARTNER-ID: 12345
X-SIGNATURE: SdYYhuHqogZwl4pAnWp0bw70lo7/sMo4saGdi3PKeurXDqtJ+lmREnmN/+jt43uk5wum0KuTrmt2hl1epO8jOA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890241",
  "inquiryRequestId": "INQ-AUTH-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890241"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809892319
X-PARTNER-ID: 12345
X-SIGNATURE: GBaT/ijGBCIKPmtrvsGh2Nos7ymotchPrrTCGcUX9lkVF9fQz3lD6ZgjbQehfMKqk5gteTKc9NYYz3HBGZ4GeQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327809969319
X-PARTNER-ID: 12345
X-SIGNATURE: t6gVvnqBZ5VqsHCfXrYuyDRmeg19rZ1smBpDFxCRZA75qcCgofZMy7/L+UeiUnUKVVWn30xs8nWslt5nHOLvnQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327810022084
X-PARTNER-ID: 12345
X-SIGNATURE: z2XzBD73CxUMGXFQzwm06++uHscReXfBdac/RfksPi6nJBS2LObrUHEO5ns5mZZp3mj3hKGcETQ3eeUlrN9EFQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
Idempotency-Key: d3b07384-d9a0-4c9b-9b0e-1f2a3b4c5d6e
X-CLIENT-KEY: some-client-key
X-Device-Id: device-1
X-EXTERNAL-ID: 1786329327810112154
X-PARTNER-ID: 12345
X-SIGNATURE: FA6uNYEKFqpSdF0ROyM7quaELVpFn9bsxtv8F3HHQX9kiLznKVbfg0m6Bgn94i09TfzPndGkPDJ8M0QzSDc8pg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890260",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-HDR-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890260"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890260",
    "virtualAccountNo": "   12345678901234567890260",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-HDR-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_UndocumentedHeadersDoNotBreakTheSignature

#### 1. `POST /openapi/v1.0/transfer-va/inquiry` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
Idempotency-Key: irrelevant
X-EXTERNAL-ID: 1786329327810281297
X-PARTNER-ID: 12345
X-SIGNATURE: TZp4CgxNuzpzAYQgGkLQxURwQ4AZsWwWFOqHZAZhgeqoUY274Z24l1QS7W/WXiWMzdK82vn6y/ROhyC2s+OYeA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890261",
  "inquiryRequestId": "INQ-HDR-1",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890261"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890261",
    "virtualAccountNo": "   12345678901234567890261",
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
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327801579762
X-PARTNER-ID: 12345
X-SIGNATURE: HQddWj6fzBDsxf6/e7wqU9HFyHodsLGMSb76Dn0tcL3Qm7O93qb2MhQ6F42wbOy9oL2SRybdbWLBPpjh/pAyUA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890300",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MV-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890300"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890300",
    "virtualAccountNo": "   12345678901234567890300",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-MV-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786329327801829475
X-PARTNER-ID: 67890
X-SIGNATURE: pM8YD2GCeoJj/E7tDZ1Nxozjj62sqyNDD1jBFEU6ARH93KDI+pxB4whCxoi3tw32yWu4iTyxkvxzYq3aBus7bA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890301",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-MV-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "2000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   67890678901234567890301"
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
    "partnerServiceId": "   67890",
    "customerNo": "678901234567890301",
    "virtualAccountNo": "   67890678901234567890301",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-MV-2",
    "paidAmount": {
      "value": "2000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "2000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_MultiVendor_CredentialsDoNotCrossOver

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327801928726
X-PARTNER-ID: 12345
X-SIGNATURE: Kvl4vKOdOx6W9Qd9Pjo9O/h5aVqSl8BNEsDS8qSx4xmg/BL3nppYPgXlHRE43UqQjZb71ESRmZH5cGMHwcdduQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890302",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MV-3",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890302"
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

### TestE2E_MultiVendor_PartnerAndChannelPinnedPerVendor

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802001404
X-PARTNER-ID: 12345
X-SIGNATURE: VO8u0+hQp5R3X+JCTsSdw9+9qirBIpf0hX6G6TbrpXnE2jl0a55sUblA37tqYROgOH+YdK37oaFn0ZIQO376nA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890303",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MV-4",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890303"
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
X-EXTERNAL-ID: 1786329327802071378
X-PARTNER-ID: 67890
X-SIGNATURE: JmEIHwUAGYkSaXkokcFGe/9EsDu145c5knigy8nSShDr6pD9zNqJggrzSEtTyUPsnQp8p/uatPyZDIQlojZ/lQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890304",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-ENC-OK",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   67890678901234567890304"
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
    "partnerServiceId": "   67890",
    "customerNo": "678901234567890304",
    "virtualAccountNo": "   67890678901234567890304",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-ENC-OK",
    "paidAmount": {
      "value": "5000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "5000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
CHANNEL-ID: 77001
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802141118
X-PARTNER-ID: 67890
X-SIGNATURE: 9jC6xO11IQtUozVDTTHwdvYUAguszdJCyWOABH15XZLS89a3EzHzBve8Sr+X2ftnzjM6vY77Gp1+6LkoT24RjA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890304",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-ENC-BAD",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "5000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   67890678901234567890304"
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
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802212309
X-PARTNER-ID: 12345
X-SIGNATURE: AB5kY7IxIuxWrJC39Ic3Mjs2IHoAGswvDEsPs9u8sfObpEaKXBzE0ShIRMDJCYXzRPc0xi41VanaxT2E/9QARQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890305",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-STRICT-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "virtualAccountNo": "   12345678901234567890305"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890305",
    "virtualAccountNo": "   12345678901234567890305",
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
X-EXTERNAL-ID: 1786329327802269594
X-PARTNER-ID: 67890
X-SIGNATURE: 2aB46dfzr+MKnVqL/rp+x0dcQPnaHdZgCSAbC2D9D2IclN7X9P+NZbYaZ4stQUNoEbAa+w7TZ/JmKGJtRXXYYA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "customerNo": "678901234567890306",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-LAX-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "virtualAccountNo": "   67890678901234567890306"
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
    "partnerServiceId": "   67890",
    "customerNo": "678901234567890306",
    "virtualAccountNo": "   67890678901234567890306",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-LAX-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27.802287496+07:00",
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
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802336934
X-PARTNER-ID: 12345
X-SIGNATURE: Ymh3KiprhksAhHOGd2JwBgU4bnKZKv7jbKahQtSNE7FrB3jigbkLgxluPvE0gqBUiApaTeXC+JJvLNcB6/zhAA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890310",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MM-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890310"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890310",
    "virtualAccountNo": "   12345678901234567890310",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-MM-1",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802407488
X-PARTNER-ID: 67890
X-SIGNATURE: vBNKpW8Sv1Jqkou9N+swwyw71sFam4rm8VJTeHW6BHUG0054dPDaBfX/39mlv3IsclnnutKfWDYHCZd19Zo3pA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890311",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-MM-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   67890678901234567890311"
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
    "partnerServiceId": "   67890",
    "customerNo": "678901234567890311",
    "virtualAccountNo": "   67890678901234567890311",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-MM-2",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 3. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802453190
X-PARTNER-ID: 67890
X-SIGNATURE: LcW1N+f4mJLN4+l9AOsfAXq9sLukSnXHlUjbISnvO1cakcWKuP65Uy9txpyk13wBxW6e9yEYVQJ/apHicdTBJg==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890312",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-MM-3",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   67890678901234567890312"
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
    "partnerServiceId": "   67890",
    "customerNo": "678901234567890312",
    "virtualAccountNo": "   67890678901234567890312",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802520317
X-PARTNER-ID: 12345
X-SIGNATURE: jnYArFOFEeYMKPK7D7bcYpIOmy2y+J+AaB7wEjOfSntIJSZB2Q7flrjeSNdfV+IOJm81Jk/jcerzFTtkOBoq8Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890399",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-MM-4",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890399"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890399",
    "virtualAccountNo": "   12345678901234567890399",
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
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802602587
X-PARTNER-ID: 12345
X-SIGNATURE: q7zqxB18CZjtmviGS9cjbFJiKF/N+EIVHs5H4gIvE80fM5Y50FV9IY/yzYZYOuJH+Q0ItM7Igup5opKsTxNGCA==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890320",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "PAY-CB-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "100000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890320"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890320",
    "virtualAccountNo": "   12345678901234567890320",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-CB-1",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-bca-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802663205
X-PARTNER-ID: 67890
X-SIGNATURE: 1bc49o8fdDOo5GE/kB0CXeBVrywobbeQF2QVfpq1k5K5dS7W1a9jDhaBdy26PxYOZ9dOvSfbolr2i/4BCkXnew==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890321",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "partnerServiceId": "   67890",
  "paymentRequestId": "PAY-CB-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "250000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   67890678901234567890321"
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
    "partnerServiceId": "   67890",
    "customerNo": "678901234567890321",
    "virtualAccountNo": "   67890678901234567890321",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-CB-2",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

### TestE2E_MultiVendor_CommaSeparatedPartnerAllowList

#### 1. `POST /openapi/v1.0/transfer-va/payment` → 200

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802747539
X-PARTNER-ID: 11111
X-SIGNATURE: jcCy/EcoXqBPJf6iEcHUHMKRbfHuLMBq938qlC+GUbOaOJVk/vMWD3TkMxx2nx23RsFJx6EwOpnXLpPlyEid9Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890330",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   11111",
  "paymentRequestId": "PAY-ALLOW-1",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   11111678901234567890330"
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
    "partnerServiceId": "   11111",
    "customerNo": "678901234567890330",
    "virtualAccountNo": "   11111678901234567890330",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "PAY-ALLOW-1",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 2. `POST /openapi/v1.0/transfer-va/payment` → 401

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327802791913
X-PARTNER-ID: 22222
X-SIGNATURE: DG6NUpK6OiNhWOuAh+f2Oo1gDSCgWhBeT397NWA+01CH1U4QHlduIihHmq1wUX6l66b40z/L63hvMC/hzIvJlQ==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890330",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "partnerServiceId": "   11111",
  "paymentRequestId": "PAY-ALLOW-2",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "1000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   11111678901234567890330"
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813330224
X-PARTNER-ID: 12345
X-SIGNATURE: ayv/pupkAyO/NOuWnWLxQmnP/gO9lXm5lsUG9KnIarIojH6EJLmz00dyym45QhzmxV6NOlLZDsMX758JbTM33Q==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890900",
  "inquiryRequestId": "202202111031031234500900",
  "partnerServiceId": "   12345",
  "trxDateInit": "2026-08-10T09:35:27+07:00",
  "virtualAccountNo": "   12345678901234567890900"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890900",
    "virtualAccountNo": "   12345678901234567890900",
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
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813379094
X-PARTNER-ID: 12345
X-SIGNATURE: KxERUC6hRgjq1VGMrhsuJB3mWqZIAnrgLVeXz4IWnDXD/8NW1TgY+n36rQF8stI3Q2Bg97/fbeWqwtG1cRidvw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890900",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "202202111031031234500900",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890900"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890900",
    "virtualAccountNo": "   12345678901234567890900",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "202202111031031234500900",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

#### 3. `POST /openapi/v1.0/transfer-va/payment` → 404

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Authorization: Bearer e2e-token-e2e-vendor-client
CHANNEL-ID: 95231
Content-Type: application/json
X-EXTERNAL-ID: 1786329327813418769
X-PARTNER-ID: 12345
X-SIGNATURE: KxERUC6hRgjq1VGMrhsuJB3mWqZIAnrgLVeXz4IWnDXD/8NW1TgY+n36rQF8stI3Q2Bg97/fbeWqwtG1cRidvw==
X-TIMESTAMP: 2026-08-10T09:35:27+07:00

{
  "additionalInfo": {},
  "channelCode": 6011,
  "customerNo": "678901234567890900",
  "flagAdvise": "N",
  "paidAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "partnerServiceId": "   12345",
  "paymentRequestId": "202202111031031234500900",
  "referenceNo": "12345678901",
  "totalAmount": {
    "currency": "IDR",
    "value": "150000.00"
  },
  "trxDateTime": "2026-08-10T09:35:27+07:00",
  "virtualAccountName": "Budi Manjo",
  "virtualAccountNo": "   12345678901234567890900"
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
    "partnerServiceId": "   12345",
    "customerNo": "678901234567890900",
    "virtualAccountNo": "   12345678901234567890900",
    "virtualAccountName": "Budi Manjo",
    "paymentRequestId": "202202111031031234500900",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-10T09:35:27+07:00",
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

