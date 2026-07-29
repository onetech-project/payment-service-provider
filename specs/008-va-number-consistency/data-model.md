# Phase 1 Data Model: Virtual Account Number Consistency with SNAP Standard

No new entities, tables, or migrations. This feature adds an invariant to an existing entity's fields at creation time; it does not alter the entity's shape.

## Entity: Virtual Account (VA) record

Existing entity — `domain.VAInquiryRecord` (`internal/domain/va.go`). Relevant fields (unchanged):

| Field | Type | Notes |
|---|---|---|
| `PartnerServiceID` | string | Merchant-supplied, mandatory. Max 8 chars per ASPI. |
| `CustomerNo` | string | Static: merchant-supplied (mandatory, registered via `RegisterStaticCustomerNo`). Dynamic: server-generated via `NextCustomerNoSequence` (2-digit vaType + 18-digit sequence, 20 chars total). |
| `VirtualAccountNo` | string | **New invariant (this feature)**: MUST equal `PartnerServiceID + CustomerNo`. Static: validated against merchant input, request rejected on mismatch. Dynamic: derived server-side when merchant leaves it empty; used as-is (subject to conflict check) when merchant supplies it. Max 28 chars (ASPI VAIdentity constraint). |
| `Status` | string | Unchanged. `"03"` (pending) on a `VirtualAccountNo` blocks reuse of that number for a new create-VA request (existing conflict rule, now also the basis for FR-005a's dynamic-merchant-supplied-value conflict case). |

## Validation Rules (new, added to existing `CreateVA` flow)

1. **Static VA / unmanaged (legacy) requests**: `VirtualAccountNo` MUST equal `PartnerServiceID + CustomerNo` exactly (byte-for-byte string concatenation). Violation → reject with error `4002707` before any persistence occurs.
2. **Dynamic VA requests**:
   - If merchant-supplied `VirtualAccountNo` is empty: derive `VirtualAccountNo = PartnerServiceID + CustomerNo` after `CustomerNo` is generated. No validation needed (server-authored).
   - If merchant-supplied `VirtualAccountNo` is non-empty: use it as-is. No equality check against `PartnerServiceID + CustomerNo` is performed for this case (the merchant is intentionally choosing an independent value per the resolved clarification) — it is still subject to the length and conflict checks below.
3. **Length constraint (both flows)**: the resulting `VirtualAccountNo` (validated, or derived, or merchant-chosen) MUST be ≤ 28 characters. If a *derived* value would exceed 28 characters (i.e. `len(PartnerServiceID) + len(CustomerNo) > 28`), reject VA creation entirely — the merchant cannot fix this by changing `VirtualAccountNo` on a dynamic request since it isn't derived from their input.
4. **Conflict rule (both flows, unchanged mechanism)**: if an existing `VAInquiryRecord` with the same `VirtualAccountNo` has `Status == "03"` (pending), reject with the existing `4092700` conflict error. This now also covers the case of a merchant-supplied `VirtualAccountNo` on a dynamic request colliding with an active pending VA.

## State / Flow Summary

```
CreateVA request
  ├─ unmanaged or static managed (vaTypeRule.Dynamic == false)
  │    customerNo = req.CustomerNo (validated/registered as today)
  │    IF req.VirtualAccountNo != partnerServiceId + customerNo → reject 4002707
  │    ELSE vaNo = req.VirtualAccountNo
  │
  └─ dynamic managed (vaTypeRule.Dynamic == true)
       customerNo = NextCustomerNoSequence(vaType)   # as today
       IF req.VirtualAccountNo == "" → vaNo = partnerServiceId + customerNo   # derived
       ELSE                          → vaNo = req.VirtualAccountNo           # merchant-chosen

  IF len(vaNo) > 28 → reject 4002700 / (derived overflow → also 4002700)
  IF existing VA with vaNo has Status == "03" → reject 4092700
  → persist record with (partnerServiceId, customerNo, vaNo) as today
```
