# End-to-end transcripts

Recorded request/response traffic from the SNAP Virtual Account end-to-end
suites, kept here so the wire format can be checked against BCA's field tables
without re-running anything or reading the tests.

Three files, three different things being proved:

| File | What produced it | What it proves |
|---|---|---|
| [e2e-transcript.md](e2e-transcript.md) | `test/e2e` (in-process) | Every scenario the Go suite covers — all three billing types, the negative cases, multi-vendor and multi-merchant isolation — through the production router, idempotency middleware, SNAP auth middleware, handler and usecase |
| [live-flows-transcript.md](live-flows-transcript.md) | `scripts/e2e-*.sh` against a real deployment | The full merchant→vendor lifecycle with real Postgres, real Redis idempotency, real B2B access tokens and real Asynq callbacks: create-va, inquiry, payment, status, delete-va, expiry |
| [live-negative-transcript.md](live-negative-transcript.md) | `scripts/e2e-negative-cases.sh` | The rejections that are decided from stored state — not found, paid bill, wrong amount, inconsistent request — plus the auth, header and payload rejections, against that same real deployment |

The in-process and live suites overlap on purpose. The in-process one is fast
and covers the widest set of cases; the live one is the only place the
database-backed decisions and the real token/signature chain are actually
exercised.

## Regenerating

**In-process transcript.** Off unless `E2E_TRANSCRIPT` names an output path, so
an ordinary `go test ./...` never dirties the working tree. A relative path is
resolved against the repository root:

```sh
E2E_TRANSCRIPT=docs/e2e/e2e-transcript.md go test ./test/e2e/...
```

**Live transcripts.** These need a running deployment with a vendor and a
merchant onboarded. Note that the vendor needs *two* credentials: the HMAC
secret from `scripts/onboard-vendor.sh` for the inbound signature, and a
registered `clientId` + RSA key (`scripts/onboard-merchant.sh -i <clientId>`)
to mint the B2B access token that `transfer-va` requires.

```sh
./scripts/e2e-va-flow.sh -s 15975 -c <18-digit customerNo> -n "Payer" -y 03 \
    -a 250000.00 -m .env.merchant.NAME -f .env.vendor.CHANNEL -u "$BASE" \
    -O docs/e2e/flow.log

./scripts/e2e-negative-cases.sh -m .env.merchant.NAME -f .env.vendor.CHANNEL \
    -u "$BASE" -O docs/e2e/live-negative-transcript.md
```

`partnerServiceId` 15973/15974/15975 are reserved in `master_partner_service_ids`
for the no-bill / variable-bill / fixed-bill VA types and reject a `create-va`
that carries no `vaType`. Flows that do not pass `-y` need an unreserved id.

## What is redacted

`Authorization: Bearer` values in the live transcripts are replaced with
`<accessToken>`. They are single-use, expire in minutes, and add nothing to a
conformance review.

Signatures are kept. They are computed over the redacted token, so they cannot
be reproduced from these files, and seeing them is the point of a signature
transcript. The in-process transcript's signatures are computed over the test
suite's own throwaway secret and change on every run.
