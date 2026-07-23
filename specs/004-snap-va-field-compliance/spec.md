# Feature Specification: SNAP VA Field & Validation Compliance Fix

**Feature Branch**: `004-snap-va-field-compliance`

**Created**: 2026-07-23

**Status**: Draft

**Input**: User description: "Fix SNAP ASPI Virtual Account field/validation bugs that break vendor compatibility: (1) inquiry field typo `trxDateInit` vs spec's `txnDateInit`, (2) missing mandatory `amount` field on Inquiry, (3) Payment endpoint requiring a non-spec `transactionDate` field while not properly enforcing the spec's actual mandatory fields (`paymentRequestId`, `paidAmount`) and wrongly requiring `totalAmount`."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Vendor sends spec-compliant Inquiry request (Priority: P1)

A partner bank/vendor system integrating via the SNAP ASPI Virtual Account standard sends an Inquiry request (Service Code 24) using the exact field names and mandatory fields defined in the ASPI spec, including `txnDateInit` and `amount`.

**Why this priority**: This is the most fundamental compliance gap — if the field name doesn't match, the value silently parses as empty/nil with no error, causing incorrect downstream behavior (e.g., wrong transaction date recorded) without any visible failure. This is the highest-risk, hardest-to-detect class of bug.

**Independent Test**: Send an Inquiry request payload containing `txnDateInit` and `amount` exactly as specified in the ASPI spec; verify both values are correctly parsed, validated, and reflected in internal processing/logs — not silently dropped.

**Acceptance Scenarios**:

1. **Given** a vendor sends an Inquiry request with `txnDateInit` set to a valid timestamp, **When** the request is processed, **Then** the system parses and stores that timestamp value (not nil/empty).
2. **Given** a vendor sends an Inquiry request without the mandatory `amount` field, **When** the request is validated, **Then** the system rejects it with a clear mandatory-field validation error referencing `amount`.
3. **Given** a vendor sends an Inquiry request with a valid `amount` field, **When** the request is processed, **Then** the system validates and makes the amount available to downstream inquiry logic.

---

### User Story 2 - Vendor sends spec-compliant Payment request (Priority: P2)

A vendor sends a Payment request (Service Code 25) using only the fields the ASPI spec defines as mandatory (`paymentRequestId`, `paidAmount`), without a `transactionDate` field (which does not exist in the spec — the spec's equivalent field is `trxDateTime`) and without necessarily including `totalAmount` (not mandatory per spec).

**Why this priority**: Directly blocks real vendor integrations — a spec-compliant payload from an actual vendor is always rejected today with a 400 error, making the Payment endpoint unusable end-to-end. Ranked after Story 1 because Inquiry is earlier in the transaction flow and its silent-failure nature is more insidious, but this defect is more immediately blocking.

**Independent Test**: Send a Payment request containing only `paymentRequestId` and `paidAmount` (omitting `transactionDate` and `totalAmount`); verify the request is accepted and processed successfully.

**Acceptance Scenarios**:

1. **Given** a vendor sends a Payment request with `paymentRequestId` and `paidAmount` but no `transactionDate` field, **When** the request is validated, **Then** the system does NOT reject it for a missing `transactionDate`.
2. **Given** a vendor sends a Payment request without `paymentRequestId`, **When** validated, **Then** the system rejects it with a mandatory-field error referencing `paymentRequestId`.
3. **Given** a vendor sends a Payment request without `paidAmount`, **When** validated, **Then** the system rejects it with a mandatory-field error referencing `paidAmount`.
4. **Given** a vendor sends a Payment request without `totalAmount`, **When** validated, **Then** the system accepts the request (since `totalAmount` is not mandatory per spec).

---

### Edge Cases

- What happens when a vendor sends both the old non-spec field name and the new spec-compliant field name in the same request (e.g., both `trxDateInit` and `txnDateInit`)? System should honor only the spec-compliant field name.
- How does the system handle an Inquiry `amount` value that is present but malformed (non-numeric, negative, wrong precision)? Should be rejected with a validation error, consistent with how other monetary fields are validated.
- How does the system handle existing internal callers/tests that currently rely on the old `trxDateInit` / `transactionDate` field names or the `totalAmount` mandatory check? These must be updated as part of this change since the field/validation names are being corrected, not aliased.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Inquiry request MUST accept and parse the transaction initiation date field using the spec-defined name `txnDateInit` (replacing the incorrectly named `trxDateInit`).
- **FR-002**: The Inquiry request MUST include an `amount` field that is bound from the incoming payload.
- **FR-003**: The Inquiry request validation MUST treat `amount` as mandatory and reject requests missing it with a clear "invalid mandatory field" style error identifying `amount`.
- **FR-004**: The Payment request validation MUST NOT require a `transactionDate` field, since it does not exist in the spec.
- **FR-005**: The Payment request validation MUST treat `paymentRequestId` as mandatory and reject requests missing it with a clear error identifying `paymentRequestId`.
- **FR-006**: The Payment request validation MUST treat `paidAmount` as mandatory and reject requests missing it with a clear error identifying `paidAmount`.
- **FR-007**: The Payment request validation MUST NOT treat `totalAmount` as mandatory.
- **FR-008**: Existing internal code paths, tests, and any stored/derived data that referenced the old field names (`trxDateInit`, `transactionDate`) or the old mandatory-field rule (`totalAmount` required) MUST be updated to match the corrected spec-aligned behavior, with no dual/legacy support retained.

### Key Entities

- **VAInquiryRequest**: Represents an incoming SNAP Inquiry request from a vendor; gains a mandatory `amount` field and corrects the transaction-date field name to `txnDateInit`.
- **VAPaymentRequest**: Represents an incoming SNAP Payment request from a vendor; its mandatory-field rules change to `paymentRequestId` + `paidAmount`, dropping the `transactionDate` requirement and the `totalAmount` requirement.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A spec-compliant Inquiry payload (using `txnDateInit` and `amount` as defined in the ASPI spec) is processed with both fields correctly recognized, in 100% of test cases.
- **SC-002**: A spec-compliant Payment payload containing only `paymentRequestId` and `paidAmount` is accepted (not rejected for missing `transactionDate` or `totalAmount`) in 100% of test cases.
- **SC-003**: Requests missing the true mandatory fields (`amount` for Inquiry; `paymentRequestId` or `paidAmount` for Payment) are rejected with a clear, field-specific validation error in 100% of test cases.
- **SC-004**: Zero regressions in existing passing test suites after the field/validation corrections are applied.

## Assumptions

- Scope for this feature is limited to the three field/validation defects the user prioritized first (typo field name, missing Inquiry `amount`, incorrect Payment mandatory-field rules). The unimplemented Service Code 28–35 endpoints remain explicitly out of scope (see Addendum below for what was and wasn't picked up afterward).
- "Spec" refers to the project's existing ASPI SNAP Virtual Account OpenAPI/YAML definition already present in the repository, treated as the source of truth for field names and mandatory-field rules.
- No backward-compatibility aliasing is required for the old field names (`trxDateInit`, `transactionDate`) or the old `totalAmount` mandatory rule — this is a correctness fix, not an additive change, per project convention of not retaining legacy shims.
- Validation error message format/style should stay consistent with the existing "Invalid Mandatory Field [fieldName]" convention already used in the codebase.

## Addendum: Additional ASPI Compliance Fixes (post-MVP, same feature branch)

After the three original bugs (US1/US2 above) shipped, further ad-hoc review against `aspi-open-api-va.yaml` and real end-to-end testing (via `scripts/e2e-va-flow.sh`) surfaced and fixed the following, all committed under this same branch rather than as separate features since they're the same class of "code deviates from `aspi-open-api-va.yaml`" defect:

- **Response echo gap (bug #4)**: `PaymentResponse.virtualAccountData` (`VAPaymentStatus`) only returned `paymentFlagStatus`/`paymentFlagReason`, dropping the identity/amount fields (`partnerServiceId`, `customerNo`, `virtualAccountNo`, `trxId`, `paymentRequestId`, `paidAmount`, `totalAmount`, `trxDateTime`, `referenceNo`) the spec's schema (l.228-254) actually returns. Fixed in both the success and idempotent-replay paths of `Payment()`.
- **`virtualAccountNo` create-VA input (bug #5)**: `MerchantCreateVARequest.VirtualAccountNo` is now mandatory, validated, and used as-is (previously ignored in favor of a server-generated `partnerServiceId+customerNo` value, contradicting `VAIdentity`'s `required` list).
- **Header mismatch**: `SNAPAuthMiddleware`'s default `RequiredHeaders` no longer includes `X-CLIENT-KEY` (spec only uses it on the access-token endpoint, never on `transfer-va/*`); `.env.vendor.channel.example` updated to match.
- **`notificationUrl` misplacement**: this proprietary field (not part of `VAUpsertRequest`) was a mandatory top-level field, rejecting spec-exact vendor payloads. It's now optional and carried in `additionalInfo.dbUrlProcess` — the spec's own extension slot for this exact endpoint (`aspi-open-api-va.yaml:317-320`) — not a bespoke top-level field.
- **Duplicate Inquiry rows**: `Inquiry()` looked up an existing merchant VA only to borrow display fields, then unconditionally inserted a new `VAInquiryRecord` keyed by the vendor's own (possibly novel) `inquiryRequestId` — so every inquiry against an already-created VA produced a phantom duplicate row with `totalAmount` hardcoded to `0.00`. Fixed: when a merchant VA record already exists for the `virtualAccountNo`, `Inquiry()` reflects it directly and skips the insert.
- **Bill details never persisted**: `CreateVA`'s bill-detail handling was dead code (`_ = billDetail`); `va_bill_details` was also missing `bill_code`/`bill_name`/`bill_short_name` columns that `GetVABillDetails` already selected. Added migration `000005`, a real `SaveBillDetails` repository method (delete+insert in one DB transaction), and wired `Inquiry()` to read bills back via `GetVABillDetails` so they appear in the Inquiry response too.
- **VA reuse conflict logic inverted**: `CreateVA`'s conflict check (`existing.Status != "03"`) blocked reuse of an already-*paid* VA number while allowing a second create-va over a still-*pending* one — backwards from "one VA can be reused across transaction cycles; it just can't have two simultaneously active transactions." Flipped to `existing.Status == "03"`. This also required fixing `GetVAByVirtualAccountNo` (added `ORDER BY created_at DESC LIMIT 1`, since a VA can now have multiple historical rows) and `UpdateVAStatus` (scoped to `AND status = '03'`, so cancelling a pending VA no longer corrupts older completed rows sharing the same number).
- **Tooling**: `scripts/vendor-inquiry-va.sh`, `scripts/vendor-payment-va.sh`, `scripts/merchant-create-va.sh` reworked to match all of the above (spec-correct payloads/headers, `-f <env-file>` secret loading, `-b`/`-d` bill details), diagnostics moved to stderr so they chain cleanly, and a new `scripts/e2e-va-flow.sh` drives the full token → create-va → inquiry → payment → merchant-callback-verification flow in one command (with a throwaway local HTTP listener to prove the async callback actually arrives).

Not picked up (still out of scope): Service Code 28–35 endpoints (`update-va`, `update-status`, `inquiry-va`, `inquiry-intrabank`, `payment-intrabank`, `notify-payment-intrabank`, `report`) — these remain a separate, larger feature requiring new DB schema decisions.
