package domain

import (
	"context"
	"time"
)

// Vendor status reconciliation (feature 014-vendor-status-reconciliation).
//
// BCA's SNAP Virtual Account Inquiry Status (service code 26) runs in the
// OPPOSITE direction to inquiry (24) and payment (25): the partner calls BCA,
// not the other way round. Its own field table says so — "03 = Pending between
// BCA and the partner. If the payment flag process is not yet completed and
// the partner performs an inquiry within that time frame, the transaction with
// status 03 will be delivered to the partner" — and its curl sample targets
// BCA's host while inquiry's and payment's target the co-partner's.
//
// That makes service 26 the recovery path for the one failure this gateway
// could not otherwise survive: a payment flag that never arrived. If BCA takes
// the customer's money and the /payment call is lost — network, a crash
// mid-write, or BCA exhausting its advice retries — this service has no record
// at all. No callback fires, the merchant never learns of the payment, and
// nothing in the system knows anything is wrong. Asking BCA is the only way to
// find out.
//
// The INBOUND /transfer-va/status endpoint is deliberately left alone. It
// serves vendors that follow the ASPI-generic model, where service 26 sits on
// the PJP; BCA never calls it, so proxying it outward would not reconcile
// anything for BCA while coupling our response latency to BCA's availability.

// ReconcileOutcome classifies what a status inquiry concluded. Stored verbatim
// in va_status_inquiry_attempts.outcome, so operators can answer "how often is
// this actually recovering money?" from the audit trail alone.
const (
	// ReconcileOutcomeSettled means the vendor reported the payment as
	// successful for a transaction this service still had pending — money that
	// was already taken and would otherwise never have been recorded. This is
	// the outcome the feature exists for.
	ReconcileOutcomeSettled = "settled"
	// ReconcileOutcomePending means the vendor's flag is still in flight
	// ("03"). Nothing to do; the next sweep asks again.
	ReconcileOutcomePending = "pending"
	// ReconcileOutcomeNotPaid means the vendor has no successful payment for
	// this VA — either no record at all (4042601) or a rejected flag ("01").
	// The transaction is correctly still pending.
	ReconcileOutcomeNotPaid = "not_paid"
	// ReconcileOutcomeAmbiguous is no longer produced. It used to mean the
	// vendor reported "02" (timeout between switcher and partner) and the
	// reconciler declined to read it either way. This company is registered at
	// BCA as force settle — "if company's reconciliation type is reversal or
	// force settle, transaction with status 02 will be considered as success
	// transaction" — so a "02" now settles like a "00".
	//
	// The constant stays because rows written before that change still carry
	// the value, and the audit trail must remain readable. Anything settled on
	// a timeout is identifiable by
	// va_status_inquiry_attempts.bca_payment_flag_status = '02'.
	ReconcileOutcomeAmbiguous = "ambiguous"
	// ReconcileOutcomeAlreadySettled means the transaction stopped being
	// pending between selection and the call (a real /payment landed, or a
	// concurrent reconcile won). No vendor call is made.
	ReconcileOutcomeAlreadySettled = "already_settled"
	// ReconcileOutcomeError means the vendor could not be reached or answered
	// unparseably. Recorded so a silently-broken reconciler is visible.
	ReconcileOutcomeError = "error"
)

// ReconcileTrigger records what caused an attempt.
const (
	ReconcileTriggerSweep = "sweep"
	ReconcileTriggerAdmin = "admin"
)

// VAStatusInquiryAttempt is one outbound status inquiry, recorded whether it
// succeeded or not. Persisted to va_status_inquiry_attempts.
type VAStatusInquiryAttempt struct {
	ID               string
	VirtualAccountNo string
	// ClientID identifies the vendor this inquiry went to, so the audit trail
	// stays readable once more than one vendor exposes a status service.
	ClientID         string
	PaymentRequestID string
	Outcome          string
	// VendorResponseCode and VendorPaymentFlagStatus are the vendor's own
	// answer, kept raw. An outcome is an interpretation; these are the
	// evidence it was interpreted from.
	VendorResponseCode      string
	VendorPaymentFlagStatus string
	DurationMs              int
	ErrorDetail             string
	AttemptedAt             time.Time
}

// ReconcileResult is what a single reconciliation concluded.
type ReconcileResult struct {
	VirtualAccountNo string
	Outcome          string
	// Settled reports whether this call moved the transaction to paid — the
	// signal an operator or the sweep log actually cares about.
	Settled                 bool
	VendorResponseCode      string
	VendorPaymentFlagStatus string
	PaidAmount              *Amount
	ReconciledAt            time.Time
}

// VAStatusInquiryRepository persists the reconciliation audit trail.
type VAStatusInquiryRepository interface {
	CreateStatusInquiryAttempt(ctx context.Context, attempt *VAStatusInquiryAttempt) error
}

// VendorSettlement carries the payment facts a status response reports. It is
// deliberately smaller than VAPaymentRecord: a status inquiry answers "was it
// paid, how much, when" and nothing else, so the settle path must not pretend
// to know holder details or channel metadata it never received.
type VendorSettlement struct {
	PaidAmount       string
	Currency         string
	PaymentRequestID string
	ReferenceNo      string
	TransactionDate  time.Time
}

// ReconciliationRepository is everything the reconciler needs from storage,
// stated as one interface at the consumer (Constitution I).
type ReconciliationRepository interface {
	StaleTransactionFinder
	VAStatusInquiryRepository
	GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*VAInquiryRecord, error)
	// SettlePendingFromVendor applies a vendor-confirmed payment to a
	// still-pending transaction, returning false when the transaction was no
	// longer pending — i.e. someone else settled it first, so this caller must
	// not notify the merchant a second time.
	SettlePendingFromVendor(ctx context.Context, virtualAccountNo string, settlement VendorSettlement) (bool, error)
}

// StaleTransactionFinder selects transactions worth asking the vendor about.
// Split out of VARepository so the sweep's needs are stated where its consumer
// can see them (Constitution I) rather than buried in the wider VA interface.
type StaleTransactionFinder interface {
	// ListStalePendingTransactions returns pending ("03") transactions created
	// before olderThan, oldest first, capped at limit.
	//
	// olderThan is what keeps the sweep off the happy path: a VA created
	// seconds ago and not yet paid is not stuck, it is simply new. Only a
	// transaction that has been pending long enough that a payment SHOULD have
	// arrived by now is worth an outbound call.
	ListStalePendingTransactions(ctx context.Context, olderThan time.Time, limit int) ([]*VAInquiryRecord, error)
}

// VAStatusReconciler asks the vendor what really happened to a transaction
// this service still believes is pending, and settles it if the vendor says it
// was paid.
type VAStatusReconciler interface {
	// Reconcile inquires the vendor about one VA number.
	Reconcile(ctx context.Context, virtualAccountNo, trigger string) (*ReconcileResult, error)
	// Sweep reconciles every transaction that has been pending longer than the
	// configured threshold, returning what it concluded for each. Errors on
	// individual transactions do not abort the sweep — one unreachable
	// transaction must not stop the others from being recovered.
	Sweep(ctx context.Context) ([]*ReconcileResult, error)
}

// VendorTokenProvider obtains an accessToken FROM a vendor, for calls this
// service originates. Named for the direction it serves: domain.JWTIssuer
// mints tokens for callers coming IN, this fetches one for going OUT.
type VendorTokenProvider interface {
	// AccessToken returns a currently-valid token, fetching a new one only
	// when the cached one is missing or close to expiry.
	AccessToken(ctx context.Context) (string, error)
}
