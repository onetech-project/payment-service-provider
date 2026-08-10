# SNAP Virtual Account — live end-to-end flow transcript

Every request and response from a full run of the `scripts/e2e-*.sh` flows
against a real deployment: real Postgres, real Redis idempotency, real B2B
access tokens, real Asynq merchant callbacks. Each step shows the URL, the
headers, the `stringToSign` the signature was computed over, the request body
and the response.

- Generated: `2026-08-08T00:30:47+07:00`
- Commit: `25b9dab`

`Authorization: Bearer` values are redacted — they are single-use and expire in
minutes. Signatures are kept: they are computed over the redacted token, so they
cannot be reproduced from this file, and seeing them is the point.

## Contents

- [Static fixed bill (vaType 03)](#static-fixed-bill-vatype-03)
- [Static variable bill (vaType 02)](#static-variable-bill-vatype-02)
- [Static no bill (vaType 01)](#static-no-bill-vatype-01)
- [Dynamic VAs (vaType 04 / 06 / 05)](#dynamic-vas-vatype-04--06--05)
- [Cancel flow](#cancel-flow)
- [Expiry and callback resend](#expiry-and-callback-resend)


---

## Static fixed bill (vaType 03)

```http

==> Local callback listener started: http://127.0.0.1:8101/callback (pid 262241)
    (only reachable if the PSP API can reach 127.0.0.1 on this machine —
     pass your own -w <notificationUrl> if the API runs elsewhere, e.g. Docker)

```

### Step 1/4: POST /openapi/v1.0/transfer-va/create-va (merchant identity)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:09+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:09+07:00
==> X-SIGNATURE: HO16s/hKYCJOmpExwz3+s0xSpkmPBXr4IN3jDK6l3USJG6D3clyZYuf5gWbLbQdOhXY/d3vlQxL29dZqzJRMBX4jm5nl1mGdWHuJAFZEkyZfmRJqsxs0+0wS21utQM8IGnZKF7equYq+OBbE7o/485xMWGwQtc1EXSTs69E7RwrEiq+igubdM4swn9r+YxqmdiHNIWOcCx2bBAfKPr37gJ5t9CLdCCLRksiRMSlW53CbzuqpkqQxd3OuZtWngI3ir1+uG0qyVqIhXUtKDc7i1cq4/6ezrn7wwCdRX5bAOsC/XGJYbPOtj/SVi/EORO1zWZ5yZQ/YHqMEqJYA5KbP2A==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 15975030030090000000001
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:09+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:1qwRLRu4vyeKBlst3Xl8hkeQrdAjiDHEq97IA5/2P/8=:2026-08-08T00:30:09+07:00
==> X-SIGNATURE: 3pHr6vYF0s3tAy8UMbpHqimhs3nU8MzkXCAYkTVoxq51U1QsWGAiexRfb1wmAfepT8W9Y4fopRIWsUMJ3VadBA==
==> Request body:
{
  "partnerServiceId": "15975",
  "customerNo": "030030090000000001",
  "virtualAccountNo": "15975030030090000000001",
  "virtualAccountName": "Static Fixed 003009",
  "trxId": "TRX-178612380918749",
  "additionalInfo": {
    "dbUrlProcess": "http://127.0.0.1:8101/callback",
    "vaType": "03"
  },
  "totalAmount": {
    "value": "250000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "030030090000000001",
    "virtualAccountNo": "15975030030090000000001",
    "virtualAccountName": "Static Fixed 003009",
    "trxId": "TRX-178612380918749",
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C",
    "additionalInfo": {
      "dbUrlProcess": "http://127.0.0.1:8101/callback",
      "vaType": "03"
    }
  }
}
==> virtualAccountNo: 15975030030090000000001
==> customerNo: 030030090000000001
==> trxId: TRX-178612380918749

```

### Step 2/4: POST /openapi/v1.0/transfer-va/inquiry (vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:09+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:09+07:00
==> X-SIGNATURE: ANe3TT2l/nVo4PLkSCyBKVdcIOUvR1esaWgVXOmWKEn1XPy8IzIC839FhuS1NpTk4iYsFJ+JjU6NaomQN1FQ6KdZ9MGkaJe2pEDZxq1MR3hR+Q5tffpkbUfo7zoc9F1zZAQyofx8eJJL27gtfGhSZVk7qMgQUR+Hk9TT+IEJ1sFJx3cGvNbMwj2DXekROzBpnkiw2lfAZDWLo+nbxtbEplojDUDR/ssNDA5nsJFbiozE6p0vpWxHAhlyRDOOs3k0vhrTSfLrfKsudlB1bPvMSKvNqv10WBA4LM0Qyh+HEpZfY3dqeTk68WPX+D5wi9/cvGUHSgQo69cFw/CVgGi2zQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:10+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:e0c0bc6d1d1fc1e5702186cc94389f02ca85ca40837498f25f4b8dfb5cacf33c:2026-08-08T00:30:10+07:00
==> X-SIGNATURE: 8qfPvpaR3vu7Kl1b4emr86mx8DJx5xMUctkhhqmxhA4BTxPLQmT9jViuqMl079bW1FEue+d5fxZtgBkoGGotIA==
==> Request body:
{
  "partnerServiceId": "15975",
  "customerNo": "030030090000000001",
  "virtualAccountNo": "15975030030090000000001",
  "trxDateInit": "2026-08-08T00:30:10+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "250000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612381012778"
}

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "030030090000000001",
    "virtualAccountNo": "15975030030090000000001",
    "virtualAccountName": "Static Fixed 003009",
    "inquiryRequestId": "INQ-178612381012778",
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

### Step 3/4: POST /openapi/v1.0/transfer-va/payment (vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:10+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:10+07:00
==> X-SIGNATURE: Ts5dMLUt2+eLQWhIBoEqlhhxg7nSuot5HUptxZ07W85eOoIz8bxc7EVB0jrykZISygDwfkHknJkqSDmQMtc6QjBHI4KXJN9zk1KGd1tVqyAmbmErlvoPC8p04Qd54STP3HWz/1gNxkVvFT0CjDRw+bEK/lvWkibROCzLmUr/wVyQgTdPBmakT71Ro6N/+rHuSRDIz0nlb864Z59c6COM/5q4lYZ2Aa5jlFCiRxitmbXnwPGM9wTUAlYTEvx06mEq+ZZJObxVVPpS19Tq/WcfrZFAUkXQvOsE1hbEZHWsk9wnk+DKuRhK0flE3/9WGacB2Ydkfsg74uo6Basat938eQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15975030030090000000001
==> paymentRequestId: INQ-178612381012778
==> X-TIMESTAMP: 2026-08-08T00:30:10+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:5be7403073956c3fb3abdceaa7a3c8d01f9f82845dc1393209483c1b0fdfa722:2026-08-08T00:30:10+07:00
==> X-SIGNATURE: Th9tuMKyqjV6sEnVCeL/vf9bUlePNTEi7TmqKun77ke1iGZakWZ2EtZT1cyqtkorfbaT8bJ1F0oDf52EZp0R+Q==
==> Request body:
{
  "partnerServiceId": "15975",
  "customerNo": "030030090000000001",
  "virtualAccountNo": "15975030030090000000001",
  "virtualAccountName": "Payer Name",
  "trxId": "TRX-178612380918749",
  "paymentRequestId": "INQ-178612381012778",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "250000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "250000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:10+07:00",
  "referenceNo": "R786123810"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "030030090000000001",
    "virtualAccountNo": "15975030030090000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "TRX-178612380918749",
    "paymentRequestId": "INQ-178612381012778",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:10+07:00",
    "referenceNo": "R786123810",
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

### Step 4/4: Merchant payment callback

```http
==> Waiting for the async callback (Asynq -> payment_notification_worker) to reach http://127.0.0.1:8101/callback ...
==> Callback received by merchant:
{
  "data": {
    "customerNo": "030030090000000001",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "paymentRequestId": "INQ-178612381012778",
    "referenceNo": "R786123810",
    "status": "00",
    "trxDateTime": "2026-08-08T00:30:10+07:00",
    "trxId": "TRX-178612380918749",
    "virtualAccountNo": "15975030030090000000001"
  },
  "eventType": "payment.received",
  "timestamp": "2026-08-08T00:30:10+07:00"
}

```

### Done: VA 15975030030090000000001 created -> inquiry confirmed -> paid -> callback checked.

```http
```


---

## Static variable bill (vaType 02)

```http

==> Local callback listener started: http://127.0.0.1:8102/callback (pid 262436)
    (only reachable if the PSP API can reach 127.0.0.1 on this machine —
     pass your own -w <notificationUrl> if the API runs elsewhere, e.g. Docker)

```

### Step 1/4: POST /openapi/v1.0/transfer-va/create-va (merchant identity)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:11+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:11+07:00
==> X-SIGNATURE: C6ny4wb4TwdJ6crFeSKNgFyHjq7VMAtG5CKada3m4UWTL7IRphv9NnUtyHWCvT2l+nuH8eq1tqkBRa9GMcKooWCIMgktFTx+aCi/ot51HyhMOzs/i/xKhxJR/C4XG8/6OMD2Xv6P7u17jHQBv9Y8ewuGBhlU7Fv0/nuoroEXj80ODjquSVC1SgIA9TiEVuaZMOU73qsqxpD8ofAqb8aoJWlh5kMCIKLDPjLfgQUyQQAhpqD164pJgoyVyxXYJ5+OyXgvQB/vomIOUamnLgrtB2+ufs9Ssx/VTIJuqY4hosEhVld0osDnf6JhibNVBwGnajcL/R2LjY+5iXndHAKzgg==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 15974020030090000000001
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:11+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:RdwVOLvyXPDe65AWgKxMB/yjuzBQrU15fDtKCr6YzKU=:2026-08-08T00:30:11+07:00
==> X-SIGNATURE: aO28ErvmWXt0L+IhwPRvvugZRH2L0SFZQYo/JBdEWRXGpA1/USzKiOyF5WF/cfgv4VN2DKCEAwLeZtF2glBevg==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "020030090000000001",
  "virtualAccountNo": "15974020030090000000001",
  "virtualAccountName": "Static Variable 003009",
  "trxId": "TRX-178612381110303",
  "additionalInfo": {
    "dbUrlProcess": "http://127.0.0.1:8102/callback",
    "vaType": "02"
  },
  "totalAmount": {
    "value": "300000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "020030090000000001",
    "virtualAccountNo": "15974020030090000000001",
    "virtualAccountName": "Static Variable 003009",
    "trxId": "TRX-178612381110303",
    "totalAmount": {
      "value": "300000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C",
    "additionalInfo": {
      "dbUrlProcess": "http://127.0.0.1:8102/callback",
      "vaType": "02"
    }
  }
}
==> virtualAccountNo: 15974020030090000000001
==> customerNo: 020030090000000001
==> trxId: TRX-178612381110303

```

### Step 2/4: POST /openapi/v1.0/transfer-va/inquiry (vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:11+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:11+07:00
==> X-SIGNATURE: TKqTcSbCkzuBBmz/Sya4Z6bHsEIS/r6F+d3Hh7nyRoctKq5fIypHgq1dDbyTIjQnLW5jdk4aYAZDTgXUWrQqMao370WfwIeeKqWEncG/EhKWEio71nw/PspCUPH74grlY9x79MJh7WodHhI2mCINpfGcYUjrQ15vZZcxGKIhiSXxl7WmF2PwJnzNytFIlBWgPotQJwNaDZbUBqkAxtmfo+G7pZihfA4GPi55lSejKoD7KHBU11CdS5lD6ZxqUu+g8nLUHVK3Ilwm7/IP3xfFMvXirXuGysDzu7NwYrcXUvlVoN7FutNjKm00pBsZugK220Li3rsxpVjqs9RsZHsYdQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:11+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:68cfcf6886bbd87f6f9c253f7ed419351efcdf6d148b22bd387d9ff0cfe18360:2026-08-08T00:30:11+07:00
==> X-SIGNATURE: G8JipEjDZ6HM0BocgaW3TBiF2zlC5rgMNpNaWZBbkia+IgFtgjwS9ebqSGCh5LVaJFgKLMF0yxMtMRdxF7+OYQ==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "020030090000000001",
  "virtualAccountNo": "15974020030090000000001",
  "trxDateInit": "2026-08-08T00:30:11+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "300000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-17861238112922"
}

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "020030090000000001",
    "virtualAccountNo": "15974020030090000000001",
    "virtualAccountName": "Static Variable 003009",
    "inquiryRequestId": "INQ-17861238112922",
    "totalAmount": {
      "value": "300000.00",
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

### Step 3/4: POST /openapi/v1.0/transfer-va/payment (vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:11+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:11+07:00
==> X-SIGNATURE: TKqTcSbCkzuBBmz/Sya4Z6bHsEIS/r6F+d3Hh7nyRoctKq5fIypHgq1dDbyTIjQnLW5jdk4aYAZDTgXUWrQqMao370WfwIeeKqWEncG/EhKWEio71nw/PspCUPH74grlY9x79MJh7WodHhI2mCINpfGcYUjrQ15vZZcxGKIhiSXxl7WmF2PwJnzNytFIlBWgPotQJwNaDZbUBqkAxtmfo+G7pZihfA4GPi55lSejKoD7KHBU11CdS5lD6ZxqUu+g8nLUHVK3Ilwm7/IP3xfFMvXirXuGysDzu7NwYrcXUvlVoN7FutNjKm00pBsZugK220Li3rsxpVjqs9RsZHsYdQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15974020030090000000001
==> paymentRequestId: INQ-17861238112922
==> X-TIMESTAMP: 2026-08-08T00:30:11+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:43e75834070d0911b89dd86c3a95acd17fba91ffab92cbbe9898ee234636e3ba:2026-08-08T00:30:11+07:00
==> X-SIGNATURE: 9OoqXP6gcWS8GAz7uJt7VLvUYB0GDaLW1tdJHu9yCQPMKmqp9lDq5Q42ZDYrpSJvj6SgivrAqpI9tb4POrCKFA==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "020030090000000001",
  "virtualAccountNo": "15974020030090000000001",
  "virtualAccountName": "Payer Name",
  "trxId": "TRX-178612381110303",
  "paymentRequestId": "INQ-17861238112922",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "300000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "300000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:11+07:00",
  "referenceNo": "R786123811"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "020030090000000001",
    "virtualAccountNo": "15974020030090000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "TRX-178612381110303",
    "paymentRequestId": "INQ-17861238112922",
    "paidAmount": {
      "value": "300000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "300000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:11+07:00",
    "referenceNo": "R786123811",
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

### Step 4/4: Merchant payment callback

```http
==> Waiting for the async callback (Asynq -> payment_notification_worker) to reach http://127.0.0.1:8102/callback ...
==> Callback received by merchant:
{
  "data": {
    "customerNo": "020030090000000001",
    "paidAmount": {
      "value": "300000.00",
      "currency": "IDR"
    },
    "paymentRequestId": "INQ-17861238112922",
    "referenceNo": "R786123811",
    "status": "00",
    "trxDateTime": "2026-08-08T00:30:11+07:00",
    "trxId": "TRX-178612381110303",
    "virtualAccountNo": "15974020030090000000001"
  },
  "eventType": "payment.received",
  "timestamp": "2026-08-08T00:30:12+07:00"
}

```

### Done: VA 15974020030090000000001 created -> inquiry confirmed -> paid -> callback checked.

```http
```


---

## Static no bill (vaType 01)

```http

==> Local callback listener started: http://127.0.0.1:8103/callback (pid 262670)
    (only reachable if the PSP API can reach 127.0.0.1 on this machine —
     pass your own -w <notificationUrl> if the API runs elsewhere, e.g. Docker)

```

### Step 1/4: POST /openapi/v1.0/transfer-va/create-va (merchant identity)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:13+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:13+07:00
==> X-SIGNATURE: GDuCixzzSEP9bNcCFIO+ZhD5pNyLYb+WIcWIK7zNW7doH/b+4vowkv2snOOso4ZpHedbSAWO/K+tpy/gfSteyvbqxvt4exkuOMU6V+SrntzNakhxE9+eODqFx67Xdkro18lF6ua5aBrmiHhpgzZJ+4qhUJFYG9zvOtcwgNuxBvwnFEv6A9HfFIh2iBB5kzrwdC6sY3C0ZBlWbvkujAr0B8RfVDrsHAAZL2hYozTyyg8/xxdR1an/pttxDnFXfWDZ+pKOwrBJUILFyivGzMMwmNzrtbVSTIcgggZLhm15t2QFFUO5knvz9xAhIFRC1iLE585lAHDFrp35xeWAUd2XgQ==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 15973010030090000000001
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:13+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:5GeklIIjMCif/rqkUBCM2NWHXKKCiQIck72EYVmgIWA=:2026-08-08T00:30:13+07:00
==> X-SIGNATURE: 7u76d83jUthGYrq5w26SwWzCGJyoRL1q6NNXTOnDVm7aqAfnR45CG6jLKTMbloOdJNxweAkSG0EdJMzlacnQ0Q==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "010030090000000001",
  "virtualAccountNo": "15973010030090000000001",
  "virtualAccountName": "Static NoBill 003009",
  "trxId": "TRX-178612381310030",
  "additionalInfo": {
    "dbUrlProcess": "http://127.0.0.1:8103/callback",
    "vaType": "01"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "010030090000000001",
    "virtualAccountNo": "15973010030090000000001",
    "virtualAccountName": "Static NoBill 003009",
    "trxId": "TRX-178612381310030",
    "virtualAccountTrxType": "C",
    "additionalInfo": {
      "dbUrlProcess": "http://127.0.0.1:8103/callback",
      "vaType": "01"
    }
  }
}
==> virtualAccountNo: 15973010030090000000001
==> customerNo: 010030090000000001
==> trxId: TRX-178612381310030
==> transactions after create-va: 0
   [PASS] no-bill create-va wrote NO transaction (expect 0, got 0)

```

### Step 2/4: POST /openapi/v1.0/transfer-va/inquiry (vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:13+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:13+07:00
==> X-SIGNATURE: eM0CaZgl61HJoiulXfwNTyIvimsDV7cwvs4UmflTimdj0eRfjYTZL8DvXTzsOUlMmbS3ZWV4xPQwZiq2Tne7QfjGD9WJZqQ+F2NjMwNojE+vBtzusXdxtDUBzipOZTps+fIqG6u+bq7xBe2uS129Xtdut5KEJAYgBVQqicCSqBlp+z+SpDI9sJFBErvC+mhbLu+h+E4T3yFNB6CXOI/EIMv3xSPytzfJhVJIZGpac66EdMxrvtMOG2Einc1KcI3KC61x7kV3jdP1gRYi03WiVtOxtdySyptMd4n+XTz1XLlWfTAYDUhaCcmG0JN6qdxD5kgmz4wGa04OYDpuVC7sCQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:eb20d6635ae61028aa928ac38725f0bc53dcaeab41e20db9532d52fec836e6db:2026-08-08T00:30:14+07:00
==> X-SIGNATURE: 4c/A+okxQZq0MgwSXiGL2RIF3Uik2mozsQsMceN72kTeipwRm7qNxNCWOnP01ZiqujOBtzzfvhWHv9UjznQd8g==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "010030090000000001",
  "virtualAccountNo": "15973010030090000000001",
  "trxDateInit": "2026-08-08T00:30:14+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612381414345"
}

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "010030090000000001",
    "virtualAccountNo": "15973010030090000000001",
    "virtualAccountName": "Static NoBill 003009",
    "inquiryRequestId": "INQ-178612381414345",
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

### Step 3/4: POST /openapi/v1.0/transfer-va/payment (vendor identity)

```http
--- payment 1/3 ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:14+07:00
==> X-SIGNATURE: WVqD7wO7mPqozSd2i5VLRu3rAagWaZiy5f4YIkvEseW8/Cv7ZxmEqbW9JKuK1X/8OdJhtklz9zUantuB2G47yfOQCseFaxuZJbGh3TuYCZBPEYWPo9MTEcSHpbxoxxR0O2adrZWy6SN5SrGjlppSvIvqta0zof6J23DuuJ4hvV79dG7c6DKYK0TbVA5lW2C+PPE5b42bAXpULeJ52+LklAFuebjjCvAy4vhUdwrzMZ9Il+/FX8XJb5WSwthoAlsgtS7MRe3IirchqDouemQxWzedHZUs6zKWJ3LqXzTJKgQ3Cz5TxPbSzKyIEMQ+dIQtLTplB/1tUqnpBdvDcU7lXw==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15973010030090000000001
==> paymentRequestId: INQ-178612381414345
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:ae907bec390c3e899d41a5ee9a3fa58c810dc439c711ad6e17c00c14c73b3102:2026-08-08T00:30:14+07:00
==> X-SIGNATURE: MkDYQLfKE7Voy7K5+hRS2xg5UXZLf5NIoxZ1WvG//u9+wgVnGCPxCob8we1DAJjHQB7pdacAas2Izo4uLxcyIw==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "010030090000000001",
  "virtualAccountNo": "15973010030090000000001",
  "virtualAccountName": "Payer Name",
  "trxId": "TRX-178612381310030",
  "paymentRequestId": "INQ-178612381414345",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:14+07:00",
  "referenceNo": "R786123814"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "010030090000000001",
    "virtualAccountNo": "15973010030090000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "TRX-178612381310030",
    "paymentRequestId": "INQ-178612381414345",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:14+07:00",
    "referenceNo": "R786123814",
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
   [PASS] payment 1 succeeded (got 2002500)

--- payment 2/3 ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:14+07:00
==> X-SIGNATURE: WVqD7wO7mPqozSd2i5VLRu3rAagWaZiy5f4YIkvEseW8/Cv7ZxmEqbW9JKuK1X/8OdJhtklz9zUantuB2G47yfOQCseFaxuZJbGh3TuYCZBPEYWPo9MTEcSHpbxoxxR0O2adrZWy6SN5SrGjlppSvIvqta0zof6J23DuuJ4hvV79dG7c6DKYK0TbVA5lW2C+PPE5b42bAXpULeJ52+LklAFuebjjCvAy4vhUdwrzMZ9Il+/FX8XJb5WSwthoAlsgtS7MRe3IirchqDouemQxWzedHZUs6zKWJ3LqXzTJKgQ3Cz5TxPbSzKyIEMQ+dIQtLTplB/1tUqnpBdvDcU7lXw==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15973010030090000000001
==> paymentRequestId: PAY-1786123814309008660-2
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:47128ab1aeefb132f31324f48c73983fc7db903c1f3b3baaa67e6376e09d5a1c:2026-08-08T00:30:14+07:00
==> X-SIGNATURE: AVRboPqB/tBPWiXU4jx5Oo9VuMSvi738cjSCM+er3QpeKFKFcrnbTX1DKSTYVclUVg7JnsTCR50E7dFcxIk+ug==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "010030090000000001",
  "virtualAccountNo": "15973010030090000000001",
  "virtualAccountName": "Payer Name",
  "trxId": "TRX-178612381310030",
  "paymentRequestId": "PAY-1786123814309008660-2",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:14+07:00",
  "referenceNo": "R786123814"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "010030090000000001",
    "virtualAccountNo": "15973010030090000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "TRX-178612381310030",
    "paymentRequestId": "PAY-1786123814309008660-2",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:14+07:00",
    "referenceNo": "R786123814",
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
   [PASS] payment 2 succeeded (got 2002500)

--- payment 3/3 ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:14+07:00
==> X-SIGNATURE: WVqD7wO7mPqozSd2i5VLRu3rAagWaZiy5f4YIkvEseW8/Cv7ZxmEqbW9JKuK1X/8OdJhtklz9zUantuB2G47yfOQCseFaxuZJbGh3TuYCZBPEYWPo9MTEcSHpbxoxxR0O2adrZWy6SN5SrGjlppSvIvqta0zof6J23DuuJ4hvV79dG7c6DKYK0TbVA5lW2C+PPE5b42bAXpULeJ52+LklAFuebjjCvAy4vhUdwrzMZ9Il+/FX8XJb5WSwthoAlsgtS7MRe3IirchqDouemQxWzedHZUs6zKWJ3LqXzTJKgQ3Cz5TxPbSzKyIEMQ+dIQtLTplB/1tUqnpBdvDcU7lXw==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15973010030090000000001
==> paymentRequestId: PAY-1786123814492849054-3
==> X-TIMESTAMP: 2026-08-08T00:30:14+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:e9397d93a9edae5ea1c17b11e8fdc470b3ab6d2313793ece579a4c4f566818b5:2026-08-08T00:30:14+07:00
==> X-SIGNATURE: /A4acxvork7HzBpoKkoWtINspgl/2SqWlzFy8jlFPdQThEkBJWIRIWexKtxjs3T3StGLHxbfKi80JFQTipB7sQ==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "010030090000000001",
  "virtualAccountNo": "15973010030090000000001",
  "virtualAccountName": "Payer Name",
  "trxId": "TRX-178612381310030",
  "paymentRequestId": "PAY-1786123814492849054-3",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:14+07:00",
  "referenceNo": "R786123814"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "010030090000000001",
    "virtualAccountNo": "15973010030090000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "TRX-178612381310030",
    "paymentRequestId": "PAY-1786123814492849054-3",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:14+07:00",
    "referenceNo": "R786123814",
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
   [PASS] payment 3 succeeded (got 2002500)

==> transactions after 3 payment(s): 3
   [PASS] each payment created its own transaction (expect 3, got 3)
   [PASS] still exactly ONE registered VA after 3 payments (expect 1, got 1)

```

### Step 4/4: Merchant payment callback

```http
==> Waiting for the async callback (Asynq -> payment_notification_worker) to reach http://127.0.0.1:8103/callback ...
==> Callback received by merchant:
{
  "data": {
    "customerNo": "010030090000000001",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "paymentRequestId": "INQ-178612381414345",
    "referenceNo": "R786123814",
    "status": "00",
    "trxDateTime": "2026-08-08T00:30:14+07:00",
    "trxId": "TRX-178612381310030",
    "virtualAccountNo": "15973010030090000000001"
  },
  "eventType": "payment.received",
  "timestamp": "2026-08-08T00:30:14+07:00"
}

==================================================================
Done: VA 15973010030090000000001 created -> inquiry confirmed -> paid -> callback checked.
Assertions: 6 passed, 0 failed
==================================================================
```


---

## Dynamic VAs (vaType 04 / 06 / 05)

```http

```

### Test 1/3: Dynamic No Bill (partnerServiceId=15973, vaType=04)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:15+07:00
==> X-SIGNATURE: Yc4Jc+aoab/JlY9PgtRBMvjySIb598jiJllFV4CgJ5jV1bve47Mv+9lUWwCc2Qc+mSjRHwZIxB1Slk1BtXO/lKhgchb6JTKdex3xQEhepBfH+UlsvkLBK6gfSnN7++G/xnmObIgulZKNklxNqyuBLXD1lUngnst7ylEqNHa1OH2C+mn/Yhtc5iTq4IDuBfqT53PpYbQl4vNQA7dbgxz9MAJMTmAsE7ziow4TsenLxbbh+U702Qoqvr1RsYOpBWArooYzzKTruQoCqEwtZG8pZiTfBsBt8I7OLT/eCVc/qnBpEzHOnUsrdQODC5XrS35eTmToeoIKYTqrfCQ7qbrHrA==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:/FiFbJQbLQs79zlw9B/XxONrhY+SM5nNWjDDM0nEQW4=:2026-08-08T00:30:15+07:00
==> X-SIGNATURE: S3bIRaTwrAkAeQJjlZ73G0m4rUY8j3T7uUUWI4+JIhTpU3KOLhIDZn/+nk7VPzpMPES2N/LeTBI5u7ValdTzaA==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "",
  "virtualAccountNo": "",
  "virtualAccountName": "Dyn NoBill 17861238146335",
  "trxId": "trx-dyn-nobill-17861238146335",
  "additionalInfo": {
    "vaType": "04"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "040000000000000001",
    "virtualAccountNo": "15973040000000000000001",
    "virtualAccountName": "Dyn NoBill 17861238146335",
    "trxId": "trx-dyn-nobill-17861238146335",
    "virtualAccountTrxType": "C",
    "additionalInfo": {
      "vaType": "04"
    }
  }
}
   [PASS] create-va succeeded (responseCode 2xx)
   [PASS] server-generated customerNo is 18 digits starting with 04
   [PASS] server-derived virtualAccountNo equals partnerServiceId+customerNo
==> generated customerNo: 040000000000000001
==> derived virtualAccountNo: 15973040000000000000001

--- inquiry (vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:15+07:00
==> X-SIGNATURE: NktgULZKgKwOc4k/YiDN4F7bBqPTkq4RmqN/UNkE0xoWCXAdHpntlDuo5G11ZqoV4SoI0e5XEQwPsowqlgAj6uCqz6xVNuxuOK8Ar2VcCPMJfLostO961AgJ8h0GZ5ObW+mPnT4A9x+aVjcAGqt15cx9y5s7V7Pblrq9SYlAd669rLgioD7AV+eN9BqiKXXciFLP2i1vK3yWkLXmA4/ReeE/+8sn1furMAJeTxVTNy23kAKs7ULQ+3ZepWnusj8cP4f67++dT+C5Kp2NNb7O/ppKNUlAxiPpVPwjFPHe9MMyveEp0l6+9blbFhzu8fh2CDHh7Yjrh7u4+KN6zLNDRA==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:29576e1cbd1ed149c8a18c0055260bec20d232f7a837595bb1690e05f8bd37d6:2026-08-08T00:30:15+07:00
==> X-SIGNATURE: ryBGOaV0bdWlSRak6BKPXZyTn+3VWhWS704aJPI0+uB+np/pem25FhX6kSwsSRHH99bgDLKmIBmUvbrSX5qL1w==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "040000000000000001",
  "virtualAccountNo": "15973040000000000000001",
  "trxDateInit": "2026-08-08T00:30:15+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612381518324"
}

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "040000000000000001",
    "virtualAccountNo": "15973040000000000000001",
    "virtualAccountName": "Dyn NoBill 17861238146335",
    "inquiryRequestId": "INQ-178612381518324",
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

--- payment (any amount accepted for no-bill; vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:15+07:00
==> X-SIGNATURE: NktgULZKgKwOc4k/YiDN4F7bBqPTkq4RmqN/UNkE0xoWCXAdHpntlDuo5G11ZqoV4SoI0e5XEQwPsowqlgAj6uCqz6xVNuxuOK8Ar2VcCPMJfLostO961AgJ8h0GZ5ObW+mPnT4A9x+aVjcAGqt15cx9y5s7V7Pblrq9SYlAd669rLgioD7AV+eN9BqiKXXciFLP2i1vK3yWkLXmA4/ReeE/+8sn1furMAJeTxVTNy23kAKs7ULQ+3ZepWnusj8cP4f67++dT+C5Kp2NNb7O/ppKNUlAxiPpVPwjFPHe9MMyveEp0l6+9blbFhzu8fh2CDHh7Yjrh7u4+KN6zLNDRA==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15973040000000000000001
==> paymentRequestId: PAY-17861238157345
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:48d35eec5db864f90caca93431bb57da6aa9e298ae02fd2bc7c52ebc12042e5d:2026-08-08T00:30:15+07:00
==> X-SIGNATURE: dKKAJcFqnUqNRivMSaQbEeQL9rHye86fr77ZhJgziNfxFacw26BmnlhDpD/psrLDmnogNrnKdWAooMPO9psv3g==
==> Request body:
{
  "partnerServiceId": "15973",
  "customerNo": "040000000000000001",
  "virtualAccountNo": "15973040000000000000001",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-17861238157345",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "77000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "77000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:15+07:00",
  "referenceNo": "R786123815"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15973",
    "customerNo": "040000000000000001",
    "virtualAccountNo": "15973040000000000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "trx-dyn-nobill-17861238146335",
    "paymentRequestId": "PAY-17861238157345",
    "paidAmount": {
      "value": "77000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "77000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:15+07:00",
    "referenceNo": "R786123815",
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
   [PASS] payment succeeded (responseCode 2xx)

```

### Test 2/3: Dynamic Fixed Bill (partnerServiceId=15975, vaType=06)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:15+07:00
==> X-SIGNATURE: Yc4Jc+aoab/JlY9PgtRBMvjySIb598jiJllFV4CgJ5jV1bve47Mv+9lUWwCc2Qc+mSjRHwZIxB1Slk1BtXO/lKhgchb6JTKdex3xQEhepBfH+UlsvkLBK6gfSnN7++G/xnmObIgulZKNklxNqyuBLXD1lUngnst7ylEqNHa1OH2C+mn/Yhtc5iTq4IDuBfqT53PpYbQl4vNQA7dbgxz9MAJMTmAsE7ziow4TsenLxbbh+U702Qoqvr1RsYOpBWArooYzzKTruQoCqEwtZG8pZiTfBsBt8I7OLT/eCVc/qnBpEzHOnUsrdQODC5XrS35eTmToeoIKYTqrfCQ7qbrHrA==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:dcoZh6QSiIfTMMLMtX8+0fK0hU5UwaFEV96SgijdSy8=:2026-08-08T00:30:15+07:00
==> X-SIGNATURE: GVEiEwC6EY7LxwvJsUdO80S9XkvXHTU6VMcnUtR0H+a9E6Zf3N5jyMjJ1GIXeCnvbfuQHWpF9mqckzSWy3wSdg==
==> Request body:
{
  "partnerServiceId": "15975",
  "customerNo": "",
  "virtualAccountNo": "",
  "virtualAccountName": "Dyn Fixed 17861238146335",
  "trxId": "trx-dyn-fixed-17861238146335",
  "additionalInfo": {
    "vaType": "06"
  },
  "totalAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "060000000000000001",
    "virtualAccountNo": "15975060000000000000001",
    "virtualAccountName": "Dyn Fixed 17861238146335",
    "trxId": "trx-dyn-fixed-17861238146335",
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C",
    "additionalInfo": {
      "vaType": "06"
    }
  }
}
   [PASS] create-va succeeded (responseCode 2xx)
   [PASS] server-generated customerNo is 18 digits starting with 06
   [PASS] customerNo differs from Test 1's (per-vaType sequence, not shared)
   [PASS] server-derived virtualAccountNo equals partnerServiceId+customerNo
==> generated customerNo: 060000000000000001
==> derived virtualAccountNo: 15975060000000000000001

--- inquiry (vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:15+07:00
==> X-SIGNATURE: NktgULZKgKwOc4k/YiDN4F7bBqPTkq4RmqN/UNkE0xoWCXAdHpntlDuo5G11ZqoV4SoI0e5XEQwPsowqlgAj6uCqz6xVNuxuOK8Ar2VcCPMJfLostO961AgJ8h0GZ5ObW+mPnT4A9x+aVjcAGqt15cx9y5s7V7Pblrq9SYlAd669rLgioD7AV+eN9BqiKXXciFLP2i1vK3yWkLXmA4/ReeE/+8sn1furMAJeTxVTNy23kAKs7ULQ+3ZepWnusj8cP4f67++dT+C5Kp2NNb7O/ppKNUlAxiPpVPwjFPHe9MMyveEp0l6+9blbFhzu8fh2CDHh7Yjrh7u4+KN6zLNDRA==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:4b11331050968f7af05a114d52433351a3ec884edffb639affb21245cbab8a56:2026-08-08T00:30:15+07:00
==> X-SIGNATURE: 0uMFKtCMbvUn2dVxJxRZmoKS0hKwlYU0SmhrqcLVhsTGj1h4Vgob+2GyUuoT1wgVxo7ri8KahKsoJZrrkTlwUA==
==> Request body:
{
  "partnerServiceId": "15975",
  "customerNo": "060000000000000001",
  "virtualAccountNo": "15975060000000000000001",
  "trxDateInit": "2026-08-08T00:30:15+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612381528961"
}

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "060000000000000001",
    "virtualAccountNo": "15975060000000000000001",
    "virtualAccountName": "Dyn Fixed 17861238146335",
    "inquiryRequestId": "INQ-178612381528961",
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

--- payment (exact fixed amount; vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:15+07:00
==> X-SIGNATURE: NktgULZKgKwOc4k/YiDN4F7bBqPTkq4RmqN/UNkE0xoWCXAdHpntlDuo5G11ZqoV4SoI0e5XEQwPsowqlgAj6uCqz6xVNuxuOK8Ar2VcCPMJfLostO961AgJ8h0GZ5ObW+mPnT4A9x+aVjcAGqt15cx9y5s7V7Pblrq9SYlAd669rLgioD7AV+eN9BqiKXXciFLP2i1vK3yWkLXmA4/ReeE/+8sn1furMAJeTxVTNy23kAKs7ULQ+3ZepWnusj8cP4f67++dT+C5Kp2NNb7O/ppKNUlAxiPpVPwjFPHe9MMyveEp0l6+9blbFhzu8fh2CDHh7Yjrh7u4+KN6zLNDRA==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15975060000000000000001
==> paymentRequestId: PAY-178612381518114
==> X-TIMESTAMP: 2026-08-08T00:30:15+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:e99f00e8ec7dd37f3c2c357a2b6d8235ad513c042f50174242afdb35428196bf:2026-08-08T00:30:15+07:00
==> X-SIGNATURE: 3c+62KlWhcup/XFgeqUdN1xBlWABRx4T0D82GCd0Nz0JyYOewSlKyWv7bP4umrXikMLP0YPE0aKe0Nsl5LhIjw==
==> Request body:
{
  "partnerServiceId": "15975",
  "customerNo": "060000000000000001",
  "virtualAccountNo": "15975060000000000000001",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-178612381518114",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:15+07:00",
  "referenceNo": "R786123816"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "060000000000000001",
    "virtualAccountNo": "15975060000000000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "trx-dyn-fixed-17861238146335",
    "paymentRequestId": "PAY-178612381518114",
    "paidAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "150000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:15+07:00",
    "referenceNo": "R786123816",
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
   [PASS] payment succeeded (responseCode 2xx)
   [PASS] paymentFlagStatus is 00 (paid) after the single exact payment

```

### Test 3/3: Dynamic Variable Bill (partnerServiceId=15974, vaType=05)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:16+07:00
==> X-SIGNATURE: ap/88ogCprDiRFJOqj7SPSqbmsxE7D5PCr172f1iS5YUazsNHKEE56Nej9ApMwwt3KXfCpP+AxKw5aJl+wogBBXevr6RD2d3pI/dsSRc5sgN7+sqTefm78dXuz3eJwOloNVrzoKqSTWDkrgYPbQxjuMeL31XuTXp7CIF7eJTj6Rhsx6QmQ1WsoK0oTyDHv30MlOwHqIzRZZYooke27Axdfc/vc3BjXzprBYMq5+jI10T5x2VucE0BdRrX9+6oZYXa/BoTthlwlL4PVYP2wAXJKmPR7Rh1Nu1xapSzj4dEXZ8h4OhQgOmNwuHFfZbFNE8rZSfrdu3XVcONwnwejHUBQ==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:KkzC7zsr+6uf4/2guLEKwQIlBk9hzCEpZ1Xm2fMW52M=:2026-08-08T00:30:16+07:00
==> X-SIGNATURE: PyPIptSwrwA+A64lVuVIi1Nje4GHCQPmVgxUPQRzbdsaW0V7ATu7go9Flf7quoQTypKuBPtgoX9cb4kuy8W7SQ==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "",
  "virtualAccountNo": "",
  "virtualAccountName": "Dyn Var 17861238146335",
  "trxId": "trx-dyn-variable-17861238146335",
  "additionalInfo": {
    "vaType": "05"
  },
  "totalAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "050000000000000001",
    "virtualAccountNo": "15974050000000000000001",
    "virtualAccountName": "Dyn Var 17861238146335",
    "trxId": "trx-dyn-variable-17861238146335",
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C",
    "additionalInfo": {
      "vaType": "05"
    }
  }
}
   [PASS] create-va succeeded (responseCode 2xx)
   [PASS] server-generated customerNo is 18 digits starting with 05
   [PASS] server-derived virtualAccountNo equals partnerServiceId+customerNo
==> generated customerNo: 050000000000000001
==> derived virtualAccountNo: 15974050000000000000001

--- inquiry (vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:16+07:00
==> X-SIGNATURE: SjCM0OE/GyluyKErkiHWIKfmXd4nY9c5E52sA8daYy3onFjlOPhO2LROBrezzu04iXxiMrb75U6kELvXOHABxApHg3AGALyCRwSNKGyunRQAw16tJk3zmE63TkiI5ei6eT+veasrcZLMTzR6Fq45gfuM7ZKIaKj9u4Ze5xjLxZpJTwDA7YLERjOf00jWz3hCJszji+fKh3m9UWaldQaTzwoXfD0CS5tSz7C8tDRKGwvYSXZiU3Peu9x4odiTZOfbpSuqs3q7ebGdXSPDVqJm1tSI+ZKu5b4CQE3Rr0VIdwmIBx21NSa6KgF38JAWE30Nhej5Oq3jzBehfIWOTns9rQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:283cbbd779fd11d1442b875ff38b55f9a577b7f8b070b35f930a250896903bdf:2026-08-08T00:30:16+07:00
==> X-SIGNATURE: LzHrfBIc/2j+cDPO8zCvdDpNzkj3riiC1yak79+ecJAZvxSBKfdPVl9Uj7Nd3xvGzlL9ZXu/zmmdf9C/3Hcu0A==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "050000000000000001",
  "virtualAccountNo": "15974050000000000000001",
  "trxDateInit": "2026-08-08T00:30:16+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612381619145"
}

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "050000000000000001",
    "virtualAccountNo": "15974050000000000000001",
    "virtualAccountName": "Dyn Var 17861238146335",
    "inquiryRequestId": "INQ-178612381619145",
    "totalAmount": {
      "value": "100000.00",
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

--- payment 1/2: partial payment (60000.00 of 100000.00 target; vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:16+07:00
==> X-SIGNATURE: SjCM0OE/GyluyKErkiHWIKfmXd4nY9c5E52sA8daYy3onFjlOPhO2LROBrezzu04iXxiMrb75U6kELvXOHABxApHg3AGALyCRwSNKGyunRQAw16tJk3zmE63TkiI5ei6eT+veasrcZLMTzR6Fq45gfuM7ZKIaKj9u4Ze5xjLxZpJTwDA7YLERjOf00jWz3hCJszji+fKh3m9UWaldQaTzwoXfD0CS5tSz7C8tDRKGwvYSXZiU3Peu9x4odiTZOfbpSuqs3q7ebGdXSPDVqJm1tSI+ZKu5b4CQE3Rr0VIdwmIBx21NSa6KgF38JAWE30Nhej5Oq3jzBehfIWOTns9rQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15974050000000000000001
==> paymentRequestId: PAY-17861238166186
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:e462d11c1585d526f0a1ddfd0f914ab80ceaa416f8d9aeb96c270b0f58120411:2026-08-08T00:30:16+07:00
==> X-SIGNATURE: TtmGp0Uzrr0FYwXIQK9qLZVGAlQYuKuWkYDpo9NdnvNtNpQrxnIvpM2xxFu7ir2qOuEDxFx2foxLRfr6V/1Yeg==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "050000000000000001",
  "virtualAccountNo": "15974050000000000000001",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-17861238166186",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "60000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "60000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:16+07:00",
  "referenceNo": "R786123816"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "050000000000000001",
    "virtualAccountNo": "15974050000000000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "trx-dyn-variable-17861238146335",
    "paymentRequestId": "PAY-17861238166186",
    "paidAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "60000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:16+07:00",
    "referenceNo": "R786123816",
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
   [PASS] partial payment succeeded (responseCode 2xx)
   [PASS] partial payment is flagged accepted (00) — 03 is not a payment-service value
   [PASS] cumulative paidAmount reflects the first payment (60000.00)

--- payment 2/2: remaining payment (40000.00, reaches 100000.00 target; vendor identity) ---
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:16+07:00
==> X-SIGNATURE: SjCM0OE/GyluyKErkiHWIKfmXd4nY9c5E52sA8daYy3onFjlOPhO2LROBrezzu04iXxiMrb75U6kELvXOHABxApHg3AGALyCRwSNKGyunRQAw16tJk3zmE63TkiI5ei6eT+veasrcZLMTzR6Fq45gfuM7ZKIaKj9u4Ze5xjLxZpJTwDA7YLERjOf00jWz3hCJszji+fKh3m9UWaldQaTzwoXfD0CS5tSz7C8tDRKGwvYSXZiU3Peu9x4odiTZOfbpSuqs3q7ebGdXSPDVqJm1tSI+ZKu5b4CQE3Rr0VIdwmIBx21NSa6KgF38JAWE30Nhej5Oq3jzBehfIWOTns9rQ==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 15974050000000000000001
==> paymentRequestId: PAY-178612381620425
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:94a0fca2ddac6185936e04dfe9d4acf9b01d156b9d3b58fb565483b0b077d5b7:2026-08-08T00:30:16+07:00
==> X-SIGNATURE: CGTnr+2oekh1MJwdPoxX4+Qd3FS36ZbDQ1c07+aO9/rT9wmKTno96Us2SDncmKW5uE0VAJveLOuxeS28HoL0Gw==
==> Request body:
{
  "partnerServiceId": "15974",
  "customerNo": "050000000000000001",
  "virtualAccountNo": "15974050000000000000001",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-178612381620425",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "40000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "40000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:16+07:00",
  "referenceNo": "R786123816"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15974",
    "customerNo": "050000000000000001",
    "virtualAccountNo": "15974050000000000000001",
    "virtualAccountName": "Payer Name",
    "trxId": "trx-dyn-variable-17861238146335",
    "paymentRequestId": "PAY-178612381620425",
    "paidAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "40000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:16+07:00",
    "referenceNo": "R786123816",
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
   [PASS] second payment succeeded (responseCode 2xx)
   [PASS] paymentFlagStatus flips to 00 (lunas) once cumulative total is reached
   [PASS] cumulative paidAmount reflects both payments (100000.00)

```

### Summary: 19 passed, 0 failed

```http
```


---

## Cancel flow

```http

```

### Step 1/7: Create VA #1 (to be cancelled while pending; merchant identity)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:16+07:00
==> X-SIGNATURE: ap/88ogCprDiRFJOqj7SPSqbmsxE7D5PCr172f1iS5YUazsNHKEE56Nej9ApMwwt3KXfCpP+AxKw5aJl+wogBBXevr6RD2d3pI/dsSRc5sgN7+sqTefm78dXuz3eJwOloNVrzoKqSTWDkrgYPbQxjuMeL31XuTXp7CIF7eJTj6Rhsx6QmQ1WsoK0oTyDHv30MlOwHqIzRZZYooke27Axdfc/vc3BjXzprBYMq5+jI10T5x2VucE0BdRrX9+6oZYXa/BoTthlwlL4PVYP2wAXJKmPR7Rh1Nu1xapSzj4dEXZ8h4OhQgOmNwuHFfZbFNE8rZSfrdu3XVcONwnwejHUBQ==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 123450030090000000011
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:16+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:wWdo1wAR7p2K0fRU9IRNIEj496i3PF9PY5yLhph50uo=:2026-08-08T00:30:16+07:00
==> X-SIGNATURE: zR/MPuzY0GpaYbqM1qp53trcuXb1+Tm3l2mqMZzZ+9azcJIab50ToDQLl8yHNg9Nexg4MSNoipQZa6TaLuiSPQ==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "0030090000000011",
  "virtualAccountNo": "123450030090000000011",
  "virtualAccountName": "Cancel 003009",
  "trxId": "TRX-17861238161021",
  "totalAmount": {
    "value": "120000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "0030090000000011",
    "virtualAccountNo": "123450030090000000011",
    "virtualAccountName": "Cancel 003009",
    "trxId": "TRX-17861238161021",
    "totalAmount": {
      "value": "120000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C"
  }
}
==> [create VA#1] OK (responseCode: 2002700)

```

### Step 2/7: Cancel VA #1 while still pending -> expect success (merchant identity)

```http
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:17+07:00
==> X-SIGNATURE: i3NB2Bb5xa6Kv3/Kcp4JKcyfE2mYRbI8H9pwTLHmdoiTAptWzBrfdfJavyTd5x95s1aXBO+rjh9ZYZ5McA6eB43MdqzSz5W5prCbaSSyubMp5l83qivkq2nc24eFxmM4keFL4NRKqn6NI9UaaAWDwOQ1N1CF77gICFOJzZpH0GdN+EWi98Jc1SSEuWmrD3UuiWkxkXQ/Xiq/+Pmrx57LFI/OtHr7/QTVOJNat8h+SN6jWXQcDdb6llaJ/7f5LJZjQ2DQ9AywZL+lwqhVUgkY8dDou6PUJHPRYFF4ptE2DoOztlA6HW8FIFmTCZItbKTf77RB3Ka/IzjCMSRiflj8ew==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> DELETE http://localhost:18091/openapi/v1.0/transfer-va/delete-va
==> virtualAccountNo: 123450030090000000011
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: DELETE:/openapi/v1.0/transfer-va/delete-va:<accessToken>:ihgReGNrbwVcb1JO1WedbgFZdo8nn7YgNAPsAJExhG8=:2026-08-08T00:30:17+07:00
==> X-SIGNATURE: 23gUIhaqYO0BzrzQ0RI9evW/COlsCTmfFtHOtccENSSKPyIhl0kF6QL9Ztgk2phfY1fFVhNR/GXSGvH5SlYUIA==

{
  "responseCode": "2003100",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "0030090000000011",
    "virtualAccountNo": "123450030090000000011"
  }
}
==> [cancel pending VA#1] OK (responseCode: 2003100)

```

### Step 3/7: Re-create VA #1's number -> expect success (cancelled VAs are reusable)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:17+07:00
==> X-SIGNATURE: i3NB2Bb5xa6Kv3/Kcp4JKcyfE2mYRbI8H9pwTLHmdoiTAptWzBrfdfJavyTd5x95s1aXBO+rjh9ZYZ5McA6eB43MdqzSz5W5prCbaSSyubMp5l83qivkq2nc24eFxmM4keFL4NRKqn6NI9UaaAWDwOQ1N1CF77gICFOJzZpH0GdN+EWi98Jc1SSEuWmrD3UuiWkxkXQ/Xiq/+Pmrx57LFI/OtHr7/QTVOJNat8h+SN6jWXQcDdb6llaJ/7f5LJZjQ2DQ9AywZL+lwqhVUgkY8dDou6PUJHPRYFF4ptE2DoOztlA6HW8FIFmTCZItbKTf77RB3Ka/IzjCMSRiflj8ew==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 123450030090000000011
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:iGrHAYve1IKLbSxzjKP6Gfx4JJaFK50NVKdBFK7FD2c=:2026-08-08T00:30:17+07:00
==> X-SIGNATURE: 4G6UsQ6SZZjNV5r0uUtlg99Dtm5nuAkO8FggDLe0BipokqonRr3YqpkxLrlcEO0sUdCjAwC6ZoUixvBleAbVtg==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "0030090000000011",
  "virtualAccountNo": "123450030090000000011",
  "virtualAccountName": "Cancel 003009",
  "trxId": "TRX-178612381728477",
  "totalAmount": {
    "value": "120000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "0030090000000011",
    "virtualAccountNo": "123450030090000000011",
    "virtualAccountName": "Cancel 003009",
    "trxId": "TRX-178612381728477",
    "totalAmount": {
      "value": "120000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C"
  }
}
==> [re-create VA#1 after cancel] OK (responseCode: 2002700)

```

### Step 4/7: Create VA #2 (to be paid; merchant identity)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:17+07:00
==> X-SIGNATURE: i3NB2Bb5xa6Kv3/Kcp4JKcyfE2mYRbI8H9pwTLHmdoiTAptWzBrfdfJavyTd5x95s1aXBO+rjh9ZYZ5McA6eB43MdqzSz5W5prCbaSSyubMp5l83qivkq2nc24eFxmM4keFL4NRKqn6NI9UaaAWDwOQ1N1CF77gICFOJzZpH0GdN+EWi98Jc1SSEuWmrD3UuiWkxkXQ/Xiq/+Pmrx57LFI/OtHr7/QTVOJNat8h+SN6jWXQcDdb6llaJ/7f5LJZjQ2DQ9AywZL+lwqhVUgkY8dDou6PUJHPRYFF4ptE2DoOztlA6HW8FIFmTCZItbKTf77RB3Ka/IzjCMSRiflj8ew==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 123450030090000000012
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:r6Dsvngb8vh0rHhJIaUW7aCpd4ZnTCnai4T4S0/3TME=:2026-08-08T00:30:17+07:00
==> X-SIGNATURE: 42YfIiI0krto+64h3ZATP5/la0ZXMXiSIkXIx7MXNRfa/GxmiAfWbxADvPWao2tmjkKMNXJ1q6XY9A+fAf8F1Q==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "0030090000000012",
  "virtualAccountNo": "123450030090000000012",
  "virtualAccountName": "Cancel 003009",
  "trxId": "TRX-17861238174124",
  "totalAmount": {
    "value": "120000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "0030090000000012",
    "virtualAccountNo": "123450030090000000012",
    "virtualAccountName": "Cancel 003009",
    "trxId": "TRX-17861238174124",
    "totalAmount": {
      "value": "120000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C"
  }
}
==> [create VA#2] OK (responseCode: 2002700)

```

### Step 5/7: Pay VA #2 -> status becomes paid (00; vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:17+07:00
==> X-SIGNATURE: Ox/VI8OW+4rERvGqyVInz3HT5/uSNbK5iT7bufuZTUAfgTfbyrlO7MUO3f2tlQL0LF+Rt6WlgCwohYlO/H8i3mZN1zn7RUaBHXqMrrkXwjoMxATXourJRsaWLCOUTyM6uE4zPFp8K/mDTZbfODBIlx67pn0e/HCzALWTfjbCqDVj2nj2fyi7iMZWPPd9V3pMW/zwYGreLAckuZKfA/Vx6kpRiNcrfnk7a7RZeqN+qDVCqu6NIMw4pOGlHTi8MXQ83Or8UmiTGxnUMdQtB/dIPT+pQaMy6RTR8Cm8iGKasERQZGV3x67zIJv1PaPz3CA7+NutUtcK/8UYpmb47WD8mA==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 123450030090000000012
==> paymentRequestId: PAY-17861238173119
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:3262216ca3a7c99fc8099b17b7034208c604371144a70d5d6835fe2fe74a72de:2026-08-08T00:30:17+07:00
==> X-SIGNATURE: EEzN2M5oCyu/v4IpL7rbKvbma6KWFsanEnUNpHWGNh1aeXO5gV4D/DDzt0Q5iyNG9stC68sX0bBQa2NK4HN27A==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "0030090000000012",
  "virtualAccountNo": "123450030090000000012",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-17861238173119",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "120000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "120000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:17+07:00",
  "referenceNo": "R786123817"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "0030090000000012",
    "virtualAccountNo": "123450030090000000012",
    "virtualAccountName": "Payer Name",
    "trxId": "TRX-17861238174124",
    "paymentRequestId": "PAY-17861238173119",
    "paidAmount": {
      "value": "120000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "120000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:17+07:00",
    "referenceNo": "R786123817",
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
==> [pay VA#2] OK (responseCode: 2002500)

```

### Step 6/7: Try cancelling VA #2 (now paid) -> expect REJECTION (merchant identity)

```http
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:17+07:00
==> X-SIGNATURE: i3NB2Bb5xa6Kv3/Kcp4JKcyfE2mYRbI8H9pwTLHmdoiTAptWzBrfdfJavyTd5x95s1aXBO+rjh9ZYZ5McA6eB43MdqzSz5W5prCbaSSyubMp5l83qivkq2nc24eFxmM4keFL4NRKqn6NI9UaaAWDwOQ1N1CF77gICFOJzZpH0GdN+EWi98Jc1SSEuWmrD3UuiWkxkXQ/Xiq/+Pmrx57LFI/OtHr7/QTVOJNat8h+SN6jWXQcDdb6llaJ/7f5LJZjQ2DQ9AywZL+lwqhVUgkY8dDou6PUJHPRYFF4ptE2DoOztlA6HW8FIFmTCZItbKTf77RB3Ka/IzjCMSRiflj8ew==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> DELETE http://localhost:18091/openapi/v1.0/transfer-va/delete-va
==> virtualAccountNo: 123450030090000000012
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: DELETE:/openapi/v1.0/transfer-va/delete-va:<accessToken>:D8J+BHTjIRpq2JWs9/XaZBZpTucWHZLPdmllc8Gt1H0=:2026-08-08T00:30:17+07:00
==> X-SIGNATURE: vKPxi+MSwZhpB6KLk8LG/HuVWnWB2ldx6YRFeJoUUKHHr+v8HdPbpXYOwOwu01Q58TCgjBLFMsI8M2fsl3KV0g==

{
  "responseCode": "4053101",
  "responseMessage": "Requested Operation Is Not Allowed"
}
==> [cancel paid VA#2 (must be rejected)] OK (responseCode: 4053101)

```

### Step 7/7: Try paying VA #2 again with a NEW paymentRequestId -> expect REJECTION (vendor identity)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:17+07:00
==> X-SIGNATURE: Ox/VI8OW+4rERvGqyVInz3HT5/uSNbK5iT7bufuZTUAfgTfbyrlO7MUO3f2tlQL0LF+Rt6WlgCwohYlO/H8i3mZN1zn7RUaBHXqMrrkXwjoMxATXourJRsaWLCOUTyM6uE4zPFp8K/mDTZbfODBIlx67pn0e/HCzALWTfjbCqDVj2nj2fyi7iMZWPPd9V3pMW/zwYGreLAckuZKfA/Vx6kpRiNcrfnk7a7RZeqN+qDVCqu6NIMw4pOGlHTi8MXQ83Or8UmiTGxnUMdQtB/dIPT+pQaMy6RTR8Cm8iGKasERQZGV3x67zIJv1PaPz3CA7+NutUtcK/8UYpmb47WD8mA==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 123450030090000000012
==> paymentRequestId: PAY-178612381724827
==> X-TIMESTAMP: 2026-08-08T00:30:17+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:f6097da80b4740e8df54f6653ef602881b0d9006cd358f96f56b13a20e34e7ff:2026-08-08T00:30:17+07:00
==> X-SIGNATURE: Z8eIqG+SbGsIfNJA0xPq5hcj6wHeBGAksgQMXcKObnvwJjHwTWvh0HVBN+CHcV+Dg54DZEU4rH2Hc9ACRj1nug==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "0030090000000012",
  "virtualAccountNo": "123450030090000000012",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-178612381724827",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "999999.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "999999.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:17+07:00",
  "referenceNo": "R786123817"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "4042514",
  "responseMessage": "Paid Bill",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "0030090000000012",
    "virtualAccountNo": "123450030090000000012",
    "virtualAccountName": "Cancel 003009",
    "trxId": "TRX-17861238174124",
    "paymentRequestId": "PAY-178612381724827",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "120000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:17+07:00",
    "referenceNo": "R786123817",
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
==> [re-pay already-paid VA#2 (must be rejected as Paid Bill)] OK (responseCode: 4042514)

==================================================================
Done: pending VA cancel + reuse works; paid VA can neither be cancelled
      nor have its payment silently overwritten.
==================================================================
```


---

## Expiry and callback resend

```http

!! No -w given — VA will be created without a notificationUrl. Expiry status/response-code
   assertions (steps 4/5/7) still run; callback delivery checks (steps 6/9) and the resend
   endpoint (step 8, which requires a prior delivery record) will be skipped.

```

### Step 1/8: POST /openapi/v1.0/transfer-va/create-va (expiring in 5s; merchant identity)

```http
==> Fetching accessToken for merchant client 7df12682-4906-4992-a141-b95ec8e2e103...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:18+07:00
==> stringToSign: 7df12682-4906-4992-a141-b95ec8e2e103|2026-08-08T00:30:18+07:00
==> X-SIGNATURE: YNNB0RcH0evbPXY4+rqX6SMlpqrxpbw1aUpda3LKTvcFySpOHHyaJlbsv+4A4QVun0oJsUL8XBvAmtefCn65i0rgbkKCAOjATxNnUPMPJuAHb8uBRtOgtA3FjMUAjSBNNTCQw3fsBsmaVSPZnk6c6Fng+ILf+GzSLHqICaVJIvVutfZrlOTOvbjf694ORnLXy2DnaafXWrT1T45dwmUR2DHSPcA4SLWL7FZqVp7NIdjUEhc03nTXY2oKY/RXihbbGJoeswjavUxywIcsUKkguYdQCf2B8PqwXmmc5q3AcYobbS3ijfAuwmbHDeGYzzp70wrgNH6gq9gzQzIkFbpa9A==
==> X-CLIENT-KEY: 7df12682-4906-4992-a141-b95ec8e2e103
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/create-va
==> virtualAccountNo: 12345003009000000009
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:18+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/create-va:<accessToken>:WUnLn2UAJZwKZNHseJ0RQS/7MVRYDXoO+juHyFbKWcU=:2026-08-08T00:30:18+07:00
==> X-SIGNATURE: Ub3vgPegIbyaNSmQVLURKr0s7h8VI2VJ/U93PrTmckoR6m62djWioy32byROz3x8euEDzAXT/OLiAYP9XnnHbg==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "003009000000009",
  "virtualAccountNo": "12345003009000000009",
  "virtualAccountName": "Expiring 003009",
  "trxId": "TRX-178612381829212",
  "expiredDate": "2026-08-07T17:30:22+00:00",
  "totalAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "virtualAccountTrxType": "C"
}

{
  "responseCode": "2002700",
  "responseMessage": "Success",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "003009000000009",
    "virtualAccountNo": "12345003009000000009",
    "virtualAccountName": "Expiring 003009",
    "trxId": "TRX-178612381829212",
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "virtualAccountTrxType": "C",
    "expiredDate": "2026-08-07T17:30:22Z"
  }
}
==> virtualAccountNo: 12345003009000000009
==> expiredDate: 2026-08-07T17:30:22+00:00

```

### Step 2/8: waiting 10s for the VA to pass its expiredDate

```http
==> done waiting

```

### Step 3/8: POST /openapi/v1.0/transfer-va/inquiry (expect 4042419)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:28+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:28+07:00
==> X-SIGNATURE: m8JvZY+FvjjkdNiKzpQVsj7troAG7pGvkzXWMjsiCAGbxat5XO6j5gnN4so+AabTXCUU0YAD0GQhFnGrqrxy0jgjBOZUTAhZkVSXh9UO49ciEO4E88REYHxHsLEG/2uPzMnQZZbOEWanVDYeOPGREZIj1Yys68f9K5yBybkPBmh0DdJiO9edt5Vrjq2DZrGQasUkwBTkW7Xzl9E0OmTOiYeUvaOwVgLYWk0OtAFVF9BZNv7KsrzcqcpM0SPLq200pVj1f/FhICggATHWMwy6vr70t4V3erRbSkvQl0owkzl2cGM37FI9nyNHyhyfm82X/PTW5O28K9TPklWhx+gITw==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:28+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:890a5a16603b797afe063e250387af1d2f80f8fe5fb8186c2d0844751781da9a:2026-08-08T00:30:28+07:00
==> X-SIGNATURE: qakJV8eu+e8qqhDx5VhqO1gBnWPF1oBGR2klgDhKOwc2OmpkgkGthwxPJi2zzFvZpujxOZyAtfT+PIrsjZOCiQ==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "003009000000009",
  "virtualAccountNo": "12345003009000000009",
  "trxDateInit": "2026-08-08T00:30:28+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612382824339"
}

{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "003009000000009",
    "virtualAccountNo": "12345003009000000009",
    "virtualAccountName": "Expiring 003009",
    "inquiryRequestId": "INQ-178612382824339",
    "totalAmount": {
      "value": "100000.00",
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
==> PASS: inquiry correctly rejected as expired (4042419 / inquiryStatus 01)

```

### Step 4/8: POST /openapi/v1.0/transfer-va/payment (expect 4042519)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:28+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:28+07:00
==> X-SIGNATURE: m8JvZY+FvjjkdNiKzpQVsj7troAG7pGvkzXWMjsiCAGbxat5XO6j5gnN4so+AabTXCUU0YAD0GQhFnGrqrxy0jgjBOZUTAhZkVSXh9UO49ciEO4E88REYHxHsLEG/2uPzMnQZZbOEWanVDYeOPGREZIj1Yys68f9K5yBybkPBmh0DdJiO9edt5Vrjq2DZrGQasUkwBTkW7Xzl9E0OmTOiYeUvaOwVgLYWk0OtAFVF9BZNv7KsrzcqcpM0SPLq200pVj1f/FhICggATHWMwy6vr70t4V3erRbSkvQl0owkzl2cGM37FI9nyNHyhyfm82X/PTW5O28K9TPklWhx+gITw==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/payment
==> Authorization: Bearer <accessToken>
==> virtualAccountNo: 12345003009000000009
==> paymentRequestId: PAY-17861238281321
==> X-TIMESTAMP: 2026-08-08T00:30:28+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:61f1f1cd17e75f1f056d2cdff49e93e78d7165af8d11f6bc2f1db15911b7dcc4:2026-08-08T00:30:28+07:00
==> X-SIGNATURE: pANu9DMj1rkQPQ3YcW6HaWnL4i9nOGf/nqnPXJMbwzen/afcFd6QZdidCdAPN/d7JwnZLOhUNo28iia9jdrLxg==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "003009000000009",
  "virtualAccountNo": "12345003009000000009",
  "virtualAccountName": "Payer Name",
  "paymentRequestId": "PAY-17861238281321",
  "channelCode": 6011,
  "flagAdvise": "N",
  "paidAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "trxDateTime": "2026-08-08T00:30:28+07:00",
  "referenceNo": "R786123828"
}


==> If a VA with this virtualAccountNo was created via merchant-create-va.sh
    (with a notificationUrl), a callback should have been enqueued to Asynq
    and delivered by the payment_notification_worker shortly after this call.
{
  "responseCode": "4042519",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "003009000000009",
    "virtualAccountNo": "12345003009000000009",
    "virtualAccountName": "Expiring 003009",
    "trxId": "TRX-178612381829212",
    "paymentRequestId": "PAY-17861238281321",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "100000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:28+07:00",
    "referenceNo": "R786123828",
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
==> PASS: payment notification correctly rejected as expired (4042519 / paymentFlagStatus 01)

```

### Step 5/8: verify the automatic va.expired callback was delivered

```http
==> skipped (no -w webhook.site token given)

```

### Step 6/8: repeat inquiry (expect 4042419 again, NO second callback)

```http
==> Fetching accessToken for vendor client cefe3c4b-a796-4a6c-a42c-ed11c750d746...
==> POST http://localhost:18091/openapi/v1.0/access-token/b2b
==> X-TIMESTAMP: 2026-08-08T00:30:28+07:00
==> stringToSign: cefe3c4b-a796-4a6c-a42c-ed11c750d746|2026-08-08T00:30:28+07:00
==> X-SIGNATURE: m8JvZY+FvjjkdNiKzpQVsj7troAG7pGvkzXWMjsiCAGbxat5XO6j5gnN4so+AabTXCUU0YAD0GQhFnGrqrxy0jgjBOZUTAhZkVSXh9UO49ciEO4E88REYHxHsLEG/2uPzMnQZZbOEWanVDYeOPGREZIj1Yys68f9K5yBybkPBmh0DdJiO9edt5Vrjq2DZrGQasUkwBTkW7Xzl9E0OmTOiYeUvaOwVgLYWk0OtAFVF9BZNv7KsrzcqcpM0SPLq200pVj1f/FhICggATHWMwy6vr70t4V3erRbSkvQl0owkzl2cGM37FI9nyNHyhyfm82X/PTW5O28K9TPklWhx+gITw==
==> X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
==> Request body:
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}

==> POST http://localhost:18091/openapi/v1.0/transfer-va/inquiry
==> Authorization: Bearer <accessToken>
==> X-TIMESTAMP: 2026-08-08T00:30:28+07:00
==> stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:a7c41c710c64daffa20dac36a63726a1480fed4f87be5ff3bb1dac1979fa38b6:2026-08-08T00:30:28+07:00
==> X-SIGNATURE: IX3coo8reSQFSIhrIs9ZQxQovz7eBGVMehe2ILqhWb/sEhGII7JU3BfTPbucq6G/8Fn14h2y+84G7UlcpMH03A==
==> Request body:
{
  "partnerServiceId": "12345",
  "customerNo": "003009000000009",
  "virtualAccountNo": "12345003009000000009",
  "trxDateInit": "2026-08-08T00:30:28+07:00",
  "channelCode": 6011,
  "amount": {
    "value": "100000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "INQ-178612382815165"
}

{
  "responseCode": "4042419",
  "responseMessage": "Invalid Bill/Virtual Account",
  "virtualAccountData": {
    "partnerServiceId": "12345",
    "customerNo": "003009000000009",
    "virtualAccountNo": "12345003009000000009",
    "virtualAccountName": "Expiring 003009",
    "inquiryRequestId": "INQ-178612382815165",
    "totalAmount": {
      "value": "100000.00",
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
==> PASS: still returns 4042419

```

### Step 7/8: POST /admin/transactions/12345003009000000009/resend-callback

```http
==> skipped (no -w webhook.site token given, and resend requires a prior delivery record which needs a registered notificationUrl)

```

### Done. Review https://webhook.site/#!/view/<no-token-given> for the full raw delivery history (auto + manual, headers include X-Timestamp/X-Signature).

```http
```

