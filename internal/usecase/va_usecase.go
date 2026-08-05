package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"backbone-new/internal/domain"
)

// errInvalidBill is the SNAP responseMessage paired with the 404-class
// bill/VA-not-found response codes in this package.
const errInvalidBill = "Invalid Bill/Virtual Account"

// isNotFound reports whether a repository lookup failed because the row does
// not exist, as opposed to the query itself failing (missing column, closed
// pool, timeout...). The repository layer maps pgx.ErrNoRows to a sentinel —
// ErrVAInvalidBill for GetInquiry/GetPayment, ErrMerchantVANotFound for
// GetVAByVirtualAccountNo — and returns the driver error verbatim otherwise,
// so callers MUST distinguish the two: treating a broken query as "not found"
// silently degrades into wrong answers — e.g. reporting a paid VA as still
// pending, or skipping the already-paid guard on /payment — instead of
// surfacing a 500.
func isNotFound(err error) bool {
	return errors.Is(err, domain.ErrVAInvalidBill) || errors.Is(err, domain.ErrMerchantVANotFound)
}

// VAUsecase implements domain.VAUsecase
type VAUsecase struct {
	repo         domain.VARepository
	notifier     domain.NotificationEnqueuer
	deliveryRepo domain.VANotificationDeliveryRepository
}

// NewVAUsecase creates a new VA usecase. notifier may be nil, in which case
// merchant payment callbacks are skipped (e.g. when the queue is unavailable).
func NewVAUsecase(repo domain.VARepository, notifier domain.NotificationEnqueuer) *VAUsecase {
	return &VAUsecase{repo: repo, notifier: notifier}
}

// NewVAUsecaseWithDeliveryRepo creates a new VA usecase with expiry-callback
// audit/dedupe support (feature 007-merchant-expiry-callback). deliveryRepo
// may be nil, in which case the dedupe check is skipped (best-effort: a
// missing audit trail must not block the expiry-detection/status-transition
// behavior itself).
func NewVAUsecaseWithDeliveryRepo(repo domain.VARepository, notifier domain.NotificationEnqueuer, deliveryRepo domain.VANotificationDeliveryRepository) *VAUsecase {
	return &VAUsecase{repo: repo, notifier: notifier, deliveryRepo: deliveryRepo}
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
	existing, err := u.repo.GetInquiry(ctx, req.InquiryRequestID)
	if err != nil && !isNotFound(err) {
		return nil, domain.NewDomainError("5002400", errInternalServerError, err)
	}
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

	// No-bill VAs (feature 013-no-bill-payment-transaction) resolve from the VA
	// registration, not from a transaction. This branch is what makes a
	// first-ever payment possible: after this feature a freshly registered
	// no-bill VA has no transaction row for the lookup below to find, and the
	// ad-hoc fallback further down would insert a phantom one.
	//
	// A VA with no registration falls through unchanged, which is what keeps
	// VAs created before this feature working (FR-022).
	account, accErr := u.repo.GetVAAccount(ctx, req.VirtualAccountNo)
	if accErr != nil && !errors.Is(accErr, domain.ErrVAAccountNotFound) {
		return nil, domain.NewDomainError("5002400", errInternalServerError, accErr)
	}
	if account.IsNoBill() {
		return u.inquiryNoBill(ctx, req, account)
	}

	// If this virtualAccountNo already has a merchant-created VA record, the
	// inquiry reflects that existing VA and MUST NOT insert another row keyed
	// by the vendor's own (possibly brand-new) inquiryRequestId — otherwise
	// every inquiry against the same VA creates a duplicate, phantom record.
	merchantVA, merr := u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo)
	if merr != nil && !isNotFound(merr) {
		return nil, domain.NewDomainError("5002400", errInternalServerError, merr)
	}
	if merchantVA != nil {
		// Expiry detection (feature 007-merchant-expiry-callback, contracts/
		// inquiry-expired.md): a pending ("03") VA whose expired_date has
		// passed is expired, detected inline with no background scanner.
		// Already-expired ("02") VAs must keep returning this same response
		// on every subsequent inquiry (spec.md User Story 1, Acceptance
		// Scenario 4) — markExpiredAndNotify no-ops safely when called again
		// (UpdateVAStatus's WHERE status='03' guard skips the already-"02"
		// row, so no duplicate callback is sent).
		isExpired := merchantVA.Status == "02" ||
			(merchantVA.Status == "03" && merchantVA.ExpiredDate != nil && time.Now().After(*merchantVA.ExpiredDate))
		if isExpired {
			u.markExpiredAndNotify(ctx, merchantVA)
			return nil, domain.NewDomainError("4042419", errInvalidBill, domain.ErrVAExpiredInquiry)
		}

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
		return nil, domain.NewDomainError("5002400", errInternalServerError, err)
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

// inquiryNoBill answers an inquiry for a no-bill VA from its registration
// (feature 013-no-bill-payment-transaction, FR-015..FR-017).
//
// It persists nothing. A no-bill VA's inquiry is a pure read — "who owns this
// number, and is it payable?" — so creating a record here would reintroduce
// the phantom rows this feature exists to remove.
func (u *VAUsecase) inquiryNoBill(ctx context.Context, req *domain.VAInquiryRequest, account *domain.VAAccount) (*domain.VAInquiryResponse, error) {
	if account.IsExpired(time.Now()) {
		u.markRegistrationExpiredAndNotify(ctx, account)
		return nil, domain.NewDomainError("4042419", errInvalidBill, domain.ErrVAExpiredInquiry)
	}
	if account.Status != domain.VAAccountStatusActive {
		return nil, domain.NewDomainError("4042419", errInvalidBill, domain.ErrVAAccountInactive)
	}

	// A no-bill VA asserts no bill amount — the customer chose what to pay at
	// the channel — so the request's own amount is echoed back rather than a
	// stored total.
	totalAmount := &domain.Amount{Value: "0.00", Currency: "IDR"}
	if req.Amount != nil {
		totalAmount = req.Amount
	}

	return &domain.VAInquiryResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAAccountData{
			InquiryStatus:      "00",
			InquiryReason:      &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			PartnerServiceID:   account.PartnerServiceID,
			CustomerNo:         account.CustomerNo,
			VirtualAccountNo:   account.VirtualAccountNo,
			VirtualAccountName: account.CustomerName,
			InquiryRequestID:   req.InquiryRequestID,
			TotalAmount:        totalAmount,
			SubCompany:         "00000",
		},
	}, nil
}

// paymentNoBill records ONE payment against a no-bill VA as its own settled
// transaction (feature 013-no-bill-payment-transaction, FR-008..FR-014).
//
// This is the heart of the fix. The old flow settled the single pending
// transaction that /create-va had created, so the VA was payable exactly once.
// Here nothing is settled and nothing is overwritten — a new row is inserted
// per payment, keyed by the payment's own paymentRequestId, and the
// registration is left untouched so it stays payable indefinitely.
func (u *VAUsecase) paymentNoBill(ctx context.Context, req *domain.VAPaymentRequest, account *domain.VAAccount) (*domain.VAPaymentResponse, error) {
	// Expiry and deactivation are properties of the REGISTRATION now, since
	// there is no pending transaction to carry them.
	if account.IsExpired(time.Now()) {
		u.markRegistrationExpiredAndNotify(ctx, account)
		return nil, domain.NewDomainError("4042519", errInvalidBill, domain.ErrVAExpiredPayment)
	}
	if account.Status != domain.VAAccountStatusActive {
		return nil, domain.NewDomainError("4042519", errInvalidBill, domain.ErrVAAccountInactive)
	}

	// A no-bill VA has no bill to match against, so the totalAmount/paidAmount
	// equality check the billed path applies is deliberately skipped. The only
	// amount rule left is that the payment must be a positive number.
	paidValue, convErr := strconv.ParseFloat(req.PaidAmount.Value, 64)
	if convErr != nil || paidValue <= 0 {
		return nil, domain.NewDomainError("4002501", "Invalid Field Format [paidAmount]", nil)
	}

	transactionDate := time.Now()
	if req.TrxDateTime != nil {
		transactionDate = *req.TrxDateTime
	}

	// Holder details come from the registration — it is the source of truth for
	// who owns this VA — with the payment channel's own values preferred where
	// it supplied them.
	customerName := account.CustomerName
	if req.VirtualAccountName != "" {
		customerName = req.VirtualAccountName
	}
	customerEmail := account.CustomerEmail
	if req.VirtualAccountEmail != "" {
		customerEmail = req.VirtualAccountEmail
	}
	customerPhone := account.CustomerPhone
	if req.VirtualAccountPhone != "" {
		customerPhone = req.VirtualAccountPhone
	}

	record := &domain.VAPaymentRecord{
		PartnerServiceID: account.PartnerServiceID,
		CustomerNo:       account.CustomerNo,
		CustomerName:     customerName,
		CustomerEmail:    customerEmail,
		CustomerPhone:    customerPhone,
		VirtualAccountNo: account.VirtualAccountNo,
		// inquiry_request_id is set to paymentRequestId unconditionally,
		// ignoring any inquiryRequestId/trxId the vendor sent. Those are shared
		// across payments (or absent entirely), so keying on either would make
		// two payments collide onto one row. paymentRequestId is mandatory and
		// unique per payment, and the column's existing UNIQUE index then gives
		// duplicate rejection for free (research.md R-003).
		InquiryRequestID:      req.PaymentRequestID,
		TrxID:                 account.TrxID,
		NotificationURL:       account.NotificationURL,
		PaymentRequestID:      req.PaymentRequestID,
		PaidAmount:            req.PaidAmount.Value,
		TotalAmount:           req.PaidAmount.Value, // no bill: the payment IS the total
		Currency:              req.PaidAmount.Currency,
		Status:                "00", // settled outright; there is no cumulative target to reach
		VAType:                account.VAType,
		ReferenceNo:           req.ReferenceNo,
		ChannelCode:           req.ChannelCode,
		HashedSourceAccountNo: req.HashedSourceAccountNo,
		SourceBankCode:        req.SourceBankCode,
		JournalNum:            req.JournalNum,
		PaymentType:           req.PaymentType,
		FlagAdvise:            req.FlagAdvise,
		PaidBills:             req.PaidBills,
		SubCompany:            req.SubCompany,
		TrxDateTime:           req.TrxDateTime,
		FreeTexts:             req.FreeTexts,
		TransactionDate:       transactionDate,
	}

	if err := u.repo.SaveNoBillPayment(ctx, record); err != nil {
		// A duplicate means a concurrent caller with the same paymentRequestId
		// won the race past the GetPayment short-circuit above. Replay that
		// payment's result instead of failing or double-recording — and notably
		// without sending a second callback, since we are not the writer.
		if errors.Is(err, domain.ErrVAPaymentDuplicate) {
			if existing, getErr := u.repo.GetPayment(ctx, req.PaymentRequestID); getErr == nil && existing != nil {
				return paymentResponseFromRecord(existing), nil
			}
		}
		return nil, domain.NewDomainError("5002500", errInternalServerError, err)
	}

	log.Printf("event=va_nobill_payment_recorded virtual_account_no=%s payment_request_id=%s", account.VirtualAccountNo, req.PaymentRequestID)

	u.notifyMerchantForAccount(ctx, req, account, transactionDate)

	return &domain.VAPaymentResponse{
		ResponseCode:    "2002500",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:    account.PartnerServiceID,
			CustomerNo:          account.CustomerNo,
			VirtualAccountNo:    account.VirtualAccountNo,
			VirtualAccountName:  customerName,
			VirtualAccountEmail: customerEmail,
			VirtualAccountPhone: customerPhone,
			TrxID:               account.TrxID,
			PaymentRequestID:    req.PaymentRequestID,
			PaidAmount:          req.PaidAmount,
			PaidBills:           req.PaidBills,
			TotalAmount:         &domain.Amount{Value: req.PaidAmount.Value, Currency: req.PaidAmount.Currency},
			TrxDateTime:         &transactionDate,
			ReferenceNo:         req.ReferenceNo,
			JournalNum:          req.JournalNum,
			PaymentType:         req.PaymentType,
			FlagAdvise:          req.FlagAdvise,
			PaymentFlagStatus:   "00",
			PaymentFlagReason:   &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			BillDetails:         echoPaymentBillDetails(req.BillDetails),
			FreeTexts:           req.FreeTexts,
		},
	}, nil
}

// paymentResponseFromRecord rebuilds the SNAP payment response from an
// already-persisted payment, used for idempotent replays.
func paymentResponseFromRecord(existing *domain.VAPaymentRecord) *domain.VAPaymentResponse {
	txDate := existing.TransactionDate
	return &domain.VAPaymentResponse{
		ResponseCode:    "2002500",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:    existing.PartnerServiceID,
			CustomerNo:          existing.CustomerNo,
			VirtualAccountNo:    existing.VirtualAccountNo,
			VirtualAccountName:  existing.CustomerName,
			VirtualAccountEmail: existing.CustomerEmail,
			VirtualAccountPhone: existing.CustomerPhone,
			TrxID:               existing.TrxID,
			PaymentRequestID:    existing.PaymentRequestID,
			PaidAmount:          &domain.Amount{Value: existing.PaidAmount, Currency: existing.Currency},
			PaidBills:           existing.PaidBills,
			TrxDateTime:         &txDate,
			ReferenceNo:         existing.ReferenceNo,
			JournalNum:          existing.JournalNum,
			PaymentType:         existing.PaymentType,
			FlagAdvise:          existing.FlagAdvise,
			PaymentFlagStatus:   "00",
			PaymentFlagReason:   &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
		},
	}
}

// Payment handles VA payment notification from vendor
func (u *VAUsecase) Payment(ctx context.Context, req *domain.VAPaymentRequest) (*domain.VAPaymentResponse, error) {
	// Validate required fields
	if req.PaymentRequestID == "" {
		return nil, domain.NewDomainError("4002502", "Invalid Mandatory Field [paymentRequestId]", nil)
	}

	if req.PaidAmount == nil {
		return nil, domain.NewDomainError("4002502", "Invalid Mandatory Field [paidAmount]", nil)
	}

	// Check if payment already exists (idempotency)
	existing, err := u.repo.GetPayment(ctx, req.PaymentRequestID)
	if err != nil && !isNotFound(err) {
		return nil, domain.NewDomainError("5002500", errInternalServerError, err)
	}
	if existing != nil {
		// Return existing payment status, echoing the identity/amount fields
		// persisted with the original request per PaymentResponse.virtualAccountData.
		existingTxDate := existing.TransactionDate
		return &domain.VAPaymentResponse{
			ResponseCode:    "2002500",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAPaymentStatus{
				PartnerServiceID:    existing.PartnerServiceID,
				CustomerNo:          existing.CustomerNo,
				VirtualAccountNo:    existing.VirtualAccountNo,
				VirtualAccountName:  existing.CustomerName,
				VirtualAccountEmail: existing.CustomerEmail,
				VirtualAccountPhone: existing.CustomerPhone,
				TrxID:               existing.TrxID,
				PaymentRequestID:    existing.PaymentRequestID,
				PaidAmount:          &domain.Amount{Value: existing.PaidAmount, Currency: existing.Currency},
				PaidBills:           existing.PaidBills,
				TrxDateTime:         &existingTxDate,
				ReferenceNo:         existing.ReferenceNo,
				JournalNum:          existing.JournalNum,
				PaymentType:         existing.PaymentType,
				FlagAdvise:          existing.FlagAdvise,
				PaymentFlagStatus:   "00",
				PaymentFlagReason:   &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			},
		}, nil
	}

	// No-bill VAs (vaType 01/04, feature 013-no-bill-payment-transaction) are
	// durable payment addresses, not single transactions: each payment creates
	// its OWN settled transaction, so the same VA number stays payable
	// indefinitely. This branch sits ahead of the transaction lookup below
	// precisely because there is no pending transaction to find.
	//
	// A VA with no registration falls through unchanged — that is what keeps
	// VAs created before this feature working (FR-022).
	account, accErr := u.repo.GetVAAccount(ctx, req.VirtualAccountNo)
	if accErr != nil && !errors.Is(accErr, domain.ErrVAAccountNotFound) {
		return nil, domain.NewDomainError("5002500", errInternalServerError, accErr)
	}
	if account.IsNoBill() {
		return u.paymentNoBill(ctx, req, account)
	}

	// Inherit customer name / trx ID / notificationUrl / inquiry_request_id
	// from the merchant's create-va record when one exists, so the mandatory
	// columns stay populated and the UPSERT below lands on that same row
	// instead of an orphan row keyed by the vendor's own inquiryRequestId.
	customerName := "Customer"
	// inquiryRequestId is not a field of the ASPI PaymentRequest at all, and
	// trxId is only Conditional ("Mandatory if Payment comes from the Create VA
	// Request") — so neither is guaranteed to arrive. Fall back finally to
	// paymentRequestId, which IS Mandatory and unique: the ON CONFLICT linkage
	// key below must never degrade to an empty string, or two unrelated orphan
	// payments would collide onto the same va_transactions row.
	inquiryRequestID := req.InquiryRequestID
	if inquiryRequestID == "" {
		inquiryRequestID = req.TrxID
	}
	if inquiryRequestID == "" {
		inquiryRequestID = req.PaymentRequestID
	}
	trxID := req.TrxID
	notificationURL := ""
	merchantVA, merr := u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo)
	if merr != nil && !isNotFound(merr) {
		return nil, domain.NewDomainError("5002500", errInternalServerError, merr)
	}

	// A payment may only land on a transaction that is currently PENDING
	// ("03"). Without this guard, a payment with a brand-new paymentRequestId
	// (so it misses the idempotency check above) against an already-paid
	// ("00"), expired ("02"), or deleted ("04") VA would still match this same
	// virtualAccountNo and silently overwrite the completed transaction's
	// paidAmount/referenceNo/transactionDate via SavePayment's upsert — a paid
	// transaction must never be mutated after the fact.
	if merchantVA != nil {
		// Expiry detection (feature 007-merchant-expiry-callback, contracts/
		// notify-expired.md): an already-expired VA, or a pending ("03") VA
		// whose expired_date has passed, returns the expired-specific SNAP
		// response instead of the generic conflict response.
		isExpired := merchantVA.Status == "02" ||
			(merchantVA.Status == "03" && merchantVA.ExpiredDate != nil && time.Now().After(*merchantVA.ExpiredDate))
		if isExpired {
			u.markExpiredAndNotify(ctx, merchantVA)
			return nil, domain.NewDomainError("4042519", errInvalidBill, domain.ErrVAExpiredPayment)
		}

		if merchantVA.Status != "03" {
			return nil, domain.NewDomainError("4092500", "Conflict: Bill/Virtual Account already paid or inactive", nil)
		}
	}

	// Variable-bill VAs (vaType 02/05, feature 006-static-dynamic-va) accept
	// multiple payments against the same VA number until the cumulative total
	// reaches totalAmount ("lunas") — each payment is individually recorded
	// via SaveVAPayment rather than the single-settlement equal-amount path
	// below, and is not subject to the exact totalAmount match check since a
	// partial payment is expected and valid.
	if merchantVA != nil && (merchantVA.VAType == "02" || merchantVA.VAType == "05") {
		paidAmount, status, err := u.repo.SaveVAPayment(ctx, merchantVA.ID, req.PaidAmount.Value, req.ReferenceNo)
		if err != nil {
			return nil, domain.NewDomainError("5002500", errInternalServerError, err)
		}

		transactionDate := time.Now()
		if req.TrxDateTime != nil {
			transactionDate = *req.TrxDateTime
		}

		trxID := merchantVA.TrxID
		if trxID == "" {
			trxID = req.TrxID
		}

		paymentFlagStatus := "03" // pending — cumulative total not yet reached
		if status == "00" {
			paymentFlagStatus = "00"
		}

		u.notifyMerchantWithVA(ctx, req, merchantVA, trxID, merchantVA.NotificationURL)

		return &domain.VAPaymentResponse{
			ResponseCode:    "2002500",
			ResponseMessage: "Successful",
			VirtualAccountData: &domain.VAPaymentStatus{
				PartnerServiceID:    req.PartnerServiceID,
				CustomerNo:          req.CustomerNo,
				VirtualAccountNo:    req.VirtualAccountNo,
				VirtualAccountName:  req.VirtualAccountName,
				VirtualAccountEmail: req.VirtualAccountEmail,
				VirtualAccountPhone: req.VirtualAccountPhone,
				TrxID:               trxID,
				PaymentRequestID:    req.PaymentRequestID,
				PaidAmount:          &domain.Amount{Value: paidAmount, Currency: req.PaidAmount.Currency},
				PaidBills:           req.PaidBills,
				TotalAmount:         req.TotalAmount,
				TrxDateTime:         &transactionDate,
				ReferenceNo:         req.ReferenceNo,
				JournalNum:          req.JournalNum,
				PaymentType:         req.PaymentType,
				FlagAdvise:          req.FlagAdvise,
				PaymentFlagStatus:   paymentFlagStatus,
				PaymentFlagReason:   getPaymentFlagReason(paymentFlagStatus),
				BillDetails:         echoPaymentBillDetails(req.BillDetails),
				FreeTexts:           req.FreeTexts,
			},
		}, nil
	}

	// Validate amount match (totalAmount is optional per spec; only checked when present)
	if req.TotalAmount != nil && req.PaidAmount.Value != req.TotalAmount.Value {
		return nil, domain.NewDomainError("4002501", "Invalid Field Format [amount mismatch]", nil)
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
	if req.VirtualAccountName != "" {
		customerName = req.VirtualAccountName
	}

	transactionDate := time.Now()
	if req.TrxDateTime != nil {
		transactionDate = *req.TrxDateTime
	}

	// Save payment record
	record := &domain.VAPaymentRecord{
		PartnerServiceID:      req.PartnerServiceID,
		CustomerNo:            req.CustomerNo,
		CustomerName:          customerName,
		CustomerEmail:         req.VirtualAccountEmail,
		CustomerPhone:         req.VirtualAccountPhone,
		VirtualAccountNo:      req.VirtualAccountNo,
		InquiryRequestID:      inquiryRequestID,
		TrxID:                 trxID,
		NotificationURL:       notificationURL,
		PaymentRequestID:      req.PaymentRequestID,
		PaidAmount:            req.PaidAmount.Value,
		TotalAmount:           paymentTotalAmountValue(req),
		Currency:              req.PaidAmount.Currency,
		Status:                "00",
		ReferenceNo:           req.ReferenceNo,
		ChannelCode:           req.ChannelCode,
		HashedSourceAccountNo: req.HashedSourceAccountNo,
		SourceBankCode:        req.SourceBankCode,
		JournalNum:            req.JournalNum,
		PaymentType:           req.PaymentType,
		FlagAdvise:            req.FlagAdvise,
		PaidBills:             req.PaidBills,
		SubCompany:            req.SubCompany,
		TrxDateTime:           req.TrxDateTime,
		FreeTexts:             req.FreeTexts,
		TransactionDate:       transactionDate,
	}

	if err := u.repo.SavePayment(ctx, record); err != nil {
		return nil, domain.NewDomainError("5002500", errInternalServerError, err)
	}

	if len(req.BillDetails) > 0 {
		if err := u.repo.SaveBillDetails(ctx, record.ID, paymentBillDetailsToBillDetail(req.BillDetails)); err != nil {
			return nil, domain.NewDomainError("5002500", errInternalServerError, err)
		}
	}

	// Notify the merchant asynchronously via their registered notificationUrl.
	// Best-effort: a failure here must not fail the vendor's payment response.
	u.notifyMerchantWithVA(ctx, req, merchantVA, trxID, notificationURL)

	// Build success response, echoing the identity/amount fields per
	// PaymentResponse.virtualAccountData.
	return &domain.VAPaymentResponse{
		ResponseCode:    "2002500",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:    req.PartnerServiceID,
			CustomerNo:          req.CustomerNo,
			VirtualAccountNo:    req.VirtualAccountNo,
			VirtualAccountName:  customerName,
			VirtualAccountEmail: req.VirtualAccountEmail,
			VirtualAccountPhone: req.VirtualAccountPhone,
			TrxID:               trxID,
			PaymentRequestID:    req.PaymentRequestID,
			PaidAmount:          req.PaidAmount,
			PaidBills:           req.PaidBills,
			TotalAmount:         req.TotalAmount,
			TrxDateTime:         req.TrxDateTime,
			ReferenceNo:         req.ReferenceNo,
			JournalNum:          req.JournalNum,
			PaymentType:         req.PaymentType,
			FlagAdvise:          req.FlagAdvise,
			PaymentFlagStatus:   "00",
			PaymentFlagReason:   &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			BillDetails:         echoPaymentBillDetails(req.BillDetails),
			FreeTexts:           req.FreeTexts,
		},
	}, nil
}

// paymentTotalAmountValue resolves the amount to persist as total_amount:
// the vendor's own totalAmount when sent, else the paidAmount (single
// full-settlement payments have no separate total).
func paymentTotalAmountValue(req *domain.VAPaymentRequest) string {
	if req.TotalAmount != nil {
		return req.TotalAmount.Value
	}
	return req.PaidAmount.Value
}

// paymentBillDetailsToBillDetail maps the inbound SNAP payment bill-detail
// shape to the shared BillDetail persistence type used by SaveBillDetails.
func paymentBillDetailsToBillDetail(bills []domain.VAPaymentBillDetail) []domain.BillDetail {
	out := make([]domain.BillDetail, 0, len(bills))
	for _, b := range bills {
		out = append(out, domain.BillDetail{
			BillCode:          b.BillCode,
			BillNo:            b.BillNo,
			BillName:          b.BillName,
			BillShortName:     b.BillShortName,
			BillDescription:   b.BillDescription,
			BillSubCompany:    b.BillSubCompany,
			BillAmount:        b.BillAmount,
			BillReferenceNo:   b.BillReferenceNo,
			BillerReferenceID: b.BillerReferenceID,
			Status:            b.Status,
			Reason:            b.Reason,
			AdditionalInfo:    b.AdditionalInfo,
		})
	}
	return out
}

// echoPaymentBillDetails echoes the vendor's bill details back in the
// response per ASPI PaymentResponse.virtualAccountData.billDetails,
// defaulting status/reason/billerReferenceId for a successful payment.
func echoPaymentBillDetails(bills []domain.VAPaymentBillDetail) []domain.VAPaymentBillDetail {
	if len(bills) == 0 {
		return nil
	}
	out := make([]domain.VAPaymentBillDetail, 0, len(bills))
	for _, b := range bills {
		billerReferenceID := b.BillerReferenceID
		if billerReferenceID == "" {
			billerReferenceID = b.BillNo
		}
		status := b.Status
		if status == "" {
			status = "00"
		}
		reason := b.Reason
		if reason == nil {
			reason = &domain.BilingualText{English: "Success", Indonesia: "Sukses"}
		}
		b.BillerReferenceID = billerReferenceID
		b.Status = status
		b.Reason = reason
		out = append(out, b)
	}
	return out
}

// markExpiredAndNotify transitions merchantVA to expired ("02") and, if this
// call is the one that actually applied the transition (i.e. it wasn't
// already expired) and the VA has a notification_url, enqueues a single
// "va.expired" merchant callback. Best-effort: notification delivery must
// never block or fail the caller's SNAP response (contracts/inquiry-expired.md,
// contracts/notify-expired.md).
func (u *VAUsecase) markExpiredAndNotify(ctx context.Context, merchantVA *domain.VAInquiryRecord) {
	// UpdateVAStatus is scoped to WHERE status = '03', so this is a no-op
	// (returns domain.ErrMerchantVANotFound) if another concurrent call, or a
	// concurrent payment, already moved the VA out of "03" — in that case we
	// must not enqueue a duplicate/incorrect notification.
	if err := u.repo.UpdateVAStatus(ctx, merchantVA.VirtualAccountNo, "02"); err != nil {
		return
	}
	log.Printf("event=va_expired virtual_account_no=%s event_type=%s", merchantVA.VirtualAccountNo, domain.NotificationEventVAExpired)

	if merchantVA.NotificationURL == "" || u.notifier == nil {
		return
	}

	// Dedupe: skip enqueueing if an auto-triggered va.expired notification was
	// already recorded for this VA (FR-005 belt-and-suspenders, on top of the
	// UpdateVAStatus guard above).
	if u.deliveryRepo != nil {
		exists, err := u.deliveryRepo.ExistsByVirtualAccountNoAndEventType(ctx, merchantVA.VirtualAccountNo, domain.NotificationEventVAExpired, domain.NotificationTriggerAuto)
		if err == nil && exists {
			return
		}
	}

	expiredAt := ""
	if merchantVA.ExpiredDate != nil {
		expiredAt = merchantVA.ExpiredDate.Format(time.RFC3339)
	}

	payload := &domain.PaymentNotificationPayload{
		EventType:        domain.NotificationEventVAExpired,
		PartnerServiceID: merchantVA.PartnerServiceID,
		CustomerNo:       merchantVA.CustomerNo,
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		TrxID:            merchantVA.TrxID,
		ReferenceNo:      "",
		NotificationURL:  merchantVA.NotificationURL,
		ExpiredAt:        expiredAt,
	}

	deliveryStatus := domain.NotificationDeliveryStatusSuccess
	errorDetail := ""
	if err := u.notifier.EnqueuePaymentNotification(ctx, payload); err != nil {
		deliveryStatus = domain.NotificationDeliveryStatusFailed
		errorDetail = err.Error()
	}

	if u.deliveryRepo != nil {
		_ = u.deliveryRepo.Create(ctx, &domain.NotificationDelivery{
			VirtualAccountNo: merchantVA.VirtualAccountNo,
			EventType:        domain.NotificationEventVAExpired,
			Trigger:          domain.NotificationTriggerAuto,
			Status:           deliveryStatus,
			AttemptedAt:      time.Now(),
			ErrorDetail:      errorDetail,
		})
	}
}

// markRegistrationExpiredAndNotify transitions a VA registration to EXPIRED
// and, if this call is the one that actually applied the transition, enqueues
// a single "va.expired" merchant callback (feature
// 013-no-bill-payment-transaction, FR-017).
//
// This is markExpiredAndNotify's sibling, one level up. A no-bill VA has no
// pending "03" transaction for that function's WHERE-clause guard to work on,
// so the same exactly-once trick is applied to the registration instead:
// UpdateVAAccountStatus is scoped to WHERE status='ACTIVE', and a zero-row
// result means someone else already detected the expiry and notified.
//
// Best-effort throughout: notification delivery must never block or fail the
// caller's SNAP response.
func (u *VAUsecase) markRegistrationExpiredAndNotify(ctx context.Context, account *domain.VAAccount) {
	if err := u.repo.UpdateVAAccountStatus(ctx, account.VirtualAccountNo, domain.VAAccountStatusExpired); err != nil {
		return
	}
	log.Printf("event=va_account_expired virtual_account_no=%s event_type=%s", account.VirtualAccountNo, domain.NotificationEventVAExpired)

	if account.NotificationURL == "" || u.notifier == nil {
		return
	}

	// Belt-and-suspenders dedupe on top of the status guard above, mirroring
	// markExpiredAndNotify (feature 007-merchant-expiry-callback, FR-005).
	if u.deliveryRepo != nil {
		exists, err := u.deliveryRepo.ExistsByVirtualAccountNoAndEventType(ctx, account.VirtualAccountNo, domain.NotificationEventVAExpired, domain.NotificationTriggerAuto)
		if err == nil && exists {
			return
		}
	}

	expiredAt := ""
	if account.ExpiredDate != nil {
		expiredAt = account.ExpiredDate.Format(time.RFC3339)
	}

	payload := &domain.PaymentNotificationPayload{
		EventType:        domain.NotificationEventVAExpired,
		PartnerServiceID: account.PartnerServiceID,
		CustomerNo:       account.CustomerNo,
		VirtualAccountNo: account.VirtualAccountNo,
		TrxID:            account.TrxID,
		NotificationURL:  account.NotificationURL,
		ExpiredAt:        expiredAt,
	}

	u.enqueueAndAudit(ctx, payload, account.VirtualAccountNo, domain.NotificationEventVAExpired)
}

// notifyMerchantForAccount enqueues the payment callback for a no-bill payment,
// sourcing the destination URL and merchant trace ID from the VA registration
// rather than from a transaction record. Best-effort: never blocks or fails the
// vendor-facing response.
func (u *VAUsecase) notifyMerchantForAccount(ctx context.Context, req *domain.VAPaymentRequest, account *domain.VAAccount, transactionDate time.Time) {
	if u.notifier == nil || account.NotificationURL == "" {
		return
	}

	payload := &domain.PaymentNotificationPayload{
		EventType:        domain.NotificationEventPaymentReceived,
		PartnerServiceID: account.PartnerServiceID,
		CustomerNo:       account.CustomerNo,
		VirtualAccountNo: account.VirtualAccountNo,
		TrxID:            account.TrxID,
		PaymentRequestID: req.PaymentRequestID,
		PaidAmount:       req.PaidAmount,
		PaidBills:        req.PaidBills,
		// A no-bill VA has no bill, so the payment is its own total.
		TotalAmount:     &domain.Amount{Value: req.PaidAmount.Value, Currency: req.PaidAmount.Currency},
		TrxDateTime:     transactionDate.Format(time.RFC3339),
		ReferenceNo:     req.ReferenceNo,
		PaymentType:     req.PaymentType,
		FlagAdvise:      req.FlagAdvise,
		NotificationURL: account.NotificationURL,
	}

	u.enqueueAndAudit(ctx, payload, account.VirtualAccountNo, domain.NotificationEventPaymentReceived)
}

// enqueueAndAudit enqueues a merchant callback and records the delivery
// attempt, swallowing both errors — callback delivery is best-effort and must
// not affect the caller's response (Constitution: vendor-facing responses are
// never blocked on merchant reachability).
func (u *VAUsecase) enqueueAndAudit(ctx context.Context, payload *domain.PaymentNotificationPayload, virtualAccountNo, eventType string) {
	deliveryStatus := domain.NotificationDeliveryStatusSuccess
	errorDetail := ""
	if err := u.notifier.EnqueuePaymentNotification(ctx, payload); err != nil {
		deliveryStatus = domain.NotificationDeliveryStatusFailed
		errorDetail = err.Error()
	}

	if u.deliveryRepo != nil {
		_ = u.deliveryRepo.Create(ctx, &domain.NotificationDelivery{
			VirtualAccountNo: virtualAccountNo,
			EventType:        eventType,
			Trigger:          domain.NotificationTriggerAuto,
			Status:           deliveryStatus,
			AttemptedAt:      time.Now(),
			ErrorDetail:      errorDetail,
		})
	}
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
		EventType:        domain.NotificationEventPaymentReceived,
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

	deliveryStatus := domain.NotificationDeliveryStatusSuccess
	errorDetail := ""
	if err := u.notifier.EnqueuePaymentNotification(ctx, payload); err != nil {
		deliveryStatus = domain.NotificationDeliveryStatusFailed
		errorDetail = err.Error()
	}

	if u.deliveryRepo != nil {
		_ = u.deliveryRepo.Create(ctx, &domain.NotificationDelivery{
			VirtualAccountNo: req.VirtualAccountNo,
			EventType:        domain.NotificationEventPaymentReceived,
			Trigger:          domain.NotificationTriggerAuto,
			Status:           deliveryStatus,
			AttemptedAt:      time.Now(),
			ErrorDetail:      errorDetail,
		})
	}
}

// Status handles VA status inquiry from vendor
func (u *VAUsecase) Status(ctx context.Context, req *domain.VAStatusRequest) (*domain.VAStatusResponse, error) {
	// Get payment record. Only a genuine "no such row" may fall through to the
	// pending/inquiry branch below — a failing query must surface as a 500,
	// otherwise a paid VA is reported back to the vendor as still pending.
	payment, err := u.repo.GetPayment(ctx, req.InquiryRequestID)
	if err != nil && !isNotFound(err) {
		return nil, domain.NewDomainError("5002600", errInternalServerError, err)
	}
	if err != nil {
		// If no payment found, check inquiry
		inquiry, inquiryErr := u.repo.GetInquiry(ctx, req.InquiryRequestID)
		if inquiryErr != nil && !isNotFound(inquiryErr) {
			return nil, domain.NewDomainError("5002600", errInternalServerError, inquiryErr)
		}
		if inquiryErr != nil {
			return nil, domain.NewDomainError("4042619", errInvalidBill, nil)
		}

		// Best-effort: bill details persisted at create-VA/inquiry time, if any.
		bills, _ := u.repo.GetVABillDetails(ctx, inquiry.ID)

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
				BillDetails:       billDetailsToStatusBillDetail(bills),
			},
		}, nil
	}

	// Best-effort: bill details persisted alongside the payment (if any).
	bills, _ := u.repo.GetVABillDetails(ctx, payment.ID)

	totalAmount := payment.TotalAmount
	if totalAmount == "" {
		totalAmount = payment.PaidAmount
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
			PaidBills:         payment.PaidBills,
			TotalAmount:       &domain.Amount{Value: totalAmount, Currency: payment.Currency},
			TrxDateTime:       payment.TrxDateTime,
			TransactionDate:   &payment.TransactionDate,
			ReferenceNo:       payment.ReferenceNo,
			PaymentType:       payment.PaymentType,
			FlagAdvise:        payment.FlagAdvise,
			BillDetails:       billDetailsToStatusBillDetail(bills),
			FreeTexts:         payment.FreeTexts,
		},
	}, nil
}

// billDetailsToStatusBillDetail maps the shared persisted BillDetail shape to
// the SNAP status-response bill-detail shape.
func billDetailsToStatusBillDetail(bills []domain.BillDetail) []domain.VAStatusBillDetail {
	if len(bills) == 0 {
		return nil
	}
	out := make([]domain.VAStatusBillDetail, 0, len(bills))
	for _, b := range bills {
		out = append(out, domain.VAStatusBillDetail{
			BillCode:        b.BillCode,
			BillNo:          b.BillNo,
			BillName:        b.BillName,
			BillShortName:   b.BillShortName,
			BillDescription: b.BillDescription,
			BillSubCompany:  b.BillSubCompany,
			BillAmount:      b.BillAmount,
			BillReferenceNo: b.BillReferenceNo,
			Status:          b.Status,
			Reason:          b.Reason,
			AdditionalInfo:  b.AdditionalInfo,
		})
	}
	return out
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
