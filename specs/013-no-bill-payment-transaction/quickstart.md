# Quickstart: No-Bill VA — Register Once, Pay Many Times

Validates the feature end-to-end against the running `payment-service-provider` stack. See [data-model.md](./data-model.md) for the schema and [contracts/](./contracts/) for the per-endpoint wire behavior.

The point of every scenario below is the same pair of assertions: **`/create-va` writes zero transactions**, and **payment N+1 succeeds just like payment N**.

## Prerequisites

- Stack up with migrations applied through `000014_create_va_accounts.up.sql`:
  ```bash
  docker compose up -d
  docker compose up -d migrate
  ```
- Merchant and vendor onboarded — reuse the existing helpers:
  ```bash
  ./scripts/onboard-merchant.sh
  ./scripts/onboard-vendor.sh
  ```
- `curl` and `jq` available. Export `BASE_URL` (default `http://localhost:8080`).
- A psql shell for the assertions:
  ```bash
  alias psqlc='docker compose exec -T postgres psql -U postgres -d payment_gateway -t -A'
  ```

The existing `./scripts/merchant-create-va.sh`, `./scripts/vendor-inquiry-va.sh`, and `./scripts/vendor-payment-va.sh` handle SNAP signing; use them instead of raw `curl` where signature headers are required.

---

## Scenario 1 — Static no-bill (`01`): registration writes no transaction

**Covers**: US1 AS1, FR-001, SC-002

```bash
./scripts/merchant-create-va.sh \
  --partner-service-id "15973" \
  --customer-no "000000000000000101" \
  --va-name "Quickstart NoBill Static" \
  --trx-id "qs-nobill-static-1" \
  --va-type "01"
```

**Expected response**: `responseCode` = `2002700`, `virtualAccountData.customerNo` = `000000000000000101`, and no `totalAmount` field.

**Assert**:

```bash
# Registration exists, exactly one row
psqlc "SELECT status, customer_name FROM va_accounts
       WHERE virtual_account_no = '15973000000000000101';"
# → ACTIVE|Quickstart NoBill Static

# Zero transactions — this is the defect being fixed
psqlc "SELECT COUNT(*) FROM va_transactions
       WHERE virtual_account_no = '15973000000000000101';"
# → 0
```

---

## Scenario 2 — Inquiry on a never-paid VA returns the holder name

**Covers**: US3 AS1, FR-015, FR-016

```bash
./scripts/vendor-inquiry-va.sh \
  --va-no "15973000000000000101" \
  --inquiry-request-id "qs-inq-1" \
  --amount "50000.00"
```

**Expected**: `responseCode` = `2002400`, `inquiryStatus` = `00`, `virtualAccountName` = `Quickstart NoBill Static`, `totalAmount.value` = `50000.00` (echoed from the request — a no-bill VA asserts no bill).

**Assert** the inquiry created nothing:

```bash
psqlc "SELECT COUNT(*) FROM va_transactions
       WHERE virtual_account_no = '15973000000000000101';"
# → 0
```

---

## Scenario 3 — Three payments into one registration

**Covers**: US2 AS1/AS2, FR-008, FR-010, SC-001, SC-007 — the headline fix

```bash
for i in 1 2 3; do
  ./scripts/vendor-payment-va.sh \
    --va-no "15973000000000000101" \
    --payment-request-id "qs-pay-$i" \
    --paid-amount "$((i * 10000)).00" \
    --reference-no "QSREF0000$i"
  echo "--- payment $i done ---"
done
```

**Expected**: all three return `responseCode` = `2002500` with `paymentFlagStatus` = `00`. Before this feature, payment 2 returned `4092500` "already paid or inactive".

**Assert**:

```bash
psqlc "SELECT payment_request_id, paid_amount, total_amount, status
       FROM va_transactions
       WHERE virtual_account_no = '15973000000000000101'
       ORDER BY created_at;"
# → qs-pay-1|10000.00|10000.00|00
#   qs-pay-2|20000.00|20000.00|00
#   qs-pay-3|30000.00|30000.00|00

# One VA, three transactions (SC-007)
psqlc "SELECT (SELECT COUNT(*) FROM va_accounts WHERE virtual_account_no='15973000000000000101'),
              (SELECT COUNT(*) FROM va_transactions WHERE virtual_account_no='15973000000000000101');"
# → 1|3

# Registration untouched by payments
psqlc "SELECT status FROM va_accounts WHERE virtual_account_no='15973000000000000101';"
# → ACTIVE
```

---

## Scenario 4 — Payment idempotency

**Covers**: US2 AS4, FR-012

Replay payment 2 verbatim:

```bash
./scripts/vendor-payment-va.sh \
  --va-no "15973000000000000101" \
  --payment-request-id "qs-pay-2" \
  --paid-amount "20000.00" \
  --reference-no "QSREF00002"
```

**Expected**: `2002500` with the original values echoed.

**Assert** still exactly three rows:

```bash
psqlc "SELECT COUNT(*) FROM va_transactions
       WHERE virtual_account_no = '15973000000000000101';"
# → 3
```

---

## Scenario 5 — Repeat `/create-va` updates the registration

**Covers**: US1 AS3, FR-005

```bash
./scripts/merchant-create-va.sh \
  --partner-service-id "15973" \
  --customer-no "000000000000000101" \
  --va-name "Quickstart Renamed Holder" \
  --trx-id "qs-nobill-static-2" \
  --va-type "01"
```

**Expected**: `2002700` — **not** `4092700`/`4092701`.

**Assert** the name changed and no fourth transaction appeared:

```bash
psqlc "SELECT customer_name FROM va_accounts WHERE virtual_account_no='15973000000000000101';"
# → Quickstart Renamed Holder

psqlc "SELECT COUNT(*) FROM va_transactions WHERE virtual_account_no='15973000000000000101';"
# → 3
```

---

## Scenario 6 — Dynamic no-bill (`04`) generates a fresh VA each call

**Covers**: US1 AS2, FR-003

```bash
./scripts/merchant-create-va.sh --partner-service-id "15973" --customer-no "" \
  --va-name "Quickstart Dyn NoBill A" --trx-id "qs-dyn-1" --va-type "04"

./scripts/merchant-create-va.sh --partner-service-id "15973" --customer-no "" \
  --va-name "Quickstart Dyn NoBill B" --trx-id "qs-dyn-2" --va-type "04"
```

**Expected**: two different `customerNo` values, each 20 digits starting `04`.

**Assert** two registrations, zero transactions:

```bash
psqlc "SELECT COUNT(*) FROM va_accounts WHERE va_type='04' AND trx_id IN ('qs-dyn-1','qs-dyn-2');"
# → 2
psqlc "SELECT COUNT(*) FROM va_transactions WHERE trx_id IN ('qs-dyn-1','qs-dyn-2');"
# → 0
```

---

## Scenario 7 — Deactivation stops payments, keeps history

**Covers**: US6 AS1/AS2/AS3, FR-019, FR-020

```bash
./scripts/merchant-delete-va.sh \
  --partner-service-id "15973" \
  --customer-no "000000000000000101" \
  --va-no "15973000000000000101" \
  --trx-id "qs-del-1"
```

**Expected**: `2003100`. Repeat the same call — still `2003100` (idempotent).

Then attempt a payment:

```bash
./scripts/vendor-payment-va.sh \
  --va-no "15973000000000000101" \
  --payment-request-id "qs-pay-99" \
  --paid-amount "10000.00"
```

**Expected**: `4042519` Invalid Bill/Virtual Account.

**Assert** history intact:

```bash
psqlc "SELECT status FROM va_accounts WHERE virtual_account_no='15973000000000000101';"
# → INACTIVE
psqlc "SELECT COUNT(*) FROM va_transactions
       WHERE virtual_account_no='15973000000000000101' AND status='00';"
# → 3   (unchanged)
```

---

## Scenario 8 — Merchant listing separates VAs from transactions

**Covers**: FR-023, SC-007

```bash
./scripts/merchant-list-va.sh --partner-service-id "15973" | jq '.data[] |
  select(.virtualAccountNo == "15973000000000000101")'
```

**Expected**: exactly one entry, with `transactionCount: 3` and `totalPaid.value: "60000.00"`.

```bash
curl -s -X POST "$BASE_URL/openapi/v1.0/transfer-va/list-transactions" \
  -H "Authorization: Bearer $MERCHANT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"virtualAccountNo":"15973000000000000101","page":1,"pageSize":20}' | jq '.pagination.totalRows'
# → 3
```

---

## Scenario 9 — Bill-bearing types unchanged (regression)

**Covers**: US5, FR-021, SC-005

```bash
# Fixed bill (03) — transaction still created at create-va time
./scripts/merchant-create-va.sh --partner-service-id "15975" \
  --customer-no "000000000000000201" --va-name "Quickstart Fixed Bill" \
  --trx-id "qs-fixed-1" --va-type "03" --total-amount "75000.00"

psqlc "SELECT status, total_amount FROM va_transactions
       WHERE virtual_account_no='15975000000000000201';"
# → 03|75000.00     (pending transaction present — unchanged behavior)
```

Then confirm the conflict guard still fires:

```bash
./scripts/merchant-create-va.sh --partner-service-id "15975" \
  --customer-no "000000000000000201" --va-name "Quickstart Fixed Bill" \
  --trx-id "qs-fixed-2" --va-type "03" --total-amount "75000.00"
# → 4092700 or 4092701 — unchanged
```

Also run the existing suites unchanged:

```bash
./scripts/e2e-va-flow.sh
./scripts/e2e-dynamic-va-flow.sh
./scripts/e2e-expired-callback-flow.sh
./scripts/e2e-va-cancel-flow.sh
```

---

## Scenario 10 — Legacy VAs still work after the migration

**Covers**: FR-022, SC-006 — run this **before** and **after** applying migration `000014`

Before the migration, capture the in-flight no-bill VAs:

```bash
psqlc "SELECT virtual_account_no FROM va_transactions
       WHERE va_type IN ('01','04') AND status='03';" > /tmp/inflight-vas.txt
wc -l < /tmp/inflight-vas.txt
```

After `docker compose up -d migrate`, confirm each got a registration:

```bash
while read -r va; do
  n=$(psqlc "SELECT COUNT(*) FROM va_accounts WHERE virtual_account_no='$va';")
  [ "$n" = "1" ] || echo "STRANDED: $va"
done < /tmp/inflight-vas.txt
# → no output = zero stranded VAs (SC-006)
```

Then pay one of them and confirm it settles through the new path:

```bash
./scripts/vendor-payment-va.sh --va-no "$(head -1 /tmp/inflight-vas.txt)" \
  --payment-request-id "qs-legacy-1" --paid-amount "5000.00"
# → 2002500
```

The pre-existing `'03'` row is intentionally left in place — it ages out through the normal expiry path rather than being settled by this payment (research.md R-006).

---

## Cleanup

```bash
psqlc "DELETE FROM va_transactions WHERE virtual_account_no LIKE '15973%0000001%';"
psqlc "DELETE FROM va_accounts     WHERE virtual_account_no LIKE '15973%0000001%';"
```

## Exit criteria

All ten scenarios pass, plus:

```bash
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out   # ≥ 90%
golangci-lint run
```
