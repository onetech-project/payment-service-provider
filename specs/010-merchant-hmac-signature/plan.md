# Implementation Plan: Merchant HMAC Signature Verification (ASPI-Compliant Two-Factor Auth)

**Branch**: `010-merchant-hmac-signature` | **Date**: 2026-07-30 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/010-merchant-hmac-signature/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Add a second, mandatory verification layer to the merchant-facing endpoints (`create-va`/`list`/`delete-va`) that `MerchantAuthMiddleware` (feature 009) already gates on a bearer `accessToken`: HMAC-SHA512 signature verification, mirroring `SNAPAuthMiddleware`'s vendor-side implementation exactly, except the `AccessToken` component of `stringToSign` is the real bearer token (since it is genuinely transmitted via `Authorization` on these endpoints) rather than an empty string. This requires a new persistence layer for merchant shared secrets — a `client_secrets` table keyed by the same `client_id` already used for `accessToken` issuance, following the exact structural pattern of the existing `client_keys` table (RSA public keys). `MerchantAuthMiddleware` is extended to, after validating the bearer token, look up that client's active secret and verify the signature; both checks must pass.

## Technical Context

**Language/Version**: Go (latest stable, per constitution) — existing module `backbone-new`

**Primary Dependencies**: None new. Reuses `internal/infrastructure/crypto/hmac.go` (`HMACSigner.Verify`, `BuildStringToSign`, `HashSHA256Hex` — already used by `SNAPAuthMiddleware`), and follows the existing `internal/domain/client.go` (`ClientRepository`, `ClientKey`) pattern for the new secret-storage type.

**Storage**: PostgreSQL — new migration adding a `client_secrets` table, structurally identical to the existing `client_keys` table (`client_id` FK to `client_apps`, `is_active` flag, timestamps) but storing a shared secret instead of a public key PEM.

**Testing**: Go `testing` + existing middleware/repository unit test patterns (`internal/adapter/delivery/http/middleware/merchant_auth_test.go`, `internal/infrastructure/database/*_test.go`); per constitution Principle III (TDD) and XI (>90% coverage).

**Target Platform**: Linux server (existing Docker Compose stack: app, postgres, redis)

**Project Type**: Web service (single Go backend, Clean Architecture layers per constitution)

**Performance Goals**: One additional indexed DB lookup (client secret by client_id) per merchant request, plus HMAC-SHA512 computation (single-digit microseconds) — negligible added latency, same order of magnitude as the existing bearer-token validation this feature runs alongside.

**Constraints**: Must not change merchant endpoint request/response contracts or business logic (spec FR-008). Must not alter vendor-side (`SNAPAuthMiddleware`) behavior at all (spec FR-009). No enable/disable configuration — unconditional from deploy (spec FR-010), consistent with feature 009's precedent. (Amended later for the timestamp-freshness check only — see `specs/009-transfer-va-auth/research.md` Decision 4 Amendment.)

**Scale/Scope**: New migration (`client_secrets` table), `internal/domain/client.go` (extend `ClientRepository` + add `ClientSecret` type), `internal/infrastructure/database/` (repository implementation), `internal/adapter/delivery/http/middleware/merchant_auth.go` (extend to add signature verification after token validation), plus an operator-facing provisioning path (CLI/seed script or admin endpoint — decided below). No changes to `internal/usecase/merchant_va_usecase.go`, handlers, or vendor-side code.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Clean Architecture**: PASS — new `ClientSecret` type and repository method live in `internal/domain` (interface) and `internal/infrastructure/database` (implementation), matching the existing `ClientKey`/`client_keys` pattern exactly. Middleware extension stays in the adapter/delivery layer.
- **II. Configuration-Driven Integrations**: N/A — this is about a first-party client credential (like `client_keys`), not a pluggable external-provider integration; no code change is needed to onboard a new merchant once the provisioning path exists.
- **III. TDD**: PASS (planned) — new tests for signature match/mismatch, missing-secret fail-closed, timestamp freshness, and dual-check (token+signature) combinations written first.
- **IV. Context-Aware Operations**: PASS — the new repository lookup accepts `ctx context.Context` and is called from the existing per-request middleware chain, consistent with `GetActiveClientPublicKey`'s existing signature.
- **VII. Zero Secrets Policy**: Reuses the exact same storage approach already in place for `VENDOR_CLIENT_SECRET` (a plaintext-at-rest shared secret, not a full Vault-backed credential) and for `client_keys` (public key material in Postgres) — consistent with existing precedent in this codebase rather than introducing a new, inconsistent secret-handling standard. Flagged in Complexity Tracking below since it is a deviation from the letter of the principle, matching a pre-existing pattern rather than a new one.
- **IX. Migrations**: PASS — new table added via a paired `NNNNNN_description.up.sql`/`.down.sql` migration under `db/migrations/`, per constitution.
- **XI. Coverage > 90%**: PASS (planned) — new branches (signature match/mismatch, missing secret, stale timestamp, token-valid/signature-invalid and vice versa) covered by new unit tests.

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
db/
└── migrations/
    ├── NNNNNN_create_client_secrets.up.sql    # New table, mirrors 000002_create_client_keys structurally
    └── NNNNNN_create_client_secrets.down.sql

internal/
├── domain/
│   └── client.go                               # Add ClientSecret type, extend ClientRepository interface,
│                                                 # add AddClientSecretRequest (mirrors AddClientKeyRequest)
├── infrastructure/
│   └── database/
│       ├── client_repository.go                # Implement GetActiveClientSecret/CreateClientSecret/RevokeClientSecret
│       └── client_repository_test.go            # New tests
├── usecase/
│   ├── client_usecase.go                       # Add AddClientSecret/RevokeClientSecret methods (mirrors AddClientKey)
│   └── client_usecase_test.go
└── adapter/
    └── delivery/
        └── http/
            ├── handler/
            │   ├── client_handler.go            # Add AddClientSecret/RevokeClientSecret admin handlers
            │   └── client_handler_test.go
            └── middleware/
                ├── merchant_auth.go              # Extend: after token validation, verify HMAC signature
                └── merchant_auth_test.go

cmd/
└── api/
    └── main.go                                   # Register POST/DELETE /admin/clients/:clientId/secret;
                                                     # pass ClientRepository into MerchantAuthMiddleware

specs/010-merchant-hmac-signature/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md         # Phase 1 output
├── contracts/            # Phase 1 output
└── tasks.md              # Phase 2 output (/speckit-tasks — not created here)
```

**Structure Decision**: Single Go backend project (existing Clean Architecture layout). Persistence and admin-provisioning code mirror the existing `client_keys`/`AddClientKey`/`RevokeClientKey` pattern exactly (new parallel `client_secrets` table and `AddClientSecret`/`RevokeClientSecret` methods/endpoints), so no new architectural pattern is introduced — this is a structural clone of proven, already-reviewed code. The only middleware change is extending `merchant_auth.go` (feature 009) to add a signature check after its existing token check.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|---------------------------------------|
| Merchant shared secret stored as plaintext in PostgreSQL (Principle VII, "Zero Secrets Policy" nominally calls for an external Vault) | Matches the existing, already-shipped precedent for both `VENDOR_CLIENT_SECRET` (plaintext in `.env.<vendor>.<channel>` files) and `client_keys.public_key_pem` (key material in Postgres, unencrypted) — introducing Vault-backed storage for only this one new secret type would create three inconsistent secret-handling standards in the same codebase for conceptually equivalent data | A full Vault/External Secret Store integration is a project-wide infrastructure change (affects `VENDOR_CLIENT_SECRET` and the JWT signing key too, per Principle VII's literal scope) — out of scope for a feature whose purpose is closing an auth-verification gap, not overhauling secret storage. Tracked as pre-existing technical debt, not introduced by this feature. |
