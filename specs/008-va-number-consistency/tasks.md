---

description: "Task list template for feature implementation"
---

# Tasks: Virtual Account Number Consistency with SNAP Standard

**Input**: Design documents from `/specs/008-va-number-consistency/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/create-va.delta.yaml, quickstart.md

**Tests**: Required — constitution Principle III (TDD) mandates writing failing tests first for all feature implementations. All test tasks below MUST be completed and MUST fail before their corresponding implementation task is started.

**Organization**: Tasks are grouped by user story (US1, US2, US3) per spec.md priorities. All three stories touch the same single function (`CreateVA` in `internal/usecase/merchant_va_usecase.go`), so implementation tasks within a story are sequenced against that shared file rather than marked `[P]` across stories — but test tasks across different test functions in the same test file are still independent enough to write in any order.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Single Go backend project (existing Clean Architecture layout):
- `internal/domain/va.go` — domain error codes / VARepository interface (reference only, no changes expected)
- `internal/usecase/merchant_va_usecase.go` — implementation
- `internal/usecase/merchant_va_usecase_test.go` — tests
- `scripts/e2e-dynamic-va-flow.sh` — existing e2e script (updated in Polish phase)

---

## Phase 1: Setup

**Purpose**: Confirm the existing feature-006 code and test scaffolding this feature builds on.

- [X] T001 Read `internal/usecase/merchant_va_usecase.go` lines 30-170 (`CreateVA`) and confirm the exact current line numbers for: the `virtualAccountNo` mandatory-field check, the `vaNo := req.VirtualAccountNo` assignment, the `customerNo` resolution branch (dynamic vs static), and the `GetVAByVirtualAccountNo`/pending-conflict check — no code changes in this task, just re-confirming anchors before editing.
- [X] T002 [P] Confirm `internal/usecase/merchant_va_usecase_test.go` builds and all existing tests pass before any change: `go test ./internal/usecase/... -run TestMerchantVAUsecase_CreateVA -v`.

**Checkpoint**: Baseline confirmed; safe to start Foundational phase.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared helper and error-code additions that every user story's implementation depends on.

**⚠️ CRITICAL**: No user story implementation task may start until this phase is complete.

- [X] T003 Add a small unexported helper `func vaNoMatchesPartnerAndCustomer(partnerServiceID, customerNo, virtualAccountNo string) bool` (or equivalent inline logic, developer's choice of a helper vs. inline — keep it colocated) near the top of `internal/usecase/merchant_va_usecase.go`, returning `virtualAccountNo == partnerServiceID+customerNo`. This is pure, dependency-free logic shared by US1's validation and US2's derivation-check paths.
- [X] T004 Confirm/document the new domain error code `4002707` ("Invalid Field Format [virtualAccountNo does not match partnerServiceId + customerNo]") as a literal string constructed via `domain.NewDomainError("4002707", "...", nil)` at its call site in `internal/usecase/merchant_va_usecase.go` — no new file/constant table exists elsewhere in the codebase for these codes (confirmed in research.md Decision 4), so no shared constants file needs to change.

**Checkpoint**: Helper exists and error-code convention confirmed — user story implementation can now begin.

---

## Phase 3: User Story 1 - Server rejects a mismatched virtualAccountNo on static VA creation (Priority: P1) 🎯 MVP

**Goal**: For static (managed, non-dynamic) and unmanaged/legacy create-VA requests, reject the request with `4002707` when the merchant-supplied `virtualAccountNo` does not equal `partnerServiceId + customerNo`; accept unchanged when it matches.

**Independent Test**: Submit a static VA create request (e.g. `partnerServiceId=15973`, `vaType=01`) with a `virtualAccountNo` that is NOT `partnerServiceId+customerNo` and confirm rejection with `4002707` and no persisted record; submit the same request with the correct concatenation and confirm it succeeds exactly as before.

### Tests for User Story 1 ⚠️ (write first, confirm they FAIL)

- [X] T005 [P] [US1] Add `TestMerchantVAUsecase_CreateVA_StaticVirtualAccountNoMismatch_Rejected` in `internal/usecase/merchant_va_usecase_test.go`: static vaType (e.g. `01`, `partnerServiceId=15973`), `customerNo="000000000000000123"`, `virtualAccountNo="9999999999999999999999"` (deliberately not the concatenation) → expect a domain error with code `4002707` and assert `repo.SaveInquiry` is never called (use `mock.AssertNotCalled` or omit the `.On("SaveInquiry", ...)` stub so an unexpected call fails the mock).
- [X] T006 [P] [US1] Add `TestMerchantVAUsecase_CreateVA_StaticVirtualAccountNoMatch_Succeeds` in `internal/usecase/merchant_va_usecase_test.go`: same static vaType/customerNo, but `virtualAccountNo="15973000000000000000123"` (the correct concatenation) → expect success (`responseCode` `2002700`) and `virtualAccountData.virtualAccountNo == "15973000000000000000123"`, matching the existing `TestMerchantVAUsecase_CreateVA_StaticNoBill_EchoesCustomerNo` pattern at line 517 but with an explicit concatenation-correct value instead of whatever value that test currently uses (confirm that test's existing `virtualAccountNo` fixture already satisfies the concatenation, or adjust its fixture value in this same task if it does not, so it keeps passing after T007's change).
- [X] T007 [P] [US1] Add `TestMerchantVAUsecase_CreateVA_UnmanagedLegacyVirtualAccountNoMismatch_Rejected` in `internal/usecase/merchant_va_usecase_test.go`: construct the usecase with a `nil` `vaTypeRules` provider (unmanaged/legacy mode, per `NewMerchantVAUsecase` doc comment) and a `partnerServiceId` outside the reserved set, with `customerNo` and a mismatched `virtualAccountNo` → expect rejection with `4002707`, proving the new check also covers legacy unmanaged requests per FR-001 (which does not restrict the rule to "managed" static VA only).

Run `go test ./internal/usecase/... -run TestMerchantVAUsecase_CreateVA_Static -v` and `-run TestMerchantVAUsecase_CreateVA_Unmanaged` now — T005 and T007 MUST fail (feature not yet implemented); T006 should already pass if the existing fixture happens to satisfy concatenation, or fail otherwise — do not proceed to implementation until you've confirmed T005/T007 fail for the right reason (wrong error code / unexpected success), not a compile error.

### Implementation for User Story 1

- [X] T008 [US1] In `internal/usecase/merchant_va_usecase.go`, locate the existing block that resolves `customerNo` for static/unmanaged requests (`customerNo := req.CustomerNo` and the `RegisterStaticCustomerNo` call, around line 108-121). Immediately after `customerNo` is finalized for the static/unmanaged path (i.e. inside the `else if managed && !vaTypeRule.Dynamic` branch, and also for the `!managed` legacy path), call the T003 helper (or inline check) comparing `req.VirtualAccountNo` against `req.PartnerServiceID + customerNo`; on mismatch, `return nil, domain.NewDomainError("4002707", "Invalid Field Format [virtualAccountNo does not match partnerServiceId + customerNo]", nil)` before any `SaveInquiry` call. Depends on T003.
- [X] T009 [US1] Run `go test ./internal/usecase/... -run TestMerchantVAUsecase_CreateVA -v` and confirm T005, T006, T007 now pass, and no previously-passing test in the file regressed (full file run, not just the new subset). Depends on T008.

**Checkpoint**: User Story 1 fully functional and independently testable — static/legacy VA creation now enforces SNAP virtualAccountNo consistency.

---

## Phase 4: User Story 2 - Dynamic VA honors a merchant-supplied virtualAccountNo, or derives one automatically when omitted (Priority: P1)

**Goal**: For dynamic (managed, `vaTypeRule.Dynamic == true`) create-VA requests: `virtualAccountNo` becomes optional; when empty, derive it as `partnerServiceId + generated customerNo`; when non-empty, use the merchant's value as-is (subject to existing length and pending-conflict checks); a colliding value on an active pending VA is rejected with the existing conflict error.

**Independent Test**: (a) Submit a dynamic VA create request with both `customerNo` and `virtualAccountNo` empty → response has a server-generated `customerNo` and `virtualAccountNo == partnerServiceId + customerNo`. (b) Submit a dynamic VA create request with `customerNo` empty but a merchant-supplied `virtualAccountNo` → response echoes that exact value. (c) Repeat (b) with a `virtualAccountNo` colliding with an existing pending VA → rejected with the existing `4092700` conflict code.

### Tests for User Story 2 ⚠️ (write first, confirm they FAIL)

- [X] T010 [P] [US2] Add `TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoEmpty_AutoDerived` in `internal/usecase/merchant_va_usecase_test.go`: dynamic vaType (e.g. `04`, `partnerServiceId=15973`), `customerNo=""`, `virtualAccountNo=""` (this requires relaxing the request builder used by existing dynamic tests, which today likely still populate `virtualAccountNo` — see existing dynamic-VA tests around line 660 for the current request-construction pattern to copy/adapt), repo's `NextCustomerNoSequence` mock returns e.g. `"04000000000000000099"` → expect success with `virtualAccountData.virtualAccountNo == "1597304000000000000099"` (i.e. `"15973"+"04000000000000000099"`) and `virtualAccountData.customerNo == "04000000000000000099"`.
- [X] T011 [P] [US2] Add `TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoSupplied_UsedAsIs` in `internal/usecase/merchant_va_usecase_test.go`: same dynamic vaType/partnerServiceId, `customerNo=""`, `virtualAccountNo="1597304999999999999999"` (merchant-chosen, deliberately NOT equal to `partnerServiceId+generatedCustomerNo`), repo's `NextCustomerNoSequence` mock returns a different generated value, and `GetVAByVirtualAccountNo` mock returns `(nil, nil)` (no existing record) → expect success with `virtualAccountData.virtualAccountNo == "1597304999999999999999"` unchanged, proving the merchant's value is not overridden.
- [X] T012 [P] [US2] Add `TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoSupplied_ConflictOnPending` in `internal/usecase/merchant_va_usecase_test.go`: same setup as T011, but repo's `GetVAByVirtualAccountNo` mock returns an existing `*domain.VAInquiryRecord` with `Status: "03"` for that `virtualAccountNo` → expect a domain error with the existing conflict code `4092700` ("Conflict: VA already has an active pending transaction"), matching the pattern already used for the static VA pending-reuse check.
- [X] T013 [P] [US2] Add `TestMerchantVAUsecase_CreateVA_DynamicDerivedVirtualAccountNoTooLong_Rejected` in `internal/usecase/merchant_va_usecase_test.go`: construct (or mock) a scenario where `partnerServiceId` + generated `customerNo` would exceed 28 characters (per FR-007) with `virtualAccountNo` left empty → expect rejection with the existing `4002700` "Invalid Field Format [virtualAccountNo too long]" error and no persisted record. If `partnerServiceId` is fixed at 5 chars and generated `customerNo` is always exactly 20 chars (2+18 per feature 006), document in the test comment whether this scenario is reachable today or is a defensive/future-proofing test — implement the check in T015 regardless per FR-007.

Run `go test ./internal/usecase/... -run TestMerchantVAUsecase_CreateVA_Dynamic -v` now — T010, T011, T013 MUST fail (dynamic path today still requires non-empty `virtualAccountNo` and never derives it); T012 may already pass if it happens to hit the existing generic pending-conflict check unchanged, or fail if the current mandatory-field check short-circuits first — confirm the actual failure reason before proceeding.

### Implementation for User Story 2

- [X] T014 [US2] In `internal/usecase/merchant_va_usecase.go`, change the unconditional mandatory-field check `if req.VirtualAccountNo == "" { return ... "4002701" ... }` (around line 55) so it only applies when NOT `(managed && vaTypeRule.Dynamic)` — i.e. `virtualAccountNo` stays mandatory for static/unmanaged (feeding US1's T008 check) but becomes optional for dynamic. Depends on T008 (must not regress US1's mandatory-field behavior for static).
- [X] T015 [US2] In `internal/usecase/merchant_va_usecase.go`, in the dynamic branch where `customerNo` is generated via `NextCustomerNoSequence` (around line 108-114), after generation: if `req.VirtualAccountNo == ""`, set `vaNo = req.PartnerServiceID + customerNo` and, if `len(vaNo) > 28`, return the existing `4002700` "too long" domain error (FR-007); otherwise (non-empty merchant-supplied value) keep `vaNo = req.VirtualAccountNo` per the existing line-101 assignment (still subject to the existing `len(vaNo) > 28` check already in place). Depends on T014.
- [X] T016 [US2] Run `go test ./internal/usecase/... -run TestMerchantVAUsecase_CreateVA -v` (full file) and confirm T010-T013 now pass and no US1 or pre-existing test regressed. Depends on T015.

**Checkpoint**: User Stories 1 and 2 both fully functional and independently testable — dynamic VA creation now supports both auto-derivation and merchant-supplied `virtualAccountNo`, with conflict handling.

---

## Phase 5: User Story 3 - Consistent VA numbers flow through inquiry and payment (Priority: P2)

**Goal**: Confirm the existing inquiry/payment use cases and the full e2e flow continue to work unmodified with the now-consistent `virtualAccountNo`/`customerNo` pairs, and update the e2e script so it no longer needs to hand-craft a `virtualAccountNo` for dynamic VA.

**Independent Test**: Run `scripts/e2e-dynamic-va-flow.sh` end-to-end against a local stack and confirm all existing checks still pass, with the dynamic-VA `virtualAccountNo` now coming from the server (empty on the request) instead of being constructed by the script.

### Tests for User Story 3 ⚠️ (write first, confirm they FAIL — or note as regression-only if no new unit-testable behavior)

- [X] T017 [P] [US3] Add `TestMerchantVAUsecase_InquiryVA_UsesServerDerivedVirtualAccountNo` (or extend an existing inquiry-usecase test, whichever file currently covers the inquiry use case — locate via `grep -rn "func.*Inquiry" internal/usecase/*_test.go`) asserting that an inquiry request using a `virtualAccountNo` that equals `partnerServiceId+customerNo` (as now guaranteed by US1/US2) resolves to the correct VA record — this is a regression/confirmation test, not new business logic, so it may simply reuse an existing dynamic-VA record fixture created via the T010-style derived value.

### Implementation for User Story 3

- [X] T018 [US3] Update `scripts/e2e-dynamic-va-flow.sh`: remove the manual `VA_NO_1`/`VA_NO_2`/`VA_NO_3` construction (lines 131, 160, 193) and the corresponding `-v "$VA_NO_N"` arguments passed to `merchant-create-va.sh` for all three dynamic tests (Tests 1-3), so the script leaves `virtualAccountNo` empty on dynamic create-VA requests and instead asserts the response's `virtualAccountData.virtualAccountNo` equals `virtualAccountData.partnerServiceId + virtualAccountData.customerNo` (new `check` calls after each create-VA response, replacing/augmenting the existing "server-generated customerNo is 20 digits" checks). Depends on T016 (server must already support empty `virtualAccountNo` for dynamic VA before the script can be changed to omit it).
- [X] T019 [US3] Run `./scripts/e2e-dynamic-va-flow.sh -f .env.bca.va -u http://localhost:8080` (per quickstart.md Scenario 6) against the local Docker stack and confirm all checks pass, including the new virtualAccountNo-consistency checks added in T018. Depends on T018.

**Checkpoint**: All three user stories independently functional; end-to-end flow confirms no regressions.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup affecting the whole feature.

- [X] T020 [P] Run `go test -race -coverprofile=coverage.out ./internal/usecase/... && go tool cover -func=coverage.out | grep merchant_va_usecase` and confirm the `CreateVA` function's coverage remains ≥ 90% per constitution Principle XI, including all new branches added in T008/T014/T015.
- [X] T021 [P] Run `golangci-lint run ./internal/usecase/...` and fix any warnings introduced by this feature's changes.
- [X] T022 Execute quickstart.md Scenarios 1-5 manually (or via a throwaway script) against a local running instance to confirm the documented curl-level behavior matches the implemented unit-test behavior, then re-run Scenario 6 (full e2e) one final time.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories (T008 needs T003's helper; T014/T015 need T003 and the T004-confirmed error-code convention).
- **User Story 1 (Phase 3)**: Depends on Foundational. No dependency on US2/US3.
- **User Story 2 (Phase 4)**: Depends on Foundational AND on US1's T008 (T014 must not regress the static mandatory-field check T008 relies on) — sequence US1 before US2 even though they could theoretically be parallelized by different developers with care.
- **User Story 3 (Phase 5)**: Depends on US2 (T016) — the e2e script change (T018) requires the server to already support empty `virtualAccountNo` for dynamic VA.
- **Polish (Phase 6)**: Depends on all three user stories being complete.

### Within Each User Story

- Tests written and confirmed failing before implementation (T005-T007 before T008; T010-T013 before T014/T015; T017 before/alongside T018).
- Implementation task order follows the single shared file's logical flow: mandatory-field check → customerNo resolution → virtualAccountNo resolution/validation → length check → conflict check → persistence.

### Parallel Opportunities

- T002 (baseline test run) can run in parallel with T001 (code reading).
- All test-writing tasks within a story (T005/T006/T007; T010/T011/T012/T013) are marked `[P]` — they add independent test functions to the same file, so write them in any order, but note they still land in one file (small serialization risk when actually saving edits; conceptually independent).
- T020 and T021 (Polish) can run in parallel.

---

## Parallel Example: User Story 2

```bash
# Write all four new test functions for User Story 2 (can be done in any order/by any contributor):
Task: "Add TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoEmpty_AutoDerived in internal/usecase/merchant_va_usecase_test.go"
Task: "Add TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoSupplied_UsedAsIs in internal/usecase/merchant_va_usecase_test.go"
Task: "Add TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoSupplied_ConflictOnPending in internal/usecase/merchant_va_usecase_test.go"
Task: "Add TestMerchantVAUsecase_CreateVA_DynamicDerivedVirtualAccountNoTooLong_Rejected in internal/usecase/merchant_va_usecase_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1 — static VA consistency validation.
4. **STOP and VALIDATE**: run the full `merchant_va_usecase_test.go` suite; this alone already closes the most-used (static) flow's SNAP-compliance gap.
5. Deploy/demo if ready — this is a safe, backward-compatible increment (only rejects requests that were already violating the standard).

### Incremental Delivery

1. Setup + Foundational → foundation ready.
2. User Story 1 → static VA validation → test independently → deploy (MVP).
3. User Story 2 → dynamic VA optional/derived virtualAccountNo → test independently → deploy.
4. User Story 3 → e2e script + regression confirmation → deploy.
5. Polish → coverage/lint/quickstart sign-off.

---

## Notes

- [P] tasks = different test functions or independent verification steps; most implementation tasks are NOT parallelizable since US1/US2 edit overlapping regions of the same `CreateVA` function.
- Commit after each task or logical group, per repository convention (see recent commit history — one feature/fix per commit).
- Verify tests fail for the right reason before implementing (wrong error code or unexpected success, not a compile error).
- Do not retroactively migrate existing VA records — explicitly out of scope per spec.md Assumptions.
- The dynamic-merchant-supplied-virtualAccountNo case (US2) intentionally does NOT enforce `virtualAccountNo == partnerServiceId+customerNo` — this is a deliberate exception per spec.md Assumptions, not a gap to "fix" later.
