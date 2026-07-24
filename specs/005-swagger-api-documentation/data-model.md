# Phase 1 Data Model: Swagger API Documentation & Testing

This feature documents existing request/response DTOs; it does not introduce new persisted entities. The "data model" here is the set of OpenAPI schema objects derived from existing Go structs in `internal/domain` and handler-local DTOs.

## API Endpoint Definition (conceptual, expressed as OpenAPI annotations per handler)

| Field | Description |
|---|---|
| `method` + `path` | e.g. `POST /openapi/v1.0/transfer-va/inquiry` |
| `tag` | Group: Health, Token, Admin Client Management, Signature Utilities, Virtual Account, Merchant VA Dashboard |
| `summary` | One-line purpose |
| `description` | Plain-language explanation, business context, side effects (state-changing flag) |
| `security` | References to `securityDefinitions` entries applicable (none / SNAP headers / admin token) |
| `parameters` | Path/query/header/body params, each with name, type, required flag, description |
| `responses` | Map of HTTP status → schema + example, covering success and every documented `@Failure` |

## Error Response Model

Derived from the existing shared shape used across SNAP handlers (`ResponseCode` + `ResponseMessage`, seen in `domain.VAInquiryResponse` and sibling response DTOs) and the admin/token handlers' error DTOs.

| Field | Type | Required | Description |
|---|---|---|---|
| `responseCode` | string | yes | SNAP/ASPI numeric-alpha response code (e.g. `4002401`) identifying the error category |
| `responseMessage` | string | yes | Human-readable error message |

Admin and token endpoints that use a differently-shaped error DTO are documented with their own existing shape (introspected per-handler during implementation) rather than forced into the SNAP shape.

## Endpoint Inventory (from current router registration)

| Tag | Method | Path | Handler | State-changing |
|---|---|---|---|---|
| Health | GET | `/health` | inline in `main.go` | No |
| Token | POST | `/openapi/v1.0/access-token/b2b` | `token_handler.GetB2BAccessToken` | No (issues a token) |
| Admin Client Management | POST | `/admin/clients` | `client_handler.RegisterClient` | Yes |
| Admin Client Management | POST | `/admin/clients/:clientId/keys` | `client_handler.AddClientKey` | Yes |
| Admin Client Management | DELETE | `/admin/clients/:clientId/keys/:keyId` | `client_handler.RevokeClientKey` | Yes |
| Signature Utilities | POST | `/api/v1/utilities/signature-auth` | `signature_handler.GenerateAccessTokenSignature` | No |
| Signature Utilities | POST | `/api/v1/utilities/signature-service` | `signature_handler.GenerateServiceSignature` | No |
| Virtual Account | POST | `/openapi/v1.0/transfer-va/inquiry` | `va_handler.Inquiry` | No |
| Virtual Account | POST | `/openapi/v1.0/transfer-va/payment` | `va_handler.Payment` | Yes |
| Virtual Account | POST | `/openapi/v1.0/transfer-va/status` | `va_handler.Status` | No |
| Merchant VA Dashboard | POST | `/openapi/v1.0/transfer-va/create-va` | `merchant_va_handler.CreateVA` | Yes |
| Merchant VA Dashboard | POST | `/openapi/v1.0/transfer-va/list` | `merchant_va_handler.ListVA` | No |
| Merchant VA Dashboard | DELETE | `/openapi/v1.0/transfer-va/delete-va` | `merchant_va_handler.DeleteVA` | Yes |

Each row above becomes a documented endpoint per FR-002; exact request/response/error schemas are sourced from the corresponding domain DTOs during annotation authorship (Phase implementation), not invented here.
