# Implementation Plan: Enforce Signature & Token Verification on Transfer-VA Endpoints

**Branch**: `009-transfer-va-auth` | **Date**: 2026-07-29 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/009-transfer-va-auth/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Wire up authentication that already has most of its building blocks but is never actually invoked. Vendor-side: extend `SNAPAuthMiddleware` (`internal/adapter/delivery/http/middleware/snap_auth.go`) to recompute the HMAC-SHA512 signature via the existing `HMACSigner.Verify` (`internal/infrastructure/crypto/hmac.go`) against the per-vendor/channel `VENDOR_CLIENT_SECRET`, and to reject stale/future timestamps using the same ±5-minute tolerance pattern already proven in `token_usecase.go`. Both checks are unconditional — no config toggle — since enforcing them correctly is the entire point of this fix. (Amended later: the timestamp-freshness check alone gained a global `APP_ENV=dev`/`uat` exception — see `research.md` Decision 4 Amendment; signature verification remains unconditional.) Merchant-side: add a new middleware that validates the `Authorization: Bearer <accessToken>` JWT via the existing `JWTIssuer.ValidateToken` (`internal/infrastructure/crypto/jwt_issuer.go`), applied only to the merchant route group (`create-va`/`list`/`delete-va`) in `cmd/api/main.go`, leaving the vendor route group untouched. No new business logic, no schema/API-response changes — this is purely an admission-control fix.

## Technical Context

**Language/Version**: Go (latest stable, per constitution) — existing module `backbone-new`

**Primary Dependencies**: None new. Reuses `internal/infrastructure/crypto/hmac.go` (`HMACSigner.Verify`), `internal/infrastructure/crypto/jwt_issuer.go` (`JWTIssuer.ValidateToken`), `internal/infrastructure/config/vendor_config.go` (`VendorConfig`, unchanged — `ClientSecret` already exists), and the Echo middleware chain already used by `SNAPAuthMiddleware`/`IdempotencyMiddleware`.

**Storage**: No database changes, no new config fields. `VENDOR_CLIENT_SECRET` (already loaded into `VendorConfig.ClientSecret`) is the only config this feature reads — no new `.env.<vendor>.<channel>` key is introduced.

**Testing**: Go `testing` + existing middleware/handler unit test patterns (`internal/adapter/delivery/http/middleware/*_test.go`, `internal/adapter/delivery/http/handler/*_test.go`); per constitution Principle III (TDD) and XI (>90% coverage), new tests are written first for each acceptance scenario.

**Target Platform**: Linux server (existing Docker Compose stack: app, postgres, redis)

**Project Type**: Web service (single Go backend, Clean Architecture layers per constitution)

**Performance Goals**: Negligible added latency — HMAC-SHA512 computation and JWT parse/verify are both single-digit-microsecond operations; no new I/O is introduced (the shared secret and JWT public key are already loaded into memory at startup, same as today).

**Constraints**: Must not change response shapes, business logic, or the VA-number-consistency rules from feature 008 — this feature only changes whether a request is admitted. Both vendor-side and merchant-side enforcement are unconditional from the moment this ships — no config toggle, no gradual rollout, per spec FR-008/Assumptions. (Amended later for the timestamp-freshness sub-check only — see `research.md` Decision 4 Amendment.)

**Scale/Scope**: Touches `internal/adapter/delivery/http/middleware/snap_auth.go` (extend), a new middleware file for merchant bearer-token auth, and `cmd/api/main.go` (wire the new middleware onto the merchant route group). No changes to `vendor_config.go`, usecase, or domain layers.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Clean Architecture**: PASS — all changes are confined to the adapter/delivery (middleware) and infrastructure/config layers, which is exactly where HTTP-level auth concerns belong. No domain or usecase code is touched.
- **II. Configuration-Driven Integrations**: N/A — no new configuration is introduced; this feature reads the already-existing `VENDOR_CLIENT_SECRET` per vendor/channel, unconditionally.
- **III. TDD**: PASS (planned) — new unit tests for HMAC mismatch rejection, timestamp staleness, fail-closed on missing secret, and bearer-token rejection cases will be written first.
- **IV. Context-Aware Operations**: PASS — no new I/O; existing `ctx`-threaded request handling is unaffected since verification is pure computation against already-loaded config/keys.
- **VI/VII. Container Security & Zero Secrets Policy**: PASS — no new secret is introduced; `VENDOR_CLIENT_SECRET` and the JWT signing key are both already sourced through the existing config/credential mechanisms. This feature only adds verification logic, not new secret storage.
- **X. Idempotency**: PASS — unaffected; auth middleware runs independently of (and, in the routing chain, before/alongside) `IdempotencyMiddleware`, with no interaction with idempotency-key handling.
- **XI. Coverage > 90%**: PASS (planned) — new branches (signature match/mismatch, stale/fresh timestamp, missing-secret fail-closed, missing/invalid/expired bearer token) will be covered by new unit tests per Phase 2 tasks.

No violations requiring Complexity Tracking justification.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── adapter/
│   └── delivery/
│       └── http/
│           ├── middleware/
│           │   ├── snap_auth.go                 # Extend: HMAC verify + timestamp freshness, always enforced
│           │   ├── snap_auth_test.go             # New/updated tests
│           │   ├── merchant_auth.go              # NEW: bearer-token (JWT) middleware for merchant routes
│           │   └── merchant_auth_test.go         # NEW tests
│           └── handler/                          # No changes expected — auth happens upstream in middleware
cmd/
└── api/
    └── main.go                                   # Wire merchant_auth middleware onto the create-va/list/delete-va group only

specs/009-transfer-va-auth/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (auth-relevant request/response deltas)
└── tasks.md             # Phase 2 output (/speckit-tasks — not created here)
```

**Structure Decision**: Single Go backend project (existing Clean Architecture layout). This feature is confined to the adapter/delivery/middleware and infrastructure/config layers plus route wiring in `cmd/api/main.go` — no new packages beyond one new middleware file (`merchant_auth.go`), which mirrors the existing `snap_auth.go` file's placement and conventions.

## Complexity Tracking

*No Constitution Check violations — this section is not applicable.*
