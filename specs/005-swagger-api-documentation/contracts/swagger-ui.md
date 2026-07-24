# Contract: Swagger Documentation Interface

This feature's "contract" is the documentation interface itself, exposed by the running service.

## Route Contract

- `GET /swagger/index.html` (and `/swagger/*` asset paths) — serves the interactive Swagger UI, backed by `swaggo/echo-swagger`.
- `GET /swagger/doc.json` — serves the generated OpenAPI 2.0 spec document consumed by the UI (and available for external tooling, e.g. Postman import).

## Generation Contract

- Source of truth: Go doc-comment annotations directly above each Echo handler method in `internal/adapter/delivery/http/handler/*.go`, plus a package-level general API info block (title, version, description, base path, security definitions) in `cmd/api/main.go` or a dedicated `docs.go`.
- Generation command: `swag init` (via `swaggo/swag` CLI), invoked through a Makefile target (e.g. `make swagger`) and/or `go generate` directive, producing `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`.
- The generated `docs` package is imported (blank import) in `cmd/api/main.go` so `echo-swagger` can serve it.
- Regenerating docs MUST NOT require hand-editing the generated files — only handler annotations change.

## Security Definitions Contract

| Name | Type | Header | Applies to |
|---|---|---|---|
| `SnapClientKey` | apiKey (header) | `X-CLIENT-KEY` | Virtual Account, Merchant VA Dashboard endpoints |
| `SnapTimestamp` | apiKey (header) | `X-TIMESTAMP` | Virtual Account, Merchant VA Dashboard endpoints |
| `SnapSignature` | apiKey (header) | `X-SIGNATURE` | Virtual Account, Merchant VA Dashboard endpoints |
| `BearerAuth` | apiKey (header) | `Authorization` | Endpoints requiring a prior B2B access token |
| `AdminToken` | apiKey (header) | (existing admin auth header, per `admin_auth.go`) | Admin Client Management endpoints |

Exact header names/schemes are confirmed against `internal/adapter/delivery/http/middleware/snap_auth.go` and `admin_auth.go` during implementation; this table is the authoring reference, not the final source of truth (the middleware code is).

## Response Contract

Every documented endpoint MUST declare:
- Exactly one 2xx success response with schema + example.
- One or more non-2xx `@Failure` responses, each mapped to an actual error branch in the handler/usecase, with schema + example.

## Non-goals

- This contract does not define new business-logic behavior or change any endpoint's runtime request/response shape — it only documents the existing shapes.
