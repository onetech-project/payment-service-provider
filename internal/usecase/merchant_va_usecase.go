package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"backbone-new/internal/domain"
)

// errInternalServerError is the SNAP responseMessage paired with the 500-class
// response codes in this file.
const errInternalServerError = "Internal Server Error"

// errOperationNotAllowed is the SNAP responseMessage for delete-VA attempts
// against a VA in a state that cannot be deleted.
const errOperationNotAllowed = "Requested Operation Is Not Allowed"

// MerchantVAUsecase implements domain.MerchantVAUsecase
type MerchantVAUsecase struct {
	repo        domain.VARepository
	vaTypeRules domain.VATypeRuleProvider
}

// vaNoMatchesPartnerAndCustomer reports whether virtualAccountNo equals the
// SNAP-standard concatenation of partnerServiceId and customerNo.
func vaNoMatchesPartnerAndCustomer(partnerServiceID, customerNo, virtualAccountNo string) bool {
	return virtualAccountNo == partnerServiceID+customerNo
}

// NewMerchantVAUsecase creates a new merchant VA usecase. vaTypeRules may be
// nil, in which case every request is treated as an unmanaged (legacy)
// request — i.e. none of the six static/dynamic VA type combinations are
// recognized. Pass a real domain.VATypeRuleProvider (see
// internal/infrastructure/cache) to enable feature 006-static-dynamic-va.
func NewMerchantVAUsecase(repo domain.VARepository, vaTypeRules domain.VATypeRuleProvider) *MerchantVAUsecase {
	return &MerchantVAUsecase{repo: repo, vaTypeRules: vaTypeRules}
}

// CreateVA handles VA creation per ASPI VAUpsertRequest (Service Code 27),
// extended per feature 006-static-dynamic-va to route requests bearing a
// reserved partnerServiceId (15973/15974/15975) or an explicit
// additionalInfo.vaType through the six static/dynamic VA type combinations.
func (u *MerchantVAUsecase) CreateVA(ctx context.Context, req *domain.MerchantCreateVARequest) (*domain.MerchantCreateVAResponse, error) {
	vaType, hasVAType := vaTypeFromAdditionalInfo(req.AdditionalInfo)
	managed := hasVAType
	if !managed && u.vaTypeRules != nil {
		reserved, err := u.vaTypeRules.IsReservedPartnerServiceID(ctx, req.PartnerServiceID)
		if err != nil {
			return nil, domain.NewDomainError("5002702", fmt.Sprintf("System Unavailable [VA type master data: %v]", err), err)
		}
		managed = reserved
	}

	// Validate required fields per ASPI VAIdentity (partnerServiceId,
	// customerNo, virtualAccountNo are all mandatory client-supplied input).
	// customerNo is validated below per VA-type mode for managed requests
	// (empty required for dynamic, non-empty required for static).
	if req.PartnerServiceID == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [partnerServiceId]", nil)
	}
	if !managed && req.CustomerNo == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [customerNo]", nil)
	}
	// virtualAccountNo remains mandatory for unmanaged (legacy) requests. For
	// managed requests it is validated per VA-type mode below (mandatory for
	// static, optional for dynamic — since customerNo, and therefore the
	// SNAP-standard virtualAccountNo, may not exist yet at this point for a
	// dynamic request).
	if !managed && req.VirtualAccountNo == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [virtualAccountNo]", nil)
	}
	if req.VirtualAccountName == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [virtualAccountName]", nil)
	}
	if req.TrxID == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [trxId]", nil)
	}

	// Reject a bill that could never be presented. These lists are echoed
	// verbatim in the SNAP inquiry response, and BCA's Notes cap both at 5
	// ("billDetails should not be greater than 5", "The occurences for
	// freeTexts field in inquiry bill should not be greater than 5"). Accepting
	// six here and discovering it at inquiry time means the merchant's VA fails
	// at the channel, in front of the customer, with no indication of why —
	// so it is refused at the point the merchant can still fix it.
	if len(req.BillDetails) > domain.MaxInquiryBillDetails {
		return nil, domain.NewDomainError("4002700", "Invalid Field Format [billDetails]", nil)
	}
	if len(req.FreeTexts) > domain.MaxInquiryFreeTexts {
		return nil, domain.NewDomainError("4002700", "Invalid Field Format [freeTexts]", nil)
	}

	var vaTypeRule domain.VATypeRule
	if managed {
		if u.vaTypeRules == nil {
			return nil, domain.NewDomainError("4002702", "Invalid Field Format [partnerServiceId/additionalInfo.vaType combination]", nil)
		}
		rule, ok, err := u.vaTypeRules.LookupVATypeRule(ctx, req.PartnerServiceID, vaType)
		if err != nil {
			return nil, domain.NewDomainError("5002702", fmt.Sprintf("System Unavailable [VA type master data: %v]", err), err)
		}
		if !ok {
			return nil, domain.NewDomainError("4002702", "Invalid Field Format [partnerServiceId/additionalInfo.vaType combination]", nil)
		}
		vaTypeRule = rule

		if vaTypeRule.Dynamic && req.CustomerNo != "" {
			return nil, domain.NewDomainError("4002703", "Invalid Field Format [customerNo must be empty for dynamic vaType]", nil)
		}
		if !vaTypeRule.Dynamic && req.CustomerNo == "" {
			return nil, domain.NewDomainError("4002704", "Invalid Mandatory Field [customerNo required for static vaType]", nil)
		}
		if !vaTypeRule.Dynamic && req.VirtualAccountNo == "" {
			return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [virtualAccountNo]", nil)
		}

		if vaTypeRule.Billing == domain.VATypeBillingNone && req.TotalAmount != nil {
			return nil, domain.NewDomainError("4002706", "Invalid Field Format [totalAmount must not be set for no-bill vaType]", nil)
		}
		if vaTypeRule.Billing != domain.VATypeBillingNone && req.TotalAmount == nil {
			return nil, domain.NewDomainError("4002705", "Invalid Mandatory Field [totalAmount required for this vaType]", nil)
		}
	}

	// Validate virtualAccountTrxType if provided
	if req.VirtualAccountTrxType != "" {
		validTypes := map[string]bool{"C": true, "O": true, "I": true, "M": true, "L": true, "N": true, "X": true}
		if !validTypes[req.VirtualAccountTrxType] {
			return nil, domain.NewDomainError("4002700", "Invalid Field Format [virtualAccountTrxType]", nil)
		}
	}

	// Use the client-supplied virtualAccountNo. The cap is BCA's payment/status
	// limit (26), not ASPI's nominal 28: a longer VA number passes inquiry and
	// is then rejected at payment, so it is refused at issue time instead —
	// see domain.MaxIssuedVirtualAccountNo. For a dynamic managed request it
	// may be empty at this point (resolved below, once customerNo is known).
	vaNo := req.VirtualAccountNo
	if vaNo != "" && len(vaNo) > domain.MaxIssuedVirtualAccountNo {
		return nil, domain.NewDomainError("4002700", "Invalid Field Format [virtualAccountNo too long]", nil)
	}
	// Same reasoning for the merchant-supplied static customerNo. The dynamic
	// path generates its own at 18 digits (NextCustomerNoSequence) and is
	// checked by the virtualAccountNo cap below.
	if len(req.CustomerNo) > domain.MaxIssuedCustomerNo {
		return nil, domain.NewDomainError("4002700", "Invalid Field Format [customerNo too long]", nil)
	}

	// A no-bill VA (vaType 01/04) is an address, not a transaction: /create-va
	// registers it once and every payment thereafter creates its own
	// transaction (feature 013-no-bill-payment-transaction, FR-001).
	//
	// The branch keys off the rule's Billing classification rather than a
	// literal vaType == "01" || "04" check, so an operator who adds a seventh
	// no-bill VA type to master_va_type gets this flow with no code change
	// (Constitution II).
	noBill := managed && vaTypeRule.Billing == domain.VATypeBillingNone

	// Resolve customerNo: system-generated for dynamic VA types, merchant-
	// supplied (echoed) for static VA types and legacy (unmanaged) requests.
	customerNo := req.CustomerNo
	if managed && vaTypeRule.Dynamic {
		generated, err := u.repo.NextCustomerNoSequence(ctx, vaType)
		if err != nil {
			return nil, domain.NewDomainError("5002702", fmt.Sprintf("System Unavailable [sequence generator: %v]", err), err)
		}
		customerNo = generated

		// virtualAccountNo is optional for dynamic VA: honor the merchant's
		// own value if supplied, otherwise derive it per the SNAP standard
		// (partnerServiceId + customerNo).
		if vaNo == "" {
			vaNo = req.PartnerServiceID + customerNo
			if len(vaNo) > domain.MaxIssuedVirtualAccountNo {
				return nil, domain.NewDomainError("4002700", "Invalid Field Format [virtualAccountNo too long]", nil)
			}
		}
	} else {
		// Static (managed non-dynamic) and unmanaged/legacy requests: the
		// merchant-supplied virtualAccountNo MUST match the SNAP-standard
		// concatenation of partnerServiceId + customerNo.
		if !vaNoMatchesPartnerAndCustomer(req.PartnerServiceID, customerNo, vaNo) {
			return nil, domain.NewDomainError("4002707", "Invalid Field Format [virtualAccountNo does not match partnerServiceId + customerNo]", nil)
		}
		// The one-shot customerNo registration is deliberately skipped for
		// no-bill VAs: a repeat /create-va on a registered no-bill VA updates
		// the holder details rather than conflicting (FR-005), and this check
		// is exactly what used to turn that second call into a 4092701.
		// Static BILL-bearing types (02/03) keep it, so their behavior is
		// unchanged (FR-021).
		if managed && !noBill {
			if err := u.repo.RegisterStaticCustomerNo(ctx, req.PartnerServiceID, customerNo); err != nil {
				if errors.Is(err, domain.ErrVACustomerNoAlreadyRegistered) {
					return nil, domain.NewDomainError("4092701", "Conflict: customerNo already registered for this partnerServiceId", nil)
				}
				return nil, domain.NewDomainError("5002702", fmt.Sprintf("System Unavailable [customerNo registration: %v]", err), err)
			}
		}
	}

	now := time.Now()

	// Persist the VA registration — the durable identity of this VA number.
	// Written for every managed VA type (spec A-002) so identity lives in one
	// place; for no-bill types it is the ONLY thing written.
	if managed {
		account := &domain.VAAccount{
			PartnerServiceID: req.PartnerServiceID,
			CustomerNo:       customerNo,
			VirtualAccountNo: vaNo,
			VAType:           vaType,
			Billing:          vaTypeRule.Billing,
			CustomerName:     req.VirtualAccountName,
			CustomerEmail:    req.VirtualAccountEmail,
			CustomerPhone:    req.VirtualAccountPhone,
			TrxID:            req.TrxID,
			NotificationURL:  notificationURLFromAdditionalInfo(req.AdditionalInfo),
			Status:           domain.VAAccountStatusActive,
			ExpiredDate:      req.ExpiredDate,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := u.repo.SaveVAAccount(ctx, account); err != nil {
			return nil, domain.NewDomainError("5002700", errInternalServerError, err)
		}
		log.Printf("event=va_account_registered virtual_account_no=%s va_type=%s", vaNo, vaType)
	}

	// No-bill VAs stop here: no transaction is created at registration time.
	// The transaction is created when the customer actually pays, one per
	// payment, which is what lets the same VA number be paid repeatedly
	// (FR-001, contracts/create-va-no-bill.md).
	if noBill {
		return buildCreateVAResponse(req, customerNo, vaNo), nil
	}

	// A virtualAccountNo is reusable across transaction cycles — only a
	// currently PENDING ("03", i.e. created but not yet paid) transaction on
	// it blocks a new create-va call. Once that transaction reaches a
	// terminal state (paid "00", expired "02", deleted "04"), the same VA
	// number is free to start a brand new transaction.
	existing, _ := u.repo.GetVAByVirtualAccountNo(ctx, vaNo)
	if existing != nil && existing.Status == "03" {
		return nil, domain.NewDomainError("4092700", "Conflict: VA already has an active pending transaction", nil)
	}

	// Save transaction.
	//
	// inquiry_request_id carries a placeholder, not "". The column is UNIQUE,
	// and the vendor's real inquiryRequestId does not exist yet at create-va
	// time — so writing "" made every billed VA after the first collide with
	// it on SaveInquiry's ON CONFLICT, leaving the second and later VAs with
	// no transaction row at all: invisible to inquiry (4042412) and, worse,
	// payable for any amount because there was no stored bill to check
	// against. The VA number is the natural placeholder: unique per VA,
	// stable across billing cycles on that number, and recognised as
	// claimable by the first inquiry (see domain.IsPlaceholderInquiryRequestID).
	record := &domain.VAInquiryRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       customerNo,
		CustomerName:     req.VirtualAccountName,
		VirtualAccountNo: vaNo,
		InquiryRequestID: vaNo,
		TrxID:            req.TrxID,
		NotificationURL:  notificationURLFromAdditionalInfo(req.AdditionalInfo),
		Status:           "03",
		TotalAmount:      "0",
		Currency:         "IDR",
		VAType:           vaType,
		SubCompany:       subCompanyFromAdditionalInfo(req.AdditionalInfo),
		FreeTexts:        req.FreeTexts,
		ExpiredDate:      req.ExpiredDate,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if req.TotalAmount != nil {
		record.TotalAmount = req.TotalAmount.Value
		record.Currency = req.TotalAmount.Currency
	}

	if err := u.repo.SaveInquiry(ctx, record); err != nil {
		return nil, domain.NewDomainError("5002700", errInternalServerError, err)
	}

	if len(req.BillDetails) > 0 {
		if err := u.repo.SaveBillDetails(ctx, record.ID, req.BillDetails); err != nil {
			return nil, domain.NewDomainError("5002700", errInternalServerError, err)
		}
	}

	return buildCreateVAResponse(req, customerNo, vaNo), nil
}

// buildCreateVAResponse assembles the ASPI VAUpsertResponse, echoing the
// request's own fields alongside the resolved customerNo/virtualAccountNo
// (which differ from the request for dynamic VA types). Shared by the no-bill
// registration-only path and the bill-bearing transaction path so the two
// cannot drift apart on the wire.
func buildCreateVAResponse(req *domain.MerchantCreateVARequest, customerNo, vaNo string) *domain.MerchantCreateVAResponse {
	return &domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			PartnerServiceID:      req.PartnerServiceID,
			CustomerNo:            customerNo,
			VirtualAccountNo:      vaNo,
			VirtualAccountName:    req.VirtualAccountName,
			VirtualAccountEmail:   req.VirtualAccountEmail,
			VirtualAccountPhone:   req.VirtualAccountPhone,
			TrxID:                 req.TrxID,
			TotalAmount:           req.TotalAmount,
			BillDetails:           req.BillDetails,
			FreeTexts:             req.FreeTexts,
			VirtualAccountTrxType: req.VirtualAccountTrxType,
			FeeAmount:             req.FeeAmount,
			ExpiredDate:           req.ExpiredDate,
			AdditionalInfo:        req.AdditionalInfo,
		},
	}
}

// normalizePaging clamps the merchant's page/pageSize into the supported range
// (page >= 1, pageSize 1..100 defaulting to 20), shared by both listings.
func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// paginationFor builds the response pagination block for a page of results.
func paginationFor(page, pageSize, total int) *domain.Pagination {
	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}
	return &domain.Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalRows:  total,
		TotalPages: totalPages,
	}
}

// ListVA lists registered VA numbers, one entry per VA (feature
// 013-no-bill-payment-transaction, FR-023).
//
// This reads the VA registry, not the transaction table. Listing transactions
// here was wrong once a no-bill VA could hold many payments: a VA paid ten
// times rendered as ten VAs. Per-payment detail now lives in ListTransactions.
func (u *MerchantVAUsecase) ListVA(ctx context.Context, req *domain.MerchantListVARequest) (*domain.MerchantListVAResponse, error) {
	page, pageSize := normalizePaging(req.Page, req.PageSize)

	filter := &domain.VAAccountListFilter{
		PartnerServiceID: req.PartnerServiceID,
		FromDate:         req.FromDate,
		ToDate:           req.ToDate,
		Status:           req.Status,
		VirtualAccountNo: req.VirtualAccountNo,
		Offset:           (page - 1) * pageSize,
		Limit:            pageSize,
	}

	items, total, err := u.repo.ListVAAccounts(ctx, filter)
	if err != nil {
		return nil, domain.NewDomainError("5002400", errInternalServerError, err)
	}

	return &domain.MerchantListVAResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		Data:            items,
		Pagination:      paginationFor(page, pageSize, total),
	}, nil
}

// ListTransactions lists individual payment events, filterable by VA number —
// the per-payment view that complements ListVA (feature
// 013-no-bill-payment-transaction, FR-023).
func (u *MerchantVAUsecase) ListTransactions(ctx context.Context, req *domain.MerchantListVARequest) (*domain.MerchantListTransactionsResponse, error) {
	page, pageSize := normalizePaging(req.Page, req.PageSize)

	filter := &domain.VAListFilter{
		PartnerServiceID: req.PartnerServiceID,
		FromDate:         req.FromDate,
		ToDate:           req.ToDate,
		Status:           req.Status,
		VirtualAccountNo: req.VirtualAccountNo,
		Offset:           (page - 1) * pageSize,
		Limit:            pageSize,
	}

	items, total, err := u.repo.ListVATransactions(ctx, filter)
	if err != nil {
		return nil, domain.NewDomainError("5002400", errInternalServerError, err)
	}

	return &domain.MerchantListTransactionsResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		Data:            items,
		Pagination:      paginationFor(page, pageSize, total),
	}, nil
}

// DeleteVA handles VA deletion per ASPI DeleteVARequest (Service Code 31)
func (u *MerchantVAUsecase) DeleteVA(ctx context.Context, req *domain.MerchantDeleteVARequest) (*domain.MerchantDeleteVAResponse, error) {
	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" {
		return nil, domain.NewDomainError("4003101", "Invalid Mandatory Field", nil)
	}

	// A no-bill VA has no pending transaction to cancel — deleting it means
	// deactivating the REGISTRATION so it stops accepting payments (feature
	// 013-no-bill-payment-transaction, FR-019). Historical settled payments are
	// deliberately left untouched and remain queryable (FR-020).
	account, accErr := u.repo.GetVAAccount(ctx, req.VirtualAccountNo)
	if accErr != nil && !errors.Is(accErr, domain.ErrVAAccountNotFound) {
		return nil, domain.NewDomainError("5003100", errInternalServerError, accErr)
	}
	if account.IsNoBill() {
		// UpdateVAAccountStatus is scoped to WHERE status='ACTIVE', so a
		// repeat delete (or one against an already-EXPIRED registration)
		// affects no rows and returns ErrVAAccountNotFound. That is the
		// idempotent case, not a failure — the merchant asked for the VA to be
		// unpayable and it already is.
		if err := u.repo.UpdateVAAccountStatus(ctx, req.VirtualAccountNo, domain.VAAccountStatusInactive); err != nil && !errors.Is(err, domain.ErrVAAccountNotFound) {
			return nil, domain.NewDomainError("5003100", errInternalServerError, err)
		}
		return buildDeleteVAResponse(req), nil
	}

	// Lookup VA
	va, err := u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.NewDomainError("4043112", "Invalid Bill/Virtual Account", nil)
		}
		return nil, domain.NewDomainError("5003100", errInternalServerError, err)
	}

	// Check status
	switch va.Status {
	case "03": // Pending — can delete
		if err := u.repo.UpdateVAStatus(ctx, req.VirtualAccountNo, "04"); err != nil {
			return nil, domain.NewDomainError("5003100", errInternalServerError, err)
		}
	case "00": // Success — cannot delete
		return nil, domain.NewDomainError("4053101", errOperationNotAllowed, nil)
	case "02": // Expired — cannot delete
		return nil, domain.NewDomainError("4053101", errOperationNotAllowed, nil)
	case "04": // Already deleted — idempotent
		// Return success
	default:
		return nil, domain.NewDomainError("4053101", errOperationNotAllowed, nil)
	}

	return buildDeleteVAResponse(req), nil
}

// buildDeleteVAResponse assembles the ASPI DeleteVAResponse. Shared by the
// no-bill registration-deactivation path and the transaction-cancellation path
// so the two cannot drift apart on the wire.
func buildDeleteVAResponse(req *domain.MerchantDeleteVARequest) *domain.MerchantDeleteVAResponse {
	return &domain.MerchantDeleteVAResponse{
		ResponseCode:    "2003100",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantDeleteVAData{
			PartnerServiceID: req.PartnerServiceID,
			CustomerNo:       req.CustomerNo,
			VirtualAccountNo: req.VirtualAccountNo,
			TrxID:            req.TrxID,
			AdditionalInfo:   req.AdditionalInfo,
		},
	}
}

// notificationURLFromAdditionalInfo extracts the merchant payment callback URL
// from additionalInfo.dbUrlProcess — the extension slot ASPI's VAUpsertRequest
// schema itself defines (aspi-open-api-va.yaml:317-320) for proprietary data
// like this, since notificationUrl is not a top-level spec field.
func notificationURLFromAdditionalInfo(additionalInfo map[string]interface{}) string {
	if additionalInfo == nil {
		return ""
	}
	if v, ok := additionalInfo["dbUrlProcess"].(string); ok {
		return v
	}
	return ""
}

// subCompanyFromAdditionalInfo extracts additionalInfo.subCompany — the
// biller's registered sub-company code. ASPI's VAUpsertRequest has no
// top-level subCompany field (it exists only on InquiryResponse and
// PaymentRequest), so the merchant declares it through the same additionalInfo
// extension slot used for dbUrlProcess/vaType; it is then persisted on the
// transaction and echoed back on every inquiry for that VA.
func subCompanyFromAdditionalInfo(additionalInfo map[string]interface{}) string {
	if additionalInfo == nil {
		return ""
	}
	if v, ok := additionalInfo["subCompany"].(string); ok {
		return v
	}
	return ""
}

// vaTypeFromAdditionalInfo extracts additionalInfo.vaType (feature
// 006-static-dynamic-va) — the 2-digit code routing a /create-va request to
// one of the six static/dynamic VA type combinations.
func vaTypeFromAdditionalInfo(additionalInfo map[string]interface{}) (string, bool) {
	if additionalInfo == nil {
		return "", false
	}
	v, ok := additionalInfo["vaType"].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// Ensure MerchantVAUsecase implements domain.MerchantVAUsecase
var _ domain.MerchantVAUsecase = (*MerchantVAUsecase)(nil)

// Ensure fmt is used
var _ = fmt.Sprintf
