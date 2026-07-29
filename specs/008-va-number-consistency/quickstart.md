# Quickstart: Validating VA Number Consistency

Validates the behavior described in [spec.md](./spec.md) and [data-model.md](./data-model.md) end-to-end against a running instance, reusing the same scripts and environment as feature 006's [e2e-dynamic-va-flow.sh](../../scripts/e2e-dynamic-va-flow.sh).

## Prerequisites

- Local stack running: `docker compose up -d` (app, postgres, redis; see `docker-compose.yml`).
- A registered B2B test client (RSA keypair inserted into `client_apps`/`client_keys`) and a vendor HMAC secret in a `.env.<vendor>.<channel>` file — see this repo's existing onboarding quickstart at `specs/001-snap-token-management/quickstart.md`, Scenario 1, for the exact `openssl`/`psql` commands.
- `curl`, `openssl`, `uuidgen`, `jq` installed.

## Scenario 1 — Static VA: matching virtualAccountNo is accepted (regression check)

```bash
./scripts/merchant-create-va.sh -s 15973 -y 01 \
  -v "1597300000000000000123" -c "000000000000000123" \
  -n "Static Consistency OK" -t "trx-static-ok-$(date +%s)" \
  -e "$VENDOR_CLIENT_SECRET" -o "$ACCESS_TOKEN" -u http://localhost:8080
```

**Expected**: `responseCode` 2xx; `virtualAccountData.virtualAccountNo` == `"1597300000000000000123"` (i.e. `partnerServiceId` `15973` + `customerNo` `000000000000000123`).

## Scenario 2 — Static VA: mismatched virtualAccountNo is rejected (new behavior, FR-001/FR-002)

```bash
./scripts/merchant-create-va.sh -s 15973 -y 01 \
  -v "9999999999999999999999" -c "000000000000000123" \
  -n "Static Consistency Mismatch" -t "trx-static-mismatch-$(date +%s)" \
  -e "$VENDOR_CLIENT_SECRET" -o "$ACCESS_TOKEN" -u http://localhost:8080
```

**Expected**: `responseCode` `4002707`; no VA record created (a follow-up create with the same `customerNo` and a correct `virtualAccountNo` must succeed, proving nothing was persisted).

## Scenario 3 — Dynamic VA: virtualAccountNo left empty is auto-derived (FR-004)

```bash
./scripts/merchant-create-va.sh -s 15973 -n "Dynamic Auto-Derive" -y 04 \
  -t "trx-dyn-auto-$(date +%s)" -e "$VENDOR_CLIENT_SECRET" -o "$ACCESS_TOKEN" -u http://localhost:8080
# note: -v (virtualAccountNo) intentionally omitted
```

**Expected**: `responseCode` 2xx; `virtualAccountData.customerNo` is a server-generated 20-digit value starting with `04`; `virtualAccountData.virtualAccountNo` == `"15973" + customerNo`.

## Scenario 4 — Dynamic VA: merchant-supplied virtualAccountNo is honored (FR-005)

```bash
VA_NO="1597304$(date +%s)999"
./scripts/merchant-create-va.sh -s 15973 -n "Dynamic Merchant Chosen" -y 04 \
  -v "$VA_NO" -t "trx-dyn-chosen-$(date +%s)" \
  -e "$VENDOR_CLIENT_SECRET" -o "$ACCESS_TOKEN" -u http://localhost:8080
```

**Expected**: `responseCode` 2xx; `virtualAccountData.virtualAccountNo` == `$VA_NO` exactly (not overridden), alongside a server-generated `customerNo`.

## Scenario 5 — Conflict on a colliding virtualAccountNo (FR-005a)

Repeat Scenario 4 with the **same** `$VA_NO` while the first VA is still pending (status `03`, i.e. before any payment against it).

**Expected**: `responseCode` `4092700` (conflict — VA already has an active pending transaction); no duplicate record created.

## Scenario 6 — Regression: inquiry/payment still work end-to-end (Story 3)

Run the existing full flow, which exercises create → inquiry → payment for all three dynamic vaTypes:

```bash
./scripts/e2e-dynamic-va-flow.sh -f .env.bca.va -u http://localhost:8080
```

**Expected**: all existing checks still pass (16/16 as of the last run), now additionally implying `virtualAccountNo == partnerServiceId + customerNo` holds for every VA the script creates (since it no longer needs to hand-craft a `virtualAccountNo` — Scenario 3 above supersedes the script's current manual construction once the tasks phase updates it, see spec Assumptions).
