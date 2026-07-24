# Phase 0 Research: Swagger API Documentation & Testing

## Decision: Documentation generation approach

**Decision**: Use `swaggo/swag` to generate an OpenAPI 2.0 (Swagger) spec from Go doc-comment annotations placed directly above each Echo handler function, served via `swaggo/echo-swagger` mounted at `GET /swagger/*` (or `/docs/*`) using Echo's native routing.

**Rationale**:
- swaggo/swag is the de facto standard for Go + Echo projects; it parses `// @Summary`, `// @Description`, `// @Param`, `// @Success`, `// @Failure`, `// @Router` comments co-located with each handler, satisfying FR-010 (docs generated from source, not hand-maintained).
- `echo-swagger` is a first-party-style middleware wrapping `swaggo/http-swagger`, integrates directly with the existing `echo.Echo` instance without adding a second HTTP server or framework.
- Generated `docs/swagger.json` / `docs/docs.go` can be produced by a `go generate`/Makefile step (`swag init`), keeping annotation authorship in Go source and regeneration a build-time concern — no runtime reflection needed, no violation of Clean Architecture (annotations live in the `adapter/delivery/http/handler` layer only, per Constitution Principle I).
- Swagger UI (bundled by echo-swagger) provides the interactive "try it out" capability required by FR-008 out of the box, including a global "Authorize" button for header-based auth schemes (usable for SNAP signature headers and admin auth headers as `apiKey`-type security definitions).

**Alternatives considered**:
- **Hand-written OpenAPI YAML maintained separately**: Rejected — violates FR-010 (docs would drift from code, exactly the risk flagged in spec Edge Cases); doubles maintenance effort.
- **`ogen`/`oapi-codegen` (spec-first / contract-first code generation)**: Rejected — would require inverting the existing flow (handlers already exist; contract-first tools generate server stubs from YAML, which is a much larger refactor than annotating existing handlers) and isn't idiomatic for the current codebase's handler-first structure.
- **Postman collection instead of Swagger UI**: Rejected — user explicitly asked for "swagger"; Postman collections aren't generated from source and would be a second document to maintain.

## Decision: Auth handling in Swagger UI for SNAP-signed endpoints

**Decision**: Model each auth tier as an OpenAPI `securityDefinitions` entry of type `apiKey` (header-based): `X-SIGNATURE`, `X-TIMESTAMP`, `X-CLIENT-KEY`, `Authorization` (Bearer, for the B2B access-token-protected endpoints) as applicable per SNAP spec, and a separate `apiKey` header definition for the admin-auth token. Each endpoint's `@Security` annotation references the definitions it needs. The endpoint description text explains that the SNAP signature value must be computed externally (e.g. using the existing `/api/v1/utilities/signature-auth` and `/api/v1/utilities/signature-service` endpoints) before pasting it into Swagger UI's "Authorize" dialog or the parameter field directly.

**Rationale**: Swagger/OpenAPI 2.0 has no native support for expressing "compute this signature dynamically," so the pragmatic approach — consistent with how other SNAP/ASPI implementations document this — is to treat the signature as an opaque header value the caller supplies, with prose guidance pointing at the signature-generation utility endpoints already present in the API. This satisfies FR-009 without inventing unsupported OpenAPI constructs.

**Alternatives considered**: Custom Swagger UI plugin to auto-compute signatures client-side — rejected as excessive complexity/YAGNI for a documentation feature; out of scope per Constitution Principle I (KISS/YAGNI).

## Decision: Error response documentation source of truth

**Decision**: For each handler, enumerate `@Failure` annotations by reading the handler's actual error-return branches (existing domain error types / echo.NewHTTPError calls / validation error paths) rather than inventing a generic error list. Where a shared error-response DTO is used across handlers (e.g. a common `ErrorResponse{Code, Message}` shape), define it once as an OpenAPI model and reference it from every `@Failure` line to avoid duplication (DRY, Constitution Principle I).

**Rationale**: Directly satisfies FR-007 and SC-003 (error docs verified against actual error-handling code). Avoids fabricated error codes that would mislead integrators.

**Alternatives considered**: Generic "4xx/5xx, see logs" documentation — rejected, fails FR-007's requirement for per-condition documented errors.

## Decision: Exposure environment

**Decision**: Swagger UI route is registered unconditionally in the Echo router (like the other routes in `cmd/api/main.go`), consistent with the existing codebase pattern of no environment-gated route registration observed for other endpoints. Whether it is reachable from the public internet in production remains governed by existing network/ingress configuration (out of scope for this feature, per spec Assumptions).

**Rationale**: Matches existing code structure (all routes registered directly in `main.go` with middleware, no env-conditional route branching found); keeps the change minimal and consistent (YAGNI — don't add new environment-gating machinery not requested by the spec).

**Alternatives considered**: Env-var toggle to disable Swagger UI in production — not requested by spec; can be added later without blocking this feature; noted as a natural extension point but not implemented now.
