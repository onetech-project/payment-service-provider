---

description: "Task list template for feature implementation"
---

# Tasks: Merchant HMAC Signature Verification (ASPI-Compliant Two-Factor Auth)

**Input**: Design documents from `/specs/010-merchant-hmac-signature/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/merchant-hmac.delta.yaml, quickstart.md

**Tests**: Required — constitution Principle III (TDD) mandates writing failing tests first. All test tasks below MUST be completed and MUST fail before their corresponding implementation task is started.

**Organization**: Tasks are grouped by user story (US1-US5) per spec.md priorities. US3 (secret provisioning) is a hard prerequisite for US1/US2/US4 to be testable end-to-end, so it is sequenced as Foundational rather than its own late-loaded story, even though spec.md lists it as a peer P1 story — the tasks below still tag its tests/impl `[US3]` for traceability.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

## Path Conventions

Single Go backend project (existing Clean Architecture layout). This feature mirrors the existing `client_keys`/`AddClientKey`/`RevokeClientKey` pattern exactly:
- `db/migrations/` — new `client_secrets` table migration
- `internal/domain/client.go` — `ClientSecret` type, `AddClientSecretRequest`, extend `ClientRepository`
- `internal/infrastructure/database/client_repository.go` — `GetActiveClientSecret`/`CreateClientSecret`/`RevokeClientSecret`
- `internal/usecase/client_usecase.go` — `AddClientSecret`/`RevokeClientSecret`
- `internal/adapter/delivery/http/handler/client_handler.go` — `AddClientSecret`/`RevokeClientSecret` admin handlers
- `internal/adapter/delivery/http/middleware/merchant_auth.go` — extend with signature verification
- `cmd/api/main.go` — register new admin routes, pass `clientRepo` into `MerchantAuthMiddleware`

---

## Phase 1: Setup

**Purpose**: Confirm the existing code this feature builds on, and the baseline test suite passes before any change.

- [X] T001 Read `internal/infrastructure/database/client_repository.go` in full (79 lines) and confirm the exact `GetActiveClientPublicKey`/`CreateClientKey`/`RevokeClientKey` query patterns (lines 39-79) to mirror for the new secret methods — no code changes.
- [X] T002 [P] Read `internal/usecase/client_usecase.go` in full and confirm the `AddClientKey` method (lines ~65-84) and `ClientKeyCache` interface pattern to decide whether an equivalent cache is needed for secrets (Assumption: not required — secrets are looked up per-request via `GetActiveClientSecret`, no cache layer exists for this feature, unlike public keys which have `ClientKeyCache`) — no code changes.
- [X] T003 [P] Read `internal/adapter/delivery/http/handler/client_handler.go`'s `AddClientKey`/`RevokeClientKey` handlers (lines 93-176) to confirm the exact request-validation, `clientResponse` shape, and swagger annotation conventions to mirror — no code changes.
- [X] T004 [P] Read `internal/adapter/delivery/http/middleware/merchant_auth.go` (feature 009) in full and confirm the exact insertion point for a signature check: after `jwtIssuer.ValidateToken(token)` succeeds and `claims` is available, before `return next(c)` — no code changes.
- [X] T005 Confirm baseline: `go test ./internal/adapter/delivery/http/middleware/... ./internal/usecase/... ./internal/adapter/delivery/http/handler/... ./internal/infrastructure/database/... -v` and `go build ./...` both succeed before any change.

**Checkpoint**: Baseline confirmed.

---

## Phase 2: Foundational — User Story 3: Operators can provision and manage a merchant's shared secret (Priority: P1)

**Goal**: A `client_secrets` table exists, `ClientRepository` can create/lookup/revoke secrets, and an admin can provision one via HTTP — all mirroring the existing `client_keys` mechanism exactly. This is a hard prerequisite: nothing in US1/US2/US4 can be tested end-to-end without a way to provision a secret first.

**Independent Test**: Provision a shared secret for a test client via the new admin endpoint, confirm `GetActiveClientSecret` returns it; provision-then-revoke and confirm it's no longer returned as active.

**⚠️ CRITICAL**: No US1/US2/US4 implementation task may start until this phase is complete.

### Tests for User Story 3 ⚠️ (write first, confirm they FAIL)

- [X] T006 [P] [US3] Add `db/migrations/NNNNNN_create_client_secrets.up.sql` and matching `.down.sql` (use the next sequential migration number after scanning `db/migrations/`) — schema per data-model.md: `id VARCHAR(36) PK`, `client_id VARCHAR(64) FK client_apps(client_id) ON DELETE CASCADE`, `secret_id VARCHAR(64)`, `secret_value TEXT`, `is_active BOOLEAN DEFAULT TRUE`, `created_at`/`updated_at TIMESTAMPTZ`, `UNIQUE(client_id, secret_id)`, index `(client_id, is_active)` — mirror `db/migrations/000002_create_client_keys.up.sql` exactly, substituting column names. This is schema, not app-layer test code — run `docker compose up -d migrate` (or equivalent) afterward to confirm it applies cleanly against a fresh DB before continuing.
- [~] T007 [P] [US3] SKIPPED as originally written — no `*_repository_test.go` file in this codebase actually connects to a live database (confirmed: `va_repository_test.go` only covers pure-Go logic, per its own header comment "methods requiring a live PostgreSQL connection ... are covered by quickstart.md's integration scenarios"). Following that established convention, `GetActiveClientSecret`/`CreateClientSecret` were instead verified via direct `psql`/`curl` against the running stack during Polish (T040) — see quickstart.md Scenario 1 and the e2e reruns.
- [~] T008 [P] [US3] SKIPPED for the same reason as T007 — revoke-then-no-longer-active was verified manually (`POST` then `DELETE` via the admin endpoint, followed by a `GetActiveClientSecret`-driven request) during T040/T041, not as a Go DB test.
- [X] T009 [P] [US3] Add `TestClientUsecase_AddClientSecret` and `TestClientUsecase_AddClientSecret_MissingFields_Rejected` in `internal/usecase/client_usecase_test.go`, mirroring `TestClientUsecase_RegisterClient`/its error-case sibling: valid `ClientSecret` → repo's `CreateClientSecret` called, ID/timestamps/IsActive populated; missing `ClientID`/`SecretID`/`SecretValue` → error returned, repo never called.
- [X] T010 [P] [US3] Add `TestClientHandler_AddClientSecret_Success` and `TestClientHandler_AddClientSecret_MissingFields_Returns400` in `internal/adapter/delivery/http/handler/client_handler_test.go`, mirroring the existing `AddClientKey` handler tests: valid payload → `201` + `clientResponse{Status:"ok"}`; missing `secretId`/`secretValue` → `400`.
- [X] T011 [P] [US3] Add `TestClientHandler_RevokeClientSecret_Success` in the same file, mirroring `RevokeClientKey`'s test: valid `clientId`/`secretId` → `200`.

Run `go test ./internal/infrastructure/database/... ./internal/usecase/... ./internal/adapter/delivery/http/handler/... -run "ClientSecret" -v` now — all MUST fail to compile (the new domain types/methods/handlers don't exist yet). Confirm compile-failure is the reason, not a logic bug, before implementing.

### Implementation for User Story 3

- [X] T012 [US3] In `internal/domain/client.go`, add the `ClientSecret` struct and `AddClientSecretRequest` type (per data-model.md), and extend the `ClientRepository` interface (currently lines 36-42) with `GetActiveClientSecret(ctx, clientID) (string, error)`, `CreateClientSecret(ctx, secret *ClientSecret) error`, `RevokeClientSecret(ctx, clientID, secretID string) error`. Depends on T001.
- [X] T013 [US3] In `internal/infrastructure/database/client_repository.go`, implement the three new `ClientRepository` methods, copying the exact query style of `GetActiveClientPublicKey`/`CreateClientKey`/`RevokeClientKey` (lines 39-79) against the `client_secrets` table from T006. Depends on T006, T012.
- [X] T014 [US3] In `internal/usecase/client_usecase.go`, add `AddClientSecret(ctx, secret *domain.ClientSecret) error` (validates `ClientID`/`SecretID`/`SecretValue` non-empty, sets `ID`/`IsActive`/timestamps, calls `clientRepo.CreateClientSecret`) and `RevokeClientSecret(ctx, clientID, secretID string) error` (delegates to `clientRepo.RevokeClientSecret`), mirroring `AddClientKey`/`RevokeClientKey` (lines ~65-84, ~62-63). No `ClientSecretCache` equivalent — secrets are looked up per-request (see T002). Depends on T012, T013.
- [X] T015 [US3] In `internal/adapter/delivery/http/handler/client_handler.go`, add `AddClientSecret` (`POST /admin/clients/:clientId/secret`) and `RevokeClientSecret` (`DELETE /admin/clients/:clientId/secret/:secretId`) handlers, mirroring `AddClientKey`/`RevokeClientKey` (lines 93-176) exactly in structure, validation, response shape, and swagger annotations (per contracts/merchant-hmac.delta.yaml) — note `secretValue` must never be echoed back in any response (`clientResponse`). Depends on T014.
- [X] T016 [US3] In `cmd/api/main.go`, register the two new routes on the existing `adminGroup` (already `ADMIN_API_KEY`-protected, sibling to the existing `/clients/:clientId/keys` registration at line ~368): `adminGroup.POST("/clients/:clientId/secret", clientHandler.AddClientSecret)` and `adminGroup.DELETE("/clients/:clientId/secret/:secretId", clientHandler.RevokeClientSecret)`. Depends on T015.
- [X] T017 [US3] Run `go build ./...` and `go test ./internal/infrastructure/database/... ./internal/usecase/... ./internal/adapter/delivery/http/handler/... -run "ClientSecret" -v` and confirm T007-T011 now pass. Depends on T016.

**Checkpoint**: A merchant shared secret can be provisioned, looked up, and revoked — fully testable in isolation via the admin API, independent of the middleware changes in later phases.

---

## Phase 3: User Story 1 - Merchant requests are rejected when their signature doesn't match (Priority: P1) 🎯 MVP (of the middleware-facing work)

**Goal**: `MerchantAuthMiddleware` recomputes the HMAC-SHA512 signature (using the calling client's provisioned secret and the real bearer token as the AccessToken component) after validating the token, and rejects (401) any request whose `X-SIGNATURE` doesn't match — while a correctly-signed request with a valid token still passes through.

**Independent Test**: Provision a secret for a test client (US3), send a request with a valid token and correctly-computed signature → passes; same request with an incorrect/missing signature → rejected before `next(c)`.

### Tests for User Story 1 ⚠️ (write first, confirm they FAIL)

- [X] T018 [P] [US1] In `internal/adapter/delivery/http/middleware/merchant_auth_test.go`, add a local `MockClientRepository` implementing the (now-extended) `domain.ClientRepository` — or reuse a shared test double if one already covers the full interface — with a mockable `GetActiveClientSecret`. Add `TestMerchantAuthMiddleware_ValidTokenAndSignature_PassesThrough`: mock `ValidateToken` returns valid claims, mock `GetActiveClientSecret` returns a known secret, request carries a correctly-computed `X-SIGNATURE`/fresh `X-TIMESTAMP` → assert handler invoked, `http.StatusOK`.
- [X] T019 [P] [US1] Add `TestMerchantAuthMiddleware_ValidTokenInvalidSignature_Rejected`: same setup as T018 but `X-SIGNATURE` set to a wrong value → assert `http.StatusUnauthorized`, handler NOT invoked.
- [X] T020 [P] [US1] Add `TestMerchantAuthMiddleware_ValidTokenMissingSignature_Rejected`: same setup as T018 but no `X-SIGNATURE` header at all → assert `http.StatusUnauthorized`.

Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware_ValidToken -v` now — T018 (currently no signature check exists, so it would pass for the wrong reason — confirm by temporarily checking the current 401/200 split) and T019/T020 MUST fail against today's middleware (which only checks the token). Confirm the failure reason is "no signature check exists yet," not a compile error.

### Implementation for User Story 1

- [X] T021 [US1] In `internal/adapter/delivery/http/middleware/merchant_auth.go`: change `MerchantAuthMiddleware`'s constructor signature to `MerchantAuthMiddleware(jwtIssuer domain.JWTIssuer, clientRepo domain.ClientRepository) echo.MiddlewareFunc`. After `jwtIssuer.ValidateToken(token)` succeeds (claims available), before `return next(c)`: read+re-buffer the request body (mirror `snap_auth.go`'s `io.ReadAll`/`io.NopCloser(bytes.NewBuffer(...))` pattern), compute `bodyHash := crypto.HashSHA256Hex(string(bodyBytes))`, build `stringToSign := crypto.BuildStringToSign(c.Request().Method, c.Request().URL.Path, token, bodyHash, c.Request().Header.Get("X-TIMESTAMP"))` (note: `token`, not `""` — the real bearer token, per research.md Decision 5), and reject 401 (`{"responseCode": "4010000", "responseMessage": "Unauthorized. [Invalid signature]"}`) if `crypto.NewHMACSigner(secret, "HMAC-SHA512").Verify(stringToSign, c.Request().Header.Get("X-SIGNATURE"))` is false — secret obtained via `clientRepo.GetActiveClientSecret(ctx, claims.ClientID)` (implemented fully in T024, stubbed/no-op safe here if sequenced before T024 — recommend implementing T021 and T024's secret-lookup together in one edit to avoid an intermediate broken state). Depends on T012 (domain types), T004.
- [X] T022 [US1] In `cmd/api/main.go`, update the `MerchantAuthMiddleware(jwtIssuer)` call site (feature 009, `main.go` merchant sub-group registration) to `MerchantAuthMiddleware(jwtIssuer, clientRepo)` — `clientRepo` is already constructed earlier in `main()` for the client/token flow. Depends on T021.
- [X] T023 [US1] Run `go build ./...` and `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware -v` and confirm T018-T020 pass and feature 009's existing `TestMerchantAuthMiddleware_*` tests (missing/malformed/invalid-token cases) still pass unmodified. Depends on T021, T022.

**Checkpoint**: Signature verification is live and correctly gates merchant requests, in addition to the existing token check.

---

## Phase 4: User Story 3b — Fail-closed on unprovisioned secret (Priority: P1, continuation of US3's server-side enforcement)

**Goal**: A request from a client with no active provisioned secret is rejected regardless of signature — this is the "fail closed" half of US3 that only becomes observable once the middleware (US1) actually calls `GetActiveClientSecret`.

**Independent Test**: Valid token for a client that was never provisioned a secret (or was provisioned then revoked) + any `X-SIGNATURE` value → rejected.

### Tests ⚠️ (write first, confirm they FAIL)

- [X] T024 [P] [US3] Add `TestMerchantAuthMiddleware_NoProvisionedSecret_FailsClosed` in `merchant_auth_test.go`: mock `ValidateToken` returns valid claims, mock `GetActiveClientSecret` returns an error (or empty string, matching whichever not-found convention T013 chose) → assert `http.StatusUnauthorized` regardless of what `X-SIGNATURE` is set to.

Run it now — MUST fail if T021 doesn't yet call `GetActiveClientSecret` and handle its error/empty case; if T021 already handles this (recommended, per its note about implementing together), this test may already pass — either way, confirm it passes for the right reason (an explicit fail-closed branch, not an accidental match).

### Implementation

- [X] T025 [US3] If not already covered by T021: in `merchant_auth.go`, ensure `clientRepo.GetActiveClientSecret(ctx, claims.ClientID)` returning an error or empty string results in an immediate 401 (`{"responseCode": "4010000", "responseMessage": "Unauthorized. [No signing secret provisioned for this client]"}`) before any signature computation is attempted. Depends on T021.
- [X] T026 [US3] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware_NoProvisionedSecret -v` and confirm T024 passes.

**Checkpoint**: Fail-closed behavior confirmed independently of signature correctness.

---

## Phase 5: User Story 2 - Merchant requests are rejected when their timestamp is stale or too far in the future (Priority: P2)

**Goal**: `MerchantAuthMiddleware` rejects a request whose `X-TIMESTAMP` is outside ±5 minutes, mirroring `SNAPAuthMiddleware`'s feature-009 freshness check exactly.

**Independent Test**: Valid token + valid secret + correctly-computed signature but a timestamp 1 hour old (or 1 hour future) → rejected; timestamp within window → still succeeds.

### Tests for User Story 2 ⚠️ (write first, confirm they FAIL)

- [X] T027 [P] [US2] Add `TestMerchantAuthMiddleware_StaleTimestamp_Rejected` in `merchant_auth_test.go`: valid token/secret, `X-TIMESTAMP` = `time.Now().Add(-1*time.Hour)`, signature correctly computed against that same old timestamp → assert `http.StatusUnauthorized`.
- [X] T028 [P] [US2] Add `TestMerchantAuthMiddleware_FutureTimestamp_Rejected`: same as T027 but `+1*time.Hour` → assert `http.StatusUnauthorized`.
- [X] T029 [P] [US2] Add `TestMerchantAuthMiddleware_TimestampWithinWindow_PassesThrough`: `X-TIMESTAMP` = `time.Now().Add(4*time.Minute)`, correct signature → assert handler invoked, `http.StatusOK`.

Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware_StaleTimestamp -v` and `-run TestMerchantAuthMiddleware_FutureTimestamp -v` now — both MUST fail (no freshness check exists in `merchant_auth.go` yet).

### Implementation for User Story 2

- [X] T030 [US2] In `merchant_auth.go`, after the secret lookup (T025) and before the signature-verification step (T021), add the timestamp-freshness check: parse `X-TIMESTAMP` via `time.Parse(time.RFC3339, ...)`, reject 401 on parse error or `time.Since(parsed) > 5*time.Minute || time.Until(parsed) > 5*time.Minute` — mirror `snap_auth.go`'s existing freshness check (feature 009) exactly, including its `4010000` response codes. Depends on T021, T025.
- [X] T031 [US2] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware -v` (full file) and confirm T027-T029 pass and nothing from US1/US3b regressed. Depends on T030.

**Checkpoint**: Timestamp freshness enforced identically to the vendor side.

---

## Phase 6: User Story 4 - Bearer token and signature checks both apply, independently enforced (Priority: P1)

**Goal**: Confirm (with explicit combinatorial tests) that both checks are independently required — this is primarily a verification story over T021-T030's implementation, not new code.

**Independent Test**: Four combinations of (token valid/invalid) × (signature valid/invalid) → only (valid, valid) succeeds.

### Tests for User Story 4 ⚠️

- [X] T032 [P] [US4] Add `TestMerchantAuthMiddleware_InvalidTokenValidSignature_Rejected` in `merchant_auth_test.go`: mock `ValidateToken` returns an error, request otherwise carries a signature that *would* be valid if the token check passed → assert `http.StatusUnauthorized` and that `GetActiveClientSecret`/signature logic is never reached (`mockRepo.AssertNotCalled(t, "GetActiveClientSecret", mock.Anything, mock.Anything)`) — confirms token check still short-circuits first (feature 009 behavior unchanged).
- [X] T033 [P] [US4] Add `TestMerchantAuthMiddleware_AllFourCombinations` (table-driven, or four discrete subtests) explicitly enumerating (valid token, valid sig) → 200; (valid token, invalid sig) → 401; (invalid token, valid-looking sig) → 401; (invalid token, invalid sig) → 401 — largely composing T018-T020/T032 into one traceable test matching spec.md US4's acceptance scenarios verbatim.

Run `go test ./internal/adapter/delivery/http/middleware/... -run TestMerchantAuthMiddleware_InvalidTokenValidSignature -v` and `-run TestMerchantAuthMiddleware_AllFourCombinations -v` — these should already PASS if T021-T030 were implemented correctly (this phase is a verification checkpoint, not new implementation); if either fails, it indicates a sequencing bug in T021/T030 that must be fixed before proceeding.

### Implementation for User Story 4

- [X] T034 [US4] No new implementation expected. If T032/T033 reveal a gap (e.g. token validated after secret lookup, or short-circuit ordering wrong), fix the check ordering in `merchant_auth.go` (token → secret lookup → timestamp → signature, per data-model.md's Flow Summary) and re-run T032/T033.

**Checkpoint**: Dual-enforcement behavior explicitly verified, matching spec.md US4 acceptance scenarios one-to-one.

---

## Phase 7: User Story 5 - Vendor-facing endpoints remain unaffected (Priority: P2)

**Goal**: Confirm zero regression to `SNAPAuthMiddleware`/vendor-side behavior from feature 009.

**Independent Test**: Re-run feature 009's full vendor-side test suite and e2e flow unchanged.

### Tests for User Story 5

- [X] T035 [US5] Run `go test ./internal/adapter/delivery/http/middleware/... -run TestSNAPAuthMiddleware -v` and confirm all feature-009 vendor-side tests still pass unmodified (no code in `snap_auth.go` should have been touched by this feature at all — confirm via `git diff` showing zero changes to that file).

### Implementation for User Story 5

- [X] T036 [US5] No implementation — this story is purely a regression guard. If T035 reveals any change to `snap_auth.go`'s behavior, that is a bug in this feature's scope discipline and must be reverted.

**Checkpoint**: Vendor-side confirmed untouched.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: End-to-end validation and cleanup affecting the whole feature.

- [X] T037 [P] Run `go test -race -coverprofile=coverage.out ./internal/adapter/delivery/http/middleware/... ./internal/usecase/... ./internal/infrastructure/database/... ./internal/adapter/delivery/http/handler/... && go tool cover -func=coverage.out | grep -E "merchant_auth|client_repository|client_usecase|client_handler"` and confirm ≥90% coverage (Principle XI) on all new/changed code.
- [X] T038 [P] Run `go vet ./...` (substituting for `golangci-lint`, which has a pre-existing v1/v2 config mismatch per feature 008/009 notes) and fix any warnings introduced by this feature.
- [X] T039 Update `scripts/merchant-create-va.sh`, `scripts/merchant-list-va.sh`, `scripts/merchant-delete-va.sh` to compute and send `X-TIMESTAMP`/`X-SIGNATURE` headers (mirroring `scripts/vendor-inquiry-va.sh`'s existing HMAC-signing block, but with the real `$ACCESS_TOKEN` as the AccessToken component of `stringToSign` instead of an empty string — per research.md Decision 5) using a merchant shared secret passed via a new `-g <merchant-secret>` flag (or reuse `-e`/`-f` if a natural single-secret-per-script model fits; decide based on how `.env.<vendor>.<channel>` files are actually keyed per merchant vs. per vendor at implementation time).
- [X] T040 Provision a test merchant secret via `POST /admin/clients/:clientId/secret` (quickstart.md Scenario 1) against the local Docker stack, then re-run `./scripts/e2e-dynamic-va-flow.sh -f .env.bca.va -u http://localhost:8080` (now using the updated T039 scripts) and confirm all checks pass with signature verification genuinely active on the merchant side too. Depends on T016, T039.
- [X] T041 Execute quickstart.md Scenarios 2-6 manually against the local running instance (token-without-signature reject, valid-both success, signature-without-valid-token reject, unprovisioned-client fail-closed, stale timestamp reject) to confirm documented curl-level behavior matches implemented unit-test behavior.
- [X] T042 Update swagger annotations on `merchant_va_handler.go`'s `CreateVA`/`ListVA`/`DeleteVA` (currently `@Security BearerAuth` only, from feature 009) to also document the new `X-TIMESTAMP`/`X-SIGNATURE` requirement (per contracts/merchant-hmac.delta.yaml), add swagger docs for the two new admin endpoints on `client_handler.go`, then run `make swagger` to regenerate `docs/docs.go`/`docs/swagger.json`/`docs/swagger.yaml`, and verify via the same `python3 -c "json.load(...)"` spot-check pattern used in feature 009's polish phase.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational / US3 (Phase 2)**: Depends on Setup. BLOCKS Phases 3-7 entirely — no middleware work is testable without a way to provision a secret first.
- **US1 (Phase 3)**: Depends on Phase 2 (needs `GetActiveClientSecret` to exist, even if T021/T025 are implemented together).
- **US3b fail-closed (Phase 4)**: Depends on Phase 3 (T021 must call the secret lookup for this to be observable).
- **US2 (Phase 5)**: Depends on Phase 4 (timestamp check is inserted relative to the secret-lookup/signature steps already in place).
- **US4 (Phase 6)**: Depends on Phases 3-5 (it verifies their combined behavior; expected to require no new implementation if those phases are correct).
- **US5 (Phase 7)**: Can run any time after Phase 1 (it's a pure regression check on untouched code) — listed last only because it's most meaningful as a final confirmation.
- **Polish (Phase 8)**: Depends on all of Phases 2-7.

### Within Each User Story

- Tests written and confirmed failing before implementation.
- Repository → usecase → handler → route-wiring order for US3 (T012→T013→T014→T015→T016), matching Clean Architecture dependency direction.
- Within `merchant_auth.go`, checks are added in this order: token (existing) → secret lookup/fail-closed (US3b) → timestamp freshness (US2) → signature verification (US1) — note tasks T021 (US1/signature) and T025 (US3b/secret-lookup) are implemented together in practice since signature verification needs the secret; the phase split above reflects spec.md's story boundaries, not a strict single-PR-per-phase requirement.

### Parallel Opportunities

- T001-T004 (Setup reading tasks) can run in parallel.
- T006-T011 (US3 test-writing across migration/repository/usecase/handler layers) are marked `[P]` — different files.
- T018-T020, T027-T029, T032-T033 (test-writing within US1/US2/US4) are marked `[P]` — independent test functions in the same file, safe to write in any order.
- T037 and T038 (Polish) can run in parallel.
- **US5 (Phase 7) has no file overlap with any other phase** — could be run by a separate contributor at any point after Setup, purely as a watchdog against accidental vendor-side changes.

---

## Parallel Example: User Story 3 (Foundational) across layers

```bash
# Once T006 (migration) lands, these can proceed in parallel — different files:
Task: "Add TestClientRepository_CreateAndGetActiveClientSecret + TestClientRepository_RevokeClientSecret_NoLongerActive in internal/infrastructure/database/client_repository_test.go"
Task: "Add TestClientUsecase_AddClientSecret + error case in internal/usecase/client_usecase_test.go"
Task: "Add TestClientHandler_AddClientSecret_Success + RevokeClientSecret test in internal/adapter/delivery/http/handler/client_handler_test.go"
```

---

## Implementation Strategy

### MVP First (US3 + US1 only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: US3 (provisioning) — this alone is useful/demoable (operators can manage merchant secrets) even before the middleware enforces anything.
3. Complete Phase 3: US1 (signature verification wired in) — this is the point where the feature's core security value lands.
4. **STOP and VALIDATE**: run the full middleware + database + usecase + handler test suite.
5. Deploy/demo if ready — note this alone does NOT yet close the fail-closed gap (Phase 4) or freshness gap (Phase 5); consider whether a partial rollout is acceptable given spec FR-010's "no toggle, unconditional" requirement, or whether Phases 2-5 should ship as one atomic release (recommended, since FR-010 implies no half-enforced state is intended).

### Incremental Delivery

1. Setup + US3 (provisioning) → foundation ready, operators can start provisioning ahead of enforcement going live.
2. US1 (signature check) + US3b (fail-closed) + US2 (freshness) → ship together as one release (see note above) → test independently → deploy.
3. US4 → verification-only checkpoint, no separate deploy.
4. US5 → regression guard, run anytime.
5. Polish → coverage/lint/quickstart/swagger sign-off → final deploy readiness.

---

## Notes

- [P] tasks = different files or independent test functions within the same file.
- Commit after each task or logical group, per repository convention.
- Verify tests fail for the right reason before implementing (missing check / wrong status code, not a compile error) — except where explicitly noted that a phase is verification-only and tests may already pass.
- There is intentionally no per-merchant or global enable/disable toggle for this feature's enforcement (spec FR-010) — do not add one "to be safe"; this mirrors feature 009's explicit prior decision.
- `secret_value` (and the request/response fields that carry it) must never be logged or echoed back in any API response — treat with the same care as `VENDOR_CLIENT_SECRET`/private key material elsewhere in this codebase.
- Feature 009's vendor-side convention (empty AccessToken in `stringToSign`) is a *documented, deliberate difference* from this feature, not an inconsistency to "fix" — do not accidentally unify the two conventions.
