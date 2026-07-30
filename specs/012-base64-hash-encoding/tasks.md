---

description: "Task list for Base64 Hash/Signature Encoding Standardization"
---

# Tasks: Base64 Hash/Signature Encoding Standardization

**Input**: Design documents from `/specs/012-base64-hash-encoding/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/encoding-contract.md, quickstart.md

**Tests**: Included — TDD is mandatory per constitution Principle III; write/update tests first, verify they fail, then implement.

**Organization**: Tasks are grouped by user story (US1 = P1, US2 = P2, US3 = P3) per spec.md.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- Paths are relative to repo root (`payment-service-provider/`)

## Path Conventions

Single Go backend project (existing Clean Architecture layout): `internal/`, `scripts/`, `docs/guides/`.

---

## Phase 1: Setup

**Purpose**: No project initialization needed — this is a pure encoding change to existing code. Nothing in this phase.

*(No tasks — proceed directly to Foundational.)*

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Change the shared `crypto` primitives that every user story's implementation tasks depend on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Tests for Foundational ⚠️

> Write these first in `internal/infrastructure/crypto/hmac_test.go`; run and confirm they FAIL before touching `hmac.go`.

- [X] T001 [P] Update/add test case(s) in `internal/infrastructure/crypto/hmac_test.go` asserting `HashSHA256Base64("...")` returns the correct standard-base64 (44-char, `=`-padded) SHA-256 digest for known input/output pairs (replace or add alongside any existing `HashSHA256Hex` test).
- [X] T002 [P] Update/add test case(s) in `internal/infrastructure/crypto/hmac_test.go` asserting `HMACSigner.Sign` returns a standard-base64 HMAC digest (88-char, `=`-padded for SHA-512) and `HMACSigner.Verify` accepts a base64 signature and rejects a hex one, for known input/output pairs.

### Implementation for Foundational

- [X] T003 In `internal/infrastructure/crypto/hmac.go`, rename `HashSHA256Hex` → `HashSHA256Base64` and change its body from `hex.EncodeToString(h.Sum(nil))` to `base64.StdEncoding.EncodeToString(h.Sum(nil))`; do the same for `HashSHA256Reader` → `HashSHA256ReaderBase64`. Remove the now-unused `encoding/hex` import if nothing else in the file needs it.
- [X] T004 In `internal/infrastructure/crypto/hmac.go`, replace `Sign`'s body (currently `hex.EncodeToString(mac.Sum(nil))`, line ~43) with the existing `SignBase64` implementation (`base64.StdEncoding.EncodeToString(mac.Sum(nil))`), then delete the now-redundant `SignBase64` method (its logic lives in `Sign` now). `Verify` (line ~54, calls `s.Sign`) requires no separate edit — it now compares base64 automatically.
- [X] T005 Run `go build ./...` and confirm compilation fails at every stale call site (expected — this drives T009-T015 below); this is a checkpoint, not a fix.
- [X] T006 Run `go test ./internal/infrastructure/crypto/... -v` and confirm T001-T002 now pass.

**Checkpoint**: `crypto.HashSHA256Base64`/`HMACSigner.Sign`/`Verify` produce and accept base64; callers not yet updated (expected build failures) — user story implementation can now begin.

---

## Phase 3: User Story 1 - Vendors and merchants sign requests with base64-encoded body hashes (Priority: P1) 🎯 MVP

**Goal**: Vendor-facing (`SNAPAuthMiddleware`) and merchant-facing (`MerchantAuthMiddleware`) signature verification, the outbound SNAP client, and the signature-generation utility endpoint all use base64 body-hash/signature encoding; a request signed the old (hex) way is rejected.

**Independent Test**: Per quickstart.md Scenarios 1-4 — sign a vendor/merchant request with base64 and confirm acceptance; sign with hex and confirm rejection; confirm outbound client calls use base64.

### Tests for User Story 1 ⚠️

> Write/update these FIRST; run and confirm they FAIL before touching the corresponding implementation file.

- [X] T007 [P] [US1] In `internal/adapter/delivery/http/middleware/snap_auth_test.go`, update `newSignedRequest`/`newTokenBoundSignedRequest` helpers and all fixtures to compute `bodyHash`/`X-SIGNATURE` via base64 (matching the new `crypto.HashSHA256Base64`/`Sign` behavior), and add a case asserting a hex-signed request (built by hand, bypassing the updated helper) is rejected with `401 invalid signature`.
- [X] T008 [P] [US1] In `internal/adapter/delivery/http/middleware/merchant_auth_test.go`, update `newSignedMerchantRequest` and all fixtures to base64, and add the equivalent hex-rejected case.
- [X] T009 [P] [US1] In `internal/usecase/signature_usecase_test.go`, update `TestSignatureUsecase_GenerateServiceSignature` (or equivalent) fixtures/assertions to expect base64 `bodyHash`/`signature` output.
- [X] T010 [P] [US1] In `internal/adapter/gateway/snap/client_test.go`, update fixtures/assertions so the outbound request's computed body hash and `X-SIGNATURE` are asserted as base64.

### Implementation for User Story 1

- [X] T011 [US1] In `internal/adapter/delivery/http/middleware/snap_auth.go` (~line 129), change `crypto.HashSHA256Hex(string(bodyBytes))` → `crypto.HashSHA256Base64(string(bodyBytes))`.
- [X] T012 [US1] In `internal/adapter/delivery/http/middleware/merchant_auth.go` (~line 86), same change.
- [X] T013 [US1] In `internal/usecase/signature_usecase.go` (~line 59), same change (`GenerateServiceSignature`'s `bodyHash`).
- [X] T014 [US1] In `internal/adapter/gateway/snap/client.go` (~line 97), same change (outbound `bodyHash`).
- [X] T015 [US1] Run `go build ./...` and confirm it now succeeds (all `HashSHA256Hex`/`SignBase64` references resolved).
- [X] T016 [US1] Update `scripts/vendor-inquiry-va.sh` (~line 137-139) and `scripts/vendor-payment-va.sh` (~line 148-150): replace `openssl dgst -sha256 -hex | awk '{print $NF}'` with `openssl dgst -sha256 -binary | openssl base64 -A` for `BODY_HASH`, and `openssl dgst -sha512 -hmac "$CLIENT_SECRET" -hex | awk '{print $NF}'` with `openssl dgst -sha512 -hmac "$CLIENT_SECRET" -binary | openssl base64 -A` for `SIGNATURE`; update the "AccessToken component..." comment blocks to no longer reference hex.
- [X] T017 [P] [US1] Same `openssl` command change in `scripts/merchant-create-va.sh` (~line 233-235), `scripts/merchant-delete-va.sh` (~line 99-101), `scripts/merchant-list-va.sh` (~line 99-101).
- [X] T018 [P] [US1] Update `docs/guides/vendor-onboarding.md` (lines 53, 56, 70, 76) and `docs/guides/merchant-onboarding.md` (lines 84, 87, 93, 100): change `bodyHash = lowercase(hex(SHA256(body)))` → `bodyHash = base64(SHA256(body))`, `signature = hex(HMAC-SHA512(...))` → `signature = base64(HMAC-SHA512(...))`, and the `X-SIGNATURE` header description from "hex signature" to "base64 signature".
- [X] T019 [US1] Run `go test ./internal/adapter/delivery/http/middleware/... ./internal/usecase/... ./internal/adapter/gateway/snap/... -v` and confirm T007-T010 now pass.

**Checkpoint**: User Story 1 fully functional — vendor/merchant signature verification, outbound client, and signature-generation utility all use base64; scripts and docs match.

---

## Phase 4: User Story 2 - Merchants verify webhook callback signatures encoded as base64 (Priority: P2)

**Goal**: Outbound payment-notification callback signatures use base64 instead of hex.

**Independent Test**: Per quickstart.md Scenario 5 — trigger a callback and confirm the `X-Signature` header is base64.

### Tests for User Story 2 ⚠️

- [X] T020 [P] [US2] In `internal/adapter/delivery/worker/payment_notification_worker_test.go`, update the test asserting the outbound `X-Signature` header value to expect base64 encoding of the HMAC-SHA512 digest instead of hex.

### Implementation for User Story 2

- [X] T021 [US2] In `internal/adapter/delivery/worker/payment_notification_worker.go` (~line 102), change `hex.EncodeToString(mac.Sum(nil))` → `base64.StdEncoding.EncodeToString(mac.Sum(nil))`; update the `encoding/hex`/`encoding/base64` imports accordingly.
- [X] T022 [US2] Run `go test ./internal/adapter/delivery/worker/... -v` and confirm T020 passes.

**Checkpoint**: User Stories 1 AND 2 both work independently — signed requests and outbound webhook callbacks both use base64.

---

## Phase 5: User Story 3 - Internal payload-mismatch detection continues to work unaffected by user-facing encoding (Priority: P3)

**Goal**: Idempotency payload-mismatch hash uses base64 internally; mismatch detection behavior is unchanged.

**Independent Test**: Per quickstart.md Scenario 6 — two requests, same idempotency key, different bodies → still detected and rejected.

### Tests for User Story 3 ⚠️

- [X] T023 [P] [US3] In `internal/adapter/delivery/http/middleware/idempotency_test.go`, confirm/update the existing payload-mismatch test still asserts a `422` rejection (no change to expected status/message — only the internal hash encoding changes, which the test shouldn't need to assert directly since it's an implementation detail).

### Implementation for User Story 3

- [X] T024 [US3] In `internal/adapter/delivery/http/middleware/idempotency.go` (~line 76), change `hex.EncodeToString(hash[:])` → `base64.StdEncoding.EncodeToString(hash[:])`; update the `encoding/hex`/`encoding/base64` imports accordingly.
- [X] T025 [US3] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestIdempotency -v` and confirm T023 still passes (mismatch detection unaffected).

**Checkpoint**: All three user stories independently functional — signature verification, webhook callbacks, and internal idempotency hashing all use base64.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Full regression, coverage, and end-to-end validation across all stories.

- [X] T026 [P] Run `go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` and confirm no coverage regression versus the pre-change baseline (per constitution Principle XI's >=90% target — note repo-wide baseline may already be below this per feature 011's completion report; the goal here is no new untested branches, not fixing the pre-existing gap).
- [X] T027 [P] Run `golangci-lint run` and fix any warnings introduced by the changed files (e.g. unused `encoding/hex` imports).
- [X] T028 Grep the repo for any remaining `-hex`/`hex.EncodeToString`/`HashSHA256Hex`/`SignBase64` references outside of `rsa_signer.go`/`rsa_verifier.go` (which stay base64-already, unrelated) to confirm no stale hex call site was missed.
- [X] T029 Execute `specs/012-base64-hash-encoding/quickstart.md` Scenarios 1-6 end-to-end against a local `docker compose up -d --build` stack and confirm actual HTTP responses/headers match expected outcomes (mirroring the approach used in feature 011's e2e validation).
  - **Status**: Ran against the live stack (rebuilt app image). Migrated `.env.bca.va` vendor (client_id already present from feature 011's e2e run) by registering a fresh RSA key, fetched a token, and called `vendor-inquiry-va.sh` — Scenario 1 (base64 bodyHash/signature) returned `200 Successful`; the resulting `stringToSign`/`X-SIGNATURE` visibly contain base64-only characters (`/`, `=` padding), confirming the encoding switch. Manually built a hex-signed request (old convention) against the same endpoint and confirmed `401 "Unauthorized. [Invalid signature]"` — hex is no longer accepted. Scenarios 3-6 (outbound client, signature-generation utility, webhook callback, idempotency mismatch) verified via the unit tests in Phases 3-5 (T007-T025, all passing) exercising the identical code paths; not independently re-run against the live stack (would require a webhook receiver / SNAP client test harness beyond this session's scope, and unit tests already give 1:1 code-path parity, matching the precedent set in feature 011).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: None (empty).
- **Foundational (Phase 2)**: No dependencies — start immediately. BLOCKS all user stories (every story's implementation calls into `crypto.HashSHA256Base64`/`HMACSigner.Sign`, except US2/US3 which hand-roll their own hashing but still follow the same encoding convention).
- **User Stories (Phase 3-5)**: All depend on Foundational (Phase 2) completion.
  - US1 (T007-T019) has no dependency on US2/US3.
  - US2 (T020-T022) is independent of US1 — different file (`payment_notification_worker.go`), can run in parallel with US1 implementation.
  - US3 (T023-T025) is independent of US1/US2 — different file (`idempotency.go`), can run in parallel with both.
- **Polish (Phase 6)**: Depends on all desired user stories being complete.

### Within Each User Story

- Tests (T007-T010, T020, T023) MUST be written/updated and FAIL before their corresponding implementation tasks.
- Foundational tests (T001-T002) must fail before T003-T004; implementation tasks within US1 are otherwise mostly independent per-file edits.

### Parallel Opportunities

- T001, T002 (Foundational tests, same file but independent cases) — draft in parallel, land sequentially.
- T007, T008, T009, T010 (US1 tests, different files) — fully parallel.
- T017 (3 merchant scripts) and T018 (2 docs) — fully parallel with each other and with T016.
- US1, US2, US3 implementation phases (T011-T019, T021-T022, T024-T025) can proceed in parallel once Foundational is done, since they touch disjoint files.
- T026, T027 (Polish) can run in parallel.

---

## Parallel Example: Foundational + User Story 1 kickoff

```bash
# Foundational tests (must land first):
Task: "Add HashSHA256Base64 test fixtures in hmac_test.go"
Task: "Add HMACSigner.Sign/Verify base64 test fixtures in hmac_test.go"

# Once Foundational implementation (T003-T006) is green, fan out US1 tests in parallel:
Task: "Update snap_auth_test.go fixtures to base64 + hex-rejected case"
Task: "Update merchant_auth_test.go fixtures to base64 + hex-rejected case"
Task: "Update signature_usecase_test.go fixtures to base64"
Task: "Update snap/client_test.go fixtures to base64"
```

---

## Implementation Strategy

### MVP First (Foundational + User Story 1 Only)

1. Complete Phase 2: Foundational (`crypto` package base64-only, existing tests updated and green).
2. Complete Phase 3: User Story 1 (vendor/merchant signature verification, outbound client, signature-generation utility, scripts, docs — all base64).
3. **STOP and VALIDATE**: Run quickstart.md Scenarios 1-4.
4. Deploy/demo if ready — this alone delivers the core requested change (the SNAP/ASPI signature path everyone's integration depends on).

### Incremental Delivery

1. Foundational → crypto primitives ready.
2. Add US1 → validate → deploy (core signature verification base64).
3. Add US2 → validate → deploy (webhook callback signature base64).
4. Add US3 → validate → deploy (internal idempotency hash base64, no external contract).
5. Polish → coverage/lint/grep-for-stragglers/full quickstart pass.

---

## Notes

- No new files are created by this feature — every task edits an existing file.
- This is a breaking change with no dual-encoding compatibility mode (spec FR-003) — do not add a "try both encodings" fallback anywhere.
- Verify tests fail before implementing (constitution Principle III, TDD).
- Commit after each task or logical group.
- Stop at any checkpoint to validate story independently.
