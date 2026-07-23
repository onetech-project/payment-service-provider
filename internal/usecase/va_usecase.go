package usecase

import (
	"context"
	"fmt"
	"time"

	"backbone-new/internal/domain"
)

// VAUsecase implements domain.VAUsecase
type VAUsecase struct {
	repo     domain.VARepository
	notifier domain.NotificationEnqueuer
}

// NewVAUsecase creates a new VA usecase. notifier may be nil, in which case
// merchant payment callbacks are skipped (e.g. when the queue is unavailable).
func NewVAUsecase(repo domain.VARepository, notifier domain.NotificationEnqueuer) *VAUsecase {
	return &VAUsecase{repo: repo, notifier: notifier}
}

// Inquiry handles VA inquiry requests from vendor
func (u *VAUsecase) Inquiry(ctx context.Context, req *domain.VAInquiryRequest) (*domain.VAInquiryResponse, error) {
	// Validate VA number format
	if len(req.VirtualAccountNo) < 8 {
		return nil, domain.NewDomainError("4002401", "Invalid Field Format [virtualAccountNo]", nil)
	}

	if req.Amount == nil {
		return nil, domain.NewDomainError("4002402", "Invalid Mandatory Field [amount]", nil)
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

	// If this virtualAccountNo already has a merchant-created VA record, the
	// inquiry reflects that existing VA and MUST NOT insert another row keyed
	// by the vendor's own (possibly brand-new) inquiryRequestId — otherwise
	// every inquiry against the same VA creates a duplicate, phantom record.
	if merchantVA, merr := u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo); merr == nil && merchantVA != nil {
		// Best-effort: bill details are supplementary — a lookup failure
		// shouldn't fail the whole inquiry, just come back without them.
		bills, _ := u.repo.GetVABillDetails(ctx, merchantVA.ID)

		return &domain.VAInquiryResponse{
			ResponseCode:    "2002400",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAAccountData{
				InquiryStatus:      "00",
				InquiryReason:      &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
				PartnerServiceID:   merchantVA.PartnerServiceID,
				CustomerNo:         merchantVA.CustomerNo,
				VirtualAccountNo:   merchantVA.VirtualAccountNo,
				VirtualAccountName: merchantVA.CustomerName,
				InquiryRequestID:   req.InquiryRequestID,
				TotalAmount:        &domain.Amount{Value: merchantVA.TotalAmount, Currency: merchantVA.Currency},
				SubCompany:         "00000",
				BillDetails:        bills,
			},
		}, nil
	}

	// No prior merchant VA record — this is an ad-hoc inquiry with nothing to
	// reference yet, so start a fresh inquiry-only record keyed by this
	// request's own inquiryRequestId.
	customerName := "Customer"
	trxID := req.InquiryRequestID
	notificationURL := ""

	// Save inquiry record
	record := &domain.VAInquiryRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     customerName,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		TrxID:            trxID,
		NotificationURL:  notificationURL,
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
			VirtualAccountName: customerName,
			InquiryRequestID:   req.InquiryRequestID,
			TotalAmount:        &domain.Amount{Value: "0.00", Currency: "IDR"},
			SubCompany:         "00000",
		},
	}, nil
}

// Payment handles VA payment notification from vendor
func (u *VAUsecase) Payment(ctx context.Context, req *domain.VAPaymentRequest) (*domain.VAPaymentResponse, error) {
	// Validate required fields
	if req.PaymentRequestID == "" {
		return nil, domain.NewDomainError("4002402", "Invalid Mandatory Field [paymentRequestId]", nil)
	}

	if req.PaidAmount == nil {
		return nil, domain.NewDomainError("4002402", "Invalid Mandatory Field [paidAmount]", nil)
	}

	// Check if payment already exists (idempotency)
	existing, _ := u.repo.GetPayment(ctx, req.PaymentRequestID)
	if existing != nil {
		// Return existing payment status, echoing the identity/amount fields
		// persisted with the original request per PaymentResponse.virtualAccountData.
		existingTxDate := existing.TransactionDate
		return &domain.VAPaymentResponse{
			ResponseCode:    "2002400",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAPaymentStatus{
				PartnerServiceID:  existing.PartnerServiceID,
				CustomerNo:        existing.CustomerNo,
				VirtualAccountNo:  existing.VirtualAccountNo,
				TrxID:             existing.TrxID,
				PaymentRequestID:  existing.PaymentRequestID,
				PaidAmount:        &domain.Amount{Value: existing.PaidAmount, Currency: existing.Currency},
				TrxDateTime:       &existingTxDate,
				ReferenceNo:       existing.ReferenceNo,
				PaymentFlagStatus: "00",
				PaymentFlagReason: &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			},
		}, nil
	}

	// Validate amount match (totalAmount is optional per spec; only checked when present)
	if req.TotalAmount != nil && req.PaidAmount.Value != req.TotalAmount.Value {
		return nil, domain.NewDomainError("4002401", "Invalid Field Format [amount mismatch]", nil)
	}

	// Inherit customer name / trx ID / notificationUrl / inquiry_request_id
	// from the merchant's create-va record when one exists, so the mandatory
	// columns stay populated and the UPSERT below lands on that same row
	// instead of an orphan row keyed by the vendor's own inquiryRequestId.
	customerName := "Customer"
	inquiryRequestID := req.InquiryRequestID
	trxID := req.InquiryRequestID
	notificationURL := ""
	merchantVA, _ := u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo)

	// A payment may only land on a transaction that is currently PENDING
	// ("03"). Without this guard, a payment with a brand-new paymentRequestId
	// (so it misses the idempotency check above) against an already-paid
	// ("00"), expired ("02"), or deleted ("04") VA would still match this same
	// virtualAccountNo and silently overwrite the completed transaction's
	// paidAmount/referenceNo/transactionDate via SavePayment's upsert — a paid
	// transaction must never be mutated after the fact.
	if merchantVA != nil && merchantVA.Status != "03" {
		return nil, domain.NewDomainError("4092500", "Conflict: Bill/Virtual Account already paid or inactive", nil)
	}

	if merchantVA != nil {
		if merchantVA.CustomerName != "" {
			customerName = merchantVA.CustomerName
		}
		if merchantVA.InquiryRequestID != "" {
			inquiryRequestID = merchantVA.InquiryRequestID
		}
		if merchantVA.TrxID != "" {
			trxID = merchantVA.TrxID
		}
		notificationURL = merchantVA.NotificationURL
	}

	transactionDate := time.Now()
	if req.TrxDateTime != nil {
		transactionDate = *req.TrxDateTime
	}

	// Save payment record
	record := &domain.VAPaymentRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     customerName,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: inquiryRequestID,
		TrxID:            trxID,
		NotificationURL:  notificationURL,
		PaymentRequestID: req.PaymentRequestID,
		PaidAmount:       req.PaidAmount.Value,
		Currency:         req.PaidAmount.Currency,
		Status:           "00",
		ReferenceNo:      req.ReferenceNo,
		TransactionDate:  transactionDate,
	}

	if err := u.repo.SavePayment(ctx, record); err != nil {
		return nil, domain.NewDomainError("5002400", "Internal Server Error", err)
	}

	// Notify the merchant asynchronously via their registered notificationUrl.
	// Best-effort: a failure here must not fail the vendor's payment response.
	u.notifyMerchantWithVA(ctx, req, merchantVA, trxID, notificationURL)

	// Build success response, echoing the identity/amount fields per
	// PaymentResponse.virtualAccountData.
	return &domain.VAPaymentResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:  req.PartnerServiceID,
			CustomerNo:        req.CustomerNo,
			VirtualAccountNo:  req.VirtualAccountNo,
			TrxID:             trxID,
			PaymentRequestID:  req.PaymentRequestID,
			PaidAmount:        req.PaidAmount,
			TotalAmount:       req.TotalAmount,
			TrxDateTime:       req.TrxDateTime,
			ReferenceNo:       req.ReferenceNo,
			PaymentFlagStatus: "00",
			PaymentFlagReason: &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
		},
	}, nil
}

// notifyMerchantWithVA enqueues an async callback carrying the payment
// details to the merchant's registered notificationUrl. It never returns an
// error to the caller: notification delivery is best-effort and must not
// block or fail the vendor-facing payment response.
func (u *VAUsecase) notifyMerchantWithVA(ctx context.Context, req *domain.VAPaymentRequest, merchantVA *domain.VAInquiryRecord, trxID, notificationURL string) {
	if u.notifier == nil || merchantVA == nil || notificationURL == "" {
		return
	}

	trxDateTime := ""
	if req.TrxDateTime != nil {
		trxDateTime = req.TrxDateTime.Format(time.RFC3339)
	}

	payload := &domain.PaymentNotificationPayload{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		TrxID:            trxID,
		PaymentRequestID: req.PaymentRequestID,
		PaidAmount:       req.PaidAmount,
		PaidBills:        req.PaidBills,
		TotalAmount:      req.TotalAmount,
		TrxDateTime:      trxDateTime,
		ReferenceNo:      req.ReferenceNo,
		PaymentType:      req.PaymentType,
		FlagAdvise:       req.FlagAdvise,
		NotificationURL:  notificationURL,
	}

	_ = u.notifier.EnqueuePaymentNotification(ctx, payload)
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
