# Postman Guide: Inquiry & Payment VA Endpoints

## Prerequisites

You need these values (from your `.env.bca.va`):

| Variable | Value |
|---|---|
| `CLIENT_ID` | `a9612651-9181-4461-af20-4ef6598a93f5` |
| `CLIENT_SECRET` | `1c19e3313badc5068995ca0b42a4938e96ae57ca1c57ad13cfeed728a9ceb15a` |
| `BASE_URL` | `https://uatbca.manjo.co.id` |
| Private key | `client_private.pem` file in your project root |

---

## Step 1: Set Up Postman Environment

1. Click **Environments** → **+** → Name it `BCA UAT`
2. Add these variables:

| Variable | Type | Initial Value |
|---|---|---|
| `base_url` | default | `https://uatbca.manjo.co.id` |
| `client_id` | default | `a9612651-9181-4461-af20-4ef6598a93f5` |
| `client_secret` | secret | `1c19e3313badc5068995ca0b42a4938e96ae57ca1c57ad13cfeed728a9ceb15a` |
| `private_key` | secret | *(paste the full PEM content of `client_private.pem` here — including `-----BEGIN PRIVATE KEY-----` and `-----END PRIVATE KEY-----`)* |
| `access_token` | default | *(leave empty — auto-filled by Step 2)* |
| `partner_id` | default | `1-MANJO-SNAP` |
| `channel_id` | default | `95231` |

3. Click **Save** and select this environment as active

---

## Step 2: Get Access Token

> [!IMPORTANT]
> You must run this first. The token expires in 15 minutes. Re-run when it expires.

### Request Setup

| Setting | Value |
|---|---|
| Method | `POST` |
| URL | `{{base_url}}/openapi/v1.0/access-token/b2b` |

### Headers

| Key | Value |
|---|---|
| `Content-Type` | `application/json` |
| `X-CLIENT-KEY` | `{{client_id}}` |
| `X-TIMESTAMP` | *(auto-set by pre-request script)* |
| `X-SIGNATURE` | *(auto-set by pre-request script)* |

### Body (raw JSON)

```json
{
  "grantType": "client_credentials",
  "additionalInfo": {}
}
```

### Pre-request Script

```javascript
// ---- B2B Token: RSA-SHA256 asymmetric signature ----
const forge = require('forge') || pm.require('forge');

const timestamp = new Date().toISOString().replace(/\.\d{3}Z$/, '+07:00');
const clientId = pm.environment.get('client_id');
const privateKeyPem = pm.environment.get('private_key');

// stringToSign = clientId|timestamp
const stringToSign = clientId + '|' + timestamp;

// Sign with SHA256withRSA
const privateKey = forge.pki.privateKeyFromPem(privateKeyPem);
const md = forge.md.sha256.create();
md.update(stringToSign, 'utf8');
const signature = forge.util.encode64(privateKey.sign(md));

pm.request.headers.upsert({ key: 'X-TIMESTAMP', value: timestamp });
pm.request.headers.upsert({ key: 'X-SIGNATURE', value: signature });

console.log('stringToSign:', stringToSign);
console.log('X-TIMESTAMP:', timestamp);
```

### Tests Script (auto-save the token)

```javascript
const resp = pm.response.json();
if (resp.accessToken) {
    pm.environment.set('access_token', resp.accessToken);
    console.log('Access token saved!');
}
```

### Expected Response

```json
{
  "responseCode": "2007300",
  "responseMessage": "Successful",
  "accessToken": "eyJhbGciOiJS...",
  "tokenType": "Bearer",
  "expiresIn": "900"
}
```

---

## Step 3: Inquiry

### Request Setup

| Setting | Value |
|---|---|
| Method | `POST` |
| URL | `{{base_url}}/openapi/v1.0/transfer-va/inquiry` |

### Headers

| Key | Value |
|---|---|
| `Content-Type` | `application/json` |
| `Authorization` | `Bearer {{access_token}}` |
| `X-TIMESTAMP` | *(auto-set by pre-request script)* |
| `X-SIGNATURE` | *(auto-set by pre-request script)* |
| `X-PARTNER-ID` | `{{partner_id}}` |
| `X-EXTERNAL-ID` | *(auto-set by pre-request script)* |
| `CHANNEL-ID` | `{{channel_id}}` |

### Body (raw JSON)

```json
{
  "partnerServiceId": "   12345",
  "customerNo": "66666666",
  "virtualAccountNo": "1597366666666",
  "txnDateInit": "{{timestamp}}",
  "amount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "inquiryRequestId": "{{inquiry_request_id}}"
}
```

### Pre-request Script

```javascript
// ---- SNAP Symmetric Signature (HMAC-SHA512) ----
const timestamp = new Date().toISOString().replace(/\.\d{3}Z$/, '+07:00');
const externalId = Date.now().toString() + Math.floor(Math.random() * 9000 + 1000);
const inquiryRequestId = 'INQ-' + Date.now() + Math.floor(Math.random() * 9000 + 1000);

pm.environment.set('timestamp', timestamp);
pm.environment.set('inquiry_request_id', inquiryRequestId);

// Build the exact body that will be sent (minified, same key order)
const body = JSON.stringify({
    partnerServiceId: "   12345",
    customerNo: "66666666",
    virtualAccountNo: "1597366666666",
    txnDateInit: timestamp,
    amount: { value: "150000.00", currency: "IDR" },
    inquiryRequestId: inquiryRequestId
});

// SHA-256 hash of the body (standard base64)
const bodyHash = CryptoJS.SHA256(body).toString(CryptoJS.enc.Base64);

// stringToSign = METHOD:ENDPOINT:ACCESS_TOKEN:SHA256(BODY):TIMESTAMP
const accessToken = pm.environment.get('access_token');
const stringToSign = 'POST:/openapi/v1.0/transfer-va/inquiry:' + accessToken + ':' + bodyHash + ':' + timestamp;

// HMAC-SHA512 signature (standard base64)
const clientSecret = pm.environment.get('client_secret');
const signature = CryptoJS.HmacSHA512(stringToSign, clientSecret).toString(CryptoJS.enc.Base64);

pm.request.headers.upsert({ key: 'X-TIMESTAMP', value: timestamp });
pm.request.headers.upsert({ key: 'X-SIGNATURE', value: signature });
pm.request.headers.upsert({ key: 'X-EXTERNAL-ID', value: externalId });

// Replace the body with the exact signed version
pm.request.body.raw = body;

console.log('inquiryRequestId:', inquiryRequestId);
console.log('bodyHash:', bodyHash);
console.log('stringToSign:', stringToSign);
```

### Expected Response

```json
{
  "responseCode": "2002400",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "inquiryStatus": "00",
    "inquiryReason": { "english": "Success", "indonesia": "Sukses" },
    "partnerServiceId": "15973",
    "customerNo": "66666666",
    "virtualAccountNo": "1597366666666",
    "virtualAccountName": "test lagi",
    "inquiryRequestId": "INQ-...",
    "totalAmount": { "value": "0", "currency": "IDR" },
    "subCompany": "00000"
  }
}
```

---

## Step 4: Payment

### Request Setup

| Setting | Value |
|---|---|
| Method | `POST` |
| URL | `{{base_url}}/openapi/v1.0/transfer-va/payment` |

### Headers

| Key | Value |
|---|---|
| `Content-Type` | `application/json` |
| `Authorization` | `Bearer {{access_token}}` |
| `X-TIMESTAMP` | *(auto-set by pre-request script)* |
| `X-SIGNATURE` | *(auto-set by pre-request script)* |
| `X-PARTNER-ID` | `{{partner_id}}` |
| `X-EXTERNAL-ID` | *(auto-set by pre-request script)* |
| `CHANNEL-ID` | `{{channel_id}}` |

### Body (raw JSON)

```json
{
  "partnerServiceId": "   12345",
  "customerNo": "66666666",
  "virtualAccountNo": "1597366666666",
  "trxId": "cmscyczsl00061wryys7zli68",
  "paymentRequestId": "{{payment_request_id}}",
  "paidAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "totalAmount": {
    "value": "150000.00",
    "currency": "IDR"
  },
  "trxDateTime": "{{timestamp}}",
  "referenceNo": "{{reference_no}}"
}
```

### Pre-request Script

```javascript
// ---- SNAP Symmetric Signature (HMAC-SHA512) ----
const timestamp = new Date().toISOString().replace(/\.\d{3}Z$/, '+07:00');
const externalId = Date.now().toString() + Math.floor(Math.random() * 9000 + 1000);
const paymentRequestId = 'PAY-' + Date.now() + Math.floor(Math.random() * 9000 + 1000);
const referenceNo = 'R' + Date.now().toString().slice(-9);

pm.environment.set('timestamp', timestamp);
pm.environment.set('payment_request_id', paymentRequestId);
pm.environment.set('reference_no', referenceNo);

// Build the exact body that will be sent (minified, same key order)
const body = JSON.stringify({
    partnerServiceId: "   12345",
    customerNo: "66666666",
    virtualAccountNo: "1597366666666",
    trxId: "cmscyczsl00061wryys7zli68",
    paymentRequestId: paymentRequestId,
    paidAmount: { value: "150000.00", currency: "IDR" },
    totalAmount: { value: "150000.00", currency: "IDR" },
    trxDateTime: timestamp,
    referenceNo: referenceNo
});

// SHA-256 hash of the body (standard base64)
const bodyHash = CryptoJS.SHA256(body).toString(CryptoJS.enc.Base64);

// stringToSign = METHOD:ENDPOINT:ACCESS_TOKEN:SHA256(BODY):TIMESTAMP
const accessToken = pm.environment.get('access_token');
const stringToSign = 'POST:/openapi/v1.0/transfer-va/payment:' + accessToken + ':' + bodyHash + ':' + timestamp;

// HMAC-SHA512 signature (standard base64)
const clientSecret = pm.environment.get('client_secret');
const signature = CryptoJS.HmacSHA512(stringToSign, clientSecret).toString(CryptoJS.enc.Base64);

pm.request.headers.upsert({ key: 'X-TIMESTAMP', value: timestamp });
pm.request.headers.upsert({ key: 'X-SIGNATURE', value: signature });
pm.request.headers.upsert({ key: 'X-EXTERNAL-ID', value: externalId });

// Replace the body with the exact signed version
pm.request.body.raw = body;

console.log('paymentRequestId:', paymentRequestId);
console.log('bodyHash:', bodyHash);
console.log('stringToSign:', stringToSign);
```

### Expected Response

```json
{
  "responseCode": "2002500",
  "responseMessage": "Successful",
  "virtualAccountData": {
    "partnerServiceId": "   12345",
    "customerNo": "66666666",
    "virtualAccountNo": "1597366666666",
    "virtualAccountName": "test lagi",
    "trxId": "cmscyczsl00061wryys7zli68",
    "paymentRequestId": "PAY-...",
    "paidAmount": { "value": "150000.00", "currency": "IDR" },
    "totalAmount": { "value": "150000.00", "currency": "IDR" },
    "paymentFlagStatus": "00",
    "paymentFlagReason": { "english": "Success", "indonesia": "Sukses" }
  }
}
```

---

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `4010000` Unauthorized [Missing required header] | A required header is missing | Check all 6 headers are present |
| `4010000` Unauthorized [Invalid signature] | Body was reformatted after signing | The pre-request script must set `pm.request.body.raw` with the **exact** minified JSON it signed. Do NOT edit the body tab after the script runs |
| `4010000` Unauthorized [Timestamp skew exceeds 5 minutes] | Token or timestamp expired | Re-run the token request, then retry |
| `4010000` Unauthorized [Invalid or expired access token] | Token expired (15 min TTL) | Re-run Step 2 to get a fresh token |
| `4002402` Invalid Mandatory Field | Missing required body fields | Ensure all required fields are in the body |
| `4092500` Conflict: already paid | VA already has a successful payment | Create a new VA or use a different `virtualAccountNo` |
| `4042419` / `4042519` Expired transaction | VA has expired | Create a new VA with a future `expiredDate` |

> [!WARNING]
> **Critical**: The `X-SIGNATURE` is an HMAC-SHA512 over the **exact bytes** of the request body. If Postman reformats, re-orders keys, or adds whitespace to the JSON body, the signature becomes invalid. The pre-request script handles this by overwriting `pm.request.body.raw` with the minified JSON it just signed.

> [!TIP]
> **Execution order**: Always run **Token → Inquiry → Payment** in sequence. The token auto-saves to the environment variable, so inquiry/payment pick it up automatically.
