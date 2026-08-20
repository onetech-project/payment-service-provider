# Vendor Onboarding Guide

This guide is for **vendors/banks** that call this PSP's `transfer-va` endpoints
(`inquiry`, `payment`, `status`) as part of the SNAP virtual-account flow —
e.g. a switching vendor notifying this PSP that a customer inquired or paid a
virtual account.

If you are a **merchant** creating/managing virtual accounts (not a bank
calling in to notify about inquiries/payments), see
[merchant-onboarding.md](./merchant-onboarding.md) instead — the two roles
use completely different, independent credentials.

## Flow at a glance

The full lifecycle of one VA, showing where your calls (vendor) sit relative
to the merchant's. Steps 1–2 are the merchant's (see
[merchant-onboarding.md](./merchant-onboarding.md)); steps 3–8 are yours.

```mermaid
sequenceDiagram
    autonumber
    participant M as Merchant
    participant P as PSP (this service)
    participant V as Vendor / Bank (you)
    participant C as Customer

    Note over M,P: Merchant side — see merchant-onboarding.md
    M->>P: POST /access-token/b2b (RSA-signed)
    P-->>M: accessToken (900s)
    M->>P: POST /transfer-va/create-va
    P-->>M: 2002700 + virtualAccountData

    Note over V,P: Vendor side — migrated path
    V->>P: POST /access-token/b2b (RSA-signed, X-CLIENT-KEY)
    P-->>V: accessToken (900s)

    C->>V: Enters VA number at your channel
    V->>P: POST /transfer-va/inquiry
    P-->>V: 2002400 + totalAmount, billDetails
    V-->>C: Shows bill

    C->>V: Confirms payment
    V->>P: POST /transfer-va/payment
    P-->>V: 2002500 + paymentFlagStatus "00"

    P-)M: async callback to notificationUrl (Asynq worker)

    opt Reconciliation
        V->>P: POST /transfer-va/status
        P-->>V: 2002600 + paymentFlagStatus
    end
```

On the **legacy path**, drop the two `/access-token/b2b` messages on the
vendor side — everything else is identical.

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
3. **Hash the body**: `bodyHash = hex(SHA256(minify(body)))`.

   **Minify first.** SNAP specifies
   `Lowercase(HexEncode(SHA-256(MinifyJson(RequestBody))))` — `minify` is the
   JSON with all insignificant whitespace removed. If you send a compact body
   this changes nothing; if you pretty-print, hashing the raw bytes gives a
   different digest than we compute and every request comes back
   `[Invalid signature]`. In `jq` terms: `jq -cj .` — the `-j` matters,
   because `jq -c` alone appends a newline that would be hashed too.

   **Encoding is lowercase hex**, per the SNAP spec. Vendors onboarded under
   feature `012-base64-hash-encoding` sign with base64 instead; that is a
   per-vendor setting (`VENDOR_BODY_HASH_ENCODING`) so one vendor's
   non-conformant encoding cannot push every other vendor off spec. Ask
   operations which one is provisioned for your channel if you are unsure —
   the merchant endpoints use base64 for the same historical reason.
4. **Build `stringToSign`**:
   ```
   stringToSign = "{METHOD}:{PATH}:{accessToken}:{bodyHash}:{timestamp}"
   ```
   `{PATH}` must be the **exact path of the URL you actually request**,
   including any prefix. We verify against the path as received, so if you
   call `https://host/openapi/v1.0/transfer-va/inquiry` you sign
   `/openapi/v1.0/transfer-va/inquiry` — and if a gateway in front of us
   rewrites the path, sign the rewritten one. A mismatch here surfaces as
   `[Invalid signature]`, which is easy to misdiagnose as a wrong secret.
   - **Legacy path**: the `{accessToken}` slot is always the **empty
     string** — no header on these endpoints carries one, so both colons
     sit next to each other:
     ```
     stringToSign = "{METHOD}:{PATH}::{bodyHash}:{timestamp}"
     ```
     Example for `POST /openapi/v1.0/transfer-va/inquiry`:
     ```
     POST:/openapi/v1.0/transfer-va/inquiry::ede1b7f180fdcb80bc6d71a3...:2026-07-30T10:15:00+07:00
     ```
   - **Migrated path**: the `{accessToken}` slot is the **real token from
     Step 1** (not empty) — you genuinely send it via `Authorization`, so
     both sides of the signature computation must agree on that same value.
     Example:
     ```
     POST:/openapi/v1.0/transfer-va/inquiry:eyJhbGciOi...:ede1b7f180fdcb80bc6d71a3...:2026-07-30T10:15:00+07:00
     ```
5. **Sign**: `signature = base64(HMAC-SHA512(yourSharedSecret, stringToSign))`.
6. **Send these headers**:

   | Header | Value | Required on |
   |---|---|---|
   | `Content-Type` | `application/json` | Both |
   | `Authorization` | `Bearer <accessToken>` from Step 1 | Migrated path only |
   | `X-TIMESTAMP` | The same timestamp used in step 2 | Both |
   | `X-SIGNATURE` | The base64 signature from step 5 | Both |
   | `X-PARTNER-ID` | Your assigned Company Code VA (`String(8)` Max) | Both |
   | `X-EXTERNAL-ID` | Numeric string, `String(36)` Max, unique per calendar day (idempotency key) | Both |
   | `CHANNEL-ID` | Your assigned channel id — `95231` for BCA VA (`String(5)` Fixed) | Both |
   | `X-CLIENT-KEY` | Your `VENDOR_CLIENT_ID` | Only if your config demands it — see below |

   Per the ASPI *Standar Teknis dan Keamanan*, `CHANNEL-ID` is **Mandatory**
   on transaction requests (not optional), and `X-EXTERNAL-ID` must be a
   numeric string unique within the same calendar day.

#### How this compares to the merchant side

Both sides use the same `stringToSign` shape and the same minify-then-hash
rule for the body. Only three things differ, and none of them is a difference
in *algorithm*:

| | Vendor (`inquiry`/`payment`/`status`) | Merchant (`create-va`/`list`/`delete-va`) |
|---|---|---|
| Body digest | `SHA-256(minify(body))` | `SHA-256(minify(body))` — same rule |
| Digest encoding | lowercase **hex** (or base64 per `VENDOR_BODY_HASH_ENCODING`) | **base64** |
| `{accessToken}` slot | empty on the legacy path, the real token once migrated | always the real token |
| `CHANNEL-ID` / `X-PARTNER-ID` | required | not enforced |

The body rule was **not** always the same: the merchant endpoints used to hash
the raw, un-minified body. If you operate on both sides and wrote your
merchant signing against that older behaviour, see
[merchant-onboarding.md](./merchant-onboarding.md#step-2-sign-your-create-valistdelete-va-request)
— there is a transitional fallback, but it will be turned off.

#### About `X-CLIENT-KEY`

Per the ASPI spec, `X-CLIENT-KEY` belongs to `/access-token/b2b` **only** and
has no meaning on `inquiry`/`payment`/`status`. However, the header set we
enforce on those endpoints is driven by `VENDOR_REQUIRED_HEADERS` in your
`.env.<vendor>.<channel>` config, and a deployment is free to list
`X-CLIENT-KEY` there. When it is listed, omitting the header fails the
request before any signature check:

```json
{"responseCode":"4010000","responseMessage":"Unauthorized. [Missing required header: X-CLIENT-KEY]"}
```

Ask operations for the exact `VENDOR_REQUIRED_HEADERS` value provisioned for
your channel. If in doubt, **sending `X-CLIENT-KEY` is always safe**: it is
never part of `stringToSign`, so it is simply ignored where it isn't
required. Our reference scripts send it automatically, defaulting to
`VENDOR_CLIENT_ID` (override with `-k`).

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
| `X-EXTERNAL-ID` reused with a *different* body | `409 Conflict` — with the calling service's own code (`4092400`/`4092500`/`4092600`) | Both |
| `X-EXTERNAL-ID` reused while the first request is still in flight | `409 Conflict` — same per-service code | Both |

Reusing an `X-EXTERNAL-ID` with the *same* body is safe — you get the
original response back, tagged with an `X-Cache-Replay: true` header. Note
the idempotency key is the `X-EXTERNAL-ID` value alone, not scoped per
endpoint, so the same value must not be reused across different endpoints
either.

> Both conflict cases used to answer `422` (`4227300`) and `409` (`4097300`).
> `422` is not a SNAP status at all, and neither `4227300` nor `4097300` is a
> code BCA publishes for the transfer-va services — every rejection now carries
> the service code of the endpoint you called, which is what BCA matches on.
>
> The `/payment` endpoint is a deliberate exception to the *replay* half: a
> resubmitted `paymentRequestId` reaches the handler instead of being replayed
> from cache, so it can be answered `4042518` (or `2002500` for an advice
> retry) rather than hidden behind a second apparent success. See
> [Retries](#retries-flagadvise).

## Request payloads

Field names and obligations below come from the ASPI portal's
[Virtual Account](https://apidevportal.aspi-indonesia.or.id/api-services/transfer-kredit/virtual-account)
tables. `additionalInfo` is an open extension slot and is not covered here.

### `POST /transfer-va/inquiry` (service code 24)

Field table per *Tech. Doc. OpenAPI VA-BillPresentment API v2.4*.

| Field | Obligation | Notes |
|---|---|---|
| `partnerServiceId` | M | `String(8)`, **left-padded with spaces** |
| `customerNo` | M | `String(18)` — we accept up to 20 inbound, see below |
| `virtualAccountNo` | M | `partnerServiceId` + `customerNo`, `String(26)` — we accept up to 28 inbound |
| `inquiryRequestId` | M | `String(30)`, unique per inquiry, generated by you |
| `trxDateInit` | M in v2.4 | ISO-8601 with timezone, `Date(25)`. **We do not require it** |
| `channelCode` | M in v2.4 | ISO-18245, `Numeric(4)`. **We do not require it**, but a value you do send must be one of `6000`,`6010`–`6020` |
| `amount` | O | `{value, currency}`; `value` is `String(13.2)` |
| `language`, `hashedSourceAccountNo`, `sourceBankCode`, `passApp` | O | |

> **We are deliberately more permissive than the spec on receive.**
> `trxDateInit` and `channelCode` became Mandatory in v2.4, but both are
> BCA-generated and always present on a real BCA inquiry; requiring them would
> only reject other vendors that omit what BCA happens to send. Likewise
> `customerNo`/`virtualAccountNo` are accepted at the older `20`/`28` widths so
> VA numbers this system issued under its previous sequence stay payable.
> Being looser than the spec on *receive* cannot produce a wrong answer — the
> reverse can, so **issuance** is at the narrower `18`/`26`.
>
> `inquiryRequestId` is the exception we *do* enforce at `30`. It is not
> independent: "if payment comes from the Inquiry process, `paymentRequestId`
> must be the same with `inquiryRequestId`", and `paymentRequestId` is capped
> at 30 on the payment service. A longer id accepted here would only fail at
> the payment that follows — after the customer has been shown a bill.

> The ASPI page spells this field `trxDateInit` in its field table but
> `txnDateInit` in its sample request; BCA's v2.4 table says `trxDateInit`. We
> bind **`trxDateInit`**.

### `POST /transfer-va/payment` (service code 25)

Field table per *Tech. Doc. OpenAPI VA-Payment-Flag API v2.3*.

| Field | Obligation | Notes |
|---|---|---|
| `partnerServiceId`, `customerNo`, `virtualAccountNo` | M | as above |
| `virtualAccountName` | M | `String(30)` |
| `paymentRequestId` | M | `String(30)`. **If the payment follows an inquiry, this must equal that inquiry's `inquiryRequestId`.** |
| `channelCode` | M | ISO-18245, one of `6000`, `6010`–`6020` |
| `paidAmount` | M | `{value, currency}`, `String(13.2)` |
| `totalAmount` | M | must equal `paidAmount`, and must match the **stored** bill — see below |
| `trxDateTime` | M | ISO-8601 with timezone |
| `flagAdvise` | M | `N` = new request, `Y` = advice/retry. See [Retries](#retries-flagadvise) |
| `trxId` | C | **Mandatory if the payment follows a create-VA request** — send the `trxId` we returned from `create-va`, not one you generate |
| `referenceNo` | C | `String(11)`, mandatory for non-multi-bill |
| `subCompany` | C | `String(5)`, mandatory for non-multi-bill with single settlement |
| `billDetails` | C | max **5** entries; mandatory for multi-bill / multi-settlement |
| `freeTexts` | O | max 9 entries, each `english`/`indonesia` up to **32** chars |
| `journalNum`, `paymentType`, `paidBills`, `cumulativePaymentAmount`, `hashedSourceAccountNo`, `sourceBankCode` | O | |

The mandatory set marked M above is enforced by default and can be relaxed
per vendor via `VENDOR_STRICT_MANDATORY_FIELDS=false` — the wider SNAP standard
leaves several of them optional, and this gateway fronts more vendors than BCA.
`trxId` is never in that set: BCA itself marks it conditional.

**`totalAmount` is checked against the stored bill, not against your own
request.** Comparing `paidAmount` to the `totalAmount` in the same request
validates nothing — both come from you. A payment of `1.00` against a
`250000.00` bill is rejected `4042513 Invalid Amount` however the request
labels itself.

**Everything wrong with `paidAmount.value` comes back as `4042513`.** This is
a deliberate departure from Appendix A, which would give a malformed field
`4002501`: one code and one reason for the whole class, so you never have to
correlate two outcomes for what is the same problem. It covers

- a value that is not `String(13.2)` with **both** decimals present —
  `"10000"`, `"10000.5"`, `"Rp10000"`, `"10000.000"` are all rejected, only
  `"10000.00"` is accepted (`totalAmount` stays lenient and still takes
  `"10000"`),
- zero or a negative figure,
- a fixed bill (vaType 03/06) whose stored amount the payment does not match,
- a variable bill (vaType 02/05) where what is already settled plus this
  payment would exceed `totalAmount` — instalments are welcome, over-payment
  is not.

All of them answer

```json
{
  "responseCode": "4042513",
  "responseMessage": "Invalid Amount",
  "virtualAccountData": {
    "paymentFlagStatus": "01",
    "paymentFlagReason": { "english": "Invalid Amount", "indonesia": "Jumlah tidak valid" }
  }
}
```

A `paidAmount` that is **missing** is still `4002502`, and a bad
`paidAmount.currency` is still `4002501`: neither is a statement about the
amount itself.

### Retries (`flagAdvise`)

`flagAdvise: "Y"` marks an advice/retry — a deliberate re-send of a payment you
believe may not have been recorded. Resending a `paymentRequestId` we already
hold behaves differently depending on it:

| You send | We answer |
|---|---|
| Same `paymentRequestId`, `flagAdvise: "Y"` | `2002500` replaying the original outcome — you asked "did this land?", and it did |
| Same `paymentRequestId`, `flagAdvise: "N"` | `4042518 Inconsistent Request`, carrying `paymentFlagStatus`/`paymentFlagReason` **of the first request** (so `00` if it settled) |
| Same `X-EXTERNAL-ID`, *different* body | `4092500 Conflict` |

Note `X-EXTERNAL-ID` must be unique per calendar day, so a genuine advice retry
carries a **new** `X-EXTERNAL-ID` with the *same* `paymentRequestId`. Reusing
both while changing `flagAdvise` changes the body, and lands on `4092500`.

**Do not send `inquiryRequestId`** — it is not a field of the ASPI
PaymentRequest. It appears only inside the *description* of
`paymentRequestId`, as the rule quoted above. We still accept it for
backward compatibility with legacy vendors, but new integrations should omit
it.

There is also no `transactionDate` on this endpoint — only `trxDateTime`.

### `POST /transfer-va/status` (service code 26)

Field table per *Tech. Doc. OpenAPI VA-Payment-Status API V2 v1.0*. Note the
path is **`/openapi/v2.0/transfer-va/status`** — BCA versions this service
separately from inquiry and payment. We keep `/openapi/v1.0` registered too,
for vendors already pointed there.

| Field | Obligation | Notes |
|---|---|---|
| `partnerServiceId` | M | `String(8)` |
| `customerNo` | M | `String(18)` |
| `virtualAccountNo` | M | `String(26)` |
| `inquiryRequestId` | M | `String(30)` |
| `additionalInfo` | O | |

`paymentRequestId` is **not** a field of the status request — it appears only
in the response. Sending it is harmless; requiring it would be wrong.

> **Direction differs between BCA and the ASPI-generic model.** ASPI puts
> service 26 on the PJP, so we expose this endpoint and you may call it. BCA
> puts it on its own side: its *Virtual Account untuk Biller* documentation
> targets the status sample at BCA's host while inquiry and payment target the
> co-partner's, and the `paymentFlagStatus` description says *"03 = Pending
> between BCA and the partner. If the payment flag process is not yet completed
> and **the partner performs an inquiry** within that time frame, the
> transaction with status 03 will be delivered to the partner."*
>
> Both work: this endpoint stays available for vendors following the generic
> model, and we call *yours* for reconciliation — see below.

## Payment reconciliation (we call you)

If a `/payment` call never reaches us — the network drops it, we crash
mid-write, or you exhaust your advice retries — we hold no evidence the
payment happened. The customer has paid, the merchant is never told, and
nothing on our side knows to look. Your `/payment` retries with
`flagAdvise: "Y"` cover the case where our *response* was lost; they cannot
cover the case where the *request* never arrived.

So we periodically call **your** SNAP Inquiry Status service (code 26) for
transactions we still record as pending, and act on what you report:

| Your `paymentFlagStatus` | What we do |
|---|---|
| `00` | Record the payment and fire the merchant callback that never went out |
| `03` | Nothing — still in flight; we ask again next cycle |
| `01` | Nothing — correctly still unpaid |
| `02` | Nothing automatic. Whether a timeout settles depends on the company's reconciliation type registered at your end, which we cannot see, so it is escalated to an operator rather than guessed |
| `4042601` | Nothing — you have no such transaction, so it was never paid |

What this means for you:

- **Expect low-volume, read-only traffic** on your status endpoint —
  transactions pending past a threshold (default 15 minutes), batched
  (default 100 per sweep, every 5 minutes). It never retries a settled
  transaction.
- **We authenticate as a normal SNAP client**: `Authorization: Bearer` from
  your `/access-token/b2b`, RSA-signed with the private key whose public half
  you registered for us, plus the same `X-SIGNATURE`/`X-TIMESTAMP`/
  `CHANNEL-ID`/`X-PARTNER-ID`/`X-EXTERNAL-ID` header set described above.
- **We need outbound credentials from you** to do this at all: your base URL,
  a `clientId` we can obtain tokens under, and your registration of our public
  key. Without them reconciliation stays disabled and this traffic never
  appears.

Every attempt is recorded — including the ones that concluded "nothing to do"
— because a reconciler that has silently stopped working is otherwise
indistinguishable from one with nothing to reconcile.

## Response codes

We follow the ASPI `AAABBCC` format — `AAA` = HTTP status, `BB` = service
code, `CC` = case code. Note the service code differs **per endpoint**, so
`/payment` never returns a `…24…` code:

| Endpoint | Success | Failures |
|---|---|---|
| `/access-token/b2b` | `2007300` "Successful" | `4007301` invalid field format (`clientId`/`clientSecret`/`grantType`, or `X-TIMESTAMP`) · `4007302` missing `X-CLIENT-KEY` · `4017300` `Unauthorized. [Signature]` / `[Unknown client]` |
| `/inquiry` (24) | `2002400` "Successful" | `4002400` bad request · `4002401` field format · `4002402` missing mandatory · `4012400`/`4012401` unauthorized · `4092400` conflict · `4042412` unregistered VA · `4042414` paid bill · `4042419` expired · `5002400` |
| `/payment` (25) | `2002500` "Successful" | `4002500` bad request · `4002501` field format · `4002502` missing mandatory · `4012500`/`4012501` unauthorized · `4092500` conflict · `4042512` unregistered VA · `4042513` invalid amount · `4042514` paid bill · `4042518` inconsistent request · `4042519` expired · `5002500` |
| `/status` (26) | `2002600` **"Success"** | `4002600` bad request · `4002601` field format · `4002602` missing mandatory · `4012600`/`4012601` unauthorized · `4042601` transaction not found · `4092600` conflict · `5002601` |

Note `2002600` pairs with `"Success"` while `2002400`/`2002500` pair with
`"Successful"`. That is BCA's own inconsistency across the three documents, not
a typo here — each is spelled as its own current table has it.

Every 4xx body except the `401`s carries `virtualAccountData` with
`inquiryStatus`/`paymentFlagStatus` `"01"` and a bilingual reason. BCA treats a
response whose status/reason are empty as a failed transaction *regardless of
the code*, so a bare `{responseCode, responseMessage}` is not a valid
rejection. The `401`s are the documented exception — BCA's tables show `-` in
the status column for them — and instead carry `"data": {}` per OAuth v1.1.

`4042518` is the one code whose `paymentFlagStatus` is **not** `01`: it echoes
the flag status of the first request, so a replayed success reports `00`.

**`paymentFlagStatus` values differ by service.** The payment service (25)
publishes only `00`/`01`/`02` and states "payment flag status other than
00,01,02 will be considered as 01". So an accepted instalment against a
variable-bill VA reports **`00`**, not `03` — the bill's remaining pending-ness
is carried by the transaction, not by this flag. `03` = Pending exists **only**
on the status service (26). Reporting `03` on a payment tells the channel the
payment was rejected while the money has in fact been recorded.

`inquiryStatus` on a `2002400` is `"00"`. `subCompany` and `totalAmount` are
read back from the stored transaction (or its bill details), so an inquiry
replay always reports the same figures as the first inquiry did; `subCompany`
falls back to BCA's default `"00000"` when the biller has none registered,
rather than being omitted — v2.4 marks it Conditional-mandatory for
non-multibill single-settlement transactions.

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

Flags worth knowing on `vendor-payment-va.sh`:

| Flag | Purpose |
|---|---|
| `-t <trxId>` | the create-va `trxId`, sent as `PaymentRequest.trxId`. Omitted from the body entirely when not given |
| `-q <paymentRequestId>` | use the inquiry's `inquiryRequestId` here when the payment follows an inquiry |
| `-k <client-key>` | override the `X-CLIENT-KEY` value (defaults to `VENDOR_CLIENT_ID`) |

[`scripts/e2e-va-flow.sh`](../../scripts/e2e-va-flow.sh) chains create-va →
inquiry → payment → callback and wires `-t`/`-q` from the previous steps'
responses automatically, so it exercises the spec-correct linkage end to end.

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
