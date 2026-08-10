package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"backbone-new/internal/domain"
)

// errInvalidBill is the SNAP responseMessage paired with the 404-class
// bill/VA-not-found response codes in this package.
const errInvalidBill = "Invalid Bill/Virtual Account"

// flagAdviseRetry is BCA's flagAdvise value for an advice (retry) request:
// "Y = advice request (retry flag) for payment flag". "N" is a new request.
// A retry is a deliberate re-send of a payment BCA believes may not have been
// recorded, so it must replay the original outcome rather than be treated as
// an accidental double-flag.
const flagAdviseRetry = "Y"

// defaultCurrency is the currency assumed when a stored record carries none.
// BCA's VA services accept IDR, SGD and USD; IDR is the only one a domestic
// biller transaction realistically settles in.
const defaultCurrency = "IDR"

// zeroAmount is the String(13.2) rendering of nothing-paid-yet. BCA documents
// amounts as carrying two decimals, so "0" is not the same wire value as
// "0.00".
const zeroAmount = "0.00"

// defaultSubCompany is BCA's documented default sub-company code, used when
// the biller has registered no product-specific code of its own: "Product Name
// and Admin Fee from subCompany 00000 would be used and shown in channel".
const defaultSubCompany = "00000"

// statusSuccessMessage is the responseMessage BCA's Appendix A pairs with
// 2002600 on the inquiry-status service. It is "Success" there, where the
// inquiry and payment services' own samples use "Successful" — the doc is not
// self-consistent across services, so each is spelled as its own table has it.
const statusSuccessMessage = "Success"

// amountEpsilon bounds float comparison of money strings. Amounts are capped
// at 13 integer digits with 2 decimals, well inside float64's exact-integer
// range, so a half-cent tolerance only absorbs representation noise — it can
// never merge two genuinely different amounts.
const amountEpsilon = 0.005

// responseMessage for domain.CodePaymentInconsistent (4042518), returned when
// a paymentRequestId that has already been recorded is submitted again without
// flagAdvise "Y" (the vendor reused both X-EXTERNAL-ID and paymentRequestId).
// The response still echoes the original payment's virtualAccountData — the
// caller needs to see which payment it collided with — but the responseCode
// marks the request itself as rejected rather than replaying it as a success.
const snapMsgInconsistentRequest = "Inconsistent Request"

// responseMessage for the "this VA exists nowhere in this system" rejection —
// no registration and no transaction. Shared by inquiry (4042412) and payment
// (4042512): BCA documents both as "Invalid Bill/Virtual Account [Reason]",
// and "[Not Found]" is the reason this system fills in.
const snapMsgVANotFound = errInvalidBill + " [Not Found]"

// The bilingual paymentFlagReason for each refusal lives in
// domain.ReasonForCode, keyed by the response code, and is stamped onto the
// response by domain.NewPaymentErrorResponse in the handler. Each names the
// refusal specifically rather than reusing getPaymentFlagReason's generic
// "Reject"/"Ditolak" for flag "01" — the vendor displays this text to the
// customer, and "rejected" alone does not say WHY.

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
	// Field-level validation lives in domain.ValidateInquiryRequest and runs in
	// the handler, which is what lets it distinguish BCA's 4002401 (Invalid
	// Field Format) from 4002402 (Invalid Mandatory Field). Notably absent
	// there and here: any requirement for an `amount` field — BCA's inquiry
	// payload has none, and demanding one rejected every conformant inquiry.

	// No-bill VAs (feature 013-no-bill-payment-transaction) are answered from
	// the VA registration, never from a transaction — so this check comes
	// before BOTH lookups below, not between them.
	//
	// That ordering is load-bearing. A no-bill VA's transactions are history,
	// not a gate: it is registered once and paid many times. If the lookups ran
	// first, the most recent settled transaction would be found and the status
	// switch below would answer 4042414 "Paid Bill" — making the VA
	// un-inquirable after its first payment, which is precisely the defect this
	// feature removes. It also has no transaction row at all until the first
	// payment, so a freshly registered VA would otherwise be reported 4042412.
	//
	// A VA with no registration falls through unchanged, which is what keeps
	// VAs created before this feature working (FR-022).
	account, accErr := u.repo.GetVAAccount(ctx, req.VirtualAccountNo)
	if accErr != nil && !errors.Is(accErr, domain.ErrVAAccountNotFound) {
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodeInquiry), errInternalServerError, accErr)
	}
	if account.IsNoBill() {
		return u.inquiryNoBill(ctx, req, account)
	}

	// Resolve the VA this inquiry refers to. Two lookups, one record: the
	// vendor's own inquiryRequestId (an idempotent replay of an inquiry we
	// already recorded) first, then the virtualAccountNo (a merchant-created VA
	// being inquired for the first time). Both return the same va_transactions
	// row shape and are answered from the SAME builder below, so a replay can
	// never report different bill data than the original inquiry did.
	record, err := u.repo.GetInquiry(ctx, req.InquiryRequestID)
	if err != nil && !isNotFound(err) {
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodeInquiry), errInternalServerError, err)
	}
	if record == nil {
		// A merchant-created VA MUST NOT get a second row inserted under the
		// vendor's own (possibly brand-new) inquiryRequestId — otherwise every
		// inquiry against the same VA creates a duplicate, phantom record.
		var merr error
		record, merr = u.repo.GetVAByVirtualAccountNo(ctx, req.VirtualAccountNo)
		if merr != nil && !isNotFound(merr) {
			return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodeInquiry), errInternalServerError, merr)
		}
	}

	if record != nil {
		// Expiry detection (feature 007-merchant-expiry-callback, contracts/
		// inquiry-expired.md): a pending ("03") VA whose expired_date has
		// passed is expired, detected inline with no background scanner.
		// Already-expired ("02") VAs must keep returning this same response
		// on every subsequent inquiry (spec.md User Story 1, Acceptance
		// Scenario 4) — markExpiredAndNotify no-ops safely when called again
		// (UpdateVAStatus's WHERE status='03' guard skips the already-"02"
		// row, so no duplicate callback is sent).
		isExpired := record.Status == "02" ||
			(record.Status == "03" && record.ExpiredDate != nil && time.Now().After(*record.ExpiredDate))
		if isExpired {
			u.markExpiredAndNotify(ctx, record)
			return nil, domain.NewInquiryError(domain.CodeInquiryExpired, errInvalidBill, domain.ErrVAExpiredInquiry,
				u.rejectedAccountData(ctx, record, req.InquiryRequestID, domain.CodeInquiryExpired))
		}

		// The persisted transaction decides the inquiry outcome — a bill that is
		// already settled or cancelled is not payable, and SNAP conveys that
		// through the responseCode plus inquiryStatus "01", not through a 200
		// that would invite a second payment. The rejection still reports the
		// full VA (name, amount, bills) so the vendor can show the customer
		// WHICH bill it is refusing and why.
		//
		// "00" is read through IsPayable rather than compared directly: a
		// variable-bill VA whose cumulative payments still fall short of
		// totalAmount is NOT a paid bill, however its status column reads, and
		// answering 4042414 there would make the outstanding balance
		// uncollectable.
		switch record.Status {
		case "00":
			if record.IsPayable() {
				break
			}
			return nil, domain.NewInquiryError(domain.CodeInquiryPaidBill, "Paid Bill", domain.ErrVAPaidBill,
				u.rejectedAccountData(ctx, record, req.InquiryRequestID, domain.CodeInquiryPaidBill))
		case "04":
			return nil, domain.NewInquiryError(domain.CodeInquiryNotFound, snapMsgVANotFound, domain.ErrVAInvalidBill,
				u.rejectedAccountData(ctx, record, req.InquiryRequestID, domain.CodeInquiryNotFound))
		}

		// A merchant-created VA carries no vendor inquiryRequestId — at create-va
		// time it does not exist yet. This first inquiry is what supplies it, so
		// claim it onto the row: Status() and Payment() resolve a transaction by
		// that id, and without this the merchant's row would stay unreachable by
		// it forever.
		//
		// What counts as "carries none" is spelled out in
		// domain.IsPlaceholderInquiryRequestID: the empty string, a copy of
		// trxId, or the VA number — one shape per generation of the create-va
		// writer. None is ever a real vendor id, so all are free to be
		// replaced here. A genuine id already claimed by an earlier inquiry is
		// left alone — the repository's guard enforces the same rule.
		if domain.IsPlaceholderInquiryRequestID(record.InquiryRequestID, record.TrxID, record.VirtualAccountNo) && req.InquiryRequestID != "" {
			if err := u.repo.ClaimInquiryRequestID(ctx, record.ID, req.InquiryRequestID); err != nil {
				return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodeInquiry), errInternalServerError, err)
			}
			record.InquiryRequestID = req.InquiryRequestID
		}

		// Best-effort: bill details are supplementary — a lookup failure
		// shouldn't fail the whole inquiry, just come back without them.
		bills, _ := u.repo.GetVABillDetails(ctx, record.ID)

		return inquiryResponseFromRecord(record, req.InquiryRequestID, bills), nil
	}

	// No prior record at all: the VA does not exist as far as this system is
	// concerned, so the inquiry is answered 4042412 (404) rather than being
	// used to conjure one. An inquiry is a read of an existing bill — only the
	// merchant's create-va brings a VA into existence, and inventing a row here
	// would hand the vendor a payable bill for a VA no merchant ever issued.
	// No InquiryData: there is no stored VA to describe. The handler answers
	// this one from the request's own identity fields, so the vendor still gets
	// its keys back next to inquiryStatus "01" without this layer inventing a
	// VA that does not exist.
	return nil, domain.NewDomainError(domain.CodeInquiryNotFound, snapMsgVANotFound, domain.ErrVAInvalidBill)
}

// inquiryReasonSuccess is the inquiryReason on a 2002400. Every rejection's
// reason comes from domain.ReasonForCode instead, so the refusal texts live in
// one table next to the codes they belong to.
var inquiryReasonSuccess = &domain.BilingualText{English: "Success", Indonesia: "Sukses"}

// rejectedAccountData builds the virtualAccountData reported with a rejected
// inquiry: the same account block a successful inquiry returns, describing the
// VA that was actually refused so the vendor can show the customer WHICH bill
// it is, plus the outcome pair for the refusal code.
//
// inquiryStatus/inquiryReason come from the code tables in domain rather than
// from literals here, so a rejection built in this layer and one built by
// domain.NewInquiryErrorResponse in the handler can never disagree about what
// the same code means.
//
// Bill details are best-effort: a lookup failure must not turn a clean 404
// into a 500.
func (u *VAUsecase) rejectedAccountData(ctx context.Context, record *domain.VAInquiryRecord, inquiryRequestID, code string) *domain.VAAccountData {
	bills, _ := u.repo.GetVABillDetails(ctx, record.ID)
	data := inquiryAccountDataFromRecord(record, inquiryRequestID, bills)
	data.InquiryStatus = domain.FlagStatusForCode(code)
	data.InquiryReason = domain.ReasonForCode(code)
	return data
}

// inquiryResponseFromRecord builds the successful InquiryResponse purely from
// the persisted transaction and its bill details, so every field the vendor
// receives (name, amount, currency, subCompany, bills) is the stored state of
// the VA rather than a constant. inquiryRequestID is echoed from the request:
// it identifies THIS inquiry, which for a merchant-created VA differs from the
// id the row was originally keyed by.
func inquiryResponseFromRecord(record *domain.VAInquiryRecord, inquiryRequestID string, bills []domain.BillDetail) *domain.VAInquiryResponse {
	data := inquiryAccountDataFromRecord(record, inquiryRequestID, bills)
	data.InquiryStatus = "00"
	data.InquiryReason = inquiryReasonSuccess

	return &domain.VAInquiryResponse{
		ResponseCode:       domain.CodeInquirySuccess,
		ResponseMessage:    "Successful",
		VirtualAccountData: data,
	}
}

// inquiryAccountDataFromRecord renders the persisted VA as the inquiry's
// virtualAccountData block, leaving inquiryStatus/inquiryReason to the caller:
// the account fields are identical whether the inquiry succeeds or is refused,
// and only the outcome pair differs.
func inquiryAccountDataFromRecord(record *domain.VAInquiryRecord, inquiryRequestID string, bills []domain.BillDetail) *domain.VAAccountData {
	currency := record.Currency
	if currency == "" {
		currency = defaultCurrency
	}
	totalAmount := record.TotalAmount
	if totalAmount == "" {
		totalAmount = zeroAmount
	}

	return &domain.VAAccountData{
		PartnerServiceID:   record.PartnerServiceID,
		CustomerNo:         record.CustomerNo,
		VirtualAccountNo:   record.VirtualAccountNo,
		VirtualAccountName: record.CustomerName,
		InquiryRequestID:   inquiryRequestID,
		TotalAmount:        &domain.Amount{Value: totalAmount, Currency: currency},
		SubCompany:         subCompanyForVA(record, bills),
		BillDetails:        capBillDetails(bills),
		FreeTexts:          capFreeTexts(record.FreeTexts),
	}
}

// capBillDetails and capFreeTexts trim the inquiry response to the maxima
// BCA's Notes impose (5 of each). Create-va rejects an over-long list up
// front, so these only matter for VAs stored before that validation existed —
// but the cap has to live here too: BCA fails the whole inquiry on an
// over-long array, and silently showing the customer the first five bills
// beats showing them an error.
func capBillDetails(bills []domain.BillDetail) []domain.BillDetail {
	if len(bills) > domain.MaxInquiryBillDetails {
		return bills[:domain.MaxInquiryBillDetails]
	}
	return bills
}

func capFreeTexts(texts []domain.BilingualText) []domain.BilingualText {
	if len(texts) > domain.MaxInquiryFreeTexts {
		return texts[:domain.MaxInquiryFreeTexts]
	}
	return texts
}

// subCompanyForVA resolves the biller sub-company code to report on inquiry.
// The transaction's own sub_company wins; failing that, the bills' shared
// billSubCompany stands in (ASPI makes billSubCompany mandatory whenever a
// subCompany is in play, so for a merchant-created VA the bill rows are where
// the code actually lives).
//
// When neither supplies one the answer is BCA's default code "00000", not an
// empty string. BCA's InquiryResponse table marks subCompany "Mandatory in
// BCA ... Mandatory for non-multibills transaction", so omitting it — which is
// what an empty string did, via omitempty — made every single-settlement
// inquiry from a merchant that never set additionalInfo.subCompany into a
// response BCA rejects. "00000" is the code BCA itself documents as the
// fallback whose product name and admin fee the channel displays, and it is
// already what the no-bill path (inquiryNoBill) reports.
func subCompanyForVA(record *domain.VAInquiryRecord, bills []domain.BillDetail) string {
	if record.SubCompany != "" {
		return record.SubCompany
	}
	for _, bill := range bills {
		if bill.BillSubCompany != "" {
			return bill.BillSubCompany
		}
	}
	return defaultSubCompany
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
		return nil, domain.NewDomainError(domain.CodeInquiryExpired, errInvalidBill, domain.ErrVAExpiredInquiry)
	}
	if account.Status != domain.VAAccountStatusActive {
		return nil, domain.NewDomainError(domain.CodeInquiryExpired, errInvalidBill, domain.ErrVAAccountInactive)
	}

	// A no-bill VA owes nothing, so totalAmount is always zero (spec A-005).
	//
	// It used to echo the request's own amount, which read as "this VA has a
	// bill of X" to vendors that display totalAmount as the amount due — the
	// exact assertion a no-bill VA must not make. The customer's chosen amount
	// belongs to the payment, not to a bill that does not exist.
	//
	// Only the value is fixed; the currency still follows the request, since
	// reporting a currency the caller never mentioned would be its own claim.
	totalAmount := &domain.Amount{Value: zeroAmount, Currency: defaultCurrency}
	if req.Amount != nil && req.Amount.Currency != "" {
		totalAmount.Currency = req.Amount.Currency
	}

	return &domain.VAInquiryResponse{
		ResponseCode:    domain.CodeInquirySuccess,
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
			SubCompany:         defaultSubCompany,
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
		return nil, domain.NewPaymentError("4042519", errInvalidBill, domain.ErrVAExpiredPayment,
			accountRejectionData(req, account, domain.CodePaymentExpired))
	}
	if account.Status != domain.VAAccountStatusActive {
		return nil, domain.NewPaymentError(domain.CodePaymentNotFound, snapMsgVANotFound, domain.ErrVAAccountInactive,
			accountRejectionData(req, account, domain.CodePaymentNotFound))
	}

	// A no-bill VA has no bill to match against, so the totalAmount/paidAmount
	// equality check the billed path applies is deliberately skipped. The only
	// amount rule left is that the payment must be a positive number.
	paidValue, convErr := strconv.ParseFloat(req.PaidAmount.Value, 64)
	if convErr != nil || paidValue <= 0 {
		return nil, domain.NewDomainError(domain.CodeInvalidField(domain.ServiceCodePayment), "Invalid Field Format [paidAmount]", nil)
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
		// won the race past the GetPayment short-circuit above. Answer exactly
		// as that short-circuit would have — 4042518 against the winner's
		// record — instead of failing or double-recording, and notably without
		// sending a second callback, since we are not the writer.
		if errors.Is(err, domain.ErrVAPaymentDuplicate) {
			// Strict lookup for the same reason as the check at the top of
			// Payment: a no-bill row carries paymentRequestId in both columns,
			// so the OR form could match a sibling payment of the same VA.
			if existing, getErr := u.repo.GetPaymentByPaymentRequestID(ctx, req.PaymentRequestID); getErr == nil && existing != nil {
				// Same advice-vs-double-flag split the short-circuit applies:
				// an advice request is asking "did this land?", anything else
				// is BCA's double flag.
				if strings.EqualFold(req.FlagAdvise, flagAdviseRetry) {
					return paymentResponseFromRecord(existing), nil
				}
				return nil, duplicatePaymentError(existing)
			}
		}
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, err)
	}

	log.Printf("event=va_nobill_payment_recorded virtual_account_no=%s payment_request_id=%s", account.VirtualAccountNo, req.PaymentRequestID)

	u.notifyMerchantForAccount(ctx, req, account, transactionDate)

	return &domain.VAPaymentResponse{
		ResponseCode:    domain.CodePaymentSuccess,
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:   account.PartnerServiceID,
			CustomerNo:         account.CustomerNo,
			VirtualAccountNo:   account.VirtualAccountNo,
			VirtualAccountName: customerName,
			PaymentRequestID:   req.PaymentRequestID,
			PaidAmount:         req.PaidAmount,
			TotalAmount:        &domain.Amount{Value: req.PaidAmount.Value, Currency: req.PaidAmount.Currency},
			TrxDateTime:        &transactionDate,
			ReferenceNo:        req.ReferenceNo,
			PaymentFlagStatus:  "00",
			PaymentFlagReason:  &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			BillDetails:        echoPaymentBillDetails(req.BillDetails),
			FreeTexts:          req.FreeTexts,
		},
	}, nil
}

// paymentResponseFromRecord rebuilds the SNAP payment response from an
// already-persisted payment, used for idempotent replays.
func paymentResponseFromRecord(existing *domain.VAPaymentRecord) *domain.VAPaymentResponse {
	txDate := existing.TransactionDate
	// totalAmount is Mandatory on BCA's PaymentResponse and this builder set it
	// nowhere at all, so every replay — including the 4042518 double-flag
	// answer, the one case BCA reads most carefully — omitted it. Falls back to
	// the paid amount, which for a settled single payment is the same figure.
	totalAmount := existing.TotalAmount
	if totalAmount == "" {
		totalAmount = existing.PaidAmount
	}
	return &domain.VAPaymentResponse{
		ResponseCode:    domain.CodePaymentSuccess,
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:   existing.PartnerServiceID,
			CustomerNo:         existing.CustomerNo,
			VirtualAccountNo:   existing.VirtualAccountNo,
			VirtualAccountName: existing.CustomerName,
			PaymentRequestID:   existing.PaymentRequestID,
			PaidAmount:         &domain.Amount{Value: existing.PaidAmount, Currency: existing.Currency},
			TotalAmount:        &domain.Amount{Value: totalAmount, Currency: existing.Currency},
			TrxDateTime:        &txDate,
			ReferenceNo:        existing.ReferenceNo,
			PaymentFlagStatus:  "00",
			PaymentFlagReason:  &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
		},
	}
}

// vaNotFoundPaymentError rejects a payment naming a VA this system has never
// issued. The rejection echoes the request's own identity fields — there is no
// stored VA to describe, so nothing here is invented; the vendor gets its keys
// back next to paymentFlagStatus "01", exactly as Inquiry does for the same
// unknown VA.
//
// paidAmount and totalAmount are reported as empty rather than echoed from the
// request on purpose: no bill was matched, so no amount was accepted, and
// echoing the tendered figure back would read as an acknowledged payment.
// virtualAccountName is empty for the same reason — naming a holder would mean
// inventing one.
func vaNotFoundPaymentError(req *domain.VAPaymentRequest) error {
	return domain.NewPaymentError(domain.CodePaymentNotFound, snapMsgVANotFound, domain.ErrVAInvalidBill,
		&domain.VAPaymentStatus{
			PartnerServiceID:  req.PartnerServiceID,
			CustomerNo:        req.CustomerNo,
			VirtualAccountNo:  req.VirtualAccountNo,
			PaymentRequestID:  req.PaymentRequestID,
			PaidAmount:        &domain.Amount{},
			TotalAmount:       &domain.Amount{},
			TrxDateTime:       req.TrxDateTime,
			ReferenceNo:       req.ReferenceNo,
			PaymentFlagStatus: domain.FlagStatusForCode(domain.CodePaymentNotFound),
			PaymentFlagReason: domain.ReasonForCode(domain.CodePaymentNotFound),
		})
}

// paymentRejectionData renders the virtualAccountData reported with a refused
// payment against a VA this system DOES know: the stored VA next to the
// outcome pair for the refusal code. The vendor needs to see WHICH bill it is
// being refused, not just that it was. Status and reason come from the domain
// code tables so this layer and the handler can never disagree about what the
// same code means.
//
// paidAmount is reported empty rather than echoed from the request, for the
// same reason as the not-found rejection: nothing was settled, so no amount
// was accepted, and echoing the tendered figure reads as an acknowledgement.
// totalAmount comes from the stored bill, which is real.
func paymentRejectionData(req *domain.VAPaymentRequest, record *domain.VAInquiryRecord, code string) *domain.VAPaymentStatus {
	currency := record.Currency
	if currency == "" {
		currency = "IDR"
	}
	return &domain.VAPaymentStatus{
		PartnerServiceID:   record.PartnerServiceID,
		CustomerNo:         record.CustomerNo,
		VirtualAccountNo:   record.VirtualAccountNo,
		VirtualAccountName: record.CustomerName,
		PaymentRequestID:   req.PaymentRequestID,
		PaidAmount:         &domain.Amount{},
		TotalAmount:        &domain.Amount{Value: record.TotalAmount, Currency: currency},
		TrxDateTime:        req.TrxDateTime,
		ReferenceNo:        req.ReferenceNo,
		PaymentFlagStatus:  domain.FlagStatusForCode(code),
		PaymentFlagReason:  domain.ReasonForCode(code),
	}
}

// accountRejectionData is paymentRejectionData for a VA answered from its
// registration rather than a transaction (no-bill VAs, which have no
// transaction to describe). There is no bill, so totalAmount stays empty.
func accountRejectionData(req *domain.VAPaymentRequest, account *domain.VAAccount, code string) *domain.VAPaymentStatus {
	return &domain.VAPaymentStatus{
		PartnerServiceID:   account.PartnerServiceID,
		CustomerNo:         account.CustomerNo,
		VirtualAccountNo:   account.VirtualAccountNo,
		VirtualAccountName: account.CustomerName,
		PaymentRequestID:   req.PaymentRequestID,
		PaidAmount:         &domain.Amount{},
		TotalAmount:        &domain.Amount{},
		TrxDateTime:        req.TrxDateTime,
		ReferenceNo:        req.ReferenceNo,
		PaymentFlagStatus:  domain.FlagStatusForCode(code),
		PaymentFlagReason:  domain.ReasonForCode(code),
	}
}

// duplicatePaymentError rejects a paymentRequestId that has already been
// recorded — the vendor resubmitted with the same X-EXTERNAL-ID and the same
// paymentRequestId. The rejection carries the colliding payment's
// virtualAccountData so the vendor can identify what it hit.
//
// paymentFlagStatus stays "00" — it describes the *original* payment, which
// really did settle. It is responseCode that rejects this second request.
func duplicatePaymentError(existing *domain.VAPaymentRecord) error {
	return domain.NewPaymentError(
		domain.CodePaymentInconsistent,
		snapMsgInconsistentRequest,
		domain.ErrVAPaymentDuplicate,
		paymentResponseFromRecord(existing).VirtualAccountData,
	)
}

// Payment handles VA payment notification from vendor.
//
// Field-level validation (mandatory fields, lengths, amount format, currency
// agreement) runs in the handler via domain.ValidatePaymentRequest, so what is
// left here is the business decision tree, in BCA's documented precedence.
func (u *VAUsecase) Payment(ctx context.Context, req *domain.VAPaymentRequest) (*domain.VAPaymentResponse, error) {
	// A payment already recorded under this paymentRequestId is either BCA
	// retrying deliberately (flagAdvise "Y" — advice request) or BCA
	// double-flagging after a system error. BCA documents the two outcomes
	// separately, and both are treated by BCA as successful responses:
	//   - advice/retry  → replay the original result as 2002500
	//   - double-flag   → 4042518 "Inconsistent Request", carrying the
	//                     paymentFlagStatus/Reason "according to the results
	//                     of the first request"
	//
	// Resolved by paymentRequestId ONLY. GetPayment's "or inquiryRequestId"
	// form would match the unpaid transaction the inquiry just claimed —
	// BCA sets paymentRequestId equal to inquiryRequestId on a payment that
	// follows an inquiry — and answer the customer's first payment 4042518.
	existing, err := u.repo.GetPaymentByPaymentRequestID(ctx, req.PaymentRequestID)
	if err != nil && !isNotFound(err) {
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, err)
	}
	if existing != nil {
		// flagAdvise "Y" is BCA's advice (retry) request: a deliberate re-send
		// of a payment BCA believes may not have been recorded. It must replay
		// the original outcome as 2002500, not be reported as a collision —
		// BCA is asking "did this land?", and the answer is yes.
		if strings.EqualFold(req.FlagAdvise, flagAdviseRetry) {
			return paymentResponseFromRecord(existing), nil
		}
		// Otherwise the vendor resubmitted this paymentRequestId without
		// advising a retry — either under the same X-EXTERNAL-ID (the
		// idempotency middleware deliberately stops replaying the cached reply
		// for this endpoint so the collision surfaces) or under a fresh one.
		// That is BCA's double-flagging case: answer 4042518 carrying the
		// paymentFlagStatus/Reason "according to the results of the first
		// request", which duplicatePaymentError takes from the stored payment.
		return nil, duplicatePaymentError(existing)
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
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, accErr)
	}
	if account.IsNoBill() {
		return u.paymentNoBill(ctx, req, account)
	}

	// Inherit customer name / trx ID / notificationUrl / inquiry_request_id
	// from the merchant's create-va record when one exists, so the mandatory
	// columns stay populated and the UPSERT below lands on that same row
	// instead of an orphan row keyed by the vendor's own inquiryRequestId.
	// Empty rather than a placeholder when neither source below has a name:
	// virtualAccountName is optional on the ASPI PaymentRequest, and inventing
	// one here would persist a fake account holder onto the transaction and
	// echo it back to the vendor as if it were the real one.
	customerName := ""
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
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, merr)
	}

	// No registration (handled above) and no transaction: the VA does not exist
	// as far as this system is concerned. Rejecting here is what stops the
	// SavePayment below from conjuring a settled row for a VA no merchant ever
	// issued — the same rule Inquiry applies when it answers 4042412 rather
	// than inventing a bill.
	if merchantVA == nil {
		return nil, vaNotFoundPaymentError(req)
	}

	// A payment may only land on a transaction that is still open. Without this
	// guard, a payment with a brand-new paymentRequestId (so it misses the
	// idempotency check above) against an already-paid ("00"), expired ("02"),
	// or deleted ("04") VA would still match this same virtualAccountNo and
	// silently overwrite the completed transaction's
	// paidAmount/referenceNo/transactionDate via SavePayment's upsert — a paid
	// transaction must never be mutated after the fact.
	//
	// A variable-bill instalment already on file is a replay and is answered
	// BEFORE the status guards below: an instalment that settled its bill
	// leaves the transaction at "00", so a retry of that same instalment would
	// otherwise be reported "Paid Bill" — telling BCA the payment failed when
	// it is the very payment that succeeded.
	if replay, err := u.replayVariableInstalment(ctx, req, merchantVA); err != nil || replay != nil {
		return replay, err
	}

	// Expiry detection (feature 007-merchant-expiry-callback, contracts/
	// notify-expired.md): an already-expired VA, or a pending ("03") VA whose
	// expired_date has passed, returns the expired-specific SNAP response
	// instead of the generic conflict response.
	isExpired := merchantVA.Status == "02" ||
		(merchantVA.Status == "03" && merchantVA.ExpiredDate != nil && time.Now().After(*merchantVA.ExpiredDate))
	if isExpired {
		u.markExpiredAndNotify(ctx, merchantVA)
		return nil, domain.NewPaymentError(domain.CodePaymentExpired, errInvalidBill, domain.ErrVAExpiredPayment,
			paymentRejectionData(req, merchantVA, domain.CodePaymentExpired))
	}

	// IsPayable, not Status == "03": a variable-bill VA that still owes part of
	// its totalAmount stays collectable even when the row already reads "00",
	// so the remaining balance is not locked out by a stale status.
	//
	// Each closed state gets its own BCA code via nonPendingPaymentError —
	// collapsing them all into 4092500 Conflict told the channel "duplicate
	// X-EXTERNAL-ID" for a bill that was simply already paid.
	if !merchantVA.IsPayable() {
		return nil, nonPendingPaymentError(req, merchantVA)
	}

	// Variable-bill VAs (billing "variable", feature 006-static-dynamic-va)
	// accept multiple payments against the same VA number until the cumulative
	// total reaches totalAmount ("lunas") — each payment is individually
	// recorded via SaveVAPayment rather than the single-settlement equal-amount
	// path below, and is not subject to the exact totalAmount match check since
	// a partial payment is expected and valid.
	if paymentBilling(account, merchantVA) == domain.VATypeBillingVariable {
		// paymentRequestId is the dedup key. Without it a retried or
		// double-flagged instalment was inserted a second time and credited
		// twice — the cumulative total, and therefore whether the bill counted
		// as settled, moved on money that was only ever paid once.
		paidAmount, _, recorded, err := u.repo.SaveVAPayment(
			ctx, merchantVA.ID, req.PaymentRequestID, req.PaidAmount.Value, req.ReferenceNo)
		if err != nil {
			return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, err)
		}

		transactionDate := time.Now()
		if req.TrxDateTime != nil {
			transactionDate = *req.TrxDateTime
		}

		trxID := merchantVA.TrxID
		if trxID == "" {
			trxID = req.TrxID
		}

		// A partial payment against a variable bill is an ACCEPTED payment
		// flag, so it reports "00". It previously reported "03", which is
		// valid only on the inquiry-status service — BCA's payment spec says
		// "Payment flag status other than 00,01,02 will be considered as 01",
		// so the channel read every accepted instalment as a rejection while
		// the money had in fact been recorded here.
		paymentFlagStatus := domain.PaymentFlagSuccess

		// An instalment already on file is a replay, not a new payment: the
		// merchant must not be notified again, and BCA gets the same
		// advice/double-flag treatment as the single-settlement path.
		responseCode := domain.CodePaymentSuccess
		responseMessage := "Successful"
		if recorded {
			u.notifyMerchantWithVA(ctx, req, merchantVA, trxID, merchantVA.NotificationURL)
		} else if !strings.EqualFold(req.FlagAdvise, flagAdviseRetry) {
			responseCode = domain.CodePaymentInconsistent
			responseMessage = snapMsgInconsistentRequest
		}

		return &domain.VAPaymentResponse{
			ResponseCode:    responseCode,
			ResponseMessage: responseMessage,
			VirtualAccountData: &domain.VAPaymentStatus{
				PartnerServiceID:   req.PartnerServiceID,
				CustomerNo:         req.CustomerNo,
				VirtualAccountNo:   req.VirtualAccountNo,
				VirtualAccountName: req.VirtualAccountName,
				PaymentRequestID:   req.PaymentRequestID,
				PaidAmount:         &domain.Amount{Value: paidAmount, Currency: req.PaidAmount.Currency},
				TotalAmount:        paymentTotalAmount(req),
				TrxDateTime:        &transactionDate,
				ReferenceNo:        req.ReferenceNo,
				PaymentFlagStatus:  paymentFlagStatus,
				PaymentFlagReason:  getPaymentFlagReason(paymentFlagStatus),
				BillDetails:        echoPaymentBillDetails(req.BillDetails),
				FreeTexts:          req.FreeTexts,
			},
		}, nil
	}

	// Amount validation for the single-settlement path.
	//
	// The bill is the source of truth, not the request. Comparing the
	// request's own paidAmount against its own totalAmount — which is what
	// this did before — validates nothing: both come from the same caller, so
	// a payment of 1 for a bill of 250000 passed as long as the caller said
	// totalAmount was 1 too. The stored merchantVA.TotalAmount is what the
	// merchant actually billed.
	//
	// Comparison is numeric: BCA sends "250000" and "250000.00"
	// interchangeably, and a string compare rejects the pair as a mismatch.
	if merchantVA != nil && merchantVA.TotalAmount != "" {
		if !amountsEqual(req.PaidAmount.Value, merchantVA.TotalAmount) {
			return nil, domain.NewDomainError(domain.CodePaymentInvalidAmt, "Invalid amount", nil)
		}
	}
	// A totalAmount that disagrees with the paidAmount in the same request is
	// still a malformed request even when there is no stored bill to check
	// against ("the totalAmount and paidAmount field value contain the total
	// amount paid by customer").
	if req.TotalAmount != nil && !amountsEqual(req.PaidAmount.Value, req.TotalAmount.Value) {
		return nil, domain.NewDomainError(domain.CodePaymentInvalidAmt, "Invalid amount", nil)
	}

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
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, err)
	}

	if len(req.BillDetails) > 0 {
		if err := u.repo.SaveBillDetails(ctx, record.ID, paymentBillDetailsToBillDetail(req.BillDetails)); err != nil {
			return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, err)
		}
	}

	// Notify the merchant asynchronously via their registered notificationUrl.
	// Best-effort: a failure here must not fail the vendor's payment response.
	u.notifyMerchantWithVA(ctx, req, merchantVA, trxID, notificationURL)

	// Build success response, echoing the identity/amount fields per
	// PaymentResponse.virtualAccountData.
	return &domain.VAPaymentResponse{
		ResponseCode:    domain.CodePaymentSuccess,
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:   req.PartnerServiceID,
			CustomerNo:         req.CustomerNo,
			VirtualAccountNo:   req.VirtualAccountNo,
			VirtualAccountName: customerName,
			PaymentRequestID:   req.PaymentRequestID,
			PaidAmount:         req.PaidAmount,
			// totalAmount and trxDateTime are Mandatory on BCA's
			// PaymentResponse; echoing the request's nil pointer straight
			// through dropped them from the wire entirely via omitempty.
			TotalAmount:       paymentTotalAmount(req),
			TrxDateTime:       &transactionDate,
			ReferenceNo:       req.ReferenceNo,
			PaymentFlagStatus: "00",
			PaymentFlagReason: &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
			BillDetails:       echoPaymentBillDetails(req.BillDetails),
			FreeTexts:         req.FreeTexts,
		},
	}, nil
}

// replayVariableInstalment answers a repeat of a variable-bill instalment that
// is already recorded, without touching the cumulative total. It returns
// (nil, nil) when this paymentRequestId is not a known instalment, so the
// caller falls through to normal processing.
//
// flagAdvise "Y" is BCA deliberately retrying and wants the original outcome
// (2002500); anything else is the double-flag case (4042518). Either way BCA
// counts the response as a successful transaction, and the merchant is not
// notified a second time.
func (u *VAUsecase) replayVariableInstalment(
	ctx context.Context,
	req *domain.VAPaymentRequest,
	merchantVA *domain.VAInquiryRecord,
) (*domain.VAPaymentResponse, error) {
	transactionID, cumulativePaid, found, err := u.repo.FindVAInstalment(ctx, req.PaymentRequestID)
	if err != nil {
		return nil, domain.NewDomainError(domain.CodeInternalError(domain.ServiceCodePayment), errInternalServerError, err)
	}
	if !found || transactionID != merchantVA.ID {
		return nil, nil
	}

	responseCode := domain.CodePaymentSuccess
	responseMessage := "Successful"
	if !strings.EqualFold(req.FlagAdvise, flagAdviseRetry) {
		responseCode = domain.CodePaymentInconsistent
		responseMessage = snapMsgInconsistentRequest
	}

	transactionDate := time.Now()
	if req.TrxDateTime != nil {
		transactionDate = *req.TrxDateTime
	}
	trxID := merchantVA.TrxID
	if trxID == "" {
		trxID = req.TrxID
	}

	return &domain.VAPaymentResponse{
		ResponseCode:    responseCode,
		ResponseMessage: responseMessage,
		VirtualAccountData: &domain.VAPaymentStatus{
			PartnerServiceID:   req.PartnerServiceID,
			CustomerNo:         req.CustomerNo,
			VirtualAccountNo:   req.VirtualAccountNo,
			VirtualAccountName: req.VirtualAccountName,
			PaymentRequestID:   req.PaymentRequestID,
			PaidAmount:         &domain.Amount{Value: cumulativePaid, Currency: req.PaidAmount.Currency},
			TotalAmount:        paymentTotalAmount(req),
			TrxDateTime:        &transactionDate,
			ReferenceNo:        req.ReferenceNo,
			// The instalment was accepted when it was first recorded, so the
			// flag it replays is that acceptance.
			PaymentFlagStatus: domain.PaymentFlagSuccess,
			PaymentFlagReason: domain.ReasonForCode(domain.CodePaymentSuccess),
			BillDetails:       echoPaymentBillDetails(req.BillDetails),
			FreeTexts:         req.FreeTexts,
		},
	}, nil
}

// amountsEqual compares two money strings numerically, so "250000" and
// "250000.00" — which BCA uses interchangeably — are the same amount. A value
// that will not parse falls back to an exact string compare rather than
// silently comparing as zero, which would make any two unparseable amounts
// "equal".
func amountsEqual(a, b string) bool {
	left, errA := strconv.ParseFloat(strings.TrimSpace(a), 64)
	right, errB := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return math.Abs(left-right) < amountEpsilon
}

// nonPendingPaymentError maps a non-pending transaction status to the BCA code
// that names why the bill is not payable. Each state has its own code in
// Appendix A; reporting them all as 4092500 Conflict told the channel the
// X-EXTERNAL-ID was duplicated when in fact the bill was already settled.
// Every branch carries the refused VA as virtualAccountData so the channel can
// show WHICH bill it is being refused, not merely that it was.
//
// There is deliberately no "03" case: the caller reaches this only when
// IsPayable() said no, and IsPayable accepts every "03". Returning a typed nil
// *DomainError here would also be a trap — assigned to an `error` return it
// reads as non-nil, turning "payable" into an empty 500.
func nonPendingPaymentError(req *domain.VAPaymentRequest, record *domain.VAInquiryRecord) error {
	switch record.Status {
	case "00":
		return domain.NewPaymentError(domain.CodePaymentPaidBill, "Paid Bill", domain.ErrVAPaidBill,
			paymentRejectionData(req, record, domain.CodePaymentPaidBill))
	case "02":
		return domain.NewPaymentError(domain.CodePaymentExpired, errInvalidBill, domain.ErrVAExpiredPayment,
			paymentRejectionData(req, record, domain.CodePaymentExpired))
	case "04":
		return domain.NewPaymentError(domain.CodePaymentNotFound, snapMsgVANotFound, domain.ErrVAInvalidBill,
			paymentRejectionData(req, record, domain.CodePaymentNotFound))
	default:
		return domain.NewPaymentError(domain.CodePaymentConflict, "Conflict", nil,
			paymentRejectionData(req, record, domain.CodePaymentConflict))
	}
}

// paymentBilling resolves how this VA's amount is determined. The va_accounts
// registration wins when there is one: its Billing was resolved from master
// data at create-va time, so the VA keeps the contract it was issued under and
// operator-added VA types need no code change. The vaType mapping is the
// fallback for transactions that predate the registry.
func paymentBilling(account *domain.VAAccount, merchantVA *domain.VAInquiryRecord) domain.VATypeBilling {
	if account != nil && account.Billing != "" {
		return account.Billing
	}
	if merchantVA != nil {
		return domain.BillingForVAType(merchantVA.VAType)
	}
	return ""
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

// paymentTotalAmount is the response-side counterpart: totalAmount is
// Mandatory on BCA's PaymentResponse, so it must never be nil (which
// omitempty would render as an absent field). It is only ever nil in the
// request when the vendor is configured non-strict, and BCA's own note —
// "the totalAmount and paidAmount field value contain the total amount paid by
// customer through BCA" — says what the fallback is.
func paymentTotalAmount(req *domain.VAPaymentRequest) *domain.Amount {
	if req.TotalAmount != nil {
		return req.TotalAmount
	}
	return &domain.Amount{Value: req.PaidAmount.Value, Currency: req.PaidAmount.Currency}
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
func echoPaymentBillDetails(bills []domain.VAPaymentBillDetail) []domain.VAPaymentResponseBillDetail {
	if len(bills) == 0 {
		return nil
	}
	out := make([]domain.VAPaymentResponseBillDetail, 0, len(bills))
	for _, b := range bills {
		// BCA's PaymentResponse table is explicit about what this field must
		// carry: "billerReferenceId ... From Payment Request. This field value
		// must be the same with billReferenceNo from payment request." The
		// REQUEST spells the field billReferenceNo (auth code generated by
		// BCA); billerReferenceId is response-side only, so the inbound
		// billerReferenceId is always empty and this fallback always fires.
		// It previously fell back to billNo — the partner's own bill number,
		// which is a different identifier entirely — so every multi-settlement
		// payment response echoed a reference BCA could not reconcile against
		// the auth code it issued.
		billerReferenceID := b.BillerReferenceID
		if billerReferenceID == "" {
			billerReferenceID = b.BillReferenceNo
		}
		status := b.Status
		if status == "" {
			status = "00"
		}
		reason := b.Reason
		if reason == nil {
			reason = &domain.BilingualText{English: "Success", Indonesia: "Sukses"}
		}
		// Only the fields BCA's Response table defines are carried across;
		// billCode/billName/billShortName/billReferenceNo stay on the request
		// side, which is why this maps between the two types rather than
		// mutating the inbound one.
		out = append(out, domain.VAPaymentResponseBillDetail{
			BillerReferenceID: billerReferenceID,
			BillNo:            b.BillNo,
			BillDescription:   b.BillDescription,
			BillSubCompany:    b.BillSubCompany,
			BillAmount:        b.BillAmount,
			AdditionalInfo:    b.AdditionalInfo,
			Status:            status,
			Reason:            reason,
		})
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
	// Resolve by paymentRequestId first when the vendor supplied one, then by
	// inquiryRequestId.
	//
	// Both identify the transaction, and for a no-bill VA only the first one
	// can: a no-bill inquiry persists nothing by design (the VA is a durable
	// address, not a transaction), so each payment is reachable solely by its
	// own paymentRequestId. Resolving by inquiryRequestId alone reported every
	// no-bill payment as Transaction Not Found.
	lookupID := req.PaymentRequestID
	if lookupID == "" {
		lookupID = req.InquiryRequestID
	}

	// Only a genuine "no such row" may fall through to the pending/inquiry
	// branch below — a failing query must surface as a 500, otherwise a paid
	// VA is reported back to the vendor as still pending.
	payment, err := u.repo.GetPayment(ctx, lookupID)
	if err != nil && isNotFound(err) && lookupID != req.InquiryRequestID {
		// A paymentRequestId that resolves to nothing is not itself an
		// answer: fall back to the mandatory inquiryRequestId before giving up.
		payment, err = u.repo.GetPayment(ctx, req.InquiryRequestID)
	}
	if err != nil && !isNotFound(err) {
		return nil, domain.NewDomainError(domain.CodeStatusInternalErr, errInternalServerError, err)
	}
	if err != nil {
		// If no payment found, check inquiry
		inquiry, inquiryErr := u.repo.GetInquiry(ctx, req.InquiryRequestID)
		if inquiryErr != nil && !isNotFound(inquiryErr) {
			return nil, domain.NewDomainError(domain.CodeStatusInternalErr, errInternalServerError, inquiryErr)
		}
		if inquiryErr != nil {
			// BCA's status service names this case explicitly: 4042601
			// "Transaction Not Found". 4042619 is not a code it publishes.
			return nil, domain.NewDomainError(domain.CodeStatusNotFound, "Transaction Not Found", nil)
		}

		// Best-effort: bill details persisted at create-VA/inquiry time, if any.
		bills, _ := u.repo.GetVABillDetails(ctx, inquiry.ID)

		// Return inquiry status (pending).
		//
		// Every field BCA marks Mandatory is populated even though no payment
		// has happened yet. Leaving paidAmount/transactionDate as nil pointers
		// rendered them as JSON null and paymentRequestId as "", and BCA reads
		// a mandatory field it cannot parse as a Response Parsing Error
		// (4002600) rather than as "pending":
		//   - paidAmount  → zero in the bill's own currency; nothing is paid yet.
		//   - paymentRequestId → the inquiryRequestId, which is exactly what the
		//     spec says this field carries ("This value must be same with
		//     inquiryRequestId").
		//   - transactionDate → the transaction's creation time. There is no
		//     payment datetime to report, and a real ISO-8601 timestamp
		//     identifying the transaction is what BCA can parse; null is not.
		currency := inquiry.Currency
		if currency == "" {
			currency = defaultCurrency
		}
		totalAmount := inquiry.TotalAmount
		if totalAmount == "" {
			totalAmount = zeroAmount
		}
		createdAt := inquiry.CreatedAt

		return &domain.VAStatusResponse{
			ResponseCode:    domain.CodeStatusSuccess,
			ResponseMessage: statusSuccessMessage,
			VirtualAccountData: &domain.VAStatusData{
				PaymentFlagStatus: domain.PaymentFlagPending,
				PaymentFlagReason: &domain.BilingualText{English: "Pending", Indonesia: "Tertunda"},
				PartnerServiceID:  inquiry.PartnerServiceID,
				CustomerNo:        inquiry.CustomerNo,
				VirtualAccountNo:  inquiry.VirtualAccountNo,
				InquiryRequestID:  inquiry.InquiryRequestID,
				PaymentRequestID:  inquiry.InquiryRequestID,
				PaidAmount:        &domain.Amount{Value: zeroAmount, Currency: currency},
				TotalAmount:       &domain.Amount{Value: totalAmount, Currency: currency},
				TransactionDate:   &createdAt,
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
		ResponseCode:    domain.CodeStatusSuccess,
		ResponseMessage: statusSuccessMessage,
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
