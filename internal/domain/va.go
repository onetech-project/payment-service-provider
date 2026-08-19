package domain

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// VA Inquiry Request/Response types

// VAInquiryRequest represents inbound inquiry from vendor
type VAInquiryRequest struct {
	PartnerServiceID string `json:"partnerServiceId"`
	CustomerNo       string `json:"customerNo"`
	VirtualAccountNo string `json:"virtualAccountNo"`
	// TrxDateInit is spelled trxDateInit by BCA (Developer API BCA, inquiry
	// payload). It was previously tagged "txnDateInit", which silently never
	// bound.
	TrxDateInit *time.Time `json:"trxDateInit,omitempty"`
	// Amount is Optional (N) on the inquiry payload. It was absent from the
	// table altogether in the older documentation and reappears in
	// VA-BillPresentment v2.4; it has never been mandatory, and requiring it
	// would reject every inquiry that omits it. Treated as a passthrough: the
	// amount a customer entered belongs to the payment, not to the bill this
	// inquiry presents.
	Amount      *Amount `json:"amount,omitempty"`
	ChannelCode int     `json:"channelCode,omitempty"`
	// Language is String(2) ISO-639-1, Optional.
	Language string `json:"language,omitempty"`
	// HashedSourceAccountNo String(32) and SourceBankCode String(3) are both
	// Optional, and both carry the paying account's identity through from the
	// channel.
	HashedSourceAccountNo string                 `json:"hashedSourceAccountNo,omitempty"`
	SourceBankCode        string                 `json:"sourceBankCode,omitempty"`
	InquiryRequestID      string                 `json:"inquiryRequestId"`
	AdditionalInfo        map[string]interface{} `json:"additionalInfo,omitempty"`
	// PassApp is String(64) Optional, "Key for 3rd party to access API like
	// client secret". Bound and length-checked but never acted on: this
	// service authenticates on X-SIGNATURE, and treating a body field as a
	// second credential would be a way in that the signature does not cover.
	PassApp string `json:"passApp,omitempty"`
}

// VAInquiryResponse represents response to vendor inquiry
type VAInquiryResponse struct {
	ResponseCode       string                 `json:"responseCode"`
	ResponseMessage    string                 `json:"responseMessage"`
	VirtualAccountData *VAAccountData         `json:"virtualAccountData,omitempty"`
	AdditionalInfo     map[string]interface{} `json:"additionalInfo"`
}

// MarshalJSON renders additionalInfo as {} rather than null when nothing was
// set. The key is mandatory in the SNAP inquiry response and vendors read it
// unconditionally, so an absent or null value is a parse failure on their side
// — normalising here means no construction site can forget it.
func (r VAInquiryResponse) MarshalJSON() ([]byte, error) {
	type alias VAInquiryResponse
	out := alias(r)
	if out.AdditionalInfo == nil {
		out.AdditionalInfo = map[string]interface{}{}
	}
	return json.Marshal(out)
}

// VAAccountData contains VA account and bill information. Field order matches
// the SNAP inquiry response layout (identity → amount → bills → status), which
// is also the order the JSON is emitted in.
type VAAccountData struct {
	PartnerServiceID      string                 `json:"partnerServiceId"`
	CustomerNo            string                 `json:"customerNo"`
	VirtualAccountNo      string                 `json:"virtualAccountNo"`
	VirtualAccountName    string                 `json:"virtualAccountName"`
	InquiryRequestID      string                 `json:"inquiryRequestId"`
	TotalAmount           *Amount                `json:"totalAmount,omitempty"`
	SubCompany            string                 `json:"subCompany,omitempty"`
	BillDetails           []BillDetail           `json:"billDetails"`
	FreeTexts             []BilingualText        `json:"freeTexts"`
	VirtualAccountTrxType string                 `json:"virtualAccountTrxType,omitempty"`
	FeeAmount             *Amount                `json:"feeAmount,omitempty"`
	InquiryStatus         string                 `json:"inquiryStatus"`
	InquiryReason         *BilingualText         `json:"inquiryReason,omitempty"`
	AdditionalInfo        map[string]interface{} `json:"additionalInfo,omitempty"`
}

// MarshalJSON renders billDetails/freeTexts as [] rather than null when the VA
// has none: both are arrays in the SNAP contract, and a vendor iterating them
// must not have to nil-check first.
func (d VAAccountData) MarshalJSON() ([]byte, error) {
	type alias VAAccountData
	out := alias(d)
	if out.BillDetails == nil {
		out.BillDetails = []BillDetail{}
	}
	if out.FreeTexts == nil {
		out.FreeTexts = []BilingualText{}
	}
	return json.Marshal(out)
}

// VA Payment Request/Response types

// VAPaymentRequest represents inbound payment notification from vendor
type VAPaymentRequest struct {
	PartnerServiceID string `json:"partnerServiceId"`
	CustomerNo       string `json:"customerNo"`
	VirtualAccountNo string `json:"virtualAccountNo"`
	// InquiryRequestID is kept for backward compatibility with legacy vendors
	// still sending it; per ASPI spec, TrxID is the mandatory trace field on
	// this endpoint (inquiryRequestId is instead resolved internally from the
	// merchant's create-VA record, see va_usecase.go Payment()).
	InquiryRequestID        string                 `json:"inquiryRequestId,omitempty"`
	TrxID                   string                 `json:"trxId"`
	PaymentRequestID        string                 `json:"paymentRequestId"`
	VirtualAccountName      string                 `json:"virtualAccountName,omitempty"`
	VirtualAccountEmail     string                 `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone     string                 `json:"virtualAccountPhone,omitempty"`
	ChannelCode             int                    `json:"channelCode,omitempty"`
	HashedSourceAccountNo   string                 `json:"hashedSourceAccountNo,omitempty"`
	SourceBankCode          string                 `json:"sourceBankCode,omitempty"`
	PaidAmount              *Amount                `json:"paidAmount"`
	CumulativePaymentAmount *Amount                `json:"cumulativePaymentAmount,omitempty"`
	PaidBills               string                 `json:"paidBills,omitempty"`
	TotalAmount             *Amount                `json:"totalAmount,omitempty"`
	TrxDateTime             *time.Time             `json:"trxDateTime,omitempty"`
	ReferenceNo             string                 `json:"referenceNo,omitempty"`
	JournalNum              string                 `json:"journalNum,omitempty"`
	PaymentType             string                 `json:"paymentType,omitempty"`
	FlagAdvise              string                 `json:"flagAdvise,omitempty"`
	SubCompany              string                 `json:"subCompany,omitempty"`
	BillDetails             []VAPaymentBillDetail  `json:"billDetails,omitempty"`
	FreeTexts               []BilingualText        `json:"freeTexts,omitempty"`
	AdditionalInfo          map[string]interface{} `json:"additionalInfo,omitempty"`

	// ExternalID carries the X-EXTERNAL-ID header down to the usecase, which
	// needs it to apply BCA's double-flagging rule ("the same X-EXTERNAL-ID and
	// paymentRequestId"). It is a header, not a body field, so it is never
	// unmarshalled from or marshalled into the request JSON — the handler sets
	// it after binding. Empty for callers that do not supply one, which simply
	// falls back to the paymentRequestId-only duplicate check.
	ExternalID string `json:"-"`
}

// VAPaymentBillDetail extends BillDetail with payment-specific fields
type VAPaymentBillDetail struct {
	BillCode          string                 `json:"billCode,omitempty"`
	BillNo            string                 `json:"billNo"`
	BillName          string                 `json:"billName,omitempty"`
	BillShortName     string                 `json:"billShortName,omitempty"`
	BillDescription   *BilingualText         `json:"billDescription"`
	BillSubCompany    string                 `json:"billSubCompany"`
	BillAmount        *Amount                `json:"billAmount"`
	AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
	BillReferenceNo   string                 `json:"billReferenceNo"`
	BillerReferenceID string                 `json:"billerReferenceId,omitempty"`
	Status            string                 `json:"status"`
	Reason            *BilingualText         `json:"reason"`
}

// NormalizeBillDetails drops billDetails entries that carry no data, so a
// request whose array holds nothing but blanks is treated exactly like one that
// never sent billDetails at all.
//
// `"billDetails": [null]` — and its cousins `[{}]` and `[null, null]` — decode
// into a slice of ZERO-VALUE VAPaymentBillDetail: a JSON null unmarshalled into
// a non-pointer struct element leaves the element untouched rather than
// removing it, so len() reports 1 where the vendor meant 0. Everything
// downstream branches on that len: SaveBillDetails would persist a blank bill
// row, and echoPaymentBillDetails would answer with a fabricated
// {status:"00", reason:"Success"} bill the vendor never sent. Collapsing the
// slice to nil keeps those checks honest.
//
// A mixed array keeps its real entries — only the blanks are removed, since a
// null alongside a genuine bill is noise in the same way.
func (r *VAPaymentRequest) NormalizeBillDetails() {
	if len(r.BillDetails) == 0 {
		return
	}
	kept := make([]VAPaymentBillDetail, 0, len(r.BillDetails))
	for _, b := range r.BillDetails {
		if b.IsEmpty() {
			continue
		}
		kept = append(kept, b)
	}
	if len(kept) == 0 {
		r.BillDetails = nil
		return
	}
	r.BillDetails = kept
}

// IsEmpty reports whether the bill detail carries no field the payment flow can
// act on. Pointer fields are judged by their content rather than merely by
// nil-ness, so an explicitly sent but hollow `{"billAmount": {}}` counts as
// empty too.
func (b VAPaymentBillDetail) IsEmpty() bool {
	return b.BillCode == "" &&
		b.BillNo == "" &&
		b.BillName == "" &&
		b.BillShortName == "" &&
		b.BillSubCompany == "" &&
		b.BillReferenceNo == "" &&
		b.BillerReferenceID == "" &&
		b.Status == "" &&
		b.BillDescription.isBlank() &&
		b.Reason.isBlank() &&
		b.BillAmount.isBlank() &&
		len(b.AdditionalInfo) == 0
}

// isBlank reports whether the text is absent or carries no words in either
// language. Nil-safe so callers can chain it on optional fields.
func (t *BilingualText) isBlank() bool {
	return t == nil || (t.English == "" && t.Indonesia == "")
}

// isBlank reports whether the amount is absent or carries neither a value nor a
// currency. Nil-safe, for the same reason as BilingualText.isBlank.
func (a *Amount) isBlank() bool {
	return a == nil || (a.Value == "" && a.Currency == "")
}

// VAPaymentResponse represents response to vendor payment notification
type VAPaymentResponse struct {
	ResponseCode       string                 `json:"responseCode"`
	ResponseMessage    string                 `json:"responseMessage"`
	VirtualAccountData *VAPaymentStatus       `json:"virtualAccountData,omitempty"`
	AdditionalInfo     map[string]interface{} `json:"additionalInfo"`
}

// MarshalJSON guarantees additionalInfo is always emitted as an object, never
// null or absent. The ASPI PaymentResponse declares additionalInfo on every
// response (success and error alike) and vendors parse it unconditionally, so
// serialising a nil map as `null` would break them. Done here rather than at
// each construction site so no response can ever be built without it.
func (r VAPaymentResponse) MarshalJSON() ([]byte, error) {
	type alias VAPaymentResponse
	out := alias(r)
	if out.AdditionalInfo == nil {
		out.AdditionalInfo = map[string]interface{}{}
	}
	return json.Marshal(out)
}

// VAPaymentStatus is PaymentResponse.virtualAccountData, and carries exactly
// the fields listed in the Response table of Tech. Doc. OpenAPI VA-Payment-Flag
// v2.3 — no more.
//
// The REQUEST table is the wider of the two: trxId, flagAdvise,
// virtualAccountEmail/Phone, paidBills, journalNum and paymentType appear there
// but NOT in the Response table, and echoing them back put fields on the wire
// that BCA's response schema does not define. They are still bound inbound on
// VAPaymentRequest, and still drive the flow (flagAdvise decides 2002500 vs
// 4042518, trxId identifies the merchant transaction) — they are simply not
// reported back.
type VAPaymentStatus struct {
	PartnerServiceID string `json:"partnerServiceId"`
	CustomerNo       string `json:"customerNo"`
	VirtualAccountNo string `json:"virtualAccountNo"`
	// VirtualAccountName carries no omitempty: it is reported even when empty,
	// including on the not-found rejection where there is no stored holder to
	// name.
	VirtualAccountName string                        `json:"virtualAccountName"`
	PaymentRequestID   string                        `json:"paymentRequestId"`
	PaidAmount         *Amount                       `json:"paidAmount"`
	TotalAmount        *Amount                       `json:"totalAmount,omitempty"`
	TrxDateTime        *time.Time                    `json:"trxDateTime,omitempty"`
	ReferenceNo        string                        `json:"referenceNo,omitempty"`
	PaymentFlagStatus  string                        `json:"paymentFlagStatus"`
	PaymentFlagReason  *BilingualText                `json:"paymentFlagReason"`
	BillDetails        []VAPaymentResponseBillDetail `json:"billDetails"`
	FreeTexts          []BilingualText               `json:"freeTexts"`
}

// VAPaymentResponseBillDetail is one entry of
// PaymentResponse.virtualAccountData.billDetails. It is deliberately a
// different type from VAPaymentBillDetail, which models the REQUEST: the two
// tables do not agree. billCode, billName, billShortName and billReferenceNo
// are request-side only — billReferenceNo especially, which has no omitempty
// there and so was emitted on every response — while billerReferenceId, status
// and reason are response-side only.
type VAPaymentResponseBillDetail struct {
	BillerReferenceID string                 `json:"billerReferenceId"`
	BillNo            string                 `json:"billNo"`
	BillDescription   *BilingualText         `json:"billDescription"`
	BillSubCompany    string                 `json:"billSubCompany"`
	BillAmount        *Amount                `json:"billAmount"`
	AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
	Status            string                 `json:"status"`
	Reason            *BilingualText         `json:"reason"`
}

// MarshalJSON guarantees billDetails and freeTexts are always emitted as
// arrays. They carry no `omitempty`, so a nil slice would serialise as `null`
// — vendors that iterate the arrays without a nil check choke on that, whereas
// `[]` is unambiguous and matches the ASPI PaymentResponse shape.
func (s VAPaymentStatus) MarshalJSON() ([]byte, error) {
	type alias VAPaymentStatus
	out := alias(s)
	if out.BillDetails == nil {
		out.BillDetails = []VAPaymentResponseBillDetail{}
	}
	if out.FreeTexts == nil {
		out.FreeTexts = []BilingualText{}
	}
	return json.Marshal(out)
}

// VA Status Request/Response types

// VAStatusRequest represents inbound status inquiry from vendor
type VAStatusRequest struct {
	PartnerServiceID string                 `json:"partnerServiceId"`
	CustomerNo       string                 `json:"customerNo"`
	VirtualAccountNo string                 `json:"virtualAccountNo"`
	InquiryRequestID string                 `json:"inquiryRequestId"`
	PaymentRequestID string                 `json:"paymentRequestId,omitempty"`
	AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VAStatusResponse represents response to vendor status inquiry
type VAStatusResponse struct {
	ResponseCode       string                 `json:"responseCode"`
	ResponseMessage    string                 `json:"responseMessage"`
	VirtualAccountData *VAStatusData          `json:"virtualAccountData,omitempty"`
	AdditionalInfo     map[string]interface{} `json:"additionalInfo"`
}

// MarshalJSON renders additionalInfo as {} rather than null when nothing was
// set, matching the inquiry and payment envelopes and BCA's own status
// response sample, which emits the key unconditionally.
func (r VAStatusResponse) MarshalJSON() ([]byte, error) {
	type alias VAStatusResponse
	out := alias(r)
	if out.AdditionalInfo == nil {
		out.AdditionalInfo = map[string]interface{}{}
	}
	return json.Marshal(out)
}

// VAStatusData contains status inquiry result
type VAStatusData struct {
	PaymentFlagStatus string                 `json:"paymentFlagStatus"`
	PaymentFlagReason *BilingualText         `json:"paymentFlagReason"`
	PartnerServiceID  string                 `json:"partnerServiceId"`
	CustomerNo        string                 `json:"customerNo"`
	VirtualAccountNo  string                 `json:"virtualAccountNo"`
	InquiryRequestID  string                 `json:"inquiryRequestId"`
	PaymentRequestID  string                 `json:"paymentRequestId"`
	PaidAmount        *Amount                `json:"paidAmount"`
	PaidBills         string                 `json:"paidBills,omitempty"`
	TotalAmount       *Amount                `json:"totalAmount"`
	TrxDateTime       *time.Time             `json:"trxDateTime,omitempty"`
	TransactionDate   *time.Time             `json:"transactionDate"`
	ReferenceNo       string                 `json:"referenceNo,omitempty"`
	PaymentType       string                 `json:"paymentType,omitempty"`
	FlagAdvise        string                 `json:"flagAdvise,omitempty"`
	BillDetails       []VAStatusBillDetail   `json:"billDetails,omitempty"`
	FreeTexts         []BilingualText        `json:"freeTexts,omitempty"`
	AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VAStatusBillDetail represents bill detail in status response
type VAStatusBillDetail struct {
	BillCode        string                 `json:"billCode,omitempty"`
	BillNo          string                 `json:"billNo"`
	BillName        string                 `json:"billName,omitempty"`
	BillShortName   string                 `json:"billShortName,omitempty"`
	BillDescription *BilingualText         `json:"billDescription"`
	BillSubCompany  string                 `json:"billSubCompany"`
	BillAmount      *Amount                `json:"billAmount"`
	BillReferenceNo string                 `json:"billReferenceNo"`
	Status          string                 `json:"status"`
	Reason          *BilingualText         `json:"reason"`
	AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
}

// Shared Value Objects

// Amount represents monetary amount with currency
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// BilingualText represents text in English and Indonesian
type BilingualText struct {
	English   string `json:"english"`
	Indonesia string `json:"indonesia"`
}

// BillDetail represents a single bill item (14 fields per ASPI OpenAPI)
type BillDetail struct {
	BillCode          string                 `json:"billCode,omitempty"`
	BillNo            string                 `json:"billNo"`
	BillName          string                 `json:"billName,omitempty"`
	BillShortName     string                 `json:"billShortName,omitempty"`
	BillDescription   *BilingualText         `json:"billDescription,omitempty"`
	BillSubCompany    string                 `json:"billSubCompany,omitempty"`
	BillAmount        *Amount                `json:"billAmount,omitempty"`
	BillAmountLabel   string                 `json:"billAmountLabel,omitempty"`
	BillAmountValue   string                 `json:"billAmountValue,omitempty"`
	BillReferenceNo   string                 `json:"billReferenceNo,omitempty"`
	BillerReferenceID string                 `json:"billerReferenceId,omitempty"`
	Status            string                 `json:"status,omitempty"`
	Reason            *BilingualText         `json:"reason,omitempty"`
	AdditionalInfo    map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VA Repository Interface

// VARepository defines persistence operations for VA transactions
type VARepository interface {
	SaveInquiry(ctx context.Context, inquiry *VAInquiryRecord) error
	GetInquiry(ctx context.Context, inquiryRequestID string) (*VAInquiryRecord, error)
	// ClaimInquiryRequestID stamps the vendor's inquiryRequestId onto a row
	// that does not have one yet. A merchant-created VA is stored with an
	// empty inquiry_request_id — the vendor's id simply does not exist at
	// create-va time — so the first inquiry against that VA is what fills it
	// in, letting later Status/Payment calls reach the same row by that id.
	ClaimInquiryRequestID(ctx context.Context, id string, inquiryRequestID string) error
	SavePayment(ctx context.Context, payment *VAPaymentRecord) error
	// GetPayment resolves a transaction by paymentRequestId OR
	// inquiryRequestId. Used by the status service, which may be handed
	// either.
	GetPayment(ctx context.Context, paymentRequestID string) (*VAPaymentRecord, error)
	// GetPaymentByPaymentRequestID resolves a transaction by paymentRequestId
	// only, and is what the payment endpoint's already-recorded check must
	// use. BCA sets paymentRequestId equal to inquiryRequestId when a payment
	// follows an inquiry, so GetPayment's OR matches the still-unpaid
	// transaction the inquiry claimed and misreports the first payment as a
	// double-flag.
	GetPaymentByPaymentRequestID(ctx context.Context, paymentRequestID string) (*VAPaymentRecord, error)
	UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error
	// Merchant dashboard methods
	GetVABillDetails(ctx context.Context, transactionID string) ([]BillDetail, error)
	SaveBillDetails(ctx context.Context, transactionID string, bills []BillDetail) error
	UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error
	GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*VAInquiryRecord, error)
	// Static/Dynamic VA methods (feature 006-static-dynamic-va)
	NextCustomerNoSequence(ctx context.Context, vaType string) (string, error)
	RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error
	// FindVAInstalment looks up an already-recorded variable-bill instalment
	// by paymentRequestId, returning the transaction it belongs to and the
	// cumulative amount paid against that transaction. found is false when no
	// such instalment exists.
	//
	// Instalments live in their own table, not in va_transactions, so the
	// GetPayment idempotency check cannot see them. Without this lookup a
	// replayed instalment whose bill has since settled is answered "Paid
	// Bill" instead of being replayed.
	FindVAInstalment(ctx context.Context, paymentRequestID string) (transactionID string, cumulativePaid string, found bool, err error)
	// SaveVAPayment records one instalment against a variable-bill VA and
	// returns the cumulative total and resulting transaction status.
	// paymentRequestID is the dedup key: recorded is false when an instalment
	// with that id was already on file, so the caller replays the outcome
	// instead of crediting the same money twice.
	SaveVAPayment(ctx context.Context, transactionID, paymentRequestID, amount, referenceNo string) (paidAmount string, status string, recorded bool, err error)
	// VA registry methods (feature 013-no-bill-payment-transaction)
	SaveVAAccount(ctx context.Context, account *VAAccount) error
	GetVAAccount(ctx context.Context, virtualAccountNo string) (*VAAccount, error)
	GetVAAccountByPartnerAndCustomer(ctx context.Context, partnerServiceID, customerNo string) (*VAAccount, error)
	UpdateVAAccountStatus(ctx context.Context, virtualAccountNo string, status string) error
	ListVAAccounts(ctx context.Context, filter *VAAccountListFilter) ([]VAAccountListItem, int, error)
	ListVATransactions(ctx context.Context, filter *VAListFilter) ([]VATransactionListItem, int, error)
	SaveNoBillPayment(ctx context.Context, payment *VAPaymentRecord) error
	// FindPaymentFlag returns the recorded outcome of an earlier payment-flag
	// request with this exact (X-EXTERNAL-ID, paymentRequestId) pair, or nil
	// when none exists. This is the lookup BCA's double-flagging rule needs and
	// the paymentRequestId-keyed lookups above cannot serve: they only see
	// payments that SUCCEEDED, so a re-flag of a rejected payment found nothing
	// and was recomputed as a fresh rejection.
	FindPaymentFlag(ctx context.Context, externalID, paymentRequestID string) (*VAPaymentFlag, error)
	// RecordPaymentFlag stores the outcome of a payment-flag request, accepted
	// or rejected. Idempotent on (externalID, paymentRequestID): a second write
	// for the same pair is discarded, leaving the FIRST request's verdict on
	// file — the one the spec says a double flag must echo.
	RecordPaymentFlag(ctx context.Context, flag *VAPaymentFlag) error
}

// VAPaymentFlag is the recorded outcome of one payment-flag request, written
// for every request that reaches the usecase whether it settled or was
// rejected. It exists so a re-flag of the same (X-EXTERNAL-ID,
// paymentRequestId) can be answered 4042518 "Inconsistent Request" carrying
// the paymentFlagStatus and paymentFlagReason of the first request, including
// the case where that first request was itself a rejection ("01").
type VAPaymentFlag struct {
	ID               string
	ExternalID       string
	PaymentRequestID string
	VirtualAccountNo string
	ResponseCode     string
	ResponseMessage  string
	// VirtualAccountData is the block the vendor received the first time,
	// stored whole rather than reassembled from parts so the replay cannot
	// drift from the original answer.
	VirtualAccountData *VAPaymentStatus
	CreatedAt          time.Time
}

// VA Registry (feature 013-no-bill-payment-transaction)

// VAAccountStatus values for VAAccount.Status.
const (
	// VAAccountStatusActive means the VA accepts inquiries and payments.
	VAAccountStatusActive = "ACTIVE"
	// VAAccountStatusInactive means the merchant deactivated the VA via
	// delete-VA. Historical transactions stay readable; new payments are
	// rejected.
	VAAccountStatusInactive = "INACTIVE"
	// VAAccountStatusExpired means ExpiredDate passed and the transition was
	// detected inline during an inquiry or payment.
	VAAccountStatusExpired = "EXPIRED"
)

// VAAccount is the durable identity of a virtual account number — one row per
// virtualAccountNo, written once at /create-va and never by inquiry or
// payment.
//
// This is the half of the old va_transactions row that was NOT the
// transaction. Splitting it out is what lets a no-bill VA be registered once
// and paid many times (feature 013-no-bill-payment-transaction): the VA number
// is now a durable payment address rather than a single pending transaction
// that the first payment consumes.
//
// For no-bill VA types it carries no amount at all — the customer chooses the
// amount at the payment channel, exactly like an e-wallet top-up address.
type VAAccount struct {
	ID               string
	PartnerServiceID string
	CustomerNo       string
	VirtualAccountNo string
	// VAType is the 2-digit code (01-06) classifying this VA per the
	// master_va_type master data (feature 006-static-dynamic-va).
	VAType string
	// Billing is resolved from master data at registration time and stored
	// with the registration, so the payment and inquiry hot paths can ask "is
	// this a no-bill VA?" without a master-data lookup, and so a VA number
	// already published to a customer keeps the contract it was issued under
	// even if an operator later edits master_va_type.
	Billing         VATypeBilling
	CustomerName    string
	CustomerEmail   string
	CustomerPhone   string
	TrxID           string
	NotificationURL string
	// Status is one of VAAccountStatusActive/Inactive/Expired.
	Status string
	// ExpiredDate nil means the registration never expires.
	ExpiredDate *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsNoBill reports whether this VA carries no bill — the classification that
// routes it to register-once/pay-many handling. Keyed off Billing rather than
// a literal vaType check so operator-added no-bill types work with no code
// change (Constitution II).
func (a *VAAccount) IsNoBill() bool {
	return a != nil && a.Billing == VATypeBillingNone
}

// IsExpired reports whether the registration has passed its expiry date, or
// was already transitioned to EXPIRED by an earlier inquiry/payment. A
// registration with no ExpiredDate never expires.
func (a *VAAccount) IsExpired(now time.Time) bool {
	if a == nil {
		return false
	}
	if a.Status == VAAccountStatusExpired {
		return true
	}
	return a.ExpiredDate != nil && now.After(*a.ExpiredDate)
}

// VAAccountListFilter contains filter criteria for the merchant VA registry
// listing. Status filters on the registration state (ACTIVE/INACTIVE/EXPIRED),
// not the transaction state.
type VAAccountListFilter struct {
	PartnerServiceID string
	FromDate         *time.Time
	ToDate           *time.Time
	Status           string
	VirtualAccountNo string
	Offset           int
	Limit            int
}

// VAAccountListItem is one registered VA number in the merchant listing.
// TransactionCount and TotalPaid aggregate that VA number's settled
// transactions, so a no-bill VA with N top-ups shows as 1 VA with N
// transactions rather than N separate VAs.
type VAAccountListItem struct {
	VirtualAccountNo string     `json:"virtualAccountNo"`
	CustomerNo       string     `json:"customerNo"`
	CustomerName     string     `json:"customerName"`
	VAType           string     `json:"vaType"`
	Status           string     `json:"status"`
	ExpiredDate      *time.Time `json:"expiredDate"`
	CreatedAt        time.Time  `json:"createdAt"`
	TransactionCount int        `json:"transactionCount"`
	TotalPaid        *Amount    `json:"totalPaid,omitempty"`
}

// VATransactionListItem is one payment/transaction event in the merchant
// transaction listing — the per-payment view that complements
// VAAccountListItem's per-VA view.
type VATransactionListItem struct {
	VirtualAccountNo string     `json:"virtualAccountNo"`
	CustomerNo       string     `json:"customerNo"`
	CustomerName     string     `json:"customerName"`
	PaymentRequestID string     `json:"paymentRequestId,omitempty"`
	ReferenceNo      string     `json:"referenceNo,omitempty"`
	PaidAmount       *Amount    `json:"paidAmount,omitempty"`
	TotalAmount      *Amount    `json:"totalAmount"`
	Status           string     `json:"status"`
	TransactionDate  *time.Time `json:"transactionDate,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// VAInquiryRecord represents a persisted inquiry
type VAInquiryRecord struct {
	ID               string
	PartnerServiceID string
	CustomerNo       string
	CustomerName     string
	VirtualAccountNo string
	InquiryRequestID string
	TrxID            string
	NotificationURL  string
	Status           string
	TotalAmount      string
	Currency         string
	// VAType classifies the VA per feature 006-static-dynamic-va (01-06);
	// empty for VAs created before this feature or outside its partnerServiceId set.
	VAType string
	// SubCompany is the biller's registered sub-company code (ASPI
	// InquiryResponse.virtualAccountData.subCompany, maxLength 5). Persisted in
	// va_transactions.sub_company — written by the merchant at create-va time
	// (additionalInfo.subCompany) or by the vendor's payment notification — and
	// echoed back on inquiry. Empty when the biller has no sub-company, in which
	// case the field is omitted from the response.
	SubCompany string
	// PaidAmount is the cumulative amount settled against this transaction
	// (va_transactions.paid_amount, kept current by SaveVAPayment). "0" when
	// nothing has been paid. Needed to tell a variable-bill VA that is
	// genuinely lunas from one whose stored status merely says so.
	PaidAmount string
	// FreeTexts is the biller's two-language free text, shown on BCA's channel
	// screen (InquiryResponse.freeTexts). Persisted in va_transactions.free_texts
	// at create-va time and echoed on inquiry — before this it was only echoed
	// back in the create-va response and then dropped, so a merchant that set it
	// never saw it reach the channel.
	FreeTexts   []BilingualText
	ExpiredDate *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsVariableBill reports whether this VA settles cumulatively across several
// payments (vaType 02/05, "variable" in master_va_type) instead of in one
// exact-amount settlement.
func (r *VAInquiryRecord) IsVariableBill() bool {
	return r.VAType == "02" || r.VAType == "05"
}

// IsPayable reports whether this transaction can still accept a payment, and
// therefore whether an inquiry should report it as an open bill.
//
// '03' is the plain pending case; for a variable-bill VA it already means
// "belum lunas", since SaveVAPayment only flips the row to '00' once the
// cumulative total reaches total_amount.
//
// The second case covers a variable-bill row that reads '00' while its
// payments still fall short of total_amount — settled by status only. The
// outstanding balance must stay collectable rather than be locked out, so it
// counts as payable. Scoped to '00' on purpose: expired ('02') and deleted
// ('04') VAs are closed for reasons no amount comparison may reopen.
//
// Amounts that will not parse fall back to the stored status, so bad data
// cannot silently reopen a closed bill.
func (r *VAInquiryRecord) IsPayable() bool {
	if r.Status == "03" {
		return true
	}
	if r.Status != "00" || !r.IsVariableBill() {
		return false
	}
	paid, errPaid := strconv.ParseFloat(r.PaidAmount, 64)
	total, errTotal := strconv.ParseFloat(r.TotalAmount, 64)
	if errPaid != nil || errTotal != nil {
		return false
	}
	return paid < total
}

// VAPaymentRecord represents a persisted payment
type VAPaymentRecord struct {
	ID               string
	PartnerServiceID string
	CustomerNo       string
	CustomerName     string
	CustomerEmail    string
	CustomerPhone    string
	VirtualAccountNo string
	InquiryRequestID string
	TrxID            string
	NotificationURL  string
	PaymentRequestID string
	PaidAmount       string
	TotalAmount      string
	Currency         string
	Status           string
	// VAType classifies the VA this payment belongs to (01-06), copied from
	// the VA registration on the no-bill payment path (feature
	// 013-no-bill-payment-transaction) so a transaction row remains
	// self-describing. Empty on the legacy upsert path.
	VAType                string
	ReferenceNo           string
	ChannelCode           int
	HashedSourceAccountNo string
	SourceBankCode        string
	JournalNum            string
	PaymentType           string
	FlagAdvise            string
	PaidBills             string
	SubCompany            string
	TrxDateTime           *time.Time
	FreeTexts             []BilingualText
	TransactionDate       time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// VA Gateway Interface

// VAGateway defines outbound vendor VA API operations
type VAGateway interface {
	Inquiry(ctx context.Context, req *VAInquiryRequest) (*VAInquiryResponse, error)
	PaymentStatus(ctx context.Context, req *VAStatusRequest) (*VAStatusResponse, error)
}

// NotificationEnqueuer schedules an async callback to the merchant's
// notificationUrl after a payment is received.
type NotificationEnqueuer interface {
	EnqueuePaymentNotification(ctx context.Context, payload *PaymentNotificationPayload) error
}

// VA Usecase Interface

// VAUsecase defines VA business operations
type VAUsecase interface {
	Inquiry(ctx context.Context, req *VAInquiryRequest) (*VAInquiryResponse, error)
	Payment(ctx context.Context, req *VAPaymentRequest) (*VAPaymentResponse, error)
	Status(ctx context.Context, req *VAStatusRequest) (*VAStatusResponse, error)
}

// Merchant VA Types (ASPI OpenAPI aligned)

// MerchantVAUsecase defines merchant-side VA operations
type MerchantVAUsecase interface {
	CreateVA(ctx context.Context, req *MerchantCreateVARequest) (*MerchantCreateVAResponse, error)
	// ListVA lists registered VA numbers — one entry per VA. Since feature
	// 013-no-bill-payment-transaction this reads the VA registry rather than
	// the transaction table, so a no-bill VA paid ten times is one entry with
	// a transaction count of ten, not ten separate VAs (FR-023).
	ListVA(ctx context.Context, req *MerchantListVARequest) (*MerchantListVAResponse, error)
	// ListTransactions lists individual payment events — the per-payment view
	// that complements ListVA's per-VA view.
	ListTransactions(ctx context.Context, req *MerchantListVARequest) (*MerchantListTransactionsResponse, error)
	DeleteVA(ctx context.Context, req *MerchantDeleteVARequest) (*MerchantDeleteVAResponse, error)
}

// MerchantCreateVARequest maps to ASPI VAUpsertRequest (Service Code 27)
type MerchantCreateVARequest struct {
	PartnerServiceID      string          `json:"partnerServiceId"`
	CustomerNo            string          `json:"customerNo"`
	VirtualAccountNo      string          `json:"virtualAccountNo"`
	VirtualAccountName    string          `json:"virtualAccountName"`
	VirtualAccountEmail   string          `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone   string          `json:"virtualAccountPhone,omitempty"`
	TrxID                 string          `json:"trxId"`
	TotalAmount           *Amount         `json:"totalAmount,omitempty"`
	BillDetails           []BillDetail    `json:"billDetails,omitempty"`
	FreeTexts             []BilingualText `json:"freeTexts,omitempty"`
	VirtualAccountTrxType string          `json:"virtualAccountTrxType,omitempty"`
	FeeAmount             *Amount         `json:"feeAmount,omitempty"`
	ExpiredDate           *time.Time      `json:"expiredDate,omitempty"`
	// AdditionalInfo carries proprietary extensions per ASPI VAUpsertRequest's
	// additionalInfo.dbUrlProcess slot (aspi-open-api-va.yaml:317-320) — the
	// merchant payment callback URL is passed here as additionalInfo.dbUrlProcess,
	// not as a top-level field (there is no such field in the spec).
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// MerchantCreateVAResponse maps to ASPI VAUpsertResponse
type MerchantCreateVAResponse struct {
	ResponseCode       string          `json:"responseCode"`
	ResponseMessage    string          `json:"responseMessage"`
	VirtualAccountData *MerchantVAData `json:"virtualAccountData,omitempty"`
}

// MerchantVAData maps to VAUpsertResponse.virtualAccountData
type MerchantVAData struct {
	PartnerServiceID      string          `json:"partnerServiceId"`
	CustomerNo            string          `json:"customerNo"`
	VirtualAccountNo      string          `json:"virtualAccountNo"`
	VirtualAccountName    string          `json:"virtualAccountName"`
	VirtualAccountEmail   string          `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone   string          `json:"virtualAccountPhone,omitempty"`
	TrxID                 string          `json:"trxId"`
	TotalAmount           *Amount         `json:"totalAmount,omitempty"`
	BillDetails           []BillDetail    `json:"billDetails,omitempty"`
	FreeTexts             []BilingualText `json:"freeTexts,omitempty"`
	VirtualAccountTrxType string          `json:"virtualAccountTrxType,omitempty"`
	FeeAmount             *Amount         `json:"feeAmount,omitempty"`
	ExpiredDate           *time.Time      `json:"expiredDate,omitempty"`
	// lastUpdateDate is deliberately absent: per the ASPI portal it belongs to
	// the update-va / update-status / inquiry-va responses, NOT to create-va
	// (service code 27), which is the only response this struct serves. The
	// local aspi-open-api-va.yaml carries it only because it models create and
	// update with one shared VAUpsertResponse schema.
	PaymentDate *time.Time `json:"paymentDate,omitempty"`
	// AdditionalInfo echoes back additionalInfo.dbUrlProcess per ASPI
	// VAUpsertResponse (aspi-open-api-va.yaml:348-351).
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// MerchantDeleteVARequest maps to ASPI DeleteVARequest (Service Code 31)
type MerchantDeleteVARequest struct {
	PartnerServiceID string                 `json:"partnerServiceId"`
	CustomerNo       string                 `json:"customerNo"`
	VirtualAccountNo string                 `json:"virtualAccountNo"`
	TrxID            string                 `json:"trxId,omitempty"`
	AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}

// MerchantDeleteVAResponse maps to ASPI DeleteVAResponse
type MerchantDeleteVAResponse struct {
	ResponseCode       string                `json:"responseCode"`
	ResponseMessage    string                `json:"responseMessage"`
	VirtualAccountData *MerchantDeleteVAData `json:"virtualAccountData,omitempty"`
}

// MerchantDeleteVAData contains delete confirmation data
type MerchantDeleteVAData struct {
	PartnerServiceID string                 `json:"partnerServiceId"`
	CustomerNo       string                 `json:"customerNo"`
	VirtualAccountNo string                 `json:"virtualAccountNo"`
	TrxID            string                 `json:"trxId,omitempty"`
	AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}

// MerchantListVARequest represents merchant's request to list VA transactions
type MerchantListVARequest struct {
	PartnerServiceID string     `json:"partnerServiceId"`
	FromDate         *time.Time `json:"fromDate,omitempty"`
	ToDate           *time.Time `json:"toDate,omitempty"`
	Status           string     `json:"status,omitempty"`
	VirtualAccountNo string     `json:"virtualAccountNo,omitempty"`
	Page             int        `json:"page"`
	PageSize         int        `json:"pageSize"`
}

// MerchantListVAResponse represents a paginated list of registered VA numbers.
//
// Since feature 013-no-bill-payment-transaction, Data holds one entry per
// registered VA (VAAccountListItem) rather than one per transaction. The
// per-transaction view moved to MerchantListTransactionsResponse.
type MerchantListVAResponse struct {
	ResponseCode    string              `json:"responseCode"`
	ResponseMessage string              `json:"responseMessage"`
	Data            []VAAccountListItem `json:"data,omitempty"`
	Pagination      *Pagination         `json:"pagination,omitempty"`
}

// MerchantListTransactionsResponse represents a paginated list of individual
// payment/transaction events (feature 013-no-bill-payment-transaction,
// FR-023). This is where a no-bill VA's N top-ups are visible.
type MerchantListTransactionsResponse struct {
	ResponseCode    string                  `json:"responseCode"`
	ResponseMessage string                  `json:"responseMessage"`
	Data            []VATransactionListItem `json:"data,omitempty"`
	Pagination      *Pagination             `json:"pagination,omitempty"`
}

// Pagination contains list pagination metadata
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalRows  int `json:"totalRows"`
	TotalPages int `json:"totalPages"`
}

// VAListFilter contains filter criteria for VA list query
type VAListFilter struct {
	PartnerServiceID string
	FromDate         *time.Time
	ToDate           *time.Time
	Status           string
	VirtualAccountNo string
	Offset           int
	Limit            int
}

// AsynqEnqueuer defines the interface for enqueueing async tasks
type AsynqEnqueuer interface {
	EnqueuePaymentNotification(ctx context.Context, payload *PaymentNotificationPayload) error
}

// Static/Dynamic VA Type Rules (feature 006-static-dynamic-va)

// VATypeBilling classifies how a VA's amount is determined.
type VATypeBilling string

const (
	VATypeBillingNone     VATypeBilling = "none"     // no bill: amount determined at inquiry/payment time
	VATypeBillingVariable VATypeBilling = "variable" // variable bill: cumulative target, multiple payments allowed
	VATypeBillingFixed    VATypeBilling = "fixed"    // fixed bill: single fixed amount set at creation
)

// IsPlaceholderInquiryRequestID reports whether a transaction's stored
// inquiry_request_id is a create-va placeholder rather than a vendor's real
// id, and may therefore be claimed by the first inquiry that arrives.
//
// Three shapes are placeholders, one per generation of the create-va writer:
// "" (before the UNIQUE constraint on the column made that unusable for more
// than one VA), a copy of trxId, and the VA number itself (current).
func IsPlaceholderInquiryRequestID(inquiryRequestID, trxID, virtualAccountNo string) bool {
	return inquiryRequestID == "" ||
		(trxID != "" && inquiryRequestID == trxID) ||
		inquiryRequestID == virtualAccountNo
}

// BillingForVAType classifies a bare vaType code when no va_accounts
// registration is available to read Billing from — i.e. transactions created
// before feature 013's registry existed. The seeded master_va_type mapping is
// mirrored here (01/04 none, 02/05 variable, 03/06 fixed).
//
// This is the FALLBACK only. When a registration exists, its stored Billing is
// authoritative: it was resolved from master data at create-va time, so a VA
// keeps the contract it was issued under, and operator-added VA types work
// with no code change.
func BillingForVAType(vaType string) VATypeBilling {
	switch vaType {
	case "01", "04":
		return VATypeBillingNone
	case "02", "05":
		return VATypeBillingVariable
	case "03", "06":
		return VATypeBillingFixed
	default:
		// An unknown or empty vaType predates the classification entirely.
		// Treating it as fixed would impose an exact-amount check that such a
		// VA was never issued under, so it is left unclassified.
		return ""
	}
}

// VATypeRule describes one of the recognized partnerServiceId +
// additionalInfo.vaType combinations, sourced from the master_va_type table
// (feature 006-static-dynamic-va amendment — previously a hardcoded map).
type VATypeRule struct {
	VAType           string
	PartnerServiceID string
	Dynamic          bool
	Billing          VATypeBilling
	Description      string
}

// PartnerServiceIDRecord mirrors a master_partner_service_ids row.
type PartnerServiceIDRecord struct {
	PartnerServiceID string
	BankCode         string
}

// VATypeRuleProvider resolves VA type routing rules and the reserved
// partnerServiceId set. Defined here (consumer: MerchantVAUsecase) per
// Constitution I; implemented in internal/infrastructure/cache by a
// PostgreSQL-backed, Redis-cached provider with an in-process fallback so
// this data can be edited by operators without a code deployment (FR-015..FR-019).
type VATypeRuleProvider interface {
	// LookupVATypeRule resolves the rule for a given partnerServiceId/vaType
	// pair. ok is false if vaType is unrecognized or does not match
	// partnerServiceId. err is non-nil only on a data-source failure (e.g.
	// cache and database both unavailable), not on a not-found lookup.
	LookupVATypeRule(ctx context.Context, partnerServiceID, vaType string) (rule VATypeRule, ok bool, err error)
	// IsReservedPartnerServiceID reports whether partnerServiceId belongs to
	// the static/dynamic VA type feature's reserved set.
	IsReservedPartnerServiceID(ctx context.Context, partnerServiceID string) (bool, error)
}

// MasterDataRepository defines PostgreSQL CRUD/List operations for the VA
// type / partner service ID master data tables (feature 006-static-dynamic-va
// amendment). Mutation methods are expected to trigger an immediate cache
// refresh (FR-017) from their caller.
type MasterDataRepository interface {
	ListVATypes(ctx context.Context) ([]VATypeRule, error)
	ListPartnerServiceIDs(ctx context.Context) ([]PartnerServiceIDRecord, error)
	CreateVAType(ctx context.Context, rule VATypeRule) error
	UpdateVAType(ctx context.Context, rule VATypeRule) error
	DeleteVAType(ctx context.Context, vaType string) error
	CreatePartnerServiceID(ctx context.Context, record PartnerServiceIDRecord) error
	UpdatePartnerServiceID(ctx context.Context, record PartnerServiceIDRecord) error
	DeletePartnerServiceID(ctx context.Context, partnerServiceID string) error
}

// PaymentNotificationPayload maps to ASPI PaymentRequest (Service Code 25),
// extended (feature 007-merchant-expiry-callback) with an EventType
// discriminator so the same enqueue/worker/signing path serves both
// "payment.received" (existing) and "va.expired" (new) events.
type PaymentNotificationPayload struct {
	// EventType distinguishes "payment.received" (default, existing
	// behavior) from "va.expired" (feature 007-merchant-expiry-callback).
	EventType               string  `json:"eventType,omitempty"`
	PartnerServiceID        string  `json:"partnerServiceId"`
	CustomerNo              string  `json:"customerNo"`
	VirtualAccountNo        string  `json:"virtualAccountNo"`
	TrxID                   string  `json:"trxId,omitempty"`
	PaymentRequestID        string  `json:"paymentRequestId"`
	PaidAmount              *Amount `json:"paidAmount,omitempty"`
	CumulativePaymentAmount *Amount `json:"cumulativePaymentAmount,omitempty"`
	PaidBills               string  `json:"paidBills,omitempty"`
	TotalAmount             *Amount `json:"totalAmount,omitempty"`
	TrxDateTime             string  `json:"trxDateTime,omitempty"`
	ReferenceNo             string  `json:"referenceNo,omitempty"`
	JournalNum              string  `json:"journalNum,omitempty"`
	PaymentType             string  `json:"paymentType,omitempty"`
	FlagAdvise              string  `json:"flagAdvise,omitempty"`
	NotificationURL         string  `json:"notificationUrl"`
	// ExpiredAt carries the expiry timestamp (ISO 8601), populated only for
	// "va.expired" events (FR-003).
	ExpiredAt string `json:"expiredAt,omitempty"`
}

// Notification Delivery Attempt (feature 007-merchant-expiry-callback)

// Event type / trigger constants for va_notification_deliveries and
// PaymentNotificationPayload.EventType.
const (
	NotificationEventPaymentReceived = "payment.received"
	NotificationEventVAExpired       = "va.expired"

	NotificationTriggerAuto   = "auto"
	NotificationTriggerManual = "manual"

	NotificationDeliveryStatusSuccess = "success"
	NotificationDeliveryStatusFailed  = "failed"
)

// NotificationDelivery represents a single attempt (auto or manual) to
// deliver a merchant callback event, persisted in va_notification_deliveries
// for audit (FR-006/FR-018/SC-007) and dedupe (FR-005) purposes.
type NotificationDelivery struct {
	ID               string
	VirtualAccountNo string
	EventType        string // "payment.received" | "va.expired"
	Trigger          string // "auto" | "manual"
	Status           string // "success" | "failed"
	AttemptedAt      time.Time
	ErrorDetail      string
}

// VANotificationDeliveryRepository defines persistence operations for the
// notification delivery-attempt audit trail.
type VANotificationDeliveryRepository interface {
	Create(ctx context.Context, delivery *NotificationDelivery) error
	GetLatestByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*NotificationDelivery, error)
	ExistsByVirtualAccountNoAndEventType(ctx context.Context, virtualAccountNo, eventType, trigger string) (bool, error)
}

// ResendCallbackResult is returned by ResendCallbackUsecase.Resend on
// success, mapped 1:1 to the resend-callback.md 200 OK response body.
type ResendCallbackResult struct {
	VirtualAccountNo string
	EventType        string
	ResentAt         time.Time
	DeliveryStatus   string // "success" | "failed"
}

// ResendCallbackUsecase defines the admin-only manual callback resend
// operation (feature 007-merchant-expiry-callback, US2).
type ResendCallbackUsecase interface {
	Resend(ctx context.Context, virtualAccountNo string) (*ResendCallbackResult, error)
}
