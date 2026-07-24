# Feature Specification: Swagger API Documentation & Testing

**Feature Branch**: `005-swagger-api-documentation`

**Created**: 2026-07-24

**Status**: Draft

**Input**: User description: "implement swagger untuk api documentation dan testing di semua endpoint lengkap dengan explanation, example request, example response, error code, properties"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse Complete API Reference (Priority: P1)

An integrating developer (internal or a merchant/partner engineer) opens the Swagger documentation page and sees every API endpoint exposed by the payment service provider — health check, SNAP token issuance, admin client onboarding, signature utilities, virtual account transfer operations, and merchant VA dashboard operations — organized by category, each with a clear explanation of what it does.

**Why this priority**: Without a complete, discoverable list of endpoints, developers cannot self-serve integration and must ask the team directly, which is the core problem this feature solves.

**Independent Test**: Can be fully tested by opening the Swagger UI URL and confirming every endpoint currently registered in the router appears, grouped logically, with a human-readable description.

**Acceptance Scenarios**:

1. **Given** the service is running, **When** a developer navigates to the Swagger UI endpoint, **Then** the page loads and lists all endpoints (health, token, admin clients, signature utilities, transfer-VA, merchant VA dashboard).
2. **Given** the Swagger UI is open, **When** a developer expands any endpoint, **Then** they see a plain-language explanation of its purpose.

---

### User Story 2 - Try Out Requests Directly From the Docs (Priority: P1)

A developer wants to verify how an endpoint behaves without leaving the documentation. They fill in parameters/body in the Swagger UI "try it out" panel, execute the request against a running instance, and see the real response.

**Why this priority**: Interactive testing is explicitly requested ("dan testing") and is what turns documentation into a functional integration tool, reducing round-trips to a separate API client (e.g. Postman).

**Independent Test**: Can be fully tested by using the "Try it out" feature on at least one endpoint per protection tier (public, SNAP-signed, admin-authenticated) and confirming a real HTTP call is made and the actual response is displayed.

**Acceptance Scenarios**:

1. **Given** an endpoint that requires no special auth (e.g. health check), **When** the developer clicks "Try it out" and executes, **Then** the actual response is shown in the UI.
2. **Given** an endpoint that requires an auth header/signature (SNAP or admin), **When** the developer supplies the required header value(s) in the UI, **Then** the request is sent with those headers and the real response (success or auth error) is displayed.

---

### User Story 3 - Understand Request/Response Shape and Error Codes (Priority: P2)

A developer implementing a client needs to know the exact request body fields (with types and whether required), the exact response body fields on success, and the full set of error codes/messages an endpoint can return, so they can build correct request payloads and handle every documented failure case.

**Why this priority**: This is the detailed contract information developers need to write working integration code and correct error handling; it's the depth layer on top of the discoverability from User Story 1.

**Independent Test**: Can be fully tested by picking any endpoint and confirming its documented request schema, example request payload, example success response payload, and list of error responses (status code + error body) match what the running service actually accepts/returns.

**Acceptance Scenarios**:

1. **Given** any endpoint's documentation, **When** a developer views the request section, **Then** each field is listed with its name, type, whether it is required, and a description.
2. **Given** any endpoint's documentation, **When** a developer views the response section, **Then** a realistic example success response body is shown alongside a description of each field.
3. **Given** any endpoint's documentation, **When** a developer views the error section, **Then** every error/status code the endpoint can return is listed with an example error body and explanation of when it occurs.

---

### Edge Cases

- What happens when an endpoint has multiple possible error codes for the same failure category (e.g. SNAP signature validation vs. expired token)? Each MUST be documented as a distinct error entry, not merged.
- How does the documentation handle endpoints protected by custom SNAP signature headers that can't be trivially filled in through a generic "Authorize" field? The docs MUST explain how to compute/obtain the header value (or link to the signature-generation utility endpoints already in the API).
- What happens when the API surface changes (an endpoint is added, removed, or its schema changes) after the docs are generated? The documentation generation MUST be tied to the source code (not maintained as a hand-written separate document) so it cannot silently drift out of sync.
- How does the documentation distinguish sandbox/testing use of "Try it out" from executing a real, potentially state-changing action (e.g. creating or deleting a real virtual account) against a live environment? This MUST be clearly called out per endpoint that mutates data.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose a Swagger/OpenAPI documentation UI reachable via a browser at a dedicated URL path on the running service.
- **FR-002**: The documentation MUST include every HTTP endpoint currently registered by the service (health check, B2B access-token issuance, admin client/key management, signature-generation utilities, virtual account inquiry/payment/status, and merchant VA create/list/delete).
- **FR-003**: Each endpoint MUST include a plain-language explanation of its purpose and business context.
- **FR-004**: Each endpoint MUST document its request: HTTP method, path (including path/query parameters), required headers, and request body schema with field name, type, required/optional, and description.
- **FR-005**: Each endpoint MUST include at least one realistic example request (headers + body) showing valid sample values.
- **FR-006**: Each endpoint MUST document its success response: status code, response body schema (field name, type, description), and at least one realistic example success response body.
- **FR-007**: Each endpoint MUST document every error/failure response it can return: status code, error response body schema/example, and a description of the condition that triggers it (covering, at minimum, validation errors, authentication/signature failures, not-found/duplicate conditions, and downstream/vendor failures where applicable).
- **FR-008**: The documentation UI MUST allow a developer to execute a real request against the running service directly from the docs ("try it out") and view the actual response returned.
- **FR-009**: The documentation MUST clearly indicate, per endpoint, which authentication/signature scheme applies (none, SNAP client signature, admin auth) and how to supply the required credentials/headers when trying the endpoint out.
- **FR-010**: The documentation content (endpoint list, schemas, examples) MUST be generated from source-code annotations/definitions co-located with the handlers, not maintained as a separate hand-written document, so it can be regenerated as the API evolves.
- **FR-011**: System MUST clearly flag endpoints that perform real, state-changing actions (e.g. creating or deleting a virtual account) so a developer using "try it out" understands the action is not a simulation.
- **FR-012**: The documentation UI MUST be organized into logical groups/tags matching the endpoint categories (e.g. Token/Auth, Admin Client Management, Signature Utilities, Virtual Account, Merchant VA Dashboard).

### Key Entities

- **API Endpoint Definition**: A single documented route — method, path, tag/group, summary, description, auth requirement, request schema, response schemas (success and per error code), examples.
- **Error Code Entry**: A documented failure outcome for an endpoint — HTTP status, application-level error code (if any), message, example body, triggering condition.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the service's currently registered HTTP endpoints appear in the documentation.
- **SC-002**: For every documented endpoint, a developer can find an example request and an example success response without leaving the documentation page.
- **SC-003**: For every documented endpoint, all error responses it can realistically return are listed with example bodies, verified against the endpoint's actual error-handling code.
- **SC-004**: A developer unfamiliar with the API can successfully execute a "try it out" call against at least one endpoint from each auth tier (public, SNAP-signed, admin) without needing to consult a separate tool.
- **SC-005**: A new engineer can locate the documentation for any given endpoint and correctly construct a valid request payload on their first attempt, without asking a teammate.

## Assumptions

- Documentation is generated for the HTTP endpoints as they exist in the codebase today; endpoints added later are expected to follow the same annotation pattern to stay included (ongoing maintenance is out of scope for this feature's completion criteria).
- The Swagger UI is exposed on the same running service (not a separately hosted static site), reachable at least in local/dev and staging environments; whether it is enabled in production is an operational/security decision left to existing environment configuration conventions and not dictated by this spec.
- "Testing" refers to the interactive "try it out" capability of Swagger UI making real HTTP calls to the running instance — not a separate automated test suite.
- Error code documentation is sourced from each handler's actual error-returning logic (existing error types/status codes in the codebase), not invented values.
- Non-English (Bahasa Indonesia) terms in the request are addressed by delivering the documentation in English, consistent with the existing codebase's language.
