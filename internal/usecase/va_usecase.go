package usecase

import (
	"context"
	"fmt"
	"time"

	"backbone-new/internal/domain"
)

// VAUsecase implements domain.VAUsecase
type VAUsecase struct {
	repo domain.VARepository
}

// NewVAUsecase creates a new VA usecase
func NewVAUsecase(repo domain.VARepository) *VAUsecase {
	return &VAUsecase{repo: repo}
}

// Inquiry handles VA inquiry requests from vendor
func (u *VAUsecase) Inquiry(ctx context.Context, req *domain.VAInquiryRequest) (*domain.VAInquiryResponse, error) {
	// Validate VA number format
	if len(req.VirtualAccountNo) < 8 {
		return nil, domain.NewDomainError("4002401", "Invalid Field Format [virtualAccountNo]", nil)
	}

	// Check if inquiry already exists (idempotency)
	existing, _ := u.repo.GetInquiry(ctx, req.InquiryRequestID)
	if existing != nil {
		// Return cached response for idempotent requests
		return &domain.VAInquiryResponse{
			ResponseCode:    "2002400",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAAccountData{
				InquiryStatus:      "00",
				InquiryReason:      &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
				PartnerServiceID:   req.PartnerServiceID,
				CustomerNo:         req.CustomerNo,
				VirtualAccountNo:   req.VirtualAccountNo,
				VirtualAccountName: "Customer",
				InquiryRequestID:   req.InquiryRequestID,
				TotalAmount:        &domain.Amount{Value: "0.00", Currency: "IDR"},
			},
		}, nil
	}

	// Save inquiry record
	record := &domain.VAInquiryRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		Status:           "00",
		TotalAmount:      "0.00",
		Currency:         "IDR",
	}

	if err := u.repo.SaveInquiry(ctx, record); err != nil {
		return nil, domain.NewDomainError("5002400", "Internal Server Error", err)
	}

	// Build success response
	return &domain.VAInquiryResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAAccountData{
			InquiryStatus:      "00",
			InquiryReason:      &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			PartnerServiceID:   req.PartnerServiceID,
			CustomerNo:         req.CustomerNo,
			VirtualAccountNo:   req.VirtualAccountNo,
			VirtualAccountName: "Customer",
			InquiryRequestID:   req.InquiryRequestID,
			TotalAmount:        &domain.Amount{Value: "0.00", Currency: "IDR"},
			SubCompany:         "00000",
		},
	}, nil
}

// Payment handles VA payment notification from vendor
func (u *VAUsecase) Payment(ctx context.Context, req *domain.VAPaymentRequest) (*domain.VAPaymentResponse, error) {
	// Validate required fields
	if req.PaidAmount == nil || req.TotalAmount == nil {
		return nil, domain.NewDomainError("4002402", "Invalid Mandatory Field [amount]", nil)
	}

	if req.TransactionDate == nil {
		return nil, domain.NewDomainError("4002402", "Invalid Mandatory Field [transactionDate]", nil)
	}

	// Check if payment already exists (idempotency)
	existing, _ := u.repo.GetPayment(ctx, req.PaymentRequestID)
	if existing != nil {
		// Return existing payment status
		return &domain.VAPaymentResponse{
			ResponseCode:    "2002400",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAPaymentStatus{
				PaymentFlagStatus: "00",
				PaymentFlagReason: &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			},
		}, nil
	}

	// Validate amount match
	if req.PaidAmount.Value != req.TotalAmount.Value {
		return nil, domain.NewDomainError("4002401", "Invalid Field Format [amount mismatch]", nil)
	}

	// Save payment record
	record := &domain.VAPaymentRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		PaymentRequestID: req.PaymentRequestID,
		PaidAmount:       req.PaidAmount.Value,
		Currency:         req.PaidAmount.Currency,
		Status:           "00",
		ReferenceNo:      req.ReferenceNo,
		TransactionDate:  *req.TransactionDate,
	}

	if err := u.repo.SavePayment(ctx, record); err != nil {
		return nil, domain.NewDomainError("5002400", "Internal Server Error", err)
	}

	// Build success response
	return &domain.VAPaymentResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PaymentFlagStatus: "00",
			PaymentFlagReason: &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
		},
	}, nil
}

// Status handles VA status inquiry from vendor
func (u *VAUsecase) Status(ctx context.Context, req *domain.VAStatusRequest) (*domain.VAStatusResponse, error) {
	// Get payment record
	payment, err := u.repo.GetPayment(ctx, req.InquiryRequestID)
	if err != nil {
		// If no payment found, check inquiry
		inquiry, inquiryErr := u.repo.GetInquiry(ctx, req.InquiryRequestID)
		if inquiryErr != nil {
			return nil, domain.NewDomainError("4042419", "Invalid Bill/Virtual Account", nil)
		}

		// Return inquiry status (pending)
		return &domain.VAStatusResponse{
			ResponseCode:    "2002600",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAStatusData{
				PaymentFlagStatus: "03",
				PaymentFlagReason: &domain.BilingualText{English: "Pending", Indonesia: "Tertunda"},
				PartnerServiceID:  inquiry.PartnerServiceID,
				CustomerNo:        inquiry.CustomerNo,
				VirtualAccountNo:  inquiry.VirtualAccountNo,
				InquiryRequestID:  inquiry.InquiryRequestID,
				TotalAmount:       &domain.Amount{Value: inquiry.TotalAmount, Currency: inquiry.Currency},
			},
		}, nil
	}

	// Build status response
	return &domain.VAStatusResponse{
		ResponseCode:    "2002600",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAStatusData{
			PaymentFlagStatus: payment.Status,
			PaymentFlagReason: getPaymentFlagReason(payment.Status),
			PartnerServiceID:  payment.PartnerServiceID,
			CustomerNo:        payment.CustomerNo,
			VirtualAccountNo:  payment.VirtualAccountNo,
			InquiryRequestID:  payment.InquiryRequestID,
			PaymentRequestID:  payment.PaymentRequestID,
			PaidAmount:        &domain.Amount{Value: payment.PaidAmount, Currency: payment.Currency},
			TotalAmount:       &domain.Amount{Value: payment.PaidAmount, Currency: payment.Currency},
			TransactionDate:   &payment.TransactionDate,
			ReferenceNo:       payment.ReferenceNo,
		},
	}, nil
}

func getPaymentFlagReason(status string) *domain.BilingualText {
	switch status {
	case "00":
		return &domain.BilingualText{English: "Success", Indonesia: "Sukses"}
	case "01":
		return &domain.BilingualText{English: "Reject", Indonesia: "Ditolak"}
	case "02":
		return &domain.BilingualText{English: "Timeout", Indonesia: "Waktu Habis"}
	case "03":
		return &domain.BilingualText{English: "Pending", Indonesia: "Tertunda"}
	default:
		return &domain.BilingualText{English: fmt.Sprintf("Status: %s", status), Indonesia: fmt.Sprintf("Status: %s", status)}
	}
}

// Ensure VAUsecase implements domain.VAUsecase
var _ domain.VAUsecase = (*VAUsecase)(nil)

// Ensure time package is used
var _ = time.Now
