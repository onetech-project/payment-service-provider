---

description: "Task list for Vendor Access Token in Symmetric Signature"
---

# Tasks: Vendor Access Token in Symmetric Signature

**Input**: Design documents from `/specs/011-vendor-access-token-signature/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/snap-auth-middleware.md, quickstart.md

**Tests**: Included — TDD is mandatory per constitution Principle III; write tests first, verify they fail, then implement.

**Organization**: Tasks are grouped by user story (US1 = P1, US2 = P2, US3 = P3) per spec.md.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- Paths are relative to repo root (`payment-service-provider/`)

## Path Conventions

Single Go backend project (existing Clean Architecture layout): `internal/`, `cmd/api/`, `scripts/`.

---

## Phase 1: Setup

**Purpose**: Confirm prerequisite state before any code changes; no code changes in this phase.

- [ ] T001 Verify a vendor `client_id` used in local/dev testing has an active `client_apps` row and registered `client_keys` RSA key (reuse the existing merchant onboarding path/admin endpoints in `internal/adapter/delivery/http/handler/client_handler.go`) so `/access-token/b2b` can issue it a token; document the vendor/channel + client_id used in `specs/011-vendor-access-token-signature/quickstart.md` if it needs updating.
  - **Status**: Not run — this is an operator/admin action against a real deployment (create a DB row + register a key), not a code change; nothing in the code depends on it existing. Left for whoever provisions the first migrated vendor.

**Checkpoint**: A test vendor client_id with a working token-issuance path exists — needed to exercise Phase 3 (US1) end-to-end.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Wire `jwtIssuer` into `SNAPAuthMiddleware`'s constructor signature — required by every user story below, since all of them add checks inside this same middleware function.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T002 Change `SNAPAuthMiddleware` signature in `internal/adapter/delivery/http/middleware/snap_auth.go` from `func SNAPAuthMiddleware(vendorConfig *config.VendorConfig) echo.MiddlewareFunc` to `func SNAPAuthMiddleware(vendorConfig *config.VendorConfig, jwtIssuer domain.JWTIssuer) echo.MiddlewareFunc` (import `backbone-new/internal/domain`), with no behavior change yet (parameter unused for now).
- [X] T003 Update the call site in `cmd/api/main.go` (~line 394, `vendorGroup.Use(customMiddleware.SNAPAuthMiddleware(vc))`) to pass the existing `jwtIssuer` variable: `customMiddleware.SNAPAuthMiddleware(vc, jwtIssuer)`.
- [X] T004 Update all existing call sites of `SNAPAuthMiddleware` in `internal/adapter/delivery/http/middleware/snap_auth_test.go` to pass a `domain.JWTIssuer` (a real `crypto.NewJWTIssuerFromPEM`-backed instance or a lightweight stub satisfying the interface) so existing tests continue to compile and pass unchanged.
- [X] T005 Run `go build ./...` and the existing `go test ./internal/adapter/delivery/http/middleware/... ./cmd/...` to confirm the signature change alone introduces no regressions before adding new behavior.

**Checkpoint**: `SNAPAuthMiddleware` accepts a `jwtIssuer` with zero behavior change; existing tests pass. User story implementation can now begin.

---

## Phase 3: User Story 1 - Vendor authenticates transactional requests with Authorization header (Priority: P1) 🎯 MVP

**Goal**: A vendor with `ClientID` configured can send transactional requests with `Authorization: Bearer <token>`, bind that token into the HMAC signature, and be accepted; the same vendor omitting the header is rejected.

**Independent Test**: Per quickstart.md Scenarios 1 and 2 — obtain a token via `/access-token/b2b`, sign+send a migrated-vendor request with `Authorization` and confirm `200`; then omit `Authorization` and confirm `401` with a message identifying the missing header.

### Tests for User Story 1 ⚠️

> Write these tests FIRST in `internal/adapter/delivery/http/middleware/snap_auth_test.go`; run and confirm they FAIL before touching `snap_auth.go`.

- [X] T006 [P] [US1] Add test case: vendor config with `ClientID` set, request sent with `Authorization: Bearer <validToken>` and `stringToSign` built with the real token → expect `200`/next handler invoked, in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.
- [X] T007 [P] [US1] Add test case: vendor config with `ClientID` set, request sent with no `Authorization` header at all → expect `401` with a response message identifying the missing/invalid `Authorization` header (distinct wording from the generic invalid-signature message), in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.
- [X] T008 [P] [US1] Add test case: vendor config with `ClientID` **empty** (legacy/non-migrated), request signed with the old convention (no `Authorization`, empty AccessToken component) → expect `200`/next handler invoked (unchanged legacy behavior), in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.

### Implementation for User Story 1

- [X] T009 [US1] In `internal/adapter/delivery/http/middleware/snap_auth.go`, after the existing timestamp-skew check (~line 100) and before the body-hash/signature block (~line 109), add: if `vendorConfig.ClientID != ""`, parse `Authorization` header requiring `Bearer ` prefix (mirror `merchant_auth.go:26-32`); on missing/malformed header, return `401` `{"responseCode": "4010000", "responseMessage": "Unauthorized. [Missing or invalid Authorization header]"}`.
- [X] T010 [US1] In the same block, when `Authorization` is present and `vendorConfig.ClientID != ""`, call `jwtIssuer.ValidateToken(token)`; on error, return `401` `{"responseCode": "4010000", "responseMessage": "Unauthorized. [Invalid or expired access token]"}` (mirror `merchant_auth.go:35-41`).
- [X] T011 [US1] Change the `stringToSign` construction (~line 125) so that when `vendorConfig.ClientID != ""`, it uses the validated bearer token as the AccessToken component (`crypto.BuildStringToSign(method, path, token, bodyHash, timestamp)`); when `vendorConfig.ClientID == ""`, keep the existing empty-string component unchanged.
- [X] T012 [US1] Update the doc comment at `snap_auth.go:119-124` (currently claiming the AccessToken component is "always the empty string") to describe the new conditional behavior based on `vendorConfig.ClientID`.
- [X] T013 [US1] Update `scripts/vendor-inquiry-va.sh` and `scripts/vendor-payment-va.sh` to obtain a token via the existing `/access-token/b2b` call pattern already used elsewhere in the scripts directory, send `Authorization: Bearer <token>`, and build `STRING_TO_SIGN` as `POST:${ENDPOINT}:${ACCESS_TOKEN}:${BODY_HASH}:${TIMESTAMP}` instead of the current `POST:${ENDPOINT}::${BODY_HASH}:${TIMESTAMP}`.
- [X] T014 [US1] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v` and confirm all T006-T008 tests now pass.

**Checkpoint**: User Story 1 fully functional — migrated vendors can authenticate with a bound access token; legacy vendors unaffected.

---

## Phase 4: User Story 2 - Vendor requests are rejected on token/signature mismatch (Priority: P2)

**Goal**: A migrated vendor's request is rejected if the token used to compute the signature doesn't match the token presented, if the token is expired/invalid, or if the token was issued to a different vendor's `client_id`.

**Independent Test**: Per quickstart.md Scenarios 3 and 4 — swap tokens after signing and confirm `401`; present a valid token from a different vendor's `client_id` and confirm `401`.

### Tests for User Story 2 ⚠️

- [X] T015 [P] [US2] Add test case: vendor config with `ClientID` set, `stringToSign` computed with token A but `Authorization: Bearer <tokenB>` sent (both otherwise-valid tokens) → expect `401 invalid signature`, in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.
- [X] T016 [P] [US2] Add test case: vendor config with `ClientID` set, `Authorization` carries a valid, well-formed token whose `ClientID` claim does **not** match `vendorConfig.ClientID` (i.e., issued for a different vendor/merchant) → expect `401 invalid signature`, in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.
- [X] T017 [P] [US2] Add test case: vendor config with `ClientID` set, `Authorization` carries an expired token → expect `401` with the invalid/expired-token message (same as T010's error path), in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.

### Implementation for User Story 2

- [X] T018 [US2] In `internal/adapter/delivery/http/middleware/snap_auth.go`, immediately after `jwtIssuer.ValidateToken` succeeds (from T010), add: if `claims.ClientID != vendorConfig.ClientID`, return the same `401 invalid signature` response used for signature mismatches (`"Unauthorized. [Invalid signature]"`) — no distinct error message, per contracts/snap-auth-middleware.md step 4.
- [X] T019 [US2] Verify (no code change expected) that T011's `stringToSign` construction already causes a token-swap (T015) to fail HMAC verification naturally, since the signature was computed over token A but validation now runs with token B as the AccessToken component.
- [X] T020 [US2] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v` and confirm all T015-T017 tests now pass alongside US1's tests.

**Checkpoint**: User Stories 1 AND 2 both work independently — token/signature binding is enforced, cross-vendor token replay is blocked.

---

## Phase 5: User Story 3 - Existing vendor integrations are migrated without silent breakage (Priority: P3)

**Goal**: A legacy-format request (no `Authorization`, empty AccessToken component) sent to a **migrated** vendor config is rejected with an error clearly distinguishing "missing Authorization" from generic signature mismatch, per spec FR-007/SC-004.

**Independent Test**: Per quickstart.md Scenario 2 (same request as US1's negative test, verified here specifically for message clarity/distinctness) — send the legacy signing convention against a migrated vendor config and confirm the error text names the `Authorization` header specifically.

### Tests for User Story 3 ⚠️

- [X] T021 [P] [US3] Add test case asserting the exact response body/message distinctness: request signed with the old convention (no `Authorization`) against a migrated (`ClientID` set) vendor config must return a `responseMessage` containing "Authorization" and must NOT equal the generic `"Unauthorized. [Invalid signature]"` message, in `internal/adapter/delivery/http/middleware/snap_auth_test.go`.

### Implementation for User Story 3

- [X] T022 [US3] Confirm (no new code expected — T009's error message already satisfies this) that the missing-`Authorization` rejection path added in T009 returns a message distinct from the generic invalid-signature message; if T009's wording doesn't clearly name "Authorization", adjust the message text in `snap_auth.go` to match contracts/snap-auth-middleware.md step 2.
- [X] T023 [US3] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v` and confirm T021 passes.

**Checkpoint**: All three user stories independently functional — migration-safety error messaging confirmed distinct.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Coverage, full regression, and end-to-end validation across all stories.

- [X] T024 [P] Run `go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` and confirm overall coverage remains >= 90% per constitution Principle XI; add any missing branch-coverage test cases in `internal/adapter/delivery/http/middleware/snap_auth_test.go` if needed.
- [X] T025 [P] Run `golangci-lint run` and fix any warnings introduced by the `snap_auth.go`/`main.go` changes.
- [ ] T026 Execute `specs/011-vendor-access-token-signature/quickstart.md` Scenarios 1-5 end-to-end against a local `docker compose up -d` stack and confirm actual HTTP responses match expected outcomes.
  - **Status**: Not run in this session — requires a running Postgres/Redis stack plus a provisioned vendor `client_id`+key (T001), neither of which exist yet in this environment. Behavior is verified instead via the unit tests in Phases 3-5 (T006-T023, all passing) which exercise the exact same code paths against `httptest` requests. Run this manually once T001 is done.
- [X] T027 Update `specs/009-transfer-va-auth/` and `specs/010-merchant-hmac-signature/` cross-references if either doc explicitly states vendor's AccessToken component is "always empty" (per research from this feature) so historical specs aren't left contradicting current behavior — add a note pointing to feature 011, do not rewrite their original content.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No code dependencies — can start immediately (admin provisioning task).
- **Foundational (Phase 2)**: Depends on Setup completion for a testable client_id, but T002-T005 (signature change) have no data dependency and can start in parallel with T001. BLOCKS all user stories.
- **User Stories (Phase 3-5)**: All depend on Foundational (Phase 2) completion.
  - US1 (T006-T014) has no dependency on US2/US3.
  - US2 (T015-T020) depends on US1's T009-T011 (Authorization parsing + token-bound stringToSign) being in place, since US2 tests exercise those same code paths with different inputs — implement sequentially after US1, not in parallel.
  - US3 (T021-T023) depends on US1's T009 (the missing-Authorization error path) already existing — implement after US1.
- **Polish (Phase 6)**: Depends on all desired user stories being complete.

### Within Each User Story

- Tests (T006-T008, T015-T017, T021) MUST be written and FAIL before implementation tasks in the same phase.
- Implementation tasks within a story are mostly sequential (all editing the same function in `snap_auth.go`).

### Parallel Opportunities

- T001 (admin provisioning) can run in parallel with T002-T005 (Foundational code change).
- T006, T007, T008 (US1 tests, same file but independent test cases) can be drafted in parallel by one author before running sequentially — mark [P] since they don't depend on each other's code, though they land in the same test file (coordinate merge).
- T015, T016, T017 (US2 tests) — same as above.
- T024, T025 (Polish coverage/lint checks) can run in parallel.

---

## Parallel Example: User Story 1

```bash
# Draft all three US1 test cases together (same file, independent scenarios):
Task: "Add test: migrated vendor + valid Authorization + bound signature -> 200"
Task: "Add test: migrated vendor + missing Authorization -> 401 distinct message"
Task: "Add test: legacy vendor (no ClientID) + old signing convention -> 200 unchanged"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (test vendor client_id provisioned).
2. Complete Phase 2: Foundational (jwtIssuer wired into SNAPAuthMiddleware, zero behavior change, existing tests green).
3. Complete Phase 3: User Story 1 (Authorization required + bound signature for migrated vendors; legacy vendors unaffected).
4. **STOP and VALIDATE**: Run quickstart.md Scenarios 1, 2, 5 independently.
5. Deploy/demo if ready — this alone delivers the core requested feature (spec User Story 1).

### Incremental Delivery

1. Setup + Foundational → foundation ready.
2. Add US1 → validate → deploy (MVP: token binding works for migrated vendors, legacy vendors untouched).
3. Add US2 → validate → deploy (token/signature mismatch and cross-vendor replay now rejected).
4. Add US3 → validate → deploy (migration error messaging confirmed distinct).
5. Polish → coverage/lint/full quickstart pass.

---

## Notes

- [P] tasks touch the same file (`snap_auth_test.go`) in several cases — parallel here means independently authorable/reviewable test cases, not literally concurrent edits to the same lines; coordinate merges.
- No new files are created by this feature — all changes land in `snap_auth.go`, its test file, `main.go`, and the two vendor shell scripts.
- Verify tests fail before implementing (constitution Principle III, TDD).
- Commit after each task or logical group.
- Stop at any checkpoint to validate story independently.
