# Quickstart: Static and Dynamic Virtual Account Creation

Validates the feature end-to-end against the running `payment-service-provider` service. See [data-model.md](./data-model.md) for schema/entity details and [contracts/create-va.yaml](./contracts/create-va.yaml) for the request/response contract.

## Prerequisites

- Service running locally with migrations applied through `000007_create_va_payments.up.sql` (run `make migrate-up` or the project's existing migration command).
- A valid SNAP/merchant auth context for calling `POST /openapi/v1.0/transfer-va/create-va` (reuse existing auth headers from feature 002).
- `curl` or an HTTP client, `jq` for pretty-printing responses.

## Scenario 1 — Dynamic VA, no bill (US1, vaType 04)

```bash
curl -s -X POST "$BASE_URL/openapi/v1.0/transfer-va/create-va" \
  -H "Content-Type: application/json" \
  -H "X-EXTERNAL-ID: quickstart-dyn-nobill-1" \
  -d '{
    "partnerServiceId": "15973",
    "customerNo": "",
    "virtualAccountNo": "15973000000000000001",
    "virtualAccountName": "Quickstart Dynamic NoBill",
    "trxId": "trx-dyn-nobill-1",
    "additionalInfo": {"vaType": "04"}
  }' | jq .
```

**Expected**: `responseCode` `2002700`; `virtualAccountData.customerNo` is a 20-digit string starting with `04`.

## Scenario 2 — Dynamic VA, fixed bill (vaType 06) with totalAmount

```bash
curl -s -X POST "$BASE_URL/openapi/v1.0/transfer-va/create-va" \
  -H "Content-Type: application/json" \
  -H "X-EXTERNAL-ID: quickstart-dyn-fixed-1" \
  -d '{
    "partnerServiceId": "15975",
    "customerNo": "",
    "virtualAccountNo": "15975000000000000002",
    "virtualAccountName": "Quickstart Dynamic Fixed",
    "trxId": "trx-dyn-fixed-1",
    "totalAmount": {"value": "150000.00", "currency": "IDR"},
    "additionalInfo": {"vaType": "06"}
  }' | jq .
```

**Expected**: `customerNo` starts with `06`; response echoes `totalAmount`.

## Scenario 3 — Two concurrent dynamic requests get distinct sequential customerNo (FR-005, SC-003)

```bash
for i in 1 2; do
  curl -s -X POST "$BASE_URL/openapi/v1.0/transfer-va/create-va" \
    -H "Content-Type: application/json" \
    -H "X-EXTERNAL-ID: quickstart-concurrent-$i" \
    -d '{
      "partnerServiceId": "15973",
      "customerNo": "",
      "virtualAccountNo": "1597300000000000000'"$i"'",
      "virtualAccountName": "Quickstart Concurrent",
      "trxId": "trx-concurrent-'"$i"'",
      "additionalInfo": {"vaType": "04"}
    }' | jq -r '.virtualAccountData.customerNo' &
done
wait
```

**Expected**: Two distinct `customerNo` values printed (no duplicates), both prefixed `04`.

## Scenario 4 — Static VA, merchant-supplied customerNo is echoed (US2, vaType 01)

```bash
curl -s -X POST "$BASE_URL/openapi/v1.0/transfer-va/create-va" \
  -H "Content-Type: application/json" \
  -H "X-EXTERNAL-ID: quickstart-static-nobill-1" \
  -d '{
    "partnerServiceId": "15973",
    "customerNo": "0001234567",
    "virtualAccountNo": "15973000012345670001",
    "virtualAccountName": "Quickstart Static NoBill",
    "trxId": "trx-static-nobill-1",
    "additionalInfo": {"vaType": "01"}
  }' | jq .
```

**Expected**: `virtualAccountData.customerNo` == `"0001234567"` (unchanged).

## Scenario 5 — Duplicate static customerNo is rejected (FR-008)

Re-run Scenario 4's exact request with a new `trxId`/`X-EXTERNAL-ID` but the same `customerNo` and `partnerServiceId`.

**Expected**: `409` response, `responseCode` indicating conflict/duplicate customer number.

## Scenario 6 — Invalid partnerServiceId/vaType combination is rejected (US3)

```bash
curl -s -X POST "$BASE_URL/openapi/v1.0/transfer-va/create-va" \
  -H "Content-Type: application/json" \
  -H "X-EXTERNAL-ID: quickstart-invalid-combo-1" \
  -d '{
    "partnerServiceId": "15973",
    "customerNo": "0009999999",
    "virtualAccountNo": "15973000099999990001",
    "virtualAccountName": "Quickstart Invalid",
    "trxId": "trx-invalid-1",
    "additionalInfo": {"vaType": "02"}
  }' | jq .
```

**Expected**: `400` response — `partnerServiceId`/`vaType` combination invalid (15973 only pairs with 01/04).

## Scenario 7 — Variable bill accepts multiple payments until "lunas" (FR-013, SC-006)

1. Create a variable-bill VA (vaType `02` or `05`) with `totalAmount.value = "100000.00"` (as in Scenario 2, swap vaType).
2. Record two payments of `60000.00` and `40000.00` against the resulting VA (via the existing payment-notification endpoint from feature 002, referencing the same `virtualAccountNo`).
3. Query the VA (existing list/inquiry endpoint).

**Expected**: Two payment records exist for the transaction; VA `status` becomes `00` (paid/lunas) only after the second payment brings the cumulative total to `100000.00`, not after the first.

## Scenario 8 — Master data change takes effect without a restart (User Story 4, FR-017, SC-008)

1. Confirm current behavior: run Scenario 6 (invalid combination `15973`/`02`) and confirm it's still rejected with `400`.
2. Directly update the database: change `master_va_type`'s row for `va_type = '02'` so its `partner_service_id` becomes `15973` instead of `15974` (simulating an operator remapping a VA type to a different partner service ID).
3. Re-run the same request from step 1 (`partnerServiceId: 15973`, `additionalInfo.vaType: 02`) without restarting the service.

**Expected**: Depending on how the change was made:
- If made through the application's own data-access layer (e.g. an internal admin call), the very next request reflects the change (accepted instead of rejected) — no 5-minute wait.
- If made via a raw SQL statement directly against PostgreSQL (bypassing the app, as in step 2 above), the change is visible only after up to 5 minutes (the scheduled cache refresh), per spec.md's documented Assumption that the application can only proactively invalidate changes it observes through its own write path.

Revert the row back to `partner_service_id = 15974` after this scenario so subsequent runs of Scenario 6 behave as documented.

## Scenario 9 — Cache absorbs read load (User Story 4, SC-009)

1. With the service running normally, issue a burst of `/create-va` requests (e.g. 100, any valid combination) in quick succession.
2. Observe PostgreSQL query logs/metrics for `master_va_type`/`master_partner_service_ids` during the burst.

**Expected**: Query count against these two tables does not scale with the 100 requests — at most one or two queries (the periodic 5-minute refresh, or a one-time cache warm-up on first access), not one per request.

## Cleanup

No teardown required — quickstart data uses distinct `trxId`/`virtualAccountNo` values per scenario and does not require deletion for repeatability across runs (use a new suffix to re-run). Scenario 8 is the exception: revert its manual DB edit as instructed so later scenario runs stay consistent.
