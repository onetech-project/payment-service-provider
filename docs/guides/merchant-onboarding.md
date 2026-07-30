# Merchant Onboarding Guide

This guide is for **merchants** that manage virtual accounts via
`create-va`, `list`, and `delete-va` — creating, listing, and cancelling VAs
on this PSP.

If you are a **vendor/bank** notifying this PSP about inquiries or payments
against a VA (not managing VAs directly), see
[vendor-onboarding.md](./vendor-onboarding.md) instead — the two roles use
completely different, independent credentials.

## What you need to provide us

An **RSA public key** (2048-bit or stronger), generated from a keypair you
control. You keep the private key; you send us only the public key.

```bash
openssl genpkey -algorithm RSA -out your-private-key.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -in your-private-key.pem -pubout -out your-public-key.pem
# Send your-public-key.pem to us. Keep your-private-key.pem secret — never
# transmit it anywhere, including to us.
```

## What we need to give you

1. A **`clientId`** (a UUID or similar identifier we assign, or one you
   propose — coordinate with operations).
2. A **shared secret** for HMAC signing, generated and provisioned on our
   side. **This value must be shared with you over a secure, out-of-band
   channel** (encrypted messaging, a secrets vault, etc.) — never over plain
   email or chat.

Both are provisioned together by our operations team running:

```bash
./scripts/onboard-merchant.sh -n <your-merchant-name> -K <admin-api-key>
```

(Or, if you already generated your own keypair per the previous section,
operations can register it directly via the admin API — see
[Manual registration](#manual-registration-without-onboard-mergchantsh) below.)

## How authentication works

Every request to `create-va`, `list`, or `delete-va` must pass **two
independent checks**, both required simultaneously:

1. **A valid, unexpired `accessToken`**, sent as `Authorization: Bearer
   <accessToken>` — obtained from `POST /openapi/v1.0/access-token/b2b`,
   asymmetrically signed with your RSA private key (proves who you are).
2. **A valid `X-SIGNATURE`**, computed with your shared secret (proves the
   request body hasn't been tampered with, and that you actually hold the
   secret — not just an intercepted/replayed token).

Passing only one is not sufficient — a leaked `accessToken` alone cannot be
used to act as you without also knowing your shared secret, and vice versa.

### Step 1: Obtain an accessToken

```
POST /openapi/v1.0/access-token/b2b
X-CLIENT-KEY: <your clientId>
X-TIMESTAMP: <ISO 8601 timestamp>
X-SIGNATURE: <base64 RSA-SHA256 signature>
Content-Type: application/json

{"grantType":"client_credentials","additionalInfo":{}}
```

Where `X-SIGNATURE = base64(RSA-SHA256-sign(yourPrivateKey, "{clientId}|{timestamp}"))`.

The response includes `accessToken`, valid for **15 minutes (900 seconds)**
— you must refresh it periodically; don't cache it longer than that.

See [`scripts/curl-b2b-token.sh`](../../scripts/curl-b2b-token.sh) for a
complete reference implementation.

### Step 2: Sign your create-va/list/delete-va request

For every request to `create-va`, `list`, or `delete-va`:

1. **Build the request body** as a JSON object per the request's schema.
2. **Compute the timestamp**: current time in ISO 8601.
3. **Hash the body**: `bodyHash = base64(SHA256(body))`.
4. **Build `stringToSign`**:
   ```
   stringToSign = "{METHOD}:{PATH}:{accessToken}:{bodyHash}:{timestamp}"
   ```
   **Important**: unlike the vendor side, the `accessToken` slot here is the
   **real token from Step 1** (not empty) — because you genuinely send it
   via the `Authorization` header on this endpoint, both sides of the
   signature computation need to agree on that same value.
5. **Sign**: `signature = base64(HMAC-SHA512(yourSharedSecret, stringToSign))`.
6. **Send these headers**:

   | Header | Value |
   |---|---|
   | `Authorization` | `Bearer <accessToken>` from Step 1 |
   | `X-TIMESTAMP` | The same timestamp used in step 2 |
   | `X-SIGNATURE` | The base64 signature from step 5 |
   | `X-EXTERNAL-ID` | A unique ID per request (idempotency) |

### Timestamp freshness

`X-TIMESTAMP` must be within **±5 minutes** of our server's clock — same
tolerance as the vendor side. Requests outside that window are rejected with
`401 Unauthorized`.

### What gets rejected

| Condition | Response |
|---|---|
| No `Authorization` header at all | `401 Unauthorized. [Missing or invalid Authorization header]` |
| Token invalid, malformed, or expired | `401 Unauthorized. [Invalid or expired access token]` |
| Your `clientId` has no shared secret provisioned | `401 Unauthorized. [No signing secret provisioned for this client]` |
| `X-TIMESTAMP` outside ±5 minute window | `401 Unauthorized. [Timestamp skew exceeds 5 minutes]` |
| `X-SIGNATURE` missing or doesn't match | `401 Unauthorized. [Invalid signature]` |

There is **no opt-out** — both checks are unconditional from deployment.
Make sure you're provisioned (Step 0 above) **before** attempting any
create-va/list/delete-va call, or every request will fail closed regardless
of how correctly it's signed.

## Reference implementation

See [`scripts/merchant-create-va.sh`](../../scripts/merchant-create-va.sh),
[`scripts/merchant-list-va.sh`](../../scripts/merchant-list-va.sh), and
[`scripts/merchant-delete-va.sh`](../../scripts/merchant-delete-va.sh) for a
complete, runnable bash implementation — including automatic accessToken
refresh when using their `-f <.env.merchant.NAME>` flag.

## Local testing quickstart

```bash
# 1. Onboard yourself as a test merchant against a local instance:
./scripts/onboard-merchant.sh -n mytest -K changeme -u http://localhost:8080
# Writes .env.merchant.mytest with your clientId, private key path, and secret.

# 2. Create a VA — token fetch + signing happen automatically:
./scripts/merchant-create-va.sh -s 088899 -n "Test VA" -c 0001234567 \
  -a 50000.00 -f .env.merchant.mytest -u http://localhost:8080

# 3. List your VAs:
./scripts/merchant-list-va.sh -s 088899 -f .env.merchant.mytest -u http://localhost:8080
```

## Manual registration (without onboard-merchant.sh)

If you already generated your own RSA keypair and want operations to
register it directly:

```bash
# 1. Register the client + public key:
curl -X POST https://<host>/admin/clients \
  -H "X-Admin-API-Key: <admin-api-key>" -H "Content-Type: application/json" \
  -d '{"clientId":"<your-client-id>","clientName":"<your-name>","keyId":"key-01","publicKeyPem":"<contents of your-public-key.pem>"}'

# 2. Provision a shared secret:
curl -X POST https://<host>/admin/clients/<your-client-id>/secret \
  -H "X-Admin-API-Key: <admin-api-key>" -H "Content-Type: application/json" \
  -d '{"secretId":"secret-01","secretValue":"<a strong random secret>"}'
```

The `secretValue` you choose in step 2 must then be shared with you securely
(or you generate it and send it to operations securely — either direction
works, as long as it never travels in plaintext over an insecure channel).

## Rotating your credentials

- **RSA key rotation**: generate a new keypair, ask operations to register
  it via `POST /admin/clients/{clientId}/keys` with a new `keyId` alongside
  your existing one, switch your signing to the new private key, then have
  operations revoke the old `keyId` via `DELETE
  /admin/clients/{clientId}/keys/{keyId}` once you've confirmed the new one
  works.
- **Shared secret rotation**: same pattern — `POST
  /admin/clients/{clientId}/secret` with a new `secretId`, switch your
  signing to the new secret, then `DELETE
  /admin/clients/{clientId}/secret/{secretId}` on the old one.

Both support a zero-downtime overlap window (old and new credential both
active simultaneously) since each is keyed by its own `keyId`/`secretId`,
unlike the vendor side's single-secret-per-file model.
