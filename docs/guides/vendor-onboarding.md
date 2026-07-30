# Vendor Onboarding Guide

This guide is for **vendors/banks** that call this PSP's `transfer-va` endpoints
(`inquiry`, `payment`, `status`) as part of the SNAP virtual-account flow —
e.g. a switching vendor notifying this PSP that a customer inquired or paid a
virtual account.

If you are a **merchant** creating/managing virtual accounts (not a bank
calling in to notify about inquiries/payments), see
[merchant-onboarding.md](./merchant-onboarding.md) instead — the two roles
use completely different, independent credentials.

## Two onboarding paths

- **Legacy (shared-secret only)**: the original, simplest path — a symmetric
  HMAC signature only, no bearer token. Still fully supported for vendors who
  haven't migrated.
- **Migrated (shared-secret + accessToken)**: newer vendors (or existing ones
  moving over) additionally send a bearer `accessToken`, bound into the
  signature (feature 011-vendor-access-token-signature). This is the
  recommended path for any new vendor integration — it binds proof-of-identity
  (the token) to proof-of-integrity (the signature) the same way the merchant
  side already works.

Which path applies to you is controlled entirely by whether your
`.env.<vendor>.<channel>` config has `VENDOR_CLIENT_ID` set — see
[What we need to give you](#what-we-need-to-give-you) below.

## What you need to provide us

- **Legacy path**: nothing. We generate your shared secret (or you propose
  one) — no keypair, no certificate, no registration request.
- **Migrated path**: additionally, an **RSA public key** (2048-bit or
  stronger), the same way merchants do (see
  [merchant-onboarding.md](./merchant-onboarding.md#what-you-need-to-provide-us)):

  ```bash
  openssl genpkey -algorithm RSA -out your-private-key.pem -pkeyopt rsa_keygen_bits:2048
  openssl rsa -in your-private-key.pem -pubout -out your-public-key.pem
  # Send your-public-key.pem to us. Keep your-private-key.pem secret — never
  # transmit it anywhere, including to us.
  ```

## What we need to give you

1. A **shared secret** (a random string, e.g. 32 bytes hex-encoded) —
   required on both paths. This is the only credential needed to sign your
   requests to us. It will be provisioned into our system via:

   ```bash
   ./scripts/onboard-vendor.sh -n <your-vendor-name> -c <channel> -o <output-dir>
   ```

   This writes a `.env.<vendor>.<channel>` file (e.g. `.env.bca.va`) containing
   your `VENDOR_CLIENT_ID` and `VENDOR_CLIENT_SECRET`, which our operations
   team deploys into the running service's `CONFIG_DIR`. **The secret value
   itself must be shared with you over a secure, out-of-band channel**
   (encrypted messaging, a secrets vault, etc.) — never over plain email or
   chat.

2. **Migrated path only**: your `VENDOR_CLIENT_ID` must also be registered as
   a `client_apps` row with your public key attached, so you can obtain an
   `accessToken`:

   ```bash
   # Register the client + public key (uses the same VENDOR_CLIENT_ID already
   # in your .env.<vendor>.<channel> file):
   curl -X POST https://<host>/admin/clients \
     -H "X-Admin-API-Key: <admin-api-key>" -H "Content-Type: application/json" \
     -d '{"clientId":"<your VENDOR_CLIENT_ID>","clientName":"<your-name>","keyId":"key-01","publicKeyPem":"<contents of your-public-key.pem>"}'
   ```

   Leaving `VENDOR_CLIENT_ID` unset in your config keeps you on the legacy
   path indefinitely — there's no forced cutover date.

## How authentication works

### Legacy path (no `VENDOR_CLIENT_ID` configured)

Every request you send to `inquiry`, `payment`, or `status` is verified with
a **symmetric HMAC-SHA512 signature only** — there is no bearer token, no
RSA keypair, and no `X-CLIENT-KEY` header involved on these endpoints. This
is deliberately simpler than a full OAuth-style flow: since these are
trusted, pre-provisioned server-to-server calls (not calls with a public
attack surface where you'd want short-lived tokens), a shared secret known
only to you and us is sufficient.

### Migrated path (`VENDOR_CLIENT_ID` configured)

You additionally obtain and send a bearer `accessToken`, which is bound into
the signature — the same two-factor model merchants use (see
[merchant-onboarding.md](./merchant-onboarding.md#how-authentication-works)).
A leaked `accessToken` alone cannot be used to act as you without also
knowing your shared secret, and vice versa.

**Step 1: Obtain an accessToken** — identical to the merchant flow:

```
POST /openapi/v1.0/access-token/b2b
X-CLIENT-KEY: <your VENDOR_CLIENT_ID>
X-TIMESTAMP: <ISO 8601 timestamp>
X-SIGNATURE: <base64 RSA-SHA256 signature>
Content-Type: application/json

{"grantType":"client_credentials","additionalInfo":{}}
```

Where `X-SIGNATURE = base64(RSA-SHA256-sign(yourPrivateKey, "{clientId}|{timestamp}"))`.
The response includes `accessToken`, valid for **15 minutes (900 seconds)** —
refresh it periodically. See
[`scripts/curl-b2b-token.sh`](../../scripts/curl-b2b-token.sh) for a complete
reference implementation.

### Request signing steps

For every request to `inquiry`, `payment`, or `status`:

1. **Build the request body** as a JSON object per the SNAP transfer-va spec.
2. **Compute the timestamp**: current time in ISO 8601, e.g.
   `2026-07-30T10:15:00+07:00`.
3. **Hash the body**: `bodyHash = base64(SHA256(body))`.
4. **Build `stringToSign`**:
   ```
   stringToSign = "{METHOD}:{PATH}:{accessToken}:{bodyHash}:{timestamp}"
   ```
   - **Legacy path**: the `{accessToken}` slot is always the **empty
     string** — no header on these endpoints carries one, so both colons
     sit next to each other:
     ```
     stringToSign = "{METHOD}:{PATH}::{bodyHash}:{timestamp}"
     ```
     Example for `POST /openapi/v1.0/transfer-va/inquiry`:
     ```
     POST:/openapi/v1.0/transfer-va/inquiry::OjVWOu4+711dTdc7...:2026-07-30T10:15:00+07:00
     ```
   - **Migrated path**: the `{accessToken}` slot is the **real token from
     Step 1** (not empty) — you genuinely send it via `Authorization`, so
     both sides of the signature computation must agree on that same value.
     Example:
     ```
     POST:/openapi/v1.0/transfer-va/inquiry:eyJhbGciOi...:OjVWOu4+711dTdc7...:2026-07-30T10:15:00+07:00
     ```
5. **Sign**: `signature = base64(HMAC-SHA512(yourSharedSecret, stringToSign))`.
6. **Send these headers**:

   | Header | Value | Required on |
   |---|---|---|
   | `Authorization` | `Bearer <accessToken>` from Step 1 | Migrated path only |
   | `X-TIMESTAMP` | The same timestamp used in step 2 | Both |
   | `X-SIGNATURE` | The base64 signature from step 5 | Both |
   | `X-PARTNER-ID` | Your assigned partner ID | Both |
   | `X-EXTERNAL-ID` | A unique ID per request (idempotency) | Both |
   | `CHANNEL-ID` | Your assigned channel ID (if your config requires it) | Both |

   **Do not send `X-CLIENT-KEY`** on `inquiry`/`payment`/`status` — per the
   SNAP spec, that header is only used on the `/access-token/b2b` endpoint,
   and sending it here has no effect.

### Timestamp freshness

`X-TIMESTAMP` must be within **±5 minutes** of our server's clock. Requests
outside that window are rejected with `401 Unauthorized` regardless of
whether the signature is otherwise correct — make sure your system clock is
NTP-synchronized.

### What gets rejected

| Condition | Response | Applies to |
|---|---|---|
| `X-SIGNATURE` missing or doesn't match | `401 Unauthorized. [Invalid signature]` | Both |
| `X-TIMESTAMP` outside ±5 minute window | `401 Unauthorized. [Timestamp skew exceeds 5 minutes]` | Both |
| A required header is missing entirely | `401 Unauthorized. [Missing required header: ...]` | Both |
| No `Authorization` header at all | `401 Unauthorized. [Missing or invalid Authorization header]` | Migrated path only |
| Token invalid, malformed, or expired | `401 Unauthorized. [Invalid or expired access token]` | Migrated path only |
| Token was issued for a different `clientId` than yours, or was swapped after signing | `401 Unauthorized. [Invalid signature]` (same generic message — not distinguished from a plain signature mismatch) | Migrated path only |

There is **no opt-out or grace period** once you're on a given path —
enforcement is unconditional from the moment your `.env.<vendor>.<channel>`
file is deployed with `VENDOR_CLIENT_SECRET` set (and, if migrated,
`VENDOR_CLIENT_ID` set). Test your signing implementation in staging before
going live.

## Reference implementation

See [`scripts/vendor-inquiry-va.sh`](../../scripts/vendor-inquiry-va.sh) and
[`scripts/vendor-payment-va.sh`](../../scripts/vendor-payment-va.sh) for a
complete, runnable bash implementation of the signing steps above — useful
both as a reference and for testing against a local instance. When your
`.env.<vendor>.<channel>` file has `VENDOR_CLIENT_ID` and
`VENDOR_PRIVATE_KEY_PATH` set, these scripts auto-fetch an `accessToken` via
[`scripts/curl-b2b-token.sh`](../../scripts/curl-b2b-token.sh) and bind it in
automatically — no need to pass one yourself. Omit both to exercise the
legacy path unchanged.

## Rotating your shared secret

Contact operations to have a new secret generated and deployed. Since the
current implementation stores a single `VENDOR_CLIENT_SECRET` per
`.env.<vendor>.<channel>` file (not a rotate-with-overlap scheme like the
merchant side's `client_secrets` table), a secret rotation is a coordinated
cutover: agree on a switchover time, update the file, and restart the
service — there is no dual-active-secret window today.

## Rotating your RSA key (migrated path only)

Same overlap-capable pattern as the merchant side: generate a new keypair,
ask operations to register it via `POST /admin/clients/{clientId}/keys` with
a new `keyId` alongside your existing one, switch your token-request signing
to the new private key, then have operations revoke the old `keyId` via
`DELETE /admin/clients/{clientId}/keys/{keyId}` once you've confirmed the new
one works.
