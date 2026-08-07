package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"backbone-new/internal/domain"
)

// ReconcileConfig tunes the sweep. Both knobs exist because the right values
// are deployment facts, not code facts: how long a vendor realistically takes
// to deliver a payment flag, and how much outbound traffic operations is
// willing to aim at that vendor per cycle.
type ReconcileConfig struct {
	// PendingAfter is how long a transaction must have been pending before it
	// is worth asking about. Too short and the sweep interrogates the vendor
	// about VAs nobody has tried to pay yet; too long and money sits
	// unrecorded.
	PendingAfter time.Duration
	// BatchSize caps how many transactions one sweep reconciles, so a backlog
	// drains steadily instead of firing thousands of calls at the vendor in
	// one burst.
	BatchSize int
	// ClientID identifies the vendor being reconciled against, recorded on
	// every audit row.
	ClientID string
}

// DefaultReconcileConfig is used for any field left zero.
var DefaultReconcileConfig = ReconcileConfig{
	PendingAfter: 15 * time.Minute,
	BatchSize:    100,
}

// ReconciliationUsecase implements domain.VAStatusReconciler.
//
// It closes the one gap the inbound endpoints cannot: a payment flag that
// never arrived. Everything else already has a recovery path — a failed
// merchant callback has resend-callback, a lost /payment response has BCA's
// own flagAdvise retry — but if the /payment call never lands at all, this
// service holds no evidence that anything happened. Only the vendor knows, and
// only service code 26 can ask it.
type ReconciliationUsecase struct {
	repo     domain.ReconciliationRepository
	gateway  domain.VAGateway
	notifier domain.NotificationEnqueuer
	config   ReconcileConfig
	now      func() time.Time
}

// NewReconciliationUsecase creates the reconciler. gateway is the outbound
// vendor client; notifier may be nil, in which case a recovered payment is
// still recorded but no merchant callback is sent.
func NewReconciliationUsecase(
	repo domain.ReconciliationRepository,
	gateway domain.VAGateway,
	notifier domain.NotificationEnqueuer,
	cfg ReconcileConfig,
) *ReconciliationUsecase {
	if cfg.PendingAfter <= 0 {
		cfg.PendingAfter = DefaultReconcileConfig.PendingAfter
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultReconcileConfig.BatchSize
	}
	return &ReconciliationUsecase{
		repo:     repo,
		gateway:  gateway,
		notifier: notifier,
		config:   cfg,
		now:      time.Now,
	}
}

// Sweep reconciles every transaction that has been pending past the threshold.
//
// One transaction's failure never aborts the run: the whole point is to
// recover money nobody knows is missing, and stopping at the first unreachable
// record would leave the rest unrecovered for another cycle.
func (u *ReconciliationUsecase) Sweep(ctx context.Context) ([]*domain.ReconcileResult, error) {
	cutoff := u.now().Add(-u.config.PendingAfter)
	stale, err := u.repo.ListStalePendingTransactions(ctx, cutoff, u.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("select stale pending transactions: %w", err)
	}
	if len(stale) == 0 {
		return nil, nil
	}

	log.Printf("event=va_reconcile_sweep_start candidates=%d pending_after=%s", len(stale), u.config.PendingAfter)

	results := make([]*domain.ReconcileResult, 0, len(stale))
	settled := 0
	for _, record := range stale {
		result, err := u.reconcileRecord(ctx, record, domain.ReconcileTriggerSweep)
		if err != nil {
			log.Printf("event=va_reconcile_error virtual_account_no=%s err=%v", record.VirtualAccountNo, err)
			continue
		}
		if result.Settled {
			settled++
		}
		results = append(results, result)
	}

	log.Printf("event=va_reconcile_sweep_done examined=%d settled=%d", len(results), settled)
	return results, nil
}

// Reconcile inquires the vendor about one VA number on demand.
func (u *ReconciliationUsecase) Reconcile(ctx context.Context, virtualAccountNo, trigger string) (*domain.ReconcileResult, error) {
	record, err := u.repo.GetVAByVirtualAccountNo(ctx, virtualAccountNo)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, domain.ErrMerchantVANotFound
	}
	return u.reconcileRecord(ctx, record, trigger)
}

// reconcileRecord is the whole decision for one transaction. Every path
// records an audit row before returning — including the paths that make no
// vendor call — because a reconciler that has silently stopped working is
// indistinguishable from one with nothing to do unless the attempts are
// written down.
func (u *ReconciliationUsecase) reconcileRecord(
	ctx context.Context,
	record *domain.VAInquiryRecord,
	trigger string,
) (*domain.ReconcileResult, error) {
	started := u.now()
	attempt := &domain.VAStatusInquiryAttempt{
		VirtualAccountNo: record.VirtualAccountNo,
		ClientID:         u.config.ClientID,
		AttemptedAt:      started,
	}
	finish := func(outcome string, settled bool, resp *domain.VAStatusResponse, errDetail string) *domain.ReconcileResult {
		attempt.Outcome = outcome
		attempt.ErrorDetail = errDetail
		attempt.DurationMs = int(u.now().Sub(started).Milliseconds())
		result := &domain.ReconcileResult{
			VirtualAccountNo: record.VirtualAccountNo,
			Outcome:          outcome,
			Settled:          settled,
			ReconciledAt:     u.now(),
		}
		if resp != nil {
			attempt.VendorResponseCode = resp.ResponseCode
			result.VendorResponseCode = resp.ResponseCode
			if resp.VirtualAccountData != nil {
				attempt.VendorPaymentFlagStatus = resp.VirtualAccountData.PaymentFlagStatus
				attempt.PaymentRequestID = resp.VirtualAccountData.PaymentRequestID
				result.VendorPaymentFlagStatus = resp.VirtualAccountData.PaymentFlagStatus
				result.PaidAmount = resp.VirtualAccountData.PaidAmount
			}
		}
		// Audit failure must not swallow the reconciliation itself — the money
		// decision has already been made and acted on by this point.
		if err := u.repo.CreateStatusInquiryAttempt(ctx, attempt); err != nil {
			log.Printf("event=va_reconcile_audit_failed virtual_account_no=%s err=%v", record.VirtualAccountNo, err)
		}
		return result
	}

	// Re-check under fresh data: a sweep selects a batch and then works
	// through it, so a transaction can be paid for real between selection and
	// its turn.
	if record.Status != "03" {
		return finish(domain.ReconcileOutcomeAlreadySettled, false, nil, ""), nil
	}

	// The status service takes the inquiryRequestId the VENDOR generated. A
	// transaction still carrying a create-va placeholder has never been
	// inquired, so the vendor has no such id and no payment against it could
	// exist — asking would earn a 4002602 and tell us nothing.
	if domain.IsPlaceholderInquiryRequestID(record.InquiryRequestID, record.TrxID, record.VirtualAccountNo) {
		return finish(domain.ReconcileOutcomeNotPaid, false, nil, "never inquired by the vendor; no vendor-side transaction to reconcile"), nil
	}

	resp, err := u.gateway.PaymentStatus(ctx, &domain.VAStatusRequest{
		PartnerServiceID: record.PartnerServiceID,
		CustomerNo:       record.CustomerNo,
		VirtualAccountNo: record.VirtualAccountNo,
		InquiryRequestID: record.InquiryRequestID,
	})
	if err != nil {
		return finish(domain.ReconcileOutcomeError, false, nil, err.Error()), nil
	}
	if resp == nil || resp.VirtualAccountData == nil {
		return finish(domain.ReconcileOutcomeError, false, resp, "vendor returned no virtualAccountData"), nil
	}

	outcome := classifyVendorStatus(resp)
	if outcome != domain.ReconcileOutcomeSettled {
		return finish(outcome, false, resp, ""), nil
	}

	settled, err := u.settle(ctx, record, resp)
	if err != nil {
		return finish(domain.ReconcileOutcomeError, false, resp, err.Error()), nil
	}
	if !settled {
		// Lost the race to a real /payment or a concurrent reconcile. The
		// money is recorded — just not by us — so no second callback.
		return finish(domain.ReconcileOutcomeAlreadySettled, false, resp, ""), nil
	}

	log.Printf("event=va_reconcile_recovered_payment virtual_account_no=%s payment_request_id=%s trigger=%s",
		record.VirtualAccountNo, resp.VirtualAccountData.PaymentRequestID, trigger)

	return finish(domain.ReconcileOutcomeSettled, true, resp, ""), nil
}

// classifyVendorStatus turns the vendor's answer into a decision.
//
// BCA's own note governs which response codes carry a readable flag at all:
// "BCA will continue to check the paymentFlagStatus ... if the responseCode
// received is 2002500, 2022500, or 4042518" for payment; the status service's
// Appendix A pairs a readable body with 2002600. A 4042601 "Transaction Not
// Found" is a genuine answer — the vendor has no record — not a failure.
func classifyVendorStatus(resp *domain.VAStatusResponse) string {
	if resp.ResponseCode == domain.CodeStatusNotFound {
		return domain.ReconcileOutcomeNotPaid
	}
	if resp.ResponseCode != domain.CodeStatusSuccess {
		// Any other code is the vendor declining to answer (bad request,
		// unauthorized, internal error). Not evidence about the payment.
		return domain.ReconcileOutcomeError
	}

	switch resp.VirtualAccountData.PaymentFlagStatus {
	case domain.PaymentFlagSuccess:
		return domain.ReconcileOutcomeSettled
	case domain.PaymentFlagPending:
		return domain.ReconcileOutcomePending
	case domain.PaymentFlagTimeout:
		// "02 = Timeout payment flag between switcher to partner. If company's
		// reconciliation type is reversal or force settle, transaction with
		// status 02 will be considered as success transaction." Whether it
		// settles is a property registered at BCA, not visible from here.
		// Guessing would either credit a merchant for reversed money or strand
		// a real payment, so an operator decides.
		return domain.ReconcileOutcomeAmbiguous
	default:
		// "01" reject, or anything unrecognised.
		return domain.ReconcileOutcomeNotPaid
	}
}

// settle records the vendor-confirmed payment and notifies the merchant.
// Returns false when another writer settled the transaction first.
func (u *ReconciliationUsecase) settle(
	ctx context.Context,
	record *domain.VAInquiryRecord,
	resp *domain.VAStatusResponse,
) (bool, error) {
	data := resp.VirtualAccountData

	paid := data.PaidAmount
	if paid == nil || paid.Value == "" {
		// A "00" flag with no amount is not something to write into a ledger.
		return false, errors.New("vendor reported a successful payment flag with no paidAmount")
	}
	currency := paid.Currency
	if currency == "" {
		currency = defaultCurrency
	}

	transactionDate := u.now()
	if data.TransactionDate != nil {
		transactionDate = *data.TransactionDate
	} else if data.TrxDateTime != nil {
		transactionDate = *data.TrxDateTime
	}

	settled, err := u.repo.SettlePendingFromVendor(ctx, record.VirtualAccountNo, domain.VendorSettlement{
		PaidAmount:       paid.Value,
		Currency:         currency,
		PaymentRequestID: data.PaymentRequestID,
		ReferenceNo:      data.ReferenceNo,
		TransactionDate:  transactionDate,
	})
	if err != nil || !settled {
		return false, err
	}

	u.notifyRecovered(ctx, record, data, currency, transactionDate)
	return true, nil
}

// notifyRecovered sends the merchant the payment callback that the missing
// /payment call never triggered. Best-effort, like every other callback in
// this service: the payment is already recorded, and merchant reachability
// must not undo that.
func (u *ReconciliationUsecase) notifyRecovered(
	ctx context.Context,
	record *domain.VAInquiryRecord,
	data *domain.VAStatusData,
	currency string,
	transactionDate time.Time,
) {
	if u.notifier == nil || record.NotificationURL == "" {
		return
	}

	paymentRequestID := data.PaymentRequestID
	if paymentRequestID == "" {
		paymentRequestID = record.InquiryRequestID
	}

	payload := &domain.PaymentNotificationPayload{
		EventType:        domain.NotificationEventPaymentReceived,
		PartnerServiceID: record.PartnerServiceID,
		CustomerNo:       record.CustomerNo,
		VirtualAccountNo: record.VirtualAccountNo,
		TrxID:            record.TrxID,
		PaymentRequestID: paymentRequestID,
		PaidAmount:       &domain.Amount{Value: data.PaidAmount.Value, Currency: currency},
		TotalAmount:      &domain.Amount{Value: data.PaidAmount.Value, Currency: currency},
		TrxDateTime:      transactionDate.Format(time.RFC3339),
		ReferenceNo:      data.ReferenceNo,
		NotificationURL:  record.NotificationURL,
	}

	if err := u.notifier.EnqueuePaymentNotification(ctx, payload); err != nil {
		log.Printf("event=va_reconcile_notify_failed virtual_account_no=%s err=%v", record.VirtualAccountNo, err)
	}
}

var _ domain.VAStatusReconciler = (*ReconciliationUsecase)(nil)
