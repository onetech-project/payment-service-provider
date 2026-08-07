package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---------------------------------------------------------------

type fakeReconcileRepo struct {
	record *domain.VAInquiryRecord
	stale  []*domain.VAInquiryRecord

	settleReturns bool
	settleErr     error
	settled       []domain.VendorSettlement
	attempts      []*domain.VAStatusInquiryAttempt
	getErr        error
	listErr       error
	auditErr      error
}

func (f *fakeReconcileRepo) ListStalePendingTransactions(_ context.Context, _ time.Time, _ int) ([]*domain.VAInquiryRecord, error) {
	return f.stale, f.listErr
}

func (f *fakeReconcileRepo) CreateStatusInquiryAttempt(_ context.Context, a *domain.VAStatusInquiryAttempt) error {
	f.attempts = append(f.attempts, a)
	return f.auditErr
}

func (f *fakeReconcileRepo) GetVAByVirtualAccountNo(_ context.Context, _ string) (*domain.VAInquiryRecord, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.record, nil
}

func (f *fakeReconcileRepo) SettlePendingFromVendor(_ context.Context, _ string, s domain.VendorSettlement) (bool, error) {
	f.settled = append(f.settled, s)
	return f.settleReturns, f.settleErr
}

type fakeGateway struct {
	resp  *domain.VAStatusResponse
	err   error
	calls int
}

func (f *fakeGateway) Inquiry(_ context.Context, _ *domain.VAInquiryRequest) (*domain.VAInquiryResponse, error) {
	return nil, errors.New("not used")
}

func (f *fakeGateway) PaymentStatus(_ context.Context, _ *domain.VAStatusRequest) (*domain.VAStatusResponse, error) {
	f.calls++
	return f.resp, f.err
}

type fakeNotifier struct {
	payloads []*domain.PaymentNotificationPayload
	err      error
}

func (f *fakeNotifier) EnqueuePaymentNotification(_ context.Context, p *domain.PaymentNotificationPayload) error {
	f.payloads = append(f.payloads, p)
	return f.err
}

// pendingRecord is a transaction this service still believes is unpaid, with a
// real vendor-issued inquiryRequestId (i.e. it HAS been inquired, so the
// vendor has something to report).
func pendingRecord() *domain.VAInquiryRecord {
	return &domain.VAInquiryRecord{
		ID:               "txn-1",
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202202111031031234500001",
		TrxID:            "trx-1",
		NotificationURL:  "https://merchant.example/callback",
		Status:           "03",
		TotalAmount:      "150000.00",
		Currency:         "IDR",
		CreatedAt:        time.Now().Add(-time.Hour),
	}
}

func statusResponse(code, flag, paidValue string) *domain.VAStatusResponse {
	data := &domain.VAStatusData{
		PaymentFlagStatus: flag,
		PaymentRequestID:  "202202111031031234500001",
		ReferenceNo:       "12345678901",
	}
	if paidValue != "" {
		data.PaidAmount = &domain.Amount{Value: paidValue, Currency: "IDR"}
	}
	return &domain.VAStatusResponse{ResponseCode: code, VirtualAccountData: data}
}

func newReconciler(repo *fakeReconcileRepo, gw *fakeGateway, n *fakeNotifier) *ReconciliationUsecase {
	return NewReconciliationUsecase(repo, gw, n, ReconcileConfig{ClientID: "vendor-client"})
}

// --- the case the feature exists for ------------------------------------

// The vendor reports the payment succeeded while this service still had the
// transaction pending: money was taken and never recorded. Reconciliation must
// record it AND tell the merchant — the callback that the missing /payment
// call never fired.
func TestReconcile_VendorSaysPaid_SettlesAndNotifies(t *testing.T) {
	repo := &fakeReconcileRepo{record: pendingRecord(), settleReturns: true}
	gw := &fakeGateway{resp: statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagSuccess, "150000.00")}
	notifier := &fakeNotifier{}

	result, err := newReconciler(repo, gw, notifier).
		Reconcile(context.Background(), " 12345123456789012345678", domain.ReconcileTriggerAdmin)

	require.NoError(t, err)
	assert.Equal(t, domain.ReconcileOutcomeSettled, result.Outcome)
	assert.True(t, result.Settled)

	require.Len(t, repo.settled, 1)
	assert.Equal(t, "150000.00", repo.settled[0].PaidAmount)
	assert.Equal(t, "IDR", repo.settled[0].Currency)
	assert.Equal(t, "12345678901", repo.settled[0].ReferenceNo)

	require.Len(t, notifier.payloads, 1, "the merchant must be told about a recovered payment")
	assert.Equal(t, domain.NotificationEventPaymentReceived, notifier.payloads[0].EventType)
	assert.Equal(t, "150000.00", notifier.payloads[0].PaidAmount.Value)

	require.Len(t, repo.attempts, 1)
	assert.Equal(t, domain.ReconcileOutcomeSettled, repo.attempts[0].Outcome)
	assert.Equal(t, "00", repo.attempts[0].VendorPaymentFlagStatus)
	assert.Equal(t, "vendor-client", repo.attempts[0].ClientID)
}

// Losing the race to a real /payment must not produce a second callback: the
// money is recorded once, and the merchant hears about it once.
func TestReconcile_LosesRaceToRealPayment_DoesNotNotifyTwice(t *testing.T) {
	repo := &fakeReconcileRepo{record: pendingRecord(), settleReturns: false}
	gw := &fakeGateway{resp: statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagSuccess, "150000.00")}
	notifier := &fakeNotifier{}

	result, err := newReconciler(repo, gw, notifier).
		Reconcile(context.Background(), " 12345123456789012345678", domain.ReconcileTriggerSweep)

	require.NoError(t, err)
	assert.Equal(t, domain.ReconcileOutcomeAlreadySettled, result.Outcome)
	assert.False(t, result.Settled)
	assert.Empty(t, notifier.payloads)
}

// --- the outcomes that must NOT settle ----------------------------------

func TestReconcile_NonSettlingVendorAnswers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		resp        *domain.VAStatusResponse
		wantOutcome string
	}{
		{
			// "03 = Pending between BCA and the partner" — still in flight.
			name:        "pending flag",
			resp:        statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagPending, ""),
			wantOutcome: domain.ReconcileOutcomePending,
		},
		{
			name:        "rejected flag",
			resp:        statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagReject, ""),
			wantOutcome: domain.ReconcileOutcomeNotPaid,
		},
		{
			// "02 = Timeout ... If company's reconciliation type is reversal or
			// force settle, transaction with status 02 will be considered as
			// success" — that property lives at BCA, so settling here would be
			// a guess about money.
			name:        "timeout flag is ambiguous, never auto-settled",
			resp:        statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagTimeout, "150000.00"),
			wantOutcome: domain.ReconcileOutcomeAmbiguous,
		},
		{
			// A real answer: the vendor has no such transaction.
			name:        "transaction not found",
			resp:        &domain.VAStatusResponse{ResponseCode: domain.CodeStatusNotFound, VirtualAccountData: &domain.VAStatusData{}},
			wantOutcome: domain.ReconcileOutcomeNotPaid,
		},
		{
			// Not evidence about the payment — the vendor declined to answer.
			name:        "unauthorized is an error, not a verdict",
			resp:        &domain.VAStatusResponse{ResponseCode: "4012600", VirtualAccountData: &domain.VAStatusData{}},
			wantOutcome: domain.ReconcileOutcomeError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeReconcileRepo{record: pendingRecord(), settleReturns: true}
			notifier := &fakeNotifier{}

			result, err := newReconciler(repo, &fakeGateway{resp: tc.resp}, notifier).
				Reconcile(context.Background(), " 12345123456789012345678", domain.ReconcileTriggerSweep)

			require.NoError(t, err)
			assert.Equal(t, tc.wantOutcome, result.Outcome)
			assert.False(t, result.Settled)
			assert.Empty(t, repo.settled, "nothing may be written to the ledger")
			assert.Empty(t, notifier.payloads)
			require.Len(t, repo.attempts, 1, "every attempt is audited, including the ones that changed nothing")
		})
	}
}

// A "00" flag with no amount is not something to write into a ledger.
func TestReconcile_PaidFlagWithoutAmount_IsAnErrorNotASettlement(t *testing.T) {
	repo := &fakeReconcileRepo{record: pendingRecord(), settleReturns: true}
	notifier := &fakeNotifier{}
	gw := &fakeGateway{resp: statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagSuccess, "")}

	result, err := newReconciler(repo, gw, notifier).
		Reconcile(context.Background(), " 12345123456789012345678", domain.ReconcileTriggerSweep)

	require.NoError(t, err)
	assert.Equal(t, domain.ReconcileOutcomeError, result.Outcome)
	assert.Empty(t, repo.settled)
	assert.Empty(t, notifier.payloads)
}

// --- calls that must never be made --------------------------------------

// A transaction still carrying a create-va placeholder was never inquired by
// the vendor, so the vendor has no transaction to report on. Asking would earn
// a mandatory-field rejection and tell us nothing.
func TestReconcile_NeverInquired_DoesNotCallTheVendor(t *testing.T) {
	record := pendingRecord()
	record.InquiryRequestID = record.VirtualAccountNo // the create-va placeholder
	repo := &fakeReconcileRepo{record: record}
	gw := &fakeGateway{}

	result, err := newReconciler(repo, gw, &fakeNotifier{}).
		Reconcile(context.Background(), record.VirtualAccountNo, domain.ReconcileTriggerSweep)

	require.NoError(t, err)
	assert.Equal(t, domain.ReconcileOutcomeNotPaid, result.Outcome)
	assert.Zero(t, gw.calls, "no outbound call for a VA the vendor has never seen")
	require.Len(t, repo.attempts, 1)
	assert.Contains(t, repo.attempts[0].ErrorDetail, "never inquired")
}

func TestReconcile_AlreadySettledTransaction_DoesNotCallTheVendor(t *testing.T) {
	record := pendingRecord()
	record.Status = "00"
	repo := &fakeReconcileRepo{record: record}
	gw := &fakeGateway{}

	result, err := newReconciler(repo, gw, &fakeNotifier{}).
		Reconcile(context.Background(), record.VirtualAccountNo, domain.ReconcileTriggerSweep)

	require.NoError(t, err)
	assert.Equal(t, domain.ReconcileOutcomeAlreadySettled, result.Outcome)
	assert.Zero(t, gw.calls)
}

// A transport failure is recorded, not swallowed: a reconciler that silently
// stops reaching the vendor looks exactly like one with nothing to reconcile.
func TestReconcile_VendorUnreachable_IsAudited(t *testing.T) {
	repo := &fakeReconcileRepo{record: pendingRecord()}
	gw := &fakeGateway{err: errors.New("dial tcp: connection refused")}

	result, err := newReconciler(repo, gw, &fakeNotifier{}).
		Reconcile(context.Background(), " 12345123456789012345678", domain.ReconcileTriggerSweep)

	require.NoError(t, err)
	assert.Equal(t, domain.ReconcileOutcomeError, result.Outcome)
	require.Len(t, repo.attempts, 1)
	assert.Contains(t, repo.attempts[0].ErrorDetail, "connection refused")
}

// --- sweep ---------------------------------------------------------------

// One bad record must not abort the run: the whole point is recovering money
// nobody knows is missing, and stopping at the first failure would strand the
// rest for another cycle.
func TestSweep_ContinuesPastAFailingRecord(t *testing.T) {
	first := pendingRecord()
	first.VirtualAccountNo = " 12345000000000000000001"
	second := pendingRecord()
	second.VirtualAccountNo = " 12345000000000000000002"

	repo := &fakeReconcileRepo{stale: []*domain.VAInquiryRecord{first, second}, settleReturns: true}
	gw := &fakeGateway{resp: statusResponse(domain.CodeStatusSuccess, domain.PaymentFlagSuccess, "150000.00")}
	notifier := &fakeNotifier{}

	results, err := newReconciler(repo, gw, notifier).Sweep(context.Background())

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, 2, gw.calls)
	assert.Len(t, notifier.payloads, 2)
}

func TestSweep_NoStaleTransactions_MakesNoCalls(t *testing.T) {
	repo := &fakeReconcileRepo{}
	gw := &fakeGateway{}

	results, err := newReconciler(repo, gw, &fakeNotifier{}).Sweep(context.Background())

	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Zero(t, gw.calls)
}

// The cutoff is what keeps the sweep off the happy path — a VA created seconds
// ago is new, not stuck.
func TestSweep_UsesTheConfiguredPendingThreshold(t *testing.T) {
	repo := &captureCutoffRepo{}
	uc := NewReconciliationUsecase(repo, &fakeGateway{}, &fakeNotifier{}, ReconcileConfig{
		PendingAfter: 30 * time.Minute,
		BatchSize:    7,
	})
	fixed := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return fixed }

	_, err := uc.Sweep(context.Background())

	require.NoError(t, err)
	assert.Equal(t, fixed.Add(-30*time.Minute), repo.cutoff)
	assert.Equal(t, 7, repo.limit)
}

type captureCutoffRepo struct {
	fakeReconcileRepo
	cutoff time.Time
	limit  int
}

func (r *captureCutoffRepo) ListStalePendingTransactions(_ context.Context, olderThan time.Time, limit int) ([]*domain.VAInquiryRecord, error) {
	r.cutoff = olderThan
	r.limit = limit
	return nil, nil
}

// Defaults must be sane rather than zero — a zero threshold would sweep every
// freshly-created VA, and a zero batch would sweep nothing at all.
func TestReconcileConfig_ZeroValuesFallBackToDefaults(t *testing.T) {
	uc := NewReconciliationUsecase(&fakeReconcileRepo{}, &fakeGateway{}, nil, ReconcileConfig{})

	assert.Equal(t, DefaultReconcileConfig.PendingAfter, uc.config.PendingAfter)
	assert.Equal(t, DefaultReconcileConfig.BatchSize, uc.config.BatchSize)
}
