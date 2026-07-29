# Phase 0 Research: Virtual Account Number Consistency with SNAP Standard

No `NEEDS CLARIFICATION` markers remain in the Technical Context (this is a small, well-scoped correction to existing, already-understood code). The items below record the concrete decisions made while confirming the approach against the current codebase.

## Decision 1: Where the validation/derivation logic lives

- **Decision**: All new logic lives inside `MerchantVAUsecase.CreateVA` (`internal/usecase/merchant_va_usecase.go`), immediately after the existing mandatory-field checks and `managed`/`vaTypeRule` resolution (around line 45-90), replacing the current unconditional `if req.VirtualAccountNo == ""` mandatory check and the unconditional `vaNo := req.VirtualAccountNo` assignment (line 101).
- **Rationale**: This is the single call site that already resolves `managed`, `vaTypeRule.Dynamic`, and `customerNo`. Keeping the new branching here avoids duplicating VA-type resolution logic elsewhere and matches Clean Architecture (Principle I): the use case layer owns business-rule orchestration.
- **Alternatives considered**: A separate domain-level `VANumberPolicy` value object. Rejected as over-engineering (YAGNI) for a single call site with three cases (static, dynamic-with-value, dynamic-without-value); a plain internal helper function suffices and can be extracted later if reused.

## Decision 2: Static VA — validation order

- **Decision**: The `virtualAccountNo == partnerServiceId + customerNo` check runs for static (non-dynamic managed, and unmanaged/legacy) requests, after `customerNo` itself is validated (existing empty/length checks) but before the record is persisted. On mismatch, return a new domain error code (see Decision 4) instead of the generic `4002701` mandatory-field error, since the field IS present — it's just inconsistent.
- **Rationale**: Existing `customerNo` format/length validation must remain the first gate (per spec Edge Cases: "malformed customerNo" is validated before the new consistency check). Reusing the existing `domain.NewDomainError` pattern keeps error handling consistent with the rest of the use case.
- **Alternatives considered**: Silently overwriting the merchant's `virtualAccountNo` with the derived value for static VA too. Rejected — spec Story 1 explicitly requires *rejection*, not silent correction, because static VA merchants are expected to already know and control their own numbering.

## Decision 3: Dynamic VA — optional virtualAccountNo

- **Decision**: For dynamic VA (`managed && vaTypeRule.Dynamic`), remove `virtualAccountNo` from the unconditional mandatory-field check. After `customerNo` is generated via `NextCustomerNoSequence`, branch on whether the merchant supplied a `virtualAccountNo`:
  - Empty → derive `vaNo = req.PartnerServiceID + customerNo`.
  - Non-empty → use `vaNo = req.VirtualAccountNo` as-is.
  In both cases, the existing `len(vaNo) > 28` check and the existing `GetVAByVirtualAccountNo` + `Status == "03"` pending-conflict check still apply unchanged — this reuses the exact conflict semantics already proven for static VA reuse, per the user's direction that a colliding value must produce a conflict response (spec FR-005a).
- **Rationale**: Matches the user's explicit resolution: "gunakan punya merchant, jika merchant tidak kirim auto generated, jika conflict response conflict" (use the merchant's value if given, auto-generate if not, respond with conflict on collision). Reusing the existing pending-transaction conflict check (`4092700`) avoids introducing a second, inconsistent notion of "conflict" for VA numbers.
- **Alternatives considered**: Always deriving `virtualAccountNo` server-side for dynamic VA regardless of merchant input (the original interpretation before user clarification). Rejected per explicit user answer — merchants may still want to control the dynamic VA's number when they have one.

## Decision 4: Error codes

- **Decision**: Introduce one new domain error code for the static-VA mismatch case, following the existing `400270X` numbering convention used elsewhere in `CreateVA` (e.g. `4002703` customerNo-must-be-empty, `4002704` customerNo-required, `4002705`/`4002706` totalAmount rules): use `4002707` — "Invalid Field Format [virtualAccountNo does not match partnerServiceId + customerNo]". The length-overflow case (FR-007, concatenation > 28 chars) reuses the existing `4002700` "Invalid Field Format [virtualAccountNo too long]" pattern. The conflict case (FR-005a) reuses the existing `4092700` "Conflict: VA already has an active pending transaction" code and message, since it is the same underlying conflict check.
- **Rationale**: Keeps error-code allocation consistent with the existing sequential convention in this file rather than inventing an unrelated scheme; minimizes changes needed in any error-code documentation/tables elsewhere in the codebase.
- **Alternatives considered**: A distinct error-code family for "VA number consistency" errors. Rejected — no evidence elsewhere in the codebase of grouping by concern rather than by endpoint+sequence; would be inconsistent with existing `merchant_va_usecase.go` conventions.

## Decision 5: Existing VA records (no migration)

- **Decision**: No migration or backfill of existing `VAInquiryRecord` rows. This feature governs new VA creation requests only, per spec Assumption.
- **Rationale**: Explicitly out of scope per the spec; retrofitting historical data risks breaking already-issued VA numbers that merchants/customers may already be using.
- **Alternatives considered**: A one-off backfill script correcting historical mismatches. Rejected as unnecessary scope expansion (YAGNI) not requested by the spec.
