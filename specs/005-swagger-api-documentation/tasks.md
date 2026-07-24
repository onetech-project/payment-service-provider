---

description: "Task list for Swagger API Documentation & Testing"
---

# Tasks: Swagger API Documentation & Testing

**Input**: Design documents from `/specs/005-swagger-api-documentation/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/swagger-ui.md, quickstart.md

**Tests**: Not explicitly requested for this feature (documentation-only change, no new business logic). No test tasks are included; validation is performed via quickstart.md's runnable scenarios (see Polish phase).

**Organization**: Tasks are grouped by user story (US1: Browse Complete API Reference, US2: Try Out Requests, US3: Understand Request/Response/Error Contract) to enable independent implementation and validation of each.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Single Go module at repository root. Handlers: `internal/adapter/delivery/http/handler/`. Entry point: `cmd/api/main.go`. Generated docs: `docs/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add doc-generation tooling and dependencies before any annotation work begins

- [X] T001 Add `github.com/swaggo/swag` and `github.com/swaggo/echo-swagger` to `go.mod`/`go.sum` via `go get github.com/swaggo/swag/cmd/swag github.com/swaggo/echo-swagger`
- [X] T002 [P] Add a `swagger` target to the project `Makefile` (or equivalent build script) running `swag init -g cmd/api/main.go --output docs`
- [X] T003 [P] Add `docs/` generation output to `.gitignore` exclusions review — confirm whether generated `docs/` is committed or generated in CI/build (decide per existing repo convention for generated code; document the choice as a comment at the top of `cmd/api/main.go` above the blank import)

**Checkpoint**: Tooling installed; `swag init` runs successfully (even with zero annotations, producing an empty spec)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: General API info, security definitions, and the served UI route — required before any endpoint can be meaningfully documented or tried out

**⚠️ CRITICAL**: No user-story work can be validated until this phase is complete

- [X] T004 Add general API info annotations (`@title`, `@version`, `@description`, `@BasePath`, `@host`) as a doc comment above `main()` in `cmd/api/main.go`
- [X] T005 Add `@securityDefinitions.apikey` blocks for `SnapClientKey` (`X-CLIENT-KEY`), `SnapTimestamp` (`X-TIMESTAMP`), `SnapSignature` (`X-SIGNATURE`), `BearerAuth` (`Authorization`), and `AdminToken` (matching the header read in `internal/adapter/delivery/http/middleware/admin_auth.go`) in the same doc-comment block in `cmd/api/main.go`, per contracts/swagger-ui.md's Security Definitions Contract
- [X] T006 Register the `echo-swagger` route (`e.GET("/swagger/*", echoSwagger.WrapHandler)`) in `cmd/api/main.go` alongside existing route registrations, and blank-import the generated `docs` package
- [X] T007 Run `swag init -g cmd/api/main.go --output docs` and confirm the service builds (`go build ./...`) and `/swagger/index.html` serves a UI (even with no endpoints annotated yet)

**Checkpoint**: Swagger UI is reachable and shows general API info; ready for per-endpoint annotation work in any order/parallel

---

## Phase 3: User Story 1 - Browse Complete API Reference (Priority: P1) 🎯 MVP

**Goal**: Every registered endpoint appears in Swagger UI, grouped by tag, each with a plain-language explanation

**Independent Test**: Open `/swagger/index.html` and confirm all 13 endpoints appear under their correct tag with a description (per data-model.md's Endpoint Inventory)

### Implementation for User Story 1

- [X] T008 [P] [US1] Add `@Tags Health`, `@Summary`, `@Description`, `@Router /health [get]` annotation above the inline health-check handler in `cmd/api/main.go`
- [X] T009 [P] [US1] Add `@Tags Token`, `@Summary`, `@Description`, `@Router /openapi/v1.0/access-token/b2b [post]` annotation above `GetB2BAccessToken` in `internal/adapter/delivery/http/handler/token_handler.go`
- [X] T010 [P] [US1] Add `@Tags Admin Client Management`, `@Summary`, `@Description`, `@Router` annotations above `RegisterClient`, `AddClientKey`, `RevokeClientKey` in `internal/adapter/delivery/http/handler/client_handler.go`
- [X] T011 [P] [US1] Add `@Tags Signature Utilities`, `@Summary`, `@Description`, `@Router` annotations above `GenerateAccessTokenSignature`, `GenerateServiceSignature` in `internal/adapter/delivery/http/handler/signature_handler.go`
- [X] T012 [P] [US1] Add `@Tags Virtual Account`, `@Summary`, `@Description`, `@Router` annotations above `Inquiry`, `Payment`, `Status` in `internal/adapter/delivery/http/handler/va_handler.go`
- [X] T013 [P] [US1] Add `@Tags Merchant VA Dashboard`, `@Summary`, `@Description`, `@Router` annotations above `CreateVA`, `ListVA`, `DeleteVA` in `internal/adapter/delivery/http/handler/merchant_va_handler.go`, noting `CreateVA`/`DeleteVA` as state-changing in their `@Description` per FR-011
- [X] T014 [US1] Regenerate docs (`swag init -g cmd/api/main.go --output docs`), rebuild, and manually verify in `/swagger/index.html` that all 6 tag groups and 13 endpoints are listed with descriptions (data-model.md Endpoint Inventory)

**Checkpoint**: User Story 1 fully functional — full endpoint list with explanations is browsable independent of request/response schema depth or try-it-out auth wiring

---

## Phase 4: User Story 2 - Try Out Requests Directly From the Docs (Priority: P1)

**Goal**: A developer can execute a real request from Swagger UI for at least one endpoint per auth tier (public, SNAP-signed, admin) and see the real response

**Independent Test**: Use "Try it out" on `GET /health` (public), `POST /openapi/v1.0/transfer-va/inquiry` (SNAP-signed), and `POST /admin/clients` (admin) per quickstart.md's User Story 2 validation steps, confirming real HTTP calls and real responses

### Implementation for User Story 2

- [X] T015 [US2] Add `@Security` annotations (`BearerAuth` where applicable) to the Token endpoint in `internal/adapter/delivery/http/handler/token_handler.go` (depends on T009, T005)
- [X] T016 [US2] Add `@Security AdminToken` annotations to all three endpoints in `internal/adapter/delivery/http/handler/client_handler.go` (depends on T010, T005)
- [X] T017 [US2] Add `@Security SnapClientKey`, `@Security SnapTimestamp`, `@Security SnapSignature` annotations to `Inquiry`, `Payment`, `Status` in `internal/adapter/delivery/http/handler/va_handler.go` (depends on T012, T005)
- [X] T018 [US2] Add the same SNAP `@Security` annotations to `CreateVA`, `ListVA`, `DeleteVA` in `internal/adapter/delivery/http/handler/merchant_va_handler.go` (depends on T013, T005)
- [X] T019 [US2] Add `@Param` header annotations for the exact header names/formats read by `internal/adapter/delivery/http/middleware/snap_auth.go` (`X-CLIENT-KEY`, `X-TIMESTAMP`, `X-SIGNATURE`) to each SNAP-protected endpoint so the header fields render individually in Swagger UI's request form, cross-referenced from `@Description` to the Signature Utilities endpoints for how to compute `X-SIGNATURE`
- [X] T020 [US2] Regenerate docs, rebuild, and manually execute quickstart.md's three "Try it out" scenarios (public/SNAP/admin) against a locally running instance, confirming real (not mocked) responses

**Checkpoint**: At least one endpoint per auth tier is fully executable from the docs; User Stories 1 and 2 both work independently

---

## Phase 5: User Story 3 - Understand Request/Response Shape and Error Codes (Priority: P2)

**Goal**: Every endpoint's request fields, example request, example success response, and every documented error code are visible and accurate

**Independent Test**: Per quickstart.md's User Story 3 steps — pick any endpoint, confirm request/response schemas match the real DTOs, and trigger a documented error case confirming the returned `responseCode`/`responseMessage` matches the docs exactly

### Implementation for User Story 3

- [X] T021 [P] [US3] Add `@Param body body ... true "..."`, `@Success 200 {object} ...`, and example struct tags/comments for the Token endpoint's request/response DTOs in `internal/adapter/delivery/http/handler/token_handler.go`
- [X] T022 [P] [US3] Add `@Param`/`@Success` request/response schema annotations for `RegisterClient`, `AddClientKey`, `RevokeClientKey` DTOs in `internal/adapter/delivery/http/handler/client_handler.go`
- [X] T023 [P] [US3] Add `@Param`/`@Success` request/response schema annotations for `GenerateAccessTokenSignature`, `GenerateServiceSignature` DTOs in `internal/adapter/delivery/http/handler/signature_handler.go`
- [X] T024 [P] [US3] Add `@Param`/`@Success` request/response schema annotations for `Inquiry`, `Payment`, `Status` DTOs (`domain.VAInquiryRequest`/`domain.VAInquiryResponse` and siblings) in `internal/adapter/delivery/http/handler/va_handler.go`
- [X] T025 [P] [US3] Add `@Param`/`@Success` request/response schema annotations for `CreateVA`, `ListVA`, `DeleteVA` DTOs in `internal/adapter/delivery/http/handler/merchant_va_handler.go`
- [X] T026 [US3] Define the shared error-response model (`responseCode`/`responseMessage`, per data-model.md's Error Response Model) once as a reusable Go struct/comment target, then add `@Failure` annotations referencing it to every error branch in `internal/adapter/delivery/http/handler/va_handler.go` and `internal/adapter/delivery/http/handler/merchant_va_handler.go` (one `@Failure` line per distinct `ResponseCode` actually returned)
- [X] T027 [US3] Add `@Failure` annotations for every distinct error branch in `internal/adapter/delivery/http/handler/token_handler.go` and `internal/adapter/delivery/http/handler/signature_handler.go`, matching their existing (non-SNAP-shaped) error DTOs
- [X] T028 [US3] Add `@Failure` annotations for every distinct error branch (including admin-auth rejection) in `internal/adapter/delivery/http/handler/client_handler.go`
- [X] T029 [US3] Regenerate docs, rebuild, and manually verify per quickstart.md's User Story 3 steps: request field docs match real structs, example bodies are realistic, and a real triggered error response's `responseCode`/`responseMessage` matches a documented `@Failure` entry exactly

**Checkpoint**: All three user stories independently functional — full endpoint list, interactive testing, and complete request/response/error documentation

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and drift-prevention check across all stories

- [X] T030 Run `go vet ./...` and `golangci-lint run` to confirm annotation comments introduced no lint/vet issues
- [X] T031 Run the full quickstart.md validation checklist end-to-end (Setup → US1 → US2 → US3 → Regeneration check) against a locally running instance
- [X] T032 Confirm `go test -race ./...` still passes with zero regressions (no handler logic was changed, only doc comments added)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories (Swagger UI route, security definitions, and general API info must exist before any endpoint annotation is visible/testable)
- **User Story 1 (Phase 3)**: Depends on Foundational — can proceed once Phase 2 is done
- **User Story 2 (Phase 4)**: Depends on Foundational and on the per-endpoint `@Tags`/`@Router` annotations from User Story 1 (T009–T013) existing so `@Security` has an annotation block to attach to
- **User Story 3 (Phase 5)**: Depends on Foundational and on User Story 1's annotation blocks (T009–T013); independent of User Story 2's `@Security` additions (can run in parallel with Phase 4 once Phase 3 completes)
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### Parallel Opportunities

- T002, T003 in Setup can run in parallel
- T008–T013 in User Story 1 are all in different files and can run fully in parallel
- T021–T025 in User Story 3 are all in different files and can run fully in parallel
- Phase 4 (US2) and Phase 5 (US3) can be worked in parallel by different people once Phase 3 is complete, since they touch the same files but different annotation lines (`@Security` vs `@Param`/`@Success`/`@Failure`) — coordinate to avoid merge conflicts within the same handler file, or sequence T015–T018 before T021–T025 per handler if solo

---

## Parallel Example: User Story 1

```bash
Task: "Add Health tag annotations in cmd/api/main.go"
Task: "Add Token tag annotations in internal/adapter/delivery/http/handler/token_handler.go"
Task: "Add Admin Client Management tag annotations in internal/adapter/delivery/http/handler/client_handler.go"
Task: "Add Signature Utilities tag annotations in internal/adapter/delivery/http/handler/signature_handler.go"
Task: "Add Virtual Account tag annotations in internal/adapter/delivery/http/handler/va_handler.go"
Task: "Add Merchant VA Dashboard tag annotations in internal/adapter/delivery/http/handler/merchant_va_handler.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Confirm all 13 endpoints are browsable with descriptions
5. This alone delivers the core "documentation" half of the ask and can be demoed/shipped independently of interactive testing depth

### Incremental Delivery

1. Setup + Foundational → Swagger UI reachable with general info
2. User Story 1 → full endpoint list browsable (MVP)
3. User Story 2 → interactive testing works per auth tier
4. User Story 3 → full request/response/error contract documented
5. Polish → lint/vet/test clean, full quickstart validated

---

## Notes

- No new test tasks were generated — this feature adds documentation comments only, not business logic; correctness is validated via quickstart.md's runnable scenarios (T014, T020, T029, T031) rather than unit tests, consistent with plan.md's Constitution Check Principle III adaptation.
- [P] tasks touch different files; sequential tasks within the same file (e.g. T015 then T021 both touching `token_handler.go`) should be done by the same contributor or coordinated to avoid conflicting edits to the same doc-comment block.
- Commit after each task or logical group per existing repo convention.
