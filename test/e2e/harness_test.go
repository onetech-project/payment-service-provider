// Package e2e drives the SNAP Virtual Account endpoints through the real HTTP
// stack — router, idempotency middleware, SNAP auth middleware, handler and
// usecase — against an in-memory repository.
//
// The point of testing here rather than at the handler is that the parts most
// likely to break BCA conformance are the ones between the socket and the
// handler: the stringToSign the signature is computed over, the service code
// carried by a middleware rejection, and the shape of the JSON body. A handler
// unit test cannot see any of those.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"backbone-new/internal/adapter/delivery/http/handler"
	customMiddleware "backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/config"
	"backbone-new/internal/infrastructure/crypto"
	"backbone-new/internal/usecase"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

const (
	inquiryPath = "/openapi/v1.0/transfer-va/inquiry"
	paymentPath = "/openapi/v1.0/transfer-va/payment"
	// BCA calls the status service at v2.0.
	statusPath = "/openapi/v2.0/transfer-va/status"

	testChannelID = "95231"
	testPartnerID = "12345"
	testSecret    = "vendor-client-secret"
	// testClientID mirrors VENDOR_CLIENT_ID in .env.bca.va: the real BCA
	// vendor is onboarded ClientID-first, so every request it sends carries
	// `Authorization: Bearer <accessToken>` and binds that token into
	// stringToSign. Leaving it empty here would silently put the whole suite
	// on the legacy no-token convention and record a transcript that does not
	// look like production traffic.
	testClientID = "e2e-vendor-client"
)

// testPartnerServiceID is testPartnerID as it goes into a PAYLOAD. The header
// and the body carry the same company code in two different shapes, and
// conflating them is why this suite spent so long sending a format BCA never
// sends.
var testPartnerServiceID = padPartnerServiceID(testPartnerID)

// padPartnerServiceID renders a company code the way partnerServiceId is
// specified across all three service docs: String(8) Fixed, "using space
// padding “ “ on the left if it doesn't reach 8 characters".
//
// X-PARTNER-ID carries the same code UNPADDED — every BCA sample sends
// `X-PARTNER-ID: 12345` next to `"partnerServiceId": "   12345"` — so the two
// are derived from one value here rather than being the same string.
//
// virtualAccountNo keeps the padding too: it is documented as "partnerServiceId
// (8 digit left padding space “ “) + customerNo", and BCA's samples show
// `"virtualAccountNo": "   12345123456789012345678"` at 26 characters. Trimming
// it would produce a VA number 3 characters shorter than the one BCA sends.
func padPartnerServiceID(partnerID string) string {
	if len(partnerID) >= 8 {
		return partnerID
	}
	return strings.Repeat(" ", 8-len(partnerID)) + partnerID
}

// --- vendor access token -------------------------------------------------

// stubJWTIssuer mints and validates the vendor access tokens this suite sends.
// Real JWT signing is not what these tests are about — the header plumbing and
// the AccessToken component of stringToSign are — and the token's own
// cryptography is pinned in middleware/snap_auth_test.go and
// usecase/token_usecase_test.go.
type stubJWTIssuer struct {
	mu     sync.Mutex
	issued map[string]string // token -> clientID
}

func newStubJWTIssuer() *stubJWTIssuer {
	return &stubJWTIssuer{issued: map[string]string{}}
}

func (i *stubJWTIssuer) GenerateB2BToken(clientID string, _ time.Duration) (string, string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	token := "e2e-token-" + clientID
	i.issued[token] = clientID
	return token, "jti-" + clientID, nil
}

func (i *stubJWTIssuer) ValidateToken(token string) (*domain.TokenClaims, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	clientID, ok := i.issued[token]
	if !ok {
		return nil, fmt.Errorf("unknown token %q", token)
	}
	return &domain.TokenClaims{ClientID: clientID, JTI: "jti-" + clientID}, nil
}

// tokenFor returns the bearer token a vendor presents. A vendor with no
// ClientID has not migrated to ClientID-based onboarding, and signs under the
// legacy empty-AccessToken convention with no Authorization header at all.
func (s *server) tokenFor(t *testing.T, clientID string) string {
	t.Helper()
	if clientID == "" {
		return ""
	}
	token, _, err := s.issuer.GenerateB2BToken(clientID, time.Hour)
	require.NoError(t, err)
	return token
}

// --- in-memory repository ----------------------------------------------

type memRepo struct {
	mu sync.Mutex

	accounts     map[string]*domain.VAAccount       // keyed by virtualAccountNo
	transactions map[string]*domain.VAInquiryRecord // keyed by virtualAccountNo
	payments     map[string]*domain.VAPaymentRecord // keyed by paymentRequestId
	bills        map[string][]domain.BillDetail     // keyed by transaction id
	cumulative   map[string]float64                 // keyed by transaction id
	instalments  map[string]string                  // variable-bill dedup: paymentRequestId -> transaction id
	// paymentFlags mirrors va_payment_flags: every flag outcome, accepted or
	// rejected, keyed by the (X-EXTERNAL-ID, paymentRequestId) pair BCA's
	// double-flagging rule names.
	paymentFlags map[string]*domain.VAPaymentFlag

	savePaymentErr error
}

// paymentFlagKey mirrors the UNIQUE (external_id, payment_request_id) index.
// The separator cannot occur in either half — both are header/body scalars
// with no colon in practice — so distinct pairs cannot collide onto one key.
func paymentFlagKey(externalID, paymentRequestID string) string {
	return externalID + "\x00" + paymentRequestID
}

func newMemRepo() *memRepo {
	return &memRepo{
		accounts:     map[string]*domain.VAAccount{},
		transactions: map[string]*domain.VAInquiryRecord{},
		payments:     map[string]*domain.VAPaymentRecord{},
		bills:        map[string][]domain.BillDetail{},
		cumulative:   map[string]float64{},
		instalments:  map[string]string{},
		paymentFlags: map[string]*domain.VAPaymentFlag{},
	}
}

func (r *memRepo) FindPaymentFlag(_ context.Context, externalID, paymentRequestID string) (*domain.VAPaymentFlag, error) {
	if externalID == "" || paymentRequestID == "" {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paymentFlags[paymentFlagKey(externalID, paymentRequestID)], nil
}

// RecordPaymentFlag is first-write-wins, matching the production
// ON CONFLICT DO NOTHING: a double flag must not overwrite the verdict it is
// supposed to be echoing.
func (r *memRepo) RecordPaymentFlag(_ context.Context, flag *domain.VAPaymentFlag) error {
	if flag.ExternalID == "" || flag.PaymentRequestID == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := paymentFlagKey(flag.ExternalID, flag.PaymentRequestID)
	if _, exists := r.paymentFlags[key]; exists {
		return nil
	}
	stored := *flag
	r.paymentFlags[key] = &stored
	return nil
}

func (r *memRepo) putTransaction(rec *domain.VAInquiryRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactions[rec.VirtualAccountNo] = rec
}

func (r *memRepo) putAccount(acc *domain.VAAccount) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accounts[acc.VirtualAccountNo] = acc
}

func (r *memRepo) GetVAAccount(_ context.Context, virtualAccountNo string) (*domain.VAAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.accounts[virtualAccountNo]; ok {
		return acc, nil
	}
	return nil, domain.ErrVAAccountNotFound
}

func (r *memRepo) GetVAByVirtualAccountNo(_ context.Context, virtualAccountNo string) (*domain.VAInquiryRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.transactions[virtualAccountNo]; ok {
		return rec, nil
	}
	return nil, domain.ErrMerchantVANotFound
}

func (r *memRepo) GetInquiry(_ context.Context, inquiryRequestID string) (*domain.VAInquiryRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.transactions {
		if rec.InquiryRequestID != "" && rec.InquiryRequestID == inquiryRequestID {
			return rec, nil
		}
	}
	return nil, domain.ErrVAInvalidBill
}

func (r *memRepo) ClaimInquiryRequestID(_ context.Context, id, inquiryRequestID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.transactions {
		if rec.ID == id {
			rec.InquiryRequestID = inquiryRequestID
		}
	}
	return nil
}

// GetPayment models the real repository's lookup, which is a single query over
// va_transactions matching "payment_request_id = $1 OR inquiry_request_id =
// $1" — one table holding both the inquiry and the payment.
//
// Modelling the OR faithfully is the point. This fake used to key on
// paymentRequestId alone, which quietly made the fake stricter than the
// database and hid a real defect: because BCA sets paymentRequestId equal to
// inquiryRequestId when a payment follows an inquiry, the production query
// matched the still-unpaid transaction and answered the customer's first
// payment 4042518 "Inconsistent Request". A fake that cannot express the
// production query cannot fail on the production bug.
func (r *memRepo) GetPayment(_ context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[paymentRequestID]; ok {
		return p, nil
	}
	// The inquiry_request_id half of the OR: a transaction that has been
	// inquired but not yet paid still matches.
	for _, txn := range r.transactions {
		if txn.InquiryRequestID != "" && txn.InquiryRequestID == paymentRequestID {
			return &domain.VAPaymentRecord{
				ID:               txn.ID,
				PartnerServiceID: txn.PartnerServiceID,
				CustomerNo:       txn.CustomerNo,
				CustomerName:     txn.CustomerName,
				VirtualAccountNo: txn.VirtualAccountNo,
				InquiryRequestID: txn.InquiryRequestID,
				TrxID:            txn.TrxID,
				TotalAmount:      txn.TotalAmount,
				Currency:         txn.Currency,
				Status:           txn.Status,
			}, nil
		}
	}
	return nil, domain.ErrVAInvalidBill
}

// GetPaymentByPaymentRequestID is the strict counterpart used by the payment
// endpoint's already-recorded check: paymentRequestId only, no OR.
func (r *memRepo) GetPaymentByPaymentRequestID(_ context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[paymentRequestID]; ok {
		return p, nil
	}
	return nil, domain.ErrVAInvalidBill
}

func (r *memRepo) SavePayment(_ context.Context, payment *domain.VAPaymentRecord) error {
	if r.savePaymentErr != nil {
		return r.savePaymentErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if payment.ID == "" {
		payment.ID = "pay-" + payment.PaymentRequestID
	}
	r.payments[payment.PaymentRequestID] = payment
	if rec, ok := r.transactions[payment.VirtualAccountNo]; ok {
		rec.Status = "00" // settled
	}
	return nil
}

func (r *memRepo) SaveNoBillPayment(_ context.Context, payment *domain.VAPaymentRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.payments[payment.PaymentRequestID]; exists {
		return domain.ErrVAPaymentDuplicate
	}
	payment.ID = "pay-" + payment.PaymentRequestID
	r.payments[payment.PaymentRequestID] = payment
	return nil
}

// FindVAInstalment mirrors the paymentRequestId lookup over va_payments.
func (r *memRepo) FindVAInstalment(_ context.Context, paymentRequestID string) (string, string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	txID, ok := r.instalments[paymentRequestID]
	if !ok || paymentRequestID == "" {
		return "", "", false, nil
	}
	return txID, strconv.FormatFloat(r.cumulative[txID], 'f', 2, 64), true, nil
}

// SaveVAPayment accumulates instalments against a variable bill and reports
// "00" once the cumulative total reaches the bill amount, mirroring the SQL
// upsert in the real repository.
func (r *memRepo) SaveVAPayment(_ context.Context, transactionID, paymentRequestID, amount, _ string) (string, string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// paymentRequestId dedup, mirroring the partial unique index in the real
	// schema: an instalment already on file must not be credited twice.
	recorded := true
	if _, seen := r.instalments[paymentRequestID]; seen && paymentRequestID != "" {
		recorded = false
	} else {
		r.instalments[paymentRequestID] = transactionID
		paid, _ := strconv.ParseFloat(amount, 64)
		r.cumulative[transactionID] += paid
	}

	status := "03"
	for _, rec := range r.transactions {
		if rec.ID != transactionID {
			continue
		}
		total, _ := strconv.ParseFloat(rec.TotalAmount, 64)
		if total > 0 && r.cumulative[transactionID] >= total {
			status = "00"
			rec.Status = "00"
		}
	}
	return strconv.FormatFloat(r.cumulative[transactionID], 'f', 2, 64), status, recorded, nil
}

func (r *memRepo) GetVABillDetails(_ context.Context, transactionID string) ([]domain.BillDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bills[transactionID], nil
}

func (r *memRepo) SaveBillDetails(_ context.Context, transactionID string, bills []domain.BillDetail) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bills[transactionID] = bills
	return nil
}

func (r *memRepo) UpdateVAStatus(_ context.Context, virtualAccountNo, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.transactions[virtualAccountNo]; ok && rec.Status == "03" {
		rec.Status = status
	}
	return nil
}

func (r *memRepo) UpdateVAAccountStatus(_ context.Context, virtualAccountNo, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if acc, ok := r.accounts[virtualAccountNo]; ok {
		acc.Status = status
	}
	return nil
}

// Unused by the VA transaction flows under test.
func (r *memRepo) SaveInquiry(context.Context, *domain.VAInquiryRecord) error { return nil }
func (r *memRepo) UpdatePaymentStatus(context.Context, string, string) error  { return nil }
func (r *memRepo) NextCustomerNoSequence(context.Context, string) (string, error) {
	return "", nil
}
func (r *memRepo) RegisterStaticCustomerNo(context.Context, string, string) error { return nil }
func (r *memRepo) SaveVAAccount(context.Context, *domain.VAAccount) error         { return nil }
func (r *memRepo) GetVAAccountByPartnerAndCustomer(context.Context, string, string) (*domain.VAAccount, error) {
	return nil, domain.ErrVAAccountNotFound
}
func (r *memRepo) ListVAAccounts(context.Context, *domain.VAAccountListFilter) ([]domain.VAAccountListItem, int, error) {
	return nil, 0, nil
}
func (r *memRepo) ListVATransactions(context.Context, *domain.VAListFilter) ([]domain.VATransactionListItem, int, error) {
	return nil, 0, nil
}

// --- notifier -----------------------------------------------------------

type recordingNotifier struct {
	mu       sync.Mutex
	payloads []*domain.PaymentNotificationPayload
}

func (n *recordingNotifier) EnqueuePaymentNotification(_ context.Context, p *domain.PaymentNotificationPayload) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.payloads = append(n.payloads, p)
	return nil
}

func (n *recordingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.payloads)
}

// --- idempotency store --------------------------------------------------

type memIdempotencyStore struct {
	mu     sync.Mutex
	cache  map[string][]byte
	locked map[string]bool
}

func newMemIdempotencyStore() *memIdempotencyStore {
	return &memIdempotencyStore{cache: map[string][]byte{}, locked: map[string]bool{}}
}

func (s *memIdempotencyStore) GetResponseCache(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache[key], nil
}

func (s *memIdempotencyStore) AcquireLock(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked[key] {
		return false, nil
	}
	s.locked[key] = true
	return true, nil
}

func (s *memIdempotencyStore) ReleaseLock(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.locked, key)
	return nil
}

func (s *memIdempotencyStore) SetResponseCache(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = value
	return nil
}

// --- server harness -----------------------------------------------------

type vendor struct {
	name   string
	config *config.VendorConfig
}

type server struct {
	echo     *echo.Echo
	repo     *memRepo
	notifier *recordingNotifier
	vendors  map[string]*config.VendorConfig
	issuer   *stubJWTIssuer
}

func defaultVendorConfig() *config.VendorConfig {
	return &config.VendorConfig{
		Vendor:                "bca",
		Channel:               "va",
		ClientID:              testClientID,
		ClientSecret:          testSecret,
		ChannelID:             testChannelID,
		PartnerID:             testPartnerID,
		SignatureAlgorithm:    "HMAC-SHA512",
		BodyHashEncoding:      crypto.BodyHashHex,
		StrictMandatoryFields: true,
		RequiredHeaders:       []string{"X-TIMESTAMP", "X-SIGNATURE"},
	}
}

// newServer wires the production router shape: idempotency in front, SNAP auth
// per vendor, then the handler. Each vendor gets its own handler so per-vendor
// strictness is exercised the way main.go wires it.
func newServer(t *testing.T, vendors ...vendor) *server {
	t.Helper()
	if len(vendors) == 0 {
		vendors = []vendor{{name: "bca", config: defaultVendorConfig()}}
	}

	repo := newMemRepo()
	notifier := &recordingNotifier{}
	uc := usecase.NewVAUsecase(repo, notifier)
	issuer := newStubJWTIssuer()

	e := echo.New()
	store := newMemIdempotencyStore()

	registered := map[string]*config.VendorConfig{}
	configs := make([]*config.VendorConfig, 0, len(vendors))
	for _, v := range vendors {
		registered[v.name] = v.config
		configs = append(configs, v.config)
	}

	// Mirrors main.go: routes registered ONCE for all vendors, with the
	// vendor resolved from the request. Registering per vendor would silently
	// leave only the last one reachable.
	vh := handler.NewVAHandler(uc)
	for _, basePath := range []string{"/openapi/v1.0", "/openapi/v2.0"} {
		group := e.Group(basePath + "/transfer-va")
		// Mirrors main.go including the payment exemption. Without it the
		// harness ran a rule production never had — a repeated
		// X-EXTERNAL-ID on /payment was answered from the middleware instead
		// of reaching the usecase that owns the 4042518 double-flag rule.
		group.Use(customMiddleware.IdempotencyMiddleware(store, time.Minute, time.Hour,
			customMiddleware.WithDoubleFlagPassthroughFor(func(c echo.Context) bool {
				return strings.HasSuffix(c.Request().URL.Path, "/transfer-va/payment")
			})))
		vendorGroup := group.Group("")
		vendorGroup.Use(customMiddleware.MultiVendorSNAPAuth(configs, issuer, true))
		vendorGroup.POST("/inquiry", vh.Inquiry)
		vendorGroup.POST("/payment", vh.Payment)
		vendorGroup.POST("/status", vh.Status)
	}

	return &server{echo: e, repo: repo, notifier: notifier, vendors: registered, issuer: issuer}
}

type response struct {
	status int
	body   map[string]any
	raw    string
}

// vaData returns the virtualAccountData object, failing the test if absent —
// BCA marks it Mandatory on every transfer-va response.
func (r response) vaData(t *testing.T) map[string]any {
	t.Helper()
	data, ok := r.body["virtualAccountData"].(map[string]any)
	require.True(t, ok, "virtualAccountData is mandatory on every transfer-va response, got: %s", r.raw)
	return data
}

func (r response) code() string {
	code, _ := r.body["responseCode"].(string)
	return code
}

// requestOptions lets a test bend one part of an otherwise-valid request,
// which is what most of the negative cases need.
type requestOptions struct {
	externalID   string
	channelID    string
	partnerID    string
	timestamp    string
	signature    string // overrides the computed one
	secret       string // signs with a different secret
	rawBody      string // sent verbatim, e.g. malformed JSON
	bodyEncoding string
	// accessToken is both the Authorization bearer and the AccessToken
	// component of stringToSign — the two must agree or the signature does not
	// verify. Empty means the legacy convention: no header, empty component.
	accessToken string
	// clientID selects which vendor the default accessToken is minted for.
	// Ignored once accessToken is set explicitly.
	clientID string
	// extraHeaders are sent on top of the documented set, to prove the
	// service ignores anything BCA does not publish.
	extraHeaders map[string]string
}

// call signs and sends a request exactly as a vendor would: stringToSign over
// the minified body hash, HMAC-SHA512, base64.
func (s *server) call(t *testing.T, path string, payload any, opts ...func(*requestOptions)) response {
	t.Helper()

	o := requestOptions{
		externalID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		channelID:    testChannelID,
		partnerID:    testPartnerID,
		timestamp:    time.Now().Format(time.RFC3339),
		secret:       testSecret,
		bodyEncoding: crypto.BodyHashHex,
		clientID:     testClientID,
	}
	for _, apply := range opts {
		apply(&o)
	}
	// Minted after the options are applied so withAccessToken/withClientID can
	// steer it, and so a test that clears the clientID gets no token at all.
	if o.accessToken == "" {
		o.accessToken = s.tokenFor(t, o.clientID)
	}

	body := o.rawBody
	if body == "" {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		body = string(encoded)
	}

	signature := o.signature
	if signature == "" {
		bodyHash := crypto.HashRequestBody([]byte(body), o.bodyEncoding)
		stringToSign := crypto.BuildStringToSign(http.MethodPost, path, o.accessToken, bodyHash, o.timestamp)
		signature = crypto.NewHMACSigner(o.secret, "HMAC-SHA512").Sign(stringToSign)
	}

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-TIMESTAMP", o.timestamp)
	req.Header.Set("X-SIGNATURE", signature)
	if o.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+o.accessToken)
	}
	if o.externalID != "" {
		req.Header.Set("X-EXTERNAL-ID", o.externalID)
	}
	if o.channelID != "" {
		req.Header.Set("CHANNEL-ID", o.channelID)
	}
	if o.partnerID != "" {
		req.Header.Set("X-PARTNER-ID", o.partnerID)
	}

	for name, value := range o.extraHeaders {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	s.echo.ServeHTTP(rec, req)
	transcript.record(t, req, body, rec)

	out := response{status: rec.Code, raw: rec.Body.String()}
	if err := json.Unmarshal(rec.Body.Bytes(), &out.body); err != nil {
		t.Fatalf("response is not JSON (%d): %s", rec.Code, out.raw)
	}
	return out
}

func withExternalID(id string) func(*requestOptions) {
	return func(o *requestOptions) { o.externalID = id }
}

func withoutExternalID() func(*requestOptions) {
	return func(o *requestOptions) { o.externalID = "" }
}

func withChannelID(id string) func(*requestOptions) {
	return func(o *requestOptions) { o.channelID = id }
}

func withPartnerID(id string) func(*requestOptions) {
	return func(o *requestOptions) { o.partnerID = id }
}

func withTimestamp(ts string) func(*requestOptions) {
	return func(o *requestOptions) { o.timestamp = ts }
}

func withSignature(sig string) func(*requestOptions) {
	return func(o *requestOptions) { o.signature = sig }
}

func withSecret(secret string) func(*requestOptions) {
	return func(o *requestOptions) { o.secret = secret }
}

func withRawBody(body string) func(*requestOptions) {
	return func(o *requestOptions) { o.rawBody = body }
}

func withBodyEncoding(encoding string) func(*requestOptions) {
	return func(o *requestOptions) { o.bodyEncoding = encoding }
}

// withClientID mints the request's accessToken for a different vendor client —
// the multi-vendor tests use it so each vendor presents its own token.
func withClientID(clientID string) func(*requestOptions) {
	return func(o *requestOptions) { o.clientID = clientID }
}

// withAccessToken sends a token the issuer never minted, to prove the
// middleware validates it rather than trusting the header.
func withAccessToken(token string) func(*requestOptions) {
	return func(o *requestOptions) { o.accessToken = token }
}

// withoutAccessToken drops the Authorization header and signs under the legacy
// empty-AccessToken convention — which a ClientID-onboarded vendor must be
// rejected for.
func withoutAccessToken() func(*requestOptions) {
	return func(o *requestOptions) {
		o.clientID = ""
		o.accessToken = ""
	}
}

func withExtraHeader(name, value string) func(*requestOptions) {
	return func(o *requestOptions) {
		if o.extraHeaders == nil {
			o.extraHeaders = map[string]string{}
		}
		o.extraHeaders[name] = value
	}
}

// --- payload builders ---------------------------------------------------

func inquiryPayload(partnerServiceID, customerNo, inquiryRequestID string) map[string]any {
	return map[string]any{
		"partnerServiceId": partnerServiceID,
		"customerNo":       customerNo,
		"virtualAccountNo": partnerServiceID + customerNo,
		"trxDateInit":      time.Now().Format(time.RFC3339),
		"channelCode":      6011,
		"inquiryRequestId": inquiryRequestID,
		"additionalInfo":   map[string]any{},
	}
}

// paymentPayload builds a request carrying every field BCA marks Mandatory.
func paymentPayload(partnerServiceID, customerNo, paymentRequestID, amount string) map[string]any {
	return map[string]any{
		"partnerServiceId":   partnerServiceID,
		"customerNo":         customerNo,
		"virtualAccountNo":   partnerServiceID + customerNo,
		"virtualAccountName": "Budi Manjo",
		"paymentRequestId":   paymentRequestID,
		"channelCode":        6011,
		"paidAmount":         map[string]any{"value": amount, "currency": "IDR"},
		"totalAmount":        map[string]any{"value": amount, "currency": "IDR"},
		"trxDateTime":        time.Now().Format(time.RFC3339),
		"referenceNo":        "12345678901",
		"flagAdvise":         "N",
		"additionalInfo":     map[string]any{},
	}
}

func statusPayload(partnerServiceID, customerNo, inquiryRequestID string) map[string]any {
	return map[string]any{
		"partnerServiceId": partnerServiceID,
		"customerNo":       customerNo,
		"virtualAccountNo": partnerServiceID + customerNo,
		"inquiryRequestId": inquiryRequestID,
		"additionalInfo":   map[string]any{},
	}
}

// mustJSON is a readability helper for tests that assert on raw bodies.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(v))
	return buf.String()
}
