# Quickstart: Validate SNAP VA Field & Validation Compliance Fix

## Prerequisites

- Go toolchain installed (per repo `go.mod`)
- Local Postgres/Redis available if running the full service (`docker-compose up`), or run unit tests only (no external deps needed for `internal/domain` and `internal/usecase` tests)

## Run the affected unit tests

```bash
go test ./internal/domain/... ./internal/usecase/... -run 'VA' -v
```

Expect: all `TestVAUsecase_Payment_*`, `TestVAUsecase_Inquiry_*`, `TestVAUsecase_Status_*`, and `va_test.go` struct tests pass with the corrected field/validation behavior described in [data-model.md](./data-model.md).

## Manual/scripted validation against the running API

The helper scripts in `scripts/` already send spec-compliant payloads (`txnDateInit`+`amount` for inquiry, `paymentRequestId`+`paidAmount`-only for payment, client-supplied `virtualAccountNo` for create-va, `additionalInfo.dbUrlProcess` for the callback URL) — see each script's own header comment and [contracts/snap-va-inquiry-payment.md](./contracts/snap-va-inquiry-payment.md) for the exact shapes.

Fastest path — run the whole flow (token → create-va → inquiry → payment → merchant-callback verification) in one command:

```bash
./scripts/e2e-va-flow.sh -s 12345678 -c 0812345678 -n "Merchant Name" \
  -i <client_id> -k <private_key.pem> -f .env.bca.va \
  -a 10000 -b INV-001 -d "Invoice Januari"
```

Or drive each endpoint individually:

```bash
./scripts/curl-b2b-token.sh -i <client_id> -p <private_key.pem>
./scripts/merchant-create-va.sh -s <partnerServiceId> -c <customerNo> -n <name> -f .env.bca.va
./scripts/vendor-inquiry-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> -f .env.bca.va
./scripts/vendor-payment-va.sh -s <partnerServiceId> -c <customerNo> -v <virtualAccountNo> -f .env.bca.va
```

**Expected outcomes**:

1. Inquiry request with `txnDateInit` and `amount` → `200 OK`, `txnDateInit` value reflected/used internally (not nil).
2. Inquiry request missing `amount` → `400` with mandatory-field error naming `amount`.
3. Payment request with only `paymentRequestId` + `paidAmount` (no `transactionDate`, no `totalAmount`) → `200 OK` (previously `400`); response echoes identity/amount fields (Phase 6).
4. Payment request missing `paymentRequestId` → `400` naming `paymentRequestId`.
5. Payment request missing `paidAmount` → `400` naming `paidAmount`.
6. Create-va with a `billDetails` entry, then inquiry against the same `virtualAccountNo`, → the bill detail appears in the inquiry response (Phase 6).
7. Create-va again for a VA whose prior transaction is already paid → `200 OK` (reuse allowed); create-va again while the prior transaction is still pending → `409` `4092700` (Phase 6).
8. `vendor-payment-va.sh` → the merchant callback registered via `additionalInfo.dbUrlProcess` is delivered asynchronously — `e2e-va-flow.sh` prints the received payload directly in the terminal.

## Regression check

```bash
go test ./... -cover
```

Confirm overall coverage stays ≥ 90% per constitution Principle XI, and no previously-passing test outside the touched files breaks.
