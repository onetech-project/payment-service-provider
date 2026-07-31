---

description: "Task list template for feature implementation"
---

# Tasks: Enforce Signature & Token Verification on Transfer-VA Endpoints

**Input**: Design documents from `/specs/009-transfer-va-auth/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/transfer-va-auth.delta.yaml, quickstart.md

**Tests**: Required — constitution Principle III (TDD) mandates writing failing tests first for all feature implementations. All test tasks below MUST be completed and MUST fail before their corresponding implementation task is started.

**Organization**: Tasks are grouped by user story (US1-US4) per spec.md priorities. US1/US2 both live in `snap_auth.go` (vendor-side); US3 is a new, separate file (`merchant_auth.go`, merchant-side); US4 is a route-wiring + isolation-test story that depends on both being done.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

Single Go backend project (existing Clean Architecture layout):
- `internal/adapter/delivery/http/middleware/snap_auth.go` — vendor-side HMAC + timestamp enforcement (extend)
- `internal/adapter/delivery/http/middleware/snap_auth_test.go` — tests
- `internal/adapter/delivery/http/middleware/merchant_auth.go` — NEW merchant bearer-token middleware
- `internal/adapter/delivery/http/middleware/merchant_auth_test.go` — NEW tests
- `internal/infrastructure/crypto/hmac.go` — existing `HMACSigner.Verify`, `BuildStringToSign`, `HashSHA256Hex` (reference only, no changes)
- `cmd/api/main.go` — route wiring
- `scripts/e2e-dynamic-va-flow.sh` — existing e2e script (regression check only, no change expected)

---

## Phase 1: Setup

**Purpose**: Confirm the existing code this feature builds on, and the baseline test suite passes before any change.

- [X] T001 Read `internal/adapter/delivery/http/middleware/snap_auth.go` in full and confirm the exact current line numbers for the header-presence loop, the `isValidISO8601` timestamp-format check, and where a new HMAC/freshness check would need to be inserted (after existing checks, before `return next(c)`) — no code changes in this task.
- [X] T002 [P] Read `internal/infrastructure/crypto/hmac.go` and confirm `HMACSigner.Verify(stringToSign, signature string) bool` (line 54), `BuildStringToSign(method, relativeURL, accessToken, requestBodyHash, timestamp string) string` (line 58), and `HashSHA256Hex(data string) string` (line 63) are usable as-is to reconstruct the client-side signing convention documented in `scripts/vendor-inquiry-va.sh:108-112` — no code changes.
- [X] T003 [P] Read `internal/infrastructure/crypto/jwt_issuer.go` and `internal/domain/token.go` and confirm `domain.JWTIssuer.ValidateToken(tokenString string) (*domain.TokenClaims, error)` (interface at `token.go:33-36`) is the mockable seam to use for the new merchant middleware — no code changes.
- [X] T004 Confirm baseline: `go test ./internal/adapter/delivery/http/middleware/... ./internal/infrastructure/crypto/... -v` and `go build ./...` both succeed before any change.

**Checkpoint**: Baseline confirmed; safe to start Foundational phase.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Nothing is shared/blocking across US1-US4 beyond what already exists (`HMACSigner`, `JWTIssuer` are already fully implemented and require no changes) — this phase is intentionally minimal.

**⚠️ CRITICAL**: No user story implementation task may start until this phase is complete.

- [X] T005 Confirm `cmd/api/main.go`'s current route registration block (`main.go:386-410`) — note the exact line numbers for `vendorGroup := transferVAGroup.Group("")` (vendor sub-group) and the three merchant route registrations (`create-va`/`list`/`delete-va`) directly on `transferVAGroup`, and confirm the `jwtIssuer` variable (declared at `main.go:203`) is in scope at that point in `main()` — needed by US3/US4's route-wiring task. No code changes in this task.

**Checkpoint**: Foundation confirmed — user story implementation can now begin.

---

## Phase 3: User Story 1 - Vendor/bank requests are rejected when their signature doesn't match (Priority: P1) 🎯 MVP

**Goal**: `SNAPAuthMiddleware` recomputes the HMAC-SHA512 signature from the raw request body + timestamp using the vendor/channel's `ClientSecret`, and rejects (401) any request whose `X-SIGNATURE` doesn't match — while still allowing a correctly-signed request through unchanged.

**Independent Test**: Send a request with a correctly-computed signature (using the real shared secret) and confirm it passes through to the next handler; send the same request with an incorrect signature and confirm it is rejected with 401 before `next(c)` is called.

### Tests for User Story 1 ⚠️ (write first, confirm they FAIL)

- [X] T006 [P] [US1] Add `TestSNAPAuthMiddleware_ValidSignature_PassesThrough` in `internal/adapter/delivery/http/middleware/snap_auth_test.go`: build a `config.VendorConfig{ClientSecret: "test-secret"}`, construct a request with a JSON body, compute the correct `X-SIGNATURE` using `crypto.NewHMACSigner("test-secret", "HMAC-SHA512").Sign(crypto.BuildStringToSign("POST", ..., ..., crypto.HashSHA256Hex(body), timestamp))` (mirroring `scripts/vendor-inquiry-va.sh:108-112`), set a fresh `X-TIMESTAMP` (now, RFC3339), and assert the wrapped handler is invoked (`http.StatusOK`, e.g. via a handler that sets a flag or returns 200).
- [X] T007 [P] [US1] Add `TestSNAPAuthMiddleware_InvalidSignature_Rejected` in `internal/adapter/delivery/http/middleware/snap_auth_test.go`: same setup as T006 but with `X-SIGNATURE` set to an arbitrary wrong value → assert `http.StatusUnauthorized` and that the wrapped handler is NOT invoked.
- [X] T008 [P] [US1] Add `TestSNAPAuthMiddleware_EmptySignatureValue_Rejected` in `internal/adapter/delivery/http/middleware/snap_auth_test.go`: `X-SIGNATURE` header present but set to `""` → assert `http.StatusUnauthorized` (covers spec Acceptance Scenario 3 — already partially covered by the existing `TestSNAPAuthMiddleware_MissingHeaders` for a fully-absent header; this test specifically covers a present-but-empty value reaching the new HMAC check rather than the header-presence loop).

Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware_ValidSignature -v` and `-run TestSNAPAuthMiddleware_InvalidSignature` now — T006 and T007 MUST fail (today's middleware has no HMAC check at all, so T007's bad signature would currently pass through). Confirm they fail for the right reason (wrong status code / handler invoked), not a compile error.

### Implementation for User Story 1

- [X] T009 [US1] In `internal/adapter/delivery/http/middleware/snap_auth.go`, after the existing timestamp-format check (`isValidISO8601`) and before `return next(c)`: read the raw request body via `io.ReadAll(c.Request().Body)`, then immediately restore it with `c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))` (mirroring the exact pattern in `internal/adapter/delivery/http/middleware/idempotency.go:71-73`) so the downstream handler can still bind it. Compute `bodyHash := crypto.HashSHA256Hex(string(bodyBytes))`, build `stringToSign := crypto.BuildStringToSign(c.Request().Method, c.Request().URL.Path, c.Request().Header.Get("Authorization") /* strip "Bearer " prefix if present, else empty */, bodyHash, timestamp)`, and call `crypto.NewHMACSigner(vendorConfig.ClientSecret, vendorConfig.SignatureAlgorithm).Verify(stringToSign, c.Request().Header.Get("X-SIGNATURE"))`; on `false`, return `401` with `{"responseCode": "4010000", "responseMessage": "Unauthorized. [Invalid signature]"}`. Depends on T001/T002.
- [X] T010 [US1] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v` and confirm T006, T007, T008 now pass and no previously-passing test in the file regressed. Depends on T009.

**Checkpoint**: User Story 1 fully functional and independently testable — vendor requests with an incorrect/missing signature are now rejected; correctly-signed requests are unaffected.

---

## Phase 4: User Story 2 - Vendor/bank requests are rejected when their timestamp is stale or too far in the future (Priority: P2)

**Goal**: `SNAPAuthMiddleware` rejects a request whose `X-TIMESTAMP` is more than 5 minutes old or more than 5 minutes in the future, independent of signature validity.

**Independent Test**: Send a correctly-signed request with a timestamp 1 hour in the past (and separately, 1 hour in the future) and confirm both are rejected with 401; send one with a timestamp within the window and confirm it still passes through.

### Tests for User Story 2 ⚠️ (write first, confirm they FAIL)

- [X] T011 [P] [US2] Add `TestSNAPAuthMiddleware_StaleTimestamp_Rejected` in `internal/adapter/delivery/http/middleware/snap_auth_test.go`: same correctly-signed setup as T006 but with `X-TIMESTAMP` set to `time.Now().Add(-1 * time.Hour)` (formatted RFC3339, with a signature computed against that same old timestamp so only staleness — not signature mismatch — is under test) → assert `http.StatusUnauthorized`.
- [X] T012 [P] [US2] Add `TestSNAPAuthMiddleware_FutureTimestamp_Rejected` in `internal/adapter/delivery/http/middleware/snap_auth_test.go`: same as T011 but `X-TIMESTAMP` set to `time.Now().Add(1 * time.Hour)` → assert `http.StatusUnauthorized`.
- [X] T013 [P] [US2] Add `TestSNAPAuthMiddleware_TimestampWithinWindow_PassesThrough` in `internal/adapter/delivery/http/middleware/snap_auth_test.go`: `X-TIMESTAMP` set to `time.Now().Add(4 * time.Minute)` (just inside the ±5 minute window) with a matching correct signature → assert the wrapped handler is invoked (not rejected for staleness).

Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware_StaleTimestamp -v` and `-run TestSNAPAuthMiddleware_FutureTimestamp -v` now — both MUST fail (no freshness check exists yet).

### Implementation for User Story 2

- [X] T014 [US2] In `internal/adapter/delivery/http/middleware/snap_auth.go`, immediately after the existing `isValidISO8601` check (before the HMAC check added in T009), parse the timestamp via `time.Parse(time.RFC3339, timestamp)` and reject (401, `{"responseCode": "4010000", "responseMessage": "Unauthorized. [Timestamp skew exceeds 5 minutes]"}`) if `time.Since(parsedTime) > 5*time.Minute || time.Until(parsedTime) > 5*time.Minute` — mirroring the exact tolerance and pattern already used in `internal/usecase/token_usecase.go:39-47`. Depends on T009 (shares the same function; sequenced after to avoid two developers editing the same insertion point simultaneously).
- [X] T015 [US2] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v` (full file) and confirm T011, T012, T013 now pass and US1's tests (T006-T008) still pass. Depends on T014.

**Checkpoint**: User Stories 1 and 2 both fully functional — vendor-side enforcement (signature + freshness) is complete and unconditional, per spec FR-001–FR-004.

---

## Phase 5: User Story 3 - Merchant requests require a valid access token (Priority: P1)

**Goal**: A new middleware rejects (401) any create-VA/list-VA/delete-VA request that lacks a valid, unexpired `Authorization: Bearer <accessToken>`, and lets a request with a valid token through unchanged.

**Independent Test**: Call the middleware directly (unit test, no real routing needed) with no `Authorization` header, a malformed one, an expired-but-well-formed JWT, and a valid JWT — confirm the first three are rejected with 401 and the fourth passes through.

### Tests for User Story 3 ⚠️ (write first, confirm they FAIL)

- [X] T016 [P] [US3] Create `internal/adapter/delivery/http/middleware/merchant_auth_test.go`. Add a `MockJWTIssuer` implementing `domain.JWTIssuer` (via `testify/mock`, mirroring the `MockVATypeRuleProvider` pattern in `internal/usecase/merchant_va_usecase_test.go`) with a `ValidateToken(tokenString string) (*domain.TokenClaims, error)` method. Add `TestMerchantAuthMiddleware_MissingAuthorizationHeader_Rejected`: no `Authorization` header at all → assert `http.StatusUnauthorized`, and assert the mock's `ValidateToken` is never called (`mockIssuer.AssertNotCalled(t, "ValidateToken", mock.Anything)`).
- [X] T017 [P] [US3] In the same new test file, add `TestMerchantAuthMiddleware_MalformedAuthorizationHeader_Rejected`: `Authorization` header set to a value that doesn't start with `"Bearer "` (e.g. `"Basic abc123"` or just `"sometoken"` with no scheme) → assert `http.StatusUnauthorized`, `ValidateToken` never called.
- [X] T018 [P] [US3] In the same new test file, add `TestMerchantAuthMiddleware_InvalidToken_Rejected`: `Authorization: Bearer bad-token`, mock `ValidateToken` returns an error (simulating both malformed-signature and expired-token cases, since `jwt.Parse`'s built-in `exp` validation already surfaces both as one error per the existing `ValidateToken` implementation) → assert `http.StatusUnauthorized`.
- [X] T019 [P] [US3] In the same new test file, add `TestMerchantAuthMiddleware_ValidToken_PassesThrough`: `Authorization: Bearer good-token`, mock `ValidateToken` returns `(&domain.TokenClaims{ClientID: "test-client"}, nil)` → assert the wrapped handler is invoked (`http.StatusOK`).

Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware -v` now — all four MUST fail to compile/run since `merchant_auth.go` and `MerchantAuthMiddleware` don't exist yet (expected — this is the TDD red state before T020).

### Implementation for User Story 3

- [X] T020 [US3] Create `internal/adapter/delivery/http/middleware/merchant_auth.go` with `func MerchantAuthMiddleware(jwtIssuer domain.JWTIssuer) echo.MiddlewareFunc`, mirroring `SNAPAuthMiddleware`'s structure (`snap_auth.go:13-14`): extract the `Authorization` header, reject 401 (`{"responseCode": "4010000", "responseMessage": "Unauthorized. [Missing or invalid Authorization header]"}`) if it's empty or doesn't start with `"Bearer "`; otherwise strip the `"Bearer "` prefix and call `jwtIssuer.ValidateToken(token)`, rejecting 401 (`{"responseCode": "4010000", "responseMessage": "Unauthorized. [Invalid or expired access token]"}`) on error; otherwise call `next(c)`. Depends on T003, T016-T019.
- [X] T021 [US3] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware -v` and confirm T016-T019 now pass. Depends on T020.

**Checkpoint**: User Story 3 fully functional and independently testable (as a standalone middleware unit) — merchant bearer-token validation logic is complete, though not yet wired into any route (that's US4).

---

## Phase 6: User Story 4 - Vendor and merchant authentication mechanisms coexist without interference (Priority: P2)

**Goal**: Wire `MerchantAuthMiddleware` onto only the merchant route group (`create-va`/`list`/`delete-va`) in `cmd/api/main.go`, leaving the existing vendor route group (`inquiry`/`payment`/`status`, already wrapped in `SNAPAuthMiddleware`) untouched — and confirm neither mechanism leaks into the other's endpoints.

**Independent Test**: Run the full e2e flow (which already exercises correctly-signed vendor calls and a real merchant `accessToken`) and confirm nothing regresses; additionally confirm a vendor call succeeds with no `Authorization` header, and a merchant call succeeds with no `X-SIGNATURE` header.

### Tests for User Story 4 ⚠️ (write first where practical — this story is primarily route-wiring + integration verification)

- [X] T022 [US4] Manually verify (no new Go unit test needed — this is route-topology wiring, not new logic) by reading `cmd/api/main.go:386-410` after T023 lands: confirm `vendorGroup` (wrapping `inquiry`/`payment`/`status`) has ONLY `SNAPAuthMiddleware(vc)` applied, and the new merchant sub-group (wrapping `create-va`/`list`/`delete-va`) has ONLY `MerchantAuthMiddleware(jwtIssuer)` applied — neither group has both. Record this as a pre-implementation checklist for T023, not a runnable test.

### Implementation for User Story 4

- [X] T023 [US4] In `cmd/api/main.go`, replace the three direct registrations `transferVAGroup.POST("/create-va", merchantVAHandler.CreateVA)` / `.POST("/list", merchantVAHandler.ListVA)` / `.DELETE("/delete-va", merchantVAHandler.DeleteVA)` (currently at `main.go:408-410`) with a new sub-group: `merchantGroup := transferVAGroup.Group(""); merchantGroup.Use(customMiddleware.MerchantAuthMiddleware(jwtIssuer)); merchantGroup.POST("/create-va", merchantVAHandler.CreateVA); merchantGroup.POST("/list", merchantVAHandler.ListVA); merchantGroup.DELETE("/delete-va", merchantVAHandler.DeleteVA)` — mirroring the existing `vendorGroup := transferVAGroup.Group("")` pattern immediately above it. Depends on T020 (middleware must exist), T005 (line numbers confirmed).
- [X] T024 [US4] Run `go build ./...` to confirm `main.go` compiles with the new route wiring. Depends on T023.
- [X] T025 [US4] Rebuild and restart the local Docker stack (`docker compose build app && docker compose up -d app`), then run quickstart.md Scenario 7 (isolation: correctly-signed vendor call with no `Authorization` header succeeds; merchant call with valid token but no `X-SIGNATURE` succeeds). Depends on T024.
- [X] T026 [US4] Run `./scripts/e2e-dynamic-va-flow.sh -f .env.bca.va -u http://localhost:8080` (quickstart.md Scenario 8) and confirm all checks still pass (19/19 as of feature 008) — the script already sends a correctly-computed vendor signature and a real merchant `accessToken`, so none of the new checks should change its outcome. Depends on T025.

**Checkpoint**: All four user stories independently functional; end-to-end flow confirms no regressions and the two auth mechanisms are cleanly isolated to their respective route groups.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup affecting the whole feature.

- [X] T027 [P] Run `go test -race -coverprofile=coverage.out ./internal/adapter/delivery/http/middleware/... && go tool cover -func=coverage.out | grep -E "snap_auth|merchant_auth"` and confirm both `snap_auth.go` and `merchant_auth.go` meet the constitution's ≥90% coverage target (Principle XI), including every new branch added in T009/T014/T020.
- [X] T028 [P] Run `go vet ./internal/adapter/delivery/http/middleware/... ./cmd/api/...` (substituting for `golangci-lint`, which has a pre-existing v1/v2 config mismatch in this environment per feature 008's notes) and fix any warnings introduced by this feature's changes.
- [X] T029 Execute quickstart.md Scenarios 1-6 manually against the local running instance (bad/good vendor signature, stale/future timestamp, fail-closed on missing secret, merchant token missing/valid/invalid) to confirm the documented curl-level behavior matches the implemented unit-test behavior, then re-run Scenario 8 (full e2e) one final time as a closing sign-off.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup — confirms route/wiring anchors needed by US4; does not block US1-US3's pure-middleware-logic work, but MUST complete before US4's route-wiring task (T023).
- **User Story 1 (Phase 3)**: Depends on Foundational (nominally; in practice only needs Setup). No dependency on US2/US3.
- **User Story 2 (Phase 4)**: Depends on US1's T009 (both checks live in the same function in `snap_auth.go`; T014 is sequenced after T009 to avoid two people editing the same insertion point at once) — otherwise independent in spirit (spec explicitly separates them as P1/P2).
- **User Story 3 (Phase 5)**: Fully independent of US1/US2 — different file (`merchant_auth.go`), no shared state. Can be developed in parallel with US1/US2 by a different contributor.
- **User Story 4 (Phase 6)**: Depends on US1+US2 (T014) being complete (vendor group already correctly configured) AND US3 (T020) being complete (middleware must exist to wire in) — this is the integration/wiring story that ties the other three together.
- **Polish (Phase 7)**: Depends on all four user stories being complete.

### Within Each User Story

- Tests written and confirmed failing before implementation (T006-T008 before T009; T011-T013 before T014; T016-T019 before T020).
- Vendor-side (`snap_auth.go`) implementation order follows the function's logical flow: header presence → timestamp format → timestamp freshness (US2) → HMAC verification (US1, though implemented first per MVP priority — see note in T009/T014 on insertion order).

### Parallel Opportunities

- T002 and T003 (Setup reading tasks) can run in parallel with T001.
- All test-writing tasks within a story are marked `[P]` (T006-T008; T011-T013; T016-T019) — independent test functions, safe to write in any order.
- **US3 can be developed fully in parallel with US1/US2** — different file, no shared insertion point, only converges at US4's wiring step.
- T027 and T028 (Polish) can run in parallel.

---

## Parallel Example: User Story 1 and User Story 3 developed simultaneously

```bash
# Contributor A — User Story 1 (vendor signature), in internal/adapter/delivery/http/middleware/snap_auth.go + snap_auth_test.go:
Task: "Add TestSNAPAuthMiddleware_ValidSignature_PassesThrough"
Task: "Add TestSNAPAuthMiddleware_InvalidSignature_Rejected"
Task: "Add TestSNAPAuthMiddleware_EmptySignatureValue_Rejected"
Task: "Implement HMAC verification in snap_auth.go"

# Contributor B — User Story 3 (merchant bearer token), in internal/adapter/delivery/http/middleware/merchant_auth.go + merchant_auth_test.go (new files, zero overlap):
Task: "Create merchant_auth_test.go with MockJWTIssuer and four test cases"
Task: "Create merchant_auth.go implementing MerchantAuthMiddleware"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1 — vendor signature verification.
4. **STOP and VALIDATE**: run the full `snap_auth_test.go` suite; this alone already closes the highest-severity gap (any caller with any signature was previously accepted).
5. Deploy/demo if ready.

### Incremental Delivery

1. Setup + Foundational → foundation ready.
2. User Story 1 → vendor signature enforcement → test independently → deploy.
3. User Story 2 → vendor timestamp freshness → test independently → deploy (can follow immediately after US1, same file).
4. User Story 3 → merchant bearer-token middleware (developed in parallel with US1/US2 if staffed) → test independently — not yet routed, no user-facing effect until US4.
5. User Story 4 → wire US3's middleware onto merchant routes, confirm isolation from vendor routes → test independently → deploy. This is the step where the merchant-side fix actually takes effect.
6. Polish → coverage/lint/quickstart sign-off.

### Parallel Team Strategy

With two developers: Developer A takes US1 → US2 (sequential, same file); Developer B takes US3 (fully independent, new file) in parallel. Both converge at US4, which only one person needs to do (small wiring change) once both are ready.

---

## Notes

- [P] tasks = different test functions or independent files; US1/US2 implementation tasks are NOT parallelizable with each other (same file, same function) but ARE parallelizable with US3 (different file).
- Commit after each task or logical group, per repository convention.
- Verify tests fail for the right reason before implementing (wrong status code or handler-invoked-when-it-shouldn't-be, not a compile error).
- There is intentionally no per-vendor/channel enforcement toggle and no merchant-side opt-out — both checks are unconditional from the moment this ships, per spec Assumptions. Do not add a bypass/feature-flag "to be safe"; that was explicitly rejected during planning. **Amended (2026-07-31)**: a single global (not per-vendor/channel), `APP_ENV`-derived exception was later added for the timestamp-freshness sub-check only (dev/uat, never prod) — see `research.md` Decision 4 Amendment for the rationale on why this doesn't reopen the rejected per-vendor toggle. Signature verification is still never bypassable.
- The vendor-side fail-closed behavior (reject all requests when `ClientSecret` is empty, per spec FR-004) falls out naturally from T009's `Verify` call against an empty secret (`Sign("")` against any real signature will never match) — no separate explicit empty-secret branch is required, but confirm this with a quick manual check during T029 if time allows.
