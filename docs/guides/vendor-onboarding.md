# Vendor Onboarding Guide

This guide is for **vendors/banks** that call this PSP's `transfer-va` endpoints
(`inquiry`, `payment`, `status`) as part of the SNAP virtual-account flow —
e.g. a switching vendor notifying this PSP that a customer inquired or paid a
virtual account.

If you are a **merchant** creating/managing virtual accounts (not a bank
calling in to notify about inquiries/payments), see
[merchant-onboarding.md](./merchant-onboarding.md) instead — the two roles
use completely different, independent credentials.

## What you need to provide us

Nothing. You generate your own shared secret (or we generate one for you and
hand it over securely) — there is no keypair, certificate, or registration
request you need to send us. See [How authentication works](#how-authentication-works)
below for why.

## What we need to give you

One value: a **shared secret** (a random string, e.g. 32 bytes hex-encoded).
This is the only credential needed to sign your requests to us. It will be
provisioned into our system via:

```bash
./scripts/onboard-vendor.sh -n <your-vendor-name> -c <channel> -o <output-dir>
```

This writes a `.env.<vendor>.<channel>` file (e.g. `.env.bca.va`) containing
your `VENDOR_CLIENT_SECRET`, which our operations team deploys into the
running service's `CONFIG_DIR`. **The secret value itself must be shared with
you over a secure, out-of-band channel** (encrypted messaging, a secrets
vault, etc.) — never over plain email or chat.

## How authentication works

Every request you send to `inquiry`, `payment`, or `status` is verified with
a **symmetric HMAC-SHA512 signature only** — there is no bearer token, no
RSA keypair, and no `X-CLIENT-KEY` header involved on these endpoints. This
is deliberately simpler than a full OAuth-style flow: since these are
trusted, pre-provisioned server-to-server calls (not calls with a public
attack surface where you'd want short-lived tokens), a shared secret known
only to you and us is sufficient.

### Request signing steps

For every request to `inquiry`, `payment`, or `status`:

1. **Build the request body** as a JSON object per the SNAP transfer-va spec.
2. **Compute the timestamp**: current time in ISO 8601, e.g.
   `2026-07-30T10:15:00+07:00`.
3. **Hash the body**: `bodyHash = base64(SHA256(body))`.
4. **Build `stringToSign`**:
   ```
   stringToSign = "{METHOD}:{PATH}::{bodyHash}:{timestamp}"
   ```
   Note the **empty section between the two colons** — that's intentional.
   The full SNAP `stringToSign` format has a slot for an `AccessToken`
   component, but on these endpoints no `accessToken` is ever transmitted
   (there's no `Authorization` header on this side), so that slot is always
   an empty string. This is different from the merchant side (see
   [merchant-onboarding.md](./merchant-onboarding.md)), where the equivalent
   slot IS the real bearer token — don't copy that convention here.

   Example for `POST /openapi/v1.0/transfer-va/inquiry`:
   ```
   POST:/openapi/v1.0/transfer-va/inquiry::OjVWOu4+711dTdc7...:2026-07-30T10:15:00+07:00
   ```
5. **Sign**: `signature = base64(HMAC-SHA512(yourSharedSecret, stringToSign))`.
6. **Send these headers**:

   | Header | Value |
   |---|---|
   | `X-TIMESTAMP` | The same timestamp used in step 2 |
   | `X-SIGNATURE` | The base64 signature from step 5 |
   | `X-PARTNER-ID` | Your assigned partner ID |
   | `X-EXTERNAL-ID` | A unique ID per request (idempotency) |
   | `CHANNEL-ID` | Your assigned channel ID (if your config requires it) |

   **Do not send `X-CLIENT-KEY`** on these endpoints — per the SNAP spec,
   that header is only used on the `/access-token/b2b` endpoint (which you
   as a vendor calling INTO us never need to call), and sending it here has
   no effect.

### Timestamp freshness

`X-TIMESTAMP` must be within **±5 minutes** of our server's clock. Requests
outside that window are rejected with `401 Unauthorized` regardless of
whether the signature is otherwise correct — make sure your system clock is
NTP-synchronized.

### What gets rejected

| Condition | Response |
|---|---|
| `X-SIGNATURE` missing or doesn't match | `401 Unauthorized. [Invalid signature]` |
| `X-TIMESTAMP` outside ±5 minute window | `401 Unauthorized. [Timestamp skew exceeds 5 minutes]` |
| A required header is missing entirely | `401 Unauthorized. [Missing required header: ...]` |

There is **no opt-out or grace period** — enforcement is unconditional from
the moment your `.env.<vendor>.<channel>` file is deployed with a
`VENDOR_CLIENT_SECRET` set. Test your signing implementation in staging
before going live.

## Reference implementation

See [`scripts/vendor-inquiry-va.sh`](../../scripts/vendor-inquiry-va.sh) and
[`scripts/vendor-payment-va.sh`](../../scripts/vendor-payment-va.sh) for a
complete, runnable bash implementation of the signing steps above — useful
both as a reference and for testing against a local instance.

## Rotating your shared secret

Contact operations to have a new secret generated and deployed. Since the
current implementation stores a single `VENDOR_CLIENT_SECRET` per
`.env.<vendor>.<channel>` file (not a rotate-with-overlap scheme like the
merchant side's `client_secrets` table), a secret rotation is a coordinated
cutover: agree on a switchover time, update the file, and restart the
service — there is no dual-active-secret window today.
