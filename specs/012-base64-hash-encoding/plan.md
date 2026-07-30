# Implementation Plan: Base64 Hash/Signature Encoding Standardization

**Branch**: `012-base64-hash-encoding` | **Date**: 2026-07-30 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/012-base64-hash-encoding/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Every hex-encoded hash/signature primitive in `internal/infrastructure/crypto/hmac.go` (`HashSHA256Hex`, `HMACSigner.Sign`/`Verify`) and its two hand-rolled duplicates (`idempotency.go`'s payload hash, `payment_notification_worker.go`'s webhook signature) switches to base64. This is a pure encoding swap — SHA-256 and HMAC-SHA512 stay the same — touching: the shared `crypto` package, all 4 callers of `HashSHA256Hex`/`Sign`/`Verify` (`snap_auth.go`, `merchant_auth.go`, `signature_usecase.go`, `snap/client.go`), the webhook worker, the idempotency middleware, all 5 vendor/merchant shell scripts that hand-compute `BODY_HASH`/`SIGNATURE` via `openssl dgst ... -hex`, and the two onboarding docs. RSA-based signing (`rsa_signer.go`/`rsa_verifier.go`, used for the B2B access-token endpoint) is already base64 and untouched.

## Technical Context

**Language/Version**: Go (latest stable, per constitution) — existing module `backbone-new`; shell (bash) for `scripts/*.sh`

**Primary Dependencies**: None new. Uses Go's existing `encoding/base64` (already imported in `hmac.go` for the pre-existing, currently-unused `SignBase64` method) instead of `encoding/hex`. Shell scripts switch from `openssl dgst -sha256 -hex` / `-sha512 -hmac ... -hex` to `openssl dgst -sha256/-sha512 -hmac ... -binary | openssl base64 -A` (or equivalent).

**Storage**: N/A — no schema/data changes. The idempotency payload hash is a short-TTL Redis cache value (per spec Edge Cases, no migration needed for in-flight records).

**Testing**: Go `testing` + existing unit test patterns for `hmac_test.go`, `snap_auth_test.go`, `merchant_auth_test.go`, `idempotency_test.go`, `payment_notification_worker_test.go`, `signature_usecase_test.go`; per constitution Principle III (TDD) and XI (>90% coverage). Shell script changes validated per quickstart.md against a live stack (same approach used in feature 011).

**Target Platform**: Linux server (existing Docker Compose stack: app, postgres, redis)

**Project Type**: Web service (single Go backend, Clean Architecture layers per constitution)

**Performance Goals**: No measurable change — base64 encoding of a 32-byte (SHA-256) or 64-byte (SHA-512 HMAC) digest is the same order of magnitude of work as hex encoding; negligible either way.

**Constraints**: No dual-encoding compatibility mode (spec FR-003, Assumptions) — this is an intentional breaking change, consistent with prior breaking-change features (009, 010, 011) in this system. Must not change SHA-256/HMAC-SHA512/RSA algorithm choices (FR-007). RSA-based B2B token-request signing is out of scope (already base64).

**Scale/Scope**: `internal/infrastructure/crypto/hmac.go` (core encoding change), `internal/adapter/delivery/http/middleware/snap_auth.go`, `internal/adapter/delivery/http/middleware/merchant_auth.go`, `internal/adapter/delivery/http/middleware/idempotency.go`, `internal/adapter/delivery/worker/payment_notification_worker.go`, `internal/usecase/signature_usecase.go`, `internal/adapter/gateway/snap/client.go`, plus `scripts/vendor-inquiry-va.sh`, `scripts/vendor-payment-va.sh`, `scripts/merchant-create-va.sh`, `scripts/merchant-delete-va.sh`, `scripts/merchant-list-va.sh`, and `docs/guides/merchant-onboarding.md` / `docs/guides/vendor-onboarding.md`. No changes to `internal/infrastructure/crypto/rsa_signer.go`/`rsa_verifier.go` (already base64) or the B2B token-issuance flow.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Clean Architecture**: PASS — change is confined to the existing `infrastructure/crypto` package and its existing callers in `adapter`/`usecase` layers; no new layers, no new cross-layer dependencies introduced.
- **II. Configuration-Driven Integrations**: N/A — this is an encoding convention change to first-party crypto primitives, not a per-provider integration parameter.
- **III. TDD**: PASS (planned) — update existing `hmac_test.go`/`snap_auth_test.go`/`merchant_auth_test.go`/`idempotency_test.go`/`payment_notification_worker_test.go`/`signature_usecase_test.go` assertions to base64 fixtures first (they will fail against unchanged code), then flip the implementation.
- **IV. Context-Aware Operations**: N/A — pure encoding change, no new I/O or context-carrying calls introduced.
- **VII. Zero Secrets Policy**: N/A — no secret-handling behavior changes, only textual encoding of derived hash/signature values.
- **IX. Migrations**: N/A — no schema change; idempotency cache entries are short-TTL and self-expire (spec Edge Cases), no backfill needed.
- **XI. Coverage > 90%**: PASS (planned) — every changed branch (hash encoding, signature verify, webhook signing, idempotency mismatch detection) already has existing test coverage that will be updated in place, not net-new untested code.

## Project Structure

### Documentation (this feature)

```text
specs/012-base64-hash-encoding/
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
├── infrastructure/
│   └── crypto/
│       ├── hmac.go                                # HashSHA256Hex -> HashSHA256Base64 (rename + re-impl);
│       │                                            # Sign/Verify switch to base64 (SignBase64 logic becomes
│       │                                            # the default; old hex Sign/Verify removed, no dual mode)
│       └── hmac_test.go                            # Update fixtures/assertions to base64
├── usecase/
│   ├── signature_usecase.go                        # GenerateServiceSignature: HashSHA256Hex -> HashSHA256Base64
│   └── signature_usecase_test.go
├── adapter/
│   ├── delivery/
│   │   ├── http/
│   │   │   └── middleware/
│   │   │       ├── snap_auth.go                    # bodyHash via HashSHA256Base64
│   │   │       ├── snap_auth_test.go
│   │   │       ├── merchant_auth.go                # bodyHash via HashSHA256Base64
│   │   │       ├── merchant_auth_test.go
│   │   │       ├── idempotency.go                  # payloadHash: hex.EncodeToString -> base64.StdEncoding
│   │   │       └── idempotency_test.go
│   │   └── worker/
│   │       ├── payment_notification_worker.go       # webhook signature: hex.EncodeToString -> base64
│   │       └── payment_notification_worker_test.go
│   └── gateway/
│       └── snap/
│           ├── client.go                            # outbound bodyHash via HashSHA256Base64, Sign already
│           │                                          # switches via shared HMACSigner.Sign change
│           └── client_test.go

scripts/
├── vendor-inquiry-va.sh                             # BODY_HASH/SIGNATURE via openssl ... -binary | base64
├── vendor-payment-va.sh                             # same
├── merchant-create-va.sh                            # same
├── merchant-delete-va.sh                            # same
└── merchant-list-va.sh                              # same

docs/
└── guides/
    ├── merchant-onboarding.md                       # bodyHash/signature formula text updated to base64
    └── vendor-onboarding.md                         # same

specs/012-base64-hash-encoding/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md         # Phase 1 output
├── contracts/            # Phase 1 output
└── tasks.md              # Phase 2 output (/speckit-tasks — not created here)
```

**Structure Decision**: Single Go backend project (existing Clean Architecture layout). The core change is a rename+reimplementation inside the existing `crypto` package (one function, one method pair), fanning out to its existing call sites — no new architectural pattern, no new files. The shell-script and doc changes are the client-facing mirror of the same change, following the same pattern feature 011 used for coordinated script updates.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|---------------------------------------|
| N/A — no unjustified violations. | | |
