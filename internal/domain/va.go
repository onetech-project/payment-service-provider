package domain

import (
	"context"
	"time"
)

// VA Inquiry Request/Response types

// VAInquiryRequest represents inbound inquiry from vendor
type VAInquiryRequest struct {
	PartnerServiceID string                 `json:"partnerServiceId"`
	CustomerNo       string                 `json:"customerNo"`
	VirtualAccountNo string                 `json:"virtualAccountNo"`
	TrxDateInit      *time.Time             `json:"txnDateInit,omitempty"`
	Amount           *Amount                `json:"amount"`
	ChannelCode      int                    `json:"channelCode,omitempty"`
	InquiryRequestID string                 `json:"inquiryRequestId"`
	AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VAInquiryResponse represents response to vendor inquiry
type VAInquiryResponse struct {
	ResponseCode       string         `json:"responseCode"`
	ResponseMessage    string         `json:"responseMessage"`
	VirtualAccountData *VAAccountData `json:"virtualAccountData,omitempty"`
}

// VAAccountData contains VA account and bill information
type VAAccountData struct {
	InquiryStatus         string                 `json:"inquiryStatus"`
	InquiryReason         *BilingualText         `json:"inquiryReason,omitempty"`
	PartnerServiceID      string                 `json:"partnerServiceId"`
	CustomerNo            string                 `json:"customerNo"`
	VirtualAccountNo      string                 `json:"virtualAccountNo"`
	VirtualAccountName    string                 `json:"virtualAccountName"`
	InquiryRequestID      string                 `json:"inquiryRequestId"`
	TotalAmount           *Amount                `json:"totalAmount,omitempty"`
	SubCompany            string                 `json:"subCompany,omitempty"`
	BillDetails           []BillDetail           `json:"billDetails,omitempty"`
	FreeTexts             []BilingualText        `json:"freeTexts,omitempty"`
	VirtualAccountTrxType string                 `json:"virtualAccountTrxType,omitempty"`
	FeeAmount             *Amount                `json:"feeAmount,omitempty"`
	AdditionalInfo        map[string]interface{} `json:"additionalInfo,omitempty"`
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

// VAPaymentResponse represents response to vendor payment notification
type VAPaymentResponse struct {
	ResponseCode       string           `json:"responseCode"`
	ResponseMessage    string           `json:"responseMessage"`
	VirtualAccountData *VAPaymentStatus `json:"virtualAccountData,omitempty"`
}

// VAPaymentStatus contains payment flag status and echoes back the
// identity/amount fields from PaymentResponse.virtualAccountData per ASPI spec.
type VAPaymentStatus struct {
	PartnerServiceID    string                `json:"partnerServiceId"`
	CustomerNo          string                `json:"customerNo"`
	VirtualAccountNo    string                `json:"virtualAccountNo"`
	VirtualAccountName  string                `json:"virtualAccountName,omitempty"`
	VirtualAccountEmail string                `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone string                `json:"virtualAccountPhone,omitempty"`
	TrxID               string                `json:"trxId,omitempty"`
	PaymentRequestID    string                `json:"paymentRequestId"`
	PaidAmount          *Amount               `json:"paidAmount"`
	PaidBills           string                `json:"paidBills,omitempty"`
	TotalAmount         *Amount               `json:"totalAmount,omitempty"`
	TrxDateTime         *time.Time            `json:"trxDateTime,omitempty"`
	ReferenceNo         string                `json:"referenceNo,omitempty"`
	JournalNum          string                `json:"journalNum,omitempty"`
	PaymentType         string                `json:"paymentType,omitempty"`
	FlagAdvise          string                `json:"flagAdvise,omitempty"`
	PaymentFlagStatus   string                `json:"paymentFlagStatus"`
	PaymentFlagReason   *BilingualText        `json:"paymentFlagReason"`
	BillDetails         []VAPaymentBillDetail `json:"billDetails,omitempty"`
	FreeTexts           []BilingualText       `json:"freeTexts,omitempty"`
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
	ResponseCode       string        `json:"responseCode"`
	ResponseMessage    string        `json:"responseMessage"`
	VirtualAccountData *VAStatusData `json:"virtualAccountData,omitempty"`
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
	GetPayment(ctx context.Context, paymentRequestID string) (*VAPaymentRecord, error)
	UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error
	// Merchant dashboard methods
	ListVA(ctx context.Context, filter *VAListFilter) ([]VAListItem, int, error)
	GetVABillDetails(ctx context.Context, transactionID string) ([]BillDetail, error)
	SaveBillDetails(ctx context.Context, transactionID string, bills []BillDetail) error
	UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error
	GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*VAInquiryRecord, error)
	// Static/Dynamic VA methods (feature 006-static-dynamic-va)
	NextCustomerNoSequence(ctx context.Context, vaType string) (string, error)
	RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error
	SaveVAPayment(ctx context.Context, transactionID string, amount string, referenceNo string) (paidAmount string, status string, err error)
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
	SubCompany  string
	ExpiredDate *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// VAPaymentRecord represents a persisted payment
type VAPaymentRecord struct {
	ID                    string
	PartnerServiceID      string
	CustomerNo            string
	CustomerName          string
	CustomerEmail         string
	CustomerPhone         string
	VirtualAccountNo      string
	InquiryRequestID      string
	TrxID                 string
	NotificationURL       string
	PaymentRequestID      string
	PaidAmount            string
	TotalAmount           string
	Currency              string
	Status                string
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
	ListVA(ctx context.Context, req *MerchantListVARequest) (*MerchantListVAResponse, error)
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

// MerchantListVAResponse represents paginated VA transaction list
type MerchantListVAResponse struct {
	ResponseCode    string       `json:"responseCode"`
	ResponseMessage string       `json:"responseMessage"`
	Data            []VAListItem `json:"data,omitempty"`
	Pagination      *Pagination  `json:"pagination,omitempty"`
}

// VAListItem represents a single VA transaction in list
type VAListItem struct {
	VirtualAccountNo string     `json:"virtualAccountNo"`
	CustomerNo       string     `json:"customerNo"`
	CustomerName     string     `json:"customerName"`
	TotalAmount      *Amount    `json:"totalAmount"`
	PaidAmount       *Amount    `json:"paidAmount,omitempty"`
	Status           string     `json:"status"`
	ExpiredDate      *time.Time `json:"expiredDate"`
	CreatedAt        time.Time  `json:"createdAt"`
	TransactionDate  *time.Time `json:"transactionDate,omitempty"`
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
