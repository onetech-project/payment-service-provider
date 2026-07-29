# Feature Specification: Virtual Account Number Consistency with SNAP Standard

**Feature Branch**: `008-va-number-consistency`

**Created**: 2026-07-29

**Status**: Draft

**Input**: User description: "Perbaikan relasi virtualAccountNo dan customerNo pada fitur create-va agar sesuai standar SNAP: virtualAccountNo = partnerServiceId + customerNo. Static VA: merchant mengirim customerNo dan virtualAccountNo, server harus memvalidasi bahwa virtualAccountNo yang dikirim merchant sama dengan partnerServiceId + customerNo, dan menolak request jika tidak sesuai. Dynamic VA: merchant hanya mengirim partnerServiceId, customerNo dan virtualAccountNo dikosongkan oleh merchant; server men-generate customerNo dan men-derive virtualAccountNo dari partnerServiceId + customerNo yang baru di-generate. Response create-va harus tetap mengembalikan virtualAccountNo dan customerNo yang konsisten untuk kedua case."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Server rejects a mismatched virtualAccountNo on static VA creation (Priority: P1)

A merchant integrates the static virtual account creation flow and supplies both `customerNo` and `virtualAccountNo` in the create-VA request, as required by the existing static VA contract. Today the server accepts any `virtualAccountNo` the merchant sends without checking it against `partnerServiceId` + `customerNo`, which lets inconsistent records be created and breaks downstream vendor/payment matching that relies on the SNAP-standard derivation.

**Why this priority**: This is the core correctness gap driving the whole feature — without this validation, static VA records can silently violate the SNAP numbering standard, causing failures or ambiguity in inquiry/payment lookups downstream. It also has the widest blast radius since static VA is the existing, most-used flow.

**Independent Test**: Can be fully tested by submitting a static VA create request where `virtualAccountNo` does not equal `partnerServiceId + customerNo` and confirming the request is rejected with a clear, actionable error — while a request where the two values are consistent is accepted unchanged.

**Acceptance Scenarios**:

1. **Given** a static VA create-VA request with `partnerServiceId="15973"` and `customerNo="000000000000000123"`, **When** the merchant submits `virtualAccountNo="15973000000000000123"` (i.e. the correct concatenation), **Then** the request is accepted and the VA is created as before.
2. **Given** the same `partnerServiceId` and `customerNo`, **When** the merchant submits a `virtualAccountNo` that does not equal the concatenation (e.g. a different or malformed value), **Then** the request is rejected with an error identifying the mismatch, and no VA record is created.
3. **Given** a static VA create-VA request where `customerNo` is present but `virtualAccountNo` is missing entirely, **Then** the request is rejected as before (no behavior change — `virtualAccountNo` remains a required field for static VA).

---

### User Story 2 - Dynamic VA honors a merchant-supplied virtualAccountNo, or derives one automatically when omitted (Priority: P1)

A merchant integrates the dynamic virtual account creation flow (no-bill, fixed-bill, or variable-bill) and, per the existing dynamic VA design, leaves `customerNo` empty so the server auto-generates it. For `virtualAccountNo`, the merchant may optionally supply their own value; if they do, the server uses it as-is (subject to the usual uniqueness rules for VA numbers). If the merchant leaves `virtualAccountNo` empty, the server auto-generates it from `partnerServiceId` + the newly generated `customerNo`, consistent with the SNAP standard. If a merchant-supplied `virtualAccountNo` collides with an existing VA record, the request is rejected with a conflict response rather than silently overwriting or renumbering it.

**Why this priority**: This closes the correctness gap for the dynamic flow (feature 006-static-dynamic-va) while preserving merchant flexibility to supply their own `virtualAccountNo` when they have one. Equal priority to Story 1 because both flows must produce valid, non-conflicting VA numbers for the feature to be complete.

**Independent Test**: Can be fully tested two ways: (a) submitting a dynamic VA create request with both `customerNo` and `virtualAccountNo` left empty, and confirming the response returns a server-generated `customerNo` together with a `virtualAccountNo` that equals `partnerServiceId` + that `customerNo`; and (b) submitting a dynamic VA create request with `customerNo` empty but a merchant-supplied `virtualAccountNo`, and confirming the response echoes that exact `virtualAccountNo` back alongside the server-generated `customerNo`.

**Acceptance Scenarios**:

1. **Given** a dynamic VA create-VA request with `partnerServiceId="15973"`, `vaType="04"` (no bill), and both `customerNo` and `virtualAccountNo` left empty, **When** the request is submitted, **Then** the response's `virtualAccountData.customerNo` is a server-generated 20-digit value starting with the vaType prefix, and `virtualAccountData.virtualAccountNo` equals `partnerServiceId + customerNo`.
2. **Given** the same conditions for vaType `05` (variable bill) and `06` (fixed bill), **Then** the same auto-generation behavior holds in both cases.
3. **Given** a dynamic VA create-VA request with `customerNo` left empty but a merchant-supplied `virtualAccountNo` that does not already exist in the system, **When** the request is submitted, **Then** the VA is created using the merchant-supplied `virtualAccountNo` as-is, together with the server-generated `customerNo`.
4. **Given** a dynamic VA create-VA request with a merchant-supplied `virtualAccountNo` that already belongs to an existing VA record, **When** the request is submitted, **Then** the system rejects the request with a conflict response, and no new VA record is created.

---

### User Story 3 - Consistent VA numbers flow through inquiry and payment (Priority: P2)

Once a VA is created (static or dynamic) with a validated/derived `virtualAccountNo`, downstream vendor-facing operations (inquiry, payment) that look up the VA by `virtualAccountNo` and/or `customerNo` continue to work correctly, now with the guarantee that the two identifiers are always consistent with each other.

**Why this priority**: This is a consequence of Stories 1 and 2 rather than new logic — it's the confirmation that the fix doesn't break existing lookup paths. Lower priority because it's primarily a regression-safety check, not new user-facing capability.

**Independent Test**: Can be fully tested by running the existing end-to-end static and dynamic VA flows (create → inquiry → payment) and confirming they complete successfully using the now-consistent `virtualAccountNo`/`customerNo` pair.

**Acceptance Scenarios**:

1. **Given** a static VA created with a validated `virtualAccountNo`, **When** an inquiry or payment request is submitted using that `virtualAccountNo` and `customerNo`, **Then** the VA is found and the operation proceeds exactly as it does today.
2. **Given** a dynamic VA created with a server-derived `virtualAccountNo`, **When** an inquiry or payment request is submitted using the `virtualAccountNo` and `customerNo` returned in the create-VA response, **Then** the VA is found and the operation proceeds exactly as it does today.

### Edge Cases

- What happens when the concatenation of `partnerServiceId` + `customerNo` would exceed the maximum allowed length for `virtualAccountNo`? The system must reject VA creation (static) or generation (dynamic) rather than silently truncating.
- How does the system handle a static VA request where `customerNo` itself is malformed (wrong length/non-numeric)? Existing `customerNo` validation continues to apply before the new virtualAccountNo-consistency check is evaluated.
- What happens to existing VA records created before this change, where `virtualAccountNo` may not equal `partnerServiceId + customerNo`? This feature only governs new VA creation; retroactively correcting existing records is out of scope.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST, for static VA creation (where the merchant supplies `customerNo`), validate that the merchant-supplied `virtualAccountNo` equals the exact concatenation of `partnerServiceId` and `customerNo`.
- **FR-002**: System MUST reject a static VA create-VA request with a clear, specific error when the supplied `virtualAccountNo` does not equal `partnerServiceId + customerNo`, and MUST NOT create a VA record for that request.
- **FR-003**: System MUST continue to require `virtualAccountNo` as a mandatory field for static VA creation (no behavior change to that existing requirement).
- **FR-004**: System MUST, for dynamic VA creation (where `customerNo` is left empty by the merchant) where the merchant also leaves `virtualAccountNo` empty, generate `customerNo` using the existing sequence-generation logic and then derive `virtualAccountNo` server-side as the concatenation of `partnerServiceId` and the generated `customerNo`.
- **FR-005**: System MUST, for dynamic VA creation where the merchant supplies a non-empty `virtualAccountNo`, use that value as-is (subject to FR-005a) rather than overriding it with a derived value.
- **FR-005a**: System MUST reject a dynamic (or static) VA creation request with a conflict response when the supplied or derived `virtualAccountNo` already belongs to an existing VA record, and MUST NOT create a duplicate record.
- **FR-006**: System MUST return, in every successful create-VA response (static and dynamic), a `virtualAccountNo` value that equals `partnerServiceId + customerNo` for the record just created.
- **FR-007**: System MUST reject VA creation (static or dynamic) if the concatenation of `partnerServiceId` and `customerNo` would exceed the maximum allowed length for `virtualAccountNo`.
- **FR-008**: Existing inquiry and payment operations MUST continue to match VAs by `virtualAccountNo`/`customerNo` without modification, relying on the new guarantee that the two are always consistent for VAs created after this change.

### Key Entities

- **Virtual Account (VA) record**: Represents a merchant's virtual account instance; key attributes include `partnerServiceId`, `customerNo`, and `virtualAccountNo`, where `virtualAccountNo` is now guaranteed to equal `partnerServiceId + customerNo` for all VAs created after this feature ships.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of newly created static VA records have `virtualAccountNo` equal to `partnerServiceId + customerNo`; any mismatched request is rejected before a record is created.
- **SC-002**: 100% of newly created dynamic VA records (across all vaTypes) have a server-derived `virtualAccountNo` equal to `partnerServiceId + customerNo`, with no dependency on a merchant-supplied value.
- **SC-003**: Existing inquiry and payment flows for both static and dynamic VAs continue to succeed at the same rate as before this change, with zero regressions introduced by the new validation/derivation logic.
- **SC-004**: Merchants integrating the dynamic VA flow no longer need to construct or guess a `virtualAccountNo` value — it is entirely absent from their required inputs.

## Assumptions

- The maximum allowed length for `virtualAccountNo` and the format/length of `partnerServiceId` and `customerNo` remain as already defined by the existing SNAP contract and feature 006-static-dynamic-va (no new length constraints introduced by this feature).
- This feature governs VA creation behavior only; it does not retroactively modify or migrate existing VA records created before this change ships.
- "Static VA" and "Dynamic VA" retain the same definitions and vaType semantics established in feature 006-static-dynamic-va.
- Vendor-facing inquiry and payment endpoints are unaffected in their own request/response contracts — only the values they look up change (to be consistent), not the lookup mechanism itself.
- When a merchant explicitly supplies their own `virtualAccountNo` on a dynamic VA request (rather than leaving it empty), that value is honored as-is and is not required to equal `partnerServiceId + customerNo` — the SNAP-standard derivation in FR-004/FR-006 applies specifically to the case where the merchant leaves `virtualAccountNo` empty and the server must produce one.
