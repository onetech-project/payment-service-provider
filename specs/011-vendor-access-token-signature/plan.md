# Implementation Plan: Vendor Access Token in Symmetric Signature

**Branch**: `011-vendor-access-token-signature` | **Date**: 2026-07-30 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/011-vendor-access-token-signature/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

`SNAPAuthMiddleware` (`internal/adapter/delivery/http/middleware/snap_auth.go`) currently hardcodes the `AccessToken` component of `stringToSign` to `""` for all vendor transfer-VA endpoints (`/transfer-va/inquiry`, `/payment`, `/status`), and never inspects an `Authorization` header. Feature 010 already solved the identical problem for merchant endpoints: `MerchantAuthMiddleware` parses `Authorization: Bearer <token>`, validates it via the shared `domain.JWTIssuer`, and feeds the real token into `stringToSign`. This feature extends `SNAPAuthMiddleware` the same way — but vendor "clients" today are static `config.VendorConfig` entries with no `client_id`/JWT linkage (unlike merchants, which are `client_apps` DB rows). This plan adds a `ClientID` field to `VendorConfig`, requires vendors to obtain a B2B access token via the existing `/access-token/b2b` endpoint using that `client_id`, and extends `SNAPAuthMiddleware` to validate the bearer token (confirming its `ClientID` claim matches the resolved vendor config) and bind it into the HMAC string-to-sign, replacing the empty-string placeholder.

## Technical Context

**Language/Version**: Go (latest stable, per constitution) — existing module `backbone-new`

**Primary Dependencies**: None new. Reuses `internal/domain.JWTIssuer` (`ValidateToken`) already wired for `MerchantAuthMiddleware`, and `internal/infrastructure/crypto` (`BuildStringToSign`, `NewHMACSigner`) already used by `SNAPAuthMiddleware`.

**Storage**: PostgreSQL — no new tables. Vendor `client_id` values are added to existing per-vendor configuration (env/config files loaded into `config.VendorConfig`), and each configured vendor `client_id` must have a corresponding `client_apps` row (and RSA key in `client_keys`) so `/access-token/b2b` can issue it a token — reusing the exact onboarding path merchants already use (feature 009/010), no new provisioning mechanism.

**Testing**: Go `testing` + existing middleware test patterns (`internal/adapter/delivery/http/middleware/snap_auth_test.go`, `merchant_auth_test.go`); per constitution Principle III (TDD) and XI (>90% coverage).

**Target Platform**: Linux server (existing Docker Compose stack: app, postgres, redis)

**Project Type**: Web service (single Go backend, Clean Architecture layers per constitution)

**Performance Goals**: One additional JWT validation (already-optimized in-process verification, no DB call — `ValidateToken` is cryptographic, not a lookup) per vendor request. Negligible added latency, same order of magnitude as the existing merchant bearer-token check.

**Constraints**: Must not change the wire format of the existing `X-SIGNATURE`/`X-TIMESTAMP`/`CHANNEL-ID`/`X-PARTNER-ID`/`X-EXTERNAL-ID` checks (spec scope is additive: new required header + signature component only). Must not alter merchant-side (`MerchantAuthMiddleware`) behavior. This is an intentionally breaking change for vendor clients per spec (User Story 3) — old-format (no `Authorization`) requests must be rejected with a distinguishable error, not silently accepted.

**Scale/Scope**: `internal/adapter/delivery/http/middleware/snap_auth.go` (add Authorization parsing + JWT validation + real-token stringToSign, gated on the already-existing `vendorConfig.ClientID` field), `cmd/api/main.go` (pass `jwtIssuer` into `SNAPAuthMiddleware`), plus vendor onboarding data (client_apps/client_keys rows per existing vendor, via existing admin provisioning) and vendor client script updates (`scripts/vendor-inquiry-va.sh`, `scripts/vendor-payment-va.sh`) to send `Authorization` and include the token in signing. No changes to `internal/infrastructure/config/vendor_config.go` (`ClientID` already exists and loads from `VENDOR_CLIENT_ID`), `internal/usecase/`, vendor business-logic handlers, or merchant-side code.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Clean Architecture**: PASS — `SNAPAuthMiddleware` stays in the adapter/delivery layer; it depends on the existing `domain.JWTIssuer` interface (already defined in `internal/domain`), no new domain/infrastructure coupling introduced. `VendorConfig.ClientID` is plain config data, not a new architectural layer.
- **II. Configuration-Driven Integrations**: PASS — the new `ClientID` field is added the same way all other vendor parameters (`PartnerID`, `ChannelID`, `ClientSecret`) already are: via `config.VendorConfig`, zero source-code changes to onboard a vendor's client_id.
- **III. TDD**: PASS (planned) — new tests for missing/malformed `Authorization`, invalid/expired token, token/signature mismatch, `ClientID` claim not matching the resolved vendor, and successful bound-token signature, written first.
- **IV. Context-Aware Operations**: PASS — `ValidateToken` and the surrounding middleware chain already operate within the request's `context.Context`; no new I/O call is introduced that needs its own context handling.
- **VII. Zero Secrets Policy**: N/A — no new secret is introduced; reuses the existing JWT signing key and vendor `ClientSecret` already governed under feature 009/010's precedent.
- **IX. Migrations**: N/A — no schema change. Vendor `client_apps`/`client_keys` rows are provisioned via the existing admin path used for merchant onboarding (feature 010), not a new migration.
- **XI. Coverage > 90%**: PASS (planned) — all new branches (header presence, JWT validity, ClientID-claim match, signature match with bound token) covered by new/extended unit tests in `snap_auth_test.go`.

## Project Structure

### Documentation (this feature)

```text
specs/011-vendor-access-token-signature/
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
└── adapter/
    └── delivery/
        └── http/
            └── middleware/
                ├── snap_auth.go                    # Add Authorization parsing, JWT validation via
                │                                     # jwtIssuer.ValidateToken, ClientID-claim match against
                │                                     # resolved VendorConfig.ClientID, real token in stringToSign
                └── snap_auth_test.go                # New/extended test cases

cmd/
└── api/
    └── main.go                                     # Pass jwtIssuer into SNAPAuthMiddleware(vc, jwtIssuer)

scripts/
├── vendor-inquiry-va.sh                            # Send Authorization header; include token in STRING_TO_SIGN
└── vendor-payment-va.sh                            # Same

specs/011-vendor-access-token-signature/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md         # Phase 1 output
├── contracts/            # Phase 1 output
└── tasks.md              # Phase 2 output (/speckit-tasks — not created here)
```

**Structure Decision**: Single Go backend project (existing Clean Architecture layout). The middleware change is a structural extension of `SNAPAuthMiddleware` mirroring the already-shipped `MerchantAuthMiddleware` pattern (feature 010) as closely as vendor config allows: same `Authorization: Bearer` parsing, same `jwtIssuer.ValidateToken` call, same "real token in `stringToSign`" behavior. The one new piece is `VendorConfig.ClientID`, needed because vendor identity today is config-based rather than a `client_apps` DB row — this field is the link that lets a vendor's issued JWT be validated against the config entry that signs its requests.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|---------------------------------------|
| N/A — no unjustified violations. | | |
