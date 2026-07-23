package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backbone-new/internal/domain"
)

// MerchantVAUsecase implements domain.MerchantVAUsecase
type MerchantVAUsecase struct {
	repo domain.VARepository
}

// NewMerchantVAUsecase creates a new merchant VA usecase
func NewMerchantVAUsecase(repo domain.VARepository) *MerchantVAUsecase {
	return &MerchantVAUsecase{repo: repo}
}

// CreateVA handles VA creation per ASPI VAUpsertRequest (Service Code 27)
func (u *MerchantVAUsecase) CreateVA(ctx context.Context, req *domain.MerchantCreateVARequest) (*domain.MerchantCreateVAResponse, error) {
	// Validate required fields per ASPI VAIdentity (partnerServiceId,
	// customerNo, virtualAccountNo are all mandatory client-supplied input).
	if req.PartnerServiceID == "" || req.CustomerNo == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [partnerServiceId/customerNo]", nil)
	}
	if req.VirtualAccountNo == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [virtualAccountNo]", nil)
	}
	if req.VirtualAccountName == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [virtualAccountName]", nil)
	}
	if req.TrxID == "" {
		return nil, domain.NewDomainError("4002701", "Invalid Mandatory Field [trxId]", nil)
	}

	// Validate virtualAccountTrxType if provided
	if req.VirtualAccountTrxType != "" {
		validTypes := map[string]bool{"C": true, "O": true, "I": true, "M": true, "L": true, "N": true, "X": true}
		if !validTypes[req.VirtualAccountTrxType] {
			return nil, domain.NewDomainError("4002700", "Invalid Field Format [virtualAccountTrxType]", nil)
		}
	}

	// Use the client-supplied virtualAccountNo per ASPI VAIdentity (maxLength 28)
	vaNo := req.VirtualAccountNo
	if len(vaNo) > 28 {
		return nil, domain.NewDomainError("4002700", "Invalid Field Format [virtualAccountNo too long]", nil)
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

	// Save transaction
	now := time.Now()
	record := &domain.VAInquiryRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     req.VirtualAccountName,
		VirtualAccountNo: vaNo,
		InquiryRequestID: req.TrxID,
		TrxID:            req.TrxID,
		NotificationURL:  notificationURLFromAdditionalInfo(req.AdditionalInfo),
		Status:           "03",
		TotalAmount:      "0",
		Currency:         "IDR",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if req.TotalAmount != nil {
		record.TotalAmount = req.TotalAmount.Value
		record.Currency = req.TotalAmount.Currency
	}

	if err := u.repo.SaveInquiry(ctx, record); err != nil {
		return nil, domain.NewDomainError("5002700", "Internal Server Error", err)
	}

	if len(req.BillDetails) > 0 {
		if err := u.repo.SaveBillDetails(ctx, record.ID, req.BillDetails); err != nil {
			return nil, domain.NewDomainError("5002700", "Internal Server Error", err)
		}
	}

	// Build VAUpsertResponse
	resp := &domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			PartnerServiceID:    req.PartnerServiceID,
			CustomerNo:          req.CustomerNo,
			VirtualAccountNo:    vaNo,
			VirtualAccountName:  req.VirtualAccountName,
			VirtualAccountEmail: req.VirtualAccountEmail,
			VirtualAccountPhone: req.VirtualAccountPhone,
			TrxID:               req.TrxID,
			TotalAmount:         req.TotalAmount,
			BillDetails:         req.BillDetails,
			FreeTexts:           req.FreeTexts,
			VirtualAccountTrxType: req.VirtualAccountTrxType,
			FeeAmount:           req.FeeAmount,
			ExpiredDate:         req.ExpiredDate,
			LastUpdateDate:      &now,
			AdditionalInfo:      req.AdditionalInfo,
		},
	}

	return resp, nil
}

// ListVA handles VA listing (merchant dashboard convenience API)
func (u *MerchantVAUsecase) ListVA(ctx context.Context, req *domain.MerchantListVARequest) (*domain.MerchantListVAResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := &domain.VAListFilter{
		PartnerServiceID: req.PartnerServiceID,
		FromDate:         req.FromDate,
		ToDate:           req.ToDate,
		Status:           req.Status,
		VirtualAccountNo: req.VirtualAccountNo,
		Offset:           (page - 1) * pageSize,
		Limit:            pageSize,
	}

	items, total, err := u.repo.ListVA(ctx, filter)
	if err != nil {
		return nil, domain.NewDomainError("5002400", "Internal Server Error", err)
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	return &domain.MerchantListVAResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		Data:            items,
		Pagination: &domain.Pagination{
			Page:       page,
			PageSize:   pageSize,
			TotalRows:  total,
			TotalPages: totalPages,
		},
	}, nil
}

// DeleteVA handles VA deletion per ASPI DeleteVARequest (Service Code 31)
func (u *MerchantVAUsecase) DeleteVA(ctx context.Context, req *domain.MerchantDeleteVARequest) (*domain.MerchantDeleteVAResponse, error) {
	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" {
		return nil, domain.NewDomainError("4003101", "Invalid Mandatory Field", nil)
	}

	// Lookup VA
	va, err := u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, domain.NewDomainError("4043112", "Invalid Bill/Virtual Account", nil)
		}
		return nil, domain.NewDomainError("5003100", "Internal Server Error", err)
	}

	// Check status
	switch va.Status {
	case "03": // Pending — can delete
		if err := u.repo.UpdateVAStatus(ctx, req.VirtualAccountNo, "04"); err != nil {
			return nil, domain.NewDomainError("5003100", "Internal Server Error", err)
		}
	case "00": // Success — cannot delete
		return nil, domain.NewDomainError("4053101", "Requested Operation Is Not Allowed", nil)
	case "02": // Expired — cannot delete
		return nil, domain.NewDomainError("4053101", "Requested Operation Is Not Allowed", nil)
	case "04": // Already deleted — idempotent
		// Return success
	default:
		return nil, domain.NewDomainError("4053101", "Requested Operation Is Not Allowed", nil)
	}

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
	}, nil
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

// Ensure MerchantVAUsecase implements domain.MerchantVAUsecase
var _ domain.MerchantVAUsecase = (*MerchantVAUsecase)(nil)

// Ensure fmt is used
var _ = fmt.Sprintf
