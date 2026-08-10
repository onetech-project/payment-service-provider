# SNAP Virtual Account — live negative-case transcript

Produced by `scripts/e2e-negative-cases.sh` against a running deployment
(real Postgres, real Redis, real access tokens). Each case shows the request
exactly as it went on the wire and the response exactly as it came back.

- Generated: `2026-08-08T00:30:35+07:00`
- Commit: `25b9dab`
- Target: `http://localhost:18091`

`Authorization: Bearer` values are redacted — they are single-use tokens and
add nothing to a conformance review. Signatures are kept: they are computed
over the redacted token, so they cannot be recomputed from this file, and
seeing them is the point of a signature transcript.


---

## Authentication and signature

### signature computed with the wrong secret

Expected `401` / `4012400` — got `401` / `4012400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:36+07:00
X-SIGNATURE: un1ikfwujGonuf3Ryh23c1/89MmvR2LPz3xjPHqJKlo9cdwVo9U+Z2ayUj5yWm/8taXKxCqYNG9yMQmCstmVeA==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030361925
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:b2aa4db875170ff40dd90d60eda0e0d3aa25082f2bb90085a4f157a2aafa764e:2026-08-08T00:30:36+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-1"}
```

**Response**

```http
HTTP/1.1 401

{
  "responseCode": "4012400",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### garbage signature

Expected `401` / `4012400` — got `401` / `4012400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:36+07:00
X-SIGNATURE: this-is-not-base64-hmac
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303618183
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:56e40c8b422ed90cc00358ed747197f85e18bc145729064a0dc34a2e510085fc:2026-08-08T00:30:36+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-2"}
```

**Response**

```http
HTTP/1.1 401

{
  "responseCode": "4012400",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### body tampered after signing

Expected `401` / `4012400` — got `401` / `4012400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:36+07:00
X-SIGNATURE: vk2vV1rBsSCIOxCybwcbvw145drUZ2tm9Zj4970gAE3w1E4ZEwl1chpideOA6QPZorFX7y7uEpW12JWnz71hDw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303614039
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:940d772435987041898a54721d6c4d11a239f4efe01a327bece107a0f08b9f85:2026-08-08T00:30:36+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-4b"}
```

**Response**

```http
HTTP/1.1 401

{
  "responseCode": "4012400",
  "responseMessage": "Unauthorized. [Signature]",
  "data": {}
}
```

### X-SIGNATURE missing

Expected `400` / `4002402` — got `400` / `4002402` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:36+07:00
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 20260808003036141
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:5e73e80ed7f2443ae2852f1f676cd8bcd53948489163048cad91474123f075db:2026-08-08T00:30:36+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-5"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [X-SIGNATURE]",
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

### X-TIMESTAMP missing

Expected `400` / `4002402` — got `400` / `4002402` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-SIGNATURE: DZfHb/PeLIw5FTWu5NBY2rv723oYykQMW1OFYsE0XDLNWp2oZguLAMfBAgVuiEhrKGX/IWpKru3xT9rbYKa17w==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303611168
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:393f7c8b87a7235b4443abd6884ac5606308f41fff997c091685232423bf1ff7:2026-08-08T00:30:36+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-6"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [X-TIMESTAMP]",
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

### X-TIMESTAMP not parseable

Expected `400` / `4002401` — got `400` / `4002401` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: not-a-timestamp
X-SIGNATURE: fc0pRu557xzTih5XNWFVZrFa/h5y9fa7GcPrNah9JeZivPCJdJ4xurVIvikcuEFbwyLxmE6vJljmKzkStflaBg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030368074
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:8070fb115a86adecbf0bec01d81b93412ead8aa58d0e97a95c2b21dabc975828:not-a-timestamp

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-7"}
```

**Response**

```http
HTTP/1.1 400

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

### Authorization missing

Expected `401` / `4012401` — got `401` / `4012401` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: DXwDSUYR6I/vLRD2TKeFtAMuvNA2pgi098WM5Tp4RKYk5LNm+APS4smffUiTLeU/QAIK0c/osoNUz/ODm3QSUg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303718607
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:5bcedca7bfe1e0a679ac02de37cfaaca59b188956895710f18fff9165b77eb7a:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:36+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-8"}
```

**Response**

```http
HTTP/1.1 401

{
  "responseCode": "4012401",
  "responseMessage": "Invalid Token (B2B)",
  "data": {}
}
```

### Authorization carries an unusable token

Expected `401` / `4012401` — got `401` / `4012401` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: HZhS7TJY1+qN679fL2AMlVAha7cl08p/HaxS1HxW0PajnXKtDUORS0ilJzBe5FvA1F3Apufoi8SRoD6PTSrrhw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303710151
Authorization: Bearer not.a.valid.token
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:e404d1305379e8ae8c499d30bdad5509c38383374172b0a18eb9b81555afd848:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:37+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-9"}
```

**Response**

```http
HTTP/1.1 401

{
  "responseCode": "4012401",
  "responseMessage": "Invalid Token (B2B)",
  "data": {}
}
```


---

## X-EXTERNAL-ID

### X-EXTERNAL-ID missing

Expected `400` / `4002402` — got `400` / `4002402` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: nO3qjGZ4DiDrLVOop/024qnjvUYBGMZgEMj+mr2+h+8DPQRp55aioFK8ooBQ7ixqp5RlTOqqB8j78vP6l/K2ew==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:316614f13a9a35851acb9604eded08c70bf1850cd317656b3587c81e71b0d054:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:37+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-10"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [X-EXTERNAL-ID]",
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

### X-EXTERNAL-ID longer than 36 characters

Expected `400` / `4002401` — got `400` / `4002401` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: d0YmhPnEUoMAlhtfrbE38URPl+dsdNeHGTHfj59UDT9vpBSUThLaS2DnELwqMOWwZhwazJzbslTvUKpyIHQdnw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 9999999999999999999999999999999999999999
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:6ccc29d1130558891a0df065b20d71b22cd7659d6f4edeeeccae47cb3b22601e:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:37+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-11"}
```

**Response**

```http
HTTP/1.1 400

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

### first use of an X-EXTERNAL-ID is answered normally

Expected `200` / `2002400` — got `200` / `2002400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: cgXhPky0ZOaQmb3hqqkqLPb6ev7ThJQjRSMwIAaPqvWEmiCr+NXnDyO3SdRD6t5VSu/thrO8KZ8Ui6rgdJ9aaw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303714741
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:870582dbcdae2aac783551f3ae0cb7ec3e1fdf74091e9c30946d3cbb8ccab1e4:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:37+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-12"}
```

**Response**

```http
HTTP/1.1 200

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "Neg Unpaid 0030354950",
    "inquiryRequestId": "INQ-NEG-0030354950-12",
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

### same X-EXTERNAL-ID with a different payload is a Conflict

Expected `409` / `4092400` — got `409` / `4092400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: tmMRFe+YYZ9aHtd7KHvggLxRTfo50Gpbe/nbAOoFbu4NR7z3zel6/BNzLMKEzu+CBBd68cRaRFkqJ/pDDurxpg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303714741
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:ded44757f45f2e63a6c562ad273c9c7dcec70b501d4659cf8a10a1afc421e6c1:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:37+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-13"}
```

**Response**

```http
HTTP/1.1 409

{
  "responseCode": "4092400",
  "responseMessage": "Conflict",
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
      "english": "Cannot use the same X-EXTERNAL-ID",
      "indonesia": "Tidak bisa menggunakan X-EXTERNAL-ID yang sama"
    }
  },
  "additionalInfo": {}
}
```


---

## Request payload

### malformed JSON on inquiry

Expected `400` / `4002400` — got `400` / `4002400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: vpPx9eCNB+pxI/GZWB8+cNfqgiMiOm2+9DtmL8bc7H2ctti0ExPy2vELjkU+O5Lmtxj7RAc/VSFfqUT6UGXn2Q==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303714339
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:fd1ea29b71f70979c029efc2c455e4749dd8187dda368eeeb6f1430f8563ccb6:2026-08-08T00:30:37+07:00

{"partnerServiceId": "15975", 
```

**Response**

```http
HTTP/1.1 400

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

### malformed JSON on payment

Expected `400` / `4002500` — got `400` / `4002500` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: Jz2IrIZT/DB/LF4XNM4ScAYFtxe9cRSuVb8RqPlT3Z4b3noTWa9Vut9+8Pwe7A8qfMb2bJNY1VgMv9Q2GYYQiA==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303714276
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:fd1ea29b71f70979c029efc2c455e4749dd8187dda368eeeb6f1430f8563ccb6:2026-08-08T00:30:37+07:00

{"partnerServiceId": "15975", 
```

**Response**

```http
HTTP/1.1 400

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

### inquiry without partnerServiceId

Expected `400` / `4002402` — got `400` / `4002402` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: s6MJGot2YxrfEKw75fBrdpyAsjdAMaaO1sl3m1xx7PeHHkJurHOEwMkehHqQaQEnXa0MLGzbUGVmtLTcQfqQmg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303725139
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:4ef5dcb4cd091eecf3b1fafda9733fbb23d349d67ebe71c9b888eb52a0111168:2026-08-08T00:30:37+07:00

{"customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:37+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-14"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [partnerServiceId]",
  "virtualAccountData": {
    "partnerServiceId": "",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-14",
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

### inquiry without trxDateInit (Mandatory in v2.4)

Expected `400` / `4002402` — got `400` / `4002402` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:37+07:00
X-SIGNATURE: a5IYTrf9Yxz8Aj9ODEQwTKt9jz2vyYTl8e9v8c5KhrutAqOGWoJY1NLUY+1wFtsMtx+4kdZhO+JM+BMa5OqbOg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303710787
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:e8e006844703edd781e81d94e7c3a69a4c1279c430c3d2c93f501922aeafea58:2026-08-08T00:30:37+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-15"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [trxDateInit]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-15",
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

### inquiry without channelCode (Mandatory in v2.4)

Expected `400` / `4002402` — got `400` / `4002402` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: nkYlS3VR4+o37KJuWl91YMGkfcfxIgiN4JtW2H+1Tic4DZor91S6eJIduSC8vdUsSSuHRGWpTIQHEqKswgzepg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303817544
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:81ccc149ad5f5053f62df17c9000708efade808140deda6063c2582d10d621da:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:38+07:00","inquiryRequestId":"INQ-NEG-0030354950-16"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002402",
  "responseMessage": "Invalid Mandatory Field [channelCode]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-16",
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

### inquiry language longer than 2 characters

Expected `400` / `4002401` — got `400` / `4002401` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: 7xXGY9veEkziH+2WIKmr/5ojWlhgXsUkBegz4SNT9YvQzLa0nao8IHSwubQJQgcmbwpWsPAeKZRdqa0BmPPCUw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303827822
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:43f713c9efb3bf6df3599243bd00f6e58ed074af4e88414b5c591da4ce483e02:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:38+07:00","channelCode":6011,"language":"idn","inquiryRequestId":"INQ-NEG-0030354950-17"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002401",
  "responseMessage": "Invalid Field Format [language]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-17",
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

### passApp is not a credential and must not gate the request

Expected `200` / `2002400` — got `200` / `2002400` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: K2TPLgEe7Q91XnC+b0LFVRZpik+AisYdYxBtt/FX1Whjn8nWzP+fo6tjwoyXBWAMmucK1C5N1zzYncZP9HU0UA==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303828061
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:fea91ce3c2d9102591be540124ed60668911ce91653a750b0930f70901b5177b:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","trxDateInit":"2026-08-08T00:30:38+07:00","channelCode":6011,"passApp":"wrong-value-entirely","inquiryRequestId":"INQ-NEG-0030354950-18"}
```

**Response**

```http
HTTP/1.1 200

{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "Neg Unpaid 0030354950",
    "inquiryRequestId": "INQ-NEG-0030354950-18",
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

### payment without paymentRequestId

Expected `400` / `4002502` — got `400` / `4002502` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: kzM3WQfq8dyHukjlfUhLO+O7Acue+4vqNhY2UUXK84pCaISYZml3lgizMVLlQnf7679pQFbC9StEkxorOxGJ4g==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303829619
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:9308e5af6145b4cb44e51a5f9aaede408320bcb24e40790f75f645a6940ac763:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","channelCode":6011,"paidAmount":{"value":"250000.00","currency":"IDR"},"totalAmount":{"value":"250000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:38+07:00","referenceNo":"12345678901","flagAdvise":"N"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [paymentRequestId]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
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

### payment without paidAmount

Expected `400` / `4002502` — got `400` / `4002502` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: og594hMztd9gijy3ZekGBbUOl83fqE9v+Sy0IfbgzRt6xXLaj2CsC9ur/eUsS2omam0wDQ1br6DcVkYc+qxgCA==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030382950
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:ed7655213f43f8b95ae79ccacbc8d76bdba5d1536613e3c1503ecf0fecf4f6b3:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-1","channelCode":6011,"totalAmount":{"value":"250000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:38+07:00","referenceNo":"12345678901","flagAdvise":"N"}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002502",
  "responseMessage": "Invalid Mandatory Field [paidAmount]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-NEG-0030354950-1",
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

### flagAdvise outside N/Y

Expected `400` / `4002501` — got `400` / `4002501` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: ZCrh5Wl9g9AOHS23SCOqEwMrHxmG8vfcPYyEoJUJSH6v4Xrd89rGZe01NUPBc7McpXrKG/045xG9CYCabf+h3g==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303816647
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:3d86bdbf2ceeca9ff671e755182fd5b1da74bda4e4a898697092ef4536392ee2:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-2","channelCode":6011,"paidAmount":{"value":"250000.00","currency":"IDR"},"totalAmount":{"value":"250000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:38+07:00","referenceNo":"12345678901","flagAdvise":"MAYBE","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [flagAdvise]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-NEG-0030354950-2",
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

### currency outside BCA's IDR/SGD/USD set

Expected `400` / `4002501` — got `400` / `4002501` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: homasVVasptyzNEAe3sKofiPT+xrryV2C4imxaKSsDQkSVpm4mmLUDrF0HXGbFDmJW3RnKDAiV93FdPwEG32AQ==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303829263
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:3b6982acb00eff980ab356ed20f2002f5f3dd53c7c91aca74a4133b31856aedc:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-3","channelCode":6011,"paidAmount":{"value":"250000.00","currency":"EUR"},"totalAmount":{"value":"250000.00","currency":"EUR"},"trxDateTime":"2026-08-08T00:30:38+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [paidAmount.currency]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-NEG-0030354950-3",
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

### currency disagrees between paidAmount and totalAmount

Expected `400` / `4002501` — got `400` / `4002501` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: IXIYH2mZWd80bZh7yNSlCGTRuaI5Sufd+utTyx1l3S2joHU5I3quOZ8TB8ryk3XsMGB/ik8eOPuBgDTURrjTLA==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303829147
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:3c70dd7f1dc017bf12b8ab3244786ab279e2764d55679f2c70b08e98abd6155a:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-3b","channelCode":6011,"paidAmount":{"value":"250000.00","currency":"USD"},"totalAmount":{"value":"250000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:38+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [totalAmount.currency]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-NEG-0030354950-3b",
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

### paidAmount is not numeric

Expected `400` / `4002501` — got `400` / `4002501` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:38+07:00
X-SIGNATURE: xMlVF7kphCs6431mRmR79qcdYyL1w4ksjLD63X2haIPMAcrCxk9J54f3kWtx8T38JnCRX8j3g84SfbU1b4gZ0g==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030381414
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:19af75ba90b75813aeff8f8830a04fc3fb66ff2ecd09d35de492c41a95468f50:2026-08-08T00:30:38+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-4","channelCode":6011,"paidAmount":{"value":"not-a-number","currency":"IDR"},"totalAmount":{"value":"not-a-number","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:38+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 400

{
  "responseCode": "4002501",
  "responseMessage": "Invalid Field Format [paidAmount.value]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-NEG-0030354950-4",
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


---

## Business rejections against stored state

### inquiry on a VA that was never registered

Expected `404` / `4042412` — got `404` / `4042412` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: l8y/LoNAiSs5rsyrFzE3yv/QchnTzTzGzuOS3ULZjydewwIO4G9UFh/5brOgwBHIV71EWf0m0TlzR0h9NZsHmw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303925006
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:33afe8a772be904a95300f3f4f1f765878bb5286e255be57de0b466f5980fb5b:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","trxDateInit":"2026-08-08T00:30:39+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-20"}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-20",
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

### payment on a VA that was never registered

Expected `404` / `4042512` — got `404` / `4042512` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: /8rw1sCQJaKwl82W1sfB0T5Qy/skmoEHbE69jzd9lwHMvuHAYAovKtZrEB3wJp1Rir/Sq9i+Ugy4cOKM5UUBcg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303925900
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:7fa4b7cb03b92ce9b5e9094797833e10df53ff2f998395bdcf40c5285939b2a1:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-5","channelCode":6011,"paidAmount":{"value":"250000.00","currency":"IDR"},"totalAmount":{"value":"250000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:39+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042512",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "virtualAccountName": "",
    "paymentRequestId": "PAY-NEG-0030354950-5",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "",
      "currency": ""
    },
    "trxDateTime": "2026-08-08T00:30:39+07:00",
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

### payment amount disagrees with the fixed bill

Expected `404` / `4042513` — got `404` / `4042513` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: bRijhMuc/wFGF+wbzsso1rd+laEXgu8THAHBDBd709KRgJApS8SOOECxq91bsHcq70mPI27T3qHBYh9QaFiOtQ==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030393193
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:0e1707408bd3ff38dba5123ae6852a8a661f9cd31701897dc32077195ceeb6c6:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000001","virtualAccountNo":"15975038612383500000001","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-6","channelCode":6011,"paidAmount":{"value":"1000.00","currency":"IDR"},"totalAmount":{"value":"1000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:39+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042513",
  "responseMessage": "Invalid amount",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000001",
    "virtualAccountNo": "15975038612383500000001",
    "virtualAccountName": "Payer Name",
    "paymentRequestId": "PAY-NEG-0030354950-6",
    "paidAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "1000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:39+07:00",
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

### inquiry on an already-paid bill

Expected `404` / `4042414` — got `404` / `4042414` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: TVz6WYnAelE9fHlA9aDoXQvPb0uNwcK3Ab/J3Xo8du/9xQSatdhDaUZ/M/RXsifBWHmbiN3fCDO3NPIJwLxaVw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030392874
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:2938a7965fd20a99e6a45694ed342679c7eb99abf9393406c4b2e7c92824ac64:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000002","virtualAccountNo":"15975038612383500000002","trxDateInit":"2026-08-08T00:30:39+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-21"}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042414",
  "responseMessage": "Paid Bill",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000002",
    "virtualAccountNo": "15975038612383500000002",
    "virtualAccountName": "Neg Paid 0030354950",
    "inquiryRequestId": "INQ-NEG-0030354950-21",
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

### second payment on an already-paid bill

Expected `404` / `4042514` — got `404` / `4042514` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: 2QgadjreuhIJyUxStHTryZsfxivQ32u0l5or8VMCf/DdgtCYPKq/Sc9QYm/Yph6eqbtj6yrJVuKGE1dn39sNuQ==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030396819
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:4c0209d26284c1fca721398d0793140bc8826d7a5d9fc9fa1de0541beceea88c:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000002","virtualAccountNo":"15975038612383500000002","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-0030354950-7","channelCode":6011,"paidAmount":{"value":"250000.00","currency":"IDR"},"totalAmount":{"value":"250000.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:39+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042514",
  "responseMessage": "Paid Bill",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000002",
    "virtualAccountNo": "15975038612383500000002",
    "virtualAccountName": "Neg Paid 0030354950",
    "trxId": "trx-neg-paid-0030354950",
    "paymentRequestId": "PAY-NEG-0030354950-7",
    "paidAmount": {
      "value": "",
      "currency": ""
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:39+07:00",
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

### same paymentRequestId resubmitted with different content

Expected `404` / `4042518` — got `404` / `4042518` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/payment HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: mO1kuygMh+//xPDRFEyij/jA2iW+McMQ/2bLhIRjH7Vf4e6bEgMsFKXAgEmKmKXwvh+XRFzip4BRD4aBO4ZFwA==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030399851
stringToSign: POST:/openapi/v1.0/transfer-va/payment:<accessToken>:eedfd258a00e60e554e01e8b061569297630b8da11d9264c8e16369a560837c6:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"038612383500000002","virtualAccountNo":"15975038612383500000002","virtualAccountName":"Payer Name","paymentRequestId":"PAY-NEG-PAID-0030354950","channelCode":6011,"paidAmount":{"value":"1.00","currency":"IDR"},"totalAmount":{"value":"1.00","currency":"IDR"},"trxDateTime":"2026-08-08T00:30:39+07:00","referenceNo":"12345678901","flagAdvise":"N","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042518",
  "responseMessage": "Inconsistent Request",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "038612383500000002",
    "virtualAccountNo": "15975038612383500000002",
    "virtualAccountName": "Neg Paid 0030354950",
    "trxId": "trx-neg-paid-0030354950",
    "paymentRequestId": "PAY-NEG-PAID-0030354950",
    "paidAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "totalAmount": {
      "value": "250000.00",
      "currency": "IDR"
    },
    "trxDateTime": "2026-08-08T00:30:36+07:00",
    "referenceNo": "R786123836",
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

### status for an id that was never issued

Expected `404` / `4042601` — got `404` / `4042601` (**PASS**)

**Request**

```http
POST /openapi/v2.0/transfer-va/status HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: t1f69kc4seDFdqCBS66aN4xhYiUcoaBGyiOmiiLz644LoSDk8+gTkfoj6SeKqyf1TXrwJCBqsO9YOOafGuGqMQ==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030396767
stringToSign: POST:/openapi/v2.0/transfer-va/status:<accessToken>:fbcdbab8540e2a8e5584c49dfcd49b963c4cfea897209cc684e93d666a14cbe4:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","inquiryRequestId":"INQ-DOES-NOT-EXIST-0030354950","additionalInfo":{}}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042601",
  "responseMessage": "Transaction Not Found",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": {
      "english": "Transaction Not Found",
      "indonesia": "Transaksi Tidak Ditemukan"
    },
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "inquiryRequestId": "INQ-DOES-NOT-EXIST-0030354950",
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

## Headers outside BCA's documented set are ignored

### Idempotency-Key changes nothing

Expected `404` / `4042412` — got `404` / `4042412` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: 8+aJhu6LMPvrKRbOSkp3E9kD9Zageed36e78NhmF6pdX9S6JGhKEMfxcw5aLARgK9ut6CrXO5gizFGy33XNTWg==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303912249
Idempotency-Key: 11111111-1111-1111-1111-111111111111
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:b72d044f71489b53ede8d276e0e8a1957b455437ffe54b1b46a4d0c6cda62212:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","trxDateInit":"2026-08-08T00:30:39+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-30"}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-30",
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

### X-CLIENT-KEY (an access-token header) changes nothing

Expected `404` / `4042412` — got `404` / `4042412` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: 5HXENalHtOib3KYKS5L9HqUV6IqhHx5pES+wXhNVq2sWCSCxUGqxZKn1OK3kzr+xsgRbC0ugP2u5MwGcIkv6lw==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030394811
X-CLIENT-KEY: cefe3c4b-a796-4a6c-a42c-ed11c750d746
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:0ba5add94e4f726fdca13ea15cdf3aa7c98c997dc72de40c0d2689bc6bfae15f:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","trxDateInit":"2026-08-08T00:30:39+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-31"}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-31",
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

### other SNAP-ecosystem headers change nothing

Expected `404` / `4042412` — got `404` / `4042412` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:39+07:00
X-SIGNATURE: 9rc3GD+YDvGRaOii8J4qJRZ0y8KkpWOf2XxQ7nszwmah4XWSQQWggv6aNIveatHF0AB8nOf7/ZZHT5zjH6JC9Q==
ORIGIN: www.hostname.com
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 2026080800303927917
X-DEVICE-ID: dev-1
X-IP-ADDRESS: 10.0.0.1
X-LATITUDE: -6.2
X-LONGITUDE: 106.8
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:3d8f2c9857e8eec268abb923ae3937ef717f6f6dcf390dff7e2aeb2962566e6b:2026-08-08T00:30:39+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","trxDateInit":"2026-08-08T00:30:39+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-32"}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-32",
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

### ORIGIN is optional (Mandatory N)

Expected `404` / `4042412` — got `404` / `4042412` (**PASS**)

**Request**

```http
POST /openapi/v1.0/transfer-va/inquiry HTTP/1.1
Content-Type: application/json
Authorization: Bearer <accessToken>
X-TIMESTAMP: 2026-08-08T00:30:40+07:00
X-SIGNATURE: yBmMus53RB8pUBfhaUnbPl/yBeUiPHD6amFRy8FAfZHE3Is/c90cXE6z4vODPCQtOaYJi4g/Ukd5oVUxcg/ftA==
CHANNEL-ID: 95231
X-PARTNER-ID: 111111
X-EXTERNAL-ID: 202608080030405617
stringToSign: POST:/openapi/v1.0/transfer-va/inquiry:<accessToken>:a48760934337f657eda98aa8e5f39f694fb80ca1c4c523b2e865e94617d32bbd:2026-08-08T00:30:40+07:00

{"partnerServiceId":"15975","customerNo":"039999999999999999","virtualAccountNo":"15975039999999999999999","trxDateInit":"2026-08-08T00:30:40+07:00","channelCode":6011,"inquiryRequestId":"INQ-NEG-0030354950-33"}
```

**Response**

```http
HTTP/1.1 404

{
  "responseCode": "4042412",
  "responseMessage": "Invalid Bill/Virtual Account [Not Found]",
  "virtualAccountData": {
    "partnerServiceId": "15975",
    "customerNo": "039999999999999999",
    "virtualAccountNo": "15975039999999999999999",
    "virtualAccountName": "",
    "inquiryRequestId": "INQ-NEG-0030354950-33",
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

