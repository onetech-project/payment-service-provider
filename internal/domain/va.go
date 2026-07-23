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
	TrxDateInit      *time.Time             `json:"trxDateInit,omitempty"`
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
	PartnerServiceID string                 `json:"partnerServiceId"`
	CustomerNo       string                 `json:"customerNo"`
	VirtualAccountNo string                 `json:"virtualAccountNo"`
	InquiryRequestID string                 `json:"inquiryRequestId"`
	PaymentRequestID string                 `json:"paymentRequestId"`
	PaidAmount       *Amount                `json:"paidAmount"`
	PaidBills        string                 `json:"paidBills,omitempty"`
	TotalAmount      *Amount                `json:"totalAmount"`
	TrxDateTime      *time.Time             `json:"trxDateTime,omitempty"`
	TransactionDate  *time.Time             `json:"transactionDate"`
	ReferenceNo      string                 `json:"referenceNo,omitempty"`
	PaymentType      string                 `json:"paymentType,omitempty"`
	FlagAdvise       string                 `json:"flagAdvise,omitempty"`
	BillDetails      []VAPaymentBillDetail  `json:"billDetails,omitempty"`
	FreeTexts        []BilingualText        `json:"freeTexts,omitempty"`
	AdditionalInfo   map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VAPaymentBillDetail extends BillDetail with payment-specific fields
type VAPaymentBillDetail struct {
	BillCode        string                 `json:"billCode,omitempty"`
	BillNo          string                 `json:"billNo"`
	BillName        string                 `json:"billName,omitempty"`
	BillShortName   string                 `json:"billShortName,omitempty"`
	BillDescription *BilingualText         `json:"billDescription"`
	BillSubCompany  string                 `json:"billSubCompany"`
	BillAmount      *Amount                `json:"billAmount"`
	AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
	BillReferenceNo string                 `json:"billReferenceNo"`
	Status          string                 `json:"status"`
	Reason          *BilingualText         `json:"reason"`
}

// VAPaymentResponse represents response to vendor payment notification
type VAPaymentResponse struct {
	ResponseCode       string           `json:"responseCode"`
	ResponseMessage    string           `json:"responseMessage"`
	VirtualAccountData *VAPaymentStatus `json:"virtualAccountData,omitempty"`
}

// VAPaymentStatus contains payment flag status
type VAPaymentStatus struct {
	PaymentFlagStatus string         `json:"paymentFlagStatus"`
	PaymentFlagReason *BilingualText `json:"paymentFlagReason"`
}

// VA Status Request/Response types

// VAStatusRequest represents inbound status inquiry from vendor
type VAStatusRequest struct {
	PartnerServiceID string                 `json:"partnerServiceId"`
	CustomerNo       string                 `json:"customerNo"`
	VirtualAccountNo string                 `json:"virtualAccountNo"`
	InquiryRequestID string                 `json:"inquiryRequestId"`
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
	BillCode        string                 `json:"billCode,omitempty"`
	BillNo          string                 `json:"billNo"`
	BillName        string                 `json:"billName,omitempty"`
	BillShortName   string                 `json:"billShortName,omitempty"`
	BillDescription *BilingualText         `json:"billDescription,omitempty"`
	BillSubCompany  string                 `json:"billSubCompany,omitempty"`
	BillAmount      *Amount                `json:"billAmount,omitempty"`
	BillAmountLabel string                 `json:"billAmountLabel,omitempty"`
	BillAmountValue string                 `json:"billAmountValue,omitempty"`
	BillReferenceNo string                 `json:"billReferenceNo,omitempty"`
	BillerReferenceID string               `json:"billerReferenceId,omitempty"`
	Status          string                 `json:"status,omitempty"`
	Reason          *BilingualText         `json:"reason,omitempty"`
	AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
}

// VA Repository Interface

// VARepository defines persistence operations for VA transactions
type VARepository interface {
	SaveInquiry(ctx context.Context, inquiry *VAInquiryRecord) error
	GetInquiry(ctx context.Context, inquiryRequestID string) (*VAInquiryRecord, error)
	SavePayment(ctx context.Context, payment *VAPaymentRecord) error
	GetPayment(ctx context.Context, paymentRequestID string) (*VAPaymentRecord, error)
	UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error
	// Merchant dashboard methods
	ListVA(ctx context.Context, filter *VAListFilter) ([]VAListItem, int, error)
	GetVABillDetails(ctx context.Context, transactionID string) ([]BillDetail, error)
	UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error
	GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*VAInquiryRecord, error)
}

// VAInquiryRecord represents a persisted inquiry
type VAInquiryRecord struct {
	ID               string
	PartnerServiceID string
	CustomerNo       string
	VirtualAccountNo string
	InquiryRequestID string
	Status           string
	TotalAmount      string
	Currency         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// VAPaymentRecord represents a persisted payment
type VAPaymentRecord struct {
	ID               string
	PartnerServiceID string
	CustomerNo       string
	VirtualAccountNo string
	InquiryRequestID string
	PaymentRequestID string
	PaidAmount       string
	Currency         string
	Status           string
	ReferenceNo      string
	TransactionDate  time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// VA Gateway Interface

// VAGateway defines outbound vendor VA API operations
type VAGateway interface {
	Inquiry(ctx context.Context, req *VAInquiryRequest) (*VAInquiryResponse, error)
	PaymentStatus(ctx context.Context, req *VAStatusRequest) (*VAStatusResponse, error)
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
	PartnerServiceID    string                 `json:"partnerServiceId"`
	CustomerNo          string                 `json:"customerNo"`
	VirtualAccountNo    string                 `json:"virtualAccountNo,omitempty"`
	VirtualAccountName  string                 `json:"virtualAccountName"`
	VirtualAccountEmail string                 `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone string                 `json:"virtualAccountPhone,omitempty"`
	TrxID               string                 `json:"trxId"`
	TotalAmount         *Amount                `json:"totalAmount,omitempty"`
	BillDetails         []BillDetail           `json:"billDetails,omitempty"`
	FreeTexts           []BilingualText        `json:"freeTexts,omitempty"`
	VirtualAccountTrxType string               `json:"virtualAccountTrxType,omitempty"`
	FeeAmount           *Amount                `json:"feeAmount,omitempty"`
	ExpiredDate         *time.Time             `json:"expiredDate,omitempty"`
	AdditionalInfo      map[string]interface{} `json:"additionalInfo,omitempty"`
	NotificationURL     string                 `json:"notificationUrl"`
}

// MerchantCreateVAResponse maps to ASPI VAUpsertResponse
type MerchantCreateVAResponse struct {
	ResponseCode       string           `json:"responseCode"`
	ResponseMessage    string           `json:"responseMessage"`
	VirtualAccountData *MerchantVAData  `json:"virtualAccountData,omitempty"`
}

// MerchantVAData maps to VAUpsertResponse.virtualAccountData
type MerchantVAData struct {
	PartnerServiceID    string                 `json:"partnerServiceId"`
	CustomerNo          string                 `json:"customerNo"`
	VirtualAccountNo    string                 `json:"virtualAccountNo"`
	VirtualAccountName  string                 `json:"virtualAccountName"`
	VirtualAccountEmail string                 `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone string                 `json:"virtualAccountPhone,omitempty"`
	TrxID               string                 `json:"trxId"`
	TotalAmount         *Amount                `json:"totalAmount,omitempty"`
	BillDetails         []BillDetail           `json:"billDetails,omitempty"`
	FreeTexts           []BilingualText        `json:"freeTexts,omitempty"`
	VirtualAccountTrxType string               `json:"virtualAccountTrxType,omitempty"`
	FeeAmount           *Amount                `json:"feeAmount,omitempty"`
	ExpiredDate         *time.Time             `json:"expiredDate,omitempty"`
	LastUpdateDate      *time.Time             `json:"lastUpdateDate,omitempty"`
	PaymentDate         *time.Time             `json:"paymentDate,omitempty"`
	AdditionalInfo      map[string]interface{} `json:"additionalInfo,omitempty"`
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
	ResponseCode       string               `json:"responseCode"`
	ResponseMessage    string               `json:"responseMessage"`
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

// PaymentNotificationPayload maps to ASPI PaymentRequest (Service Code 25)
type PaymentNotificationPayload struct {
	PartnerServiceID        string                 `json:"partnerServiceId"`
	CustomerNo              string                 `json:"customerNo"`
	VirtualAccountNo        string                 `json:"virtualAccountNo"`
	TrxID                   string                 `json:"trxId,omitempty"`
	PaymentRequestID        string                 `json:"paymentRequestId"`
	PaidAmount              *Amount                `json:"paidAmount"`
	CumulativePaymentAmount *Amount                `json:"cumulativePaymentAmount,omitempty"`
	PaidBills               string                 `json:"paidBills,omitempty"`
	TotalAmount             *Amount                `json:"totalAmount,omitempty"`
	TrxDateTime             string                 `json:"trxDateTime,omitempty"`
	ReferenceNo             string                 `json:"referenceNo,omitempty"`
	JournalNum              string                 `json:"journalNum,omitempty"`
	PaymentType             string                 `json:"paymentType,omitempty"`
	FlagAdvise              string                 `json:"flagAdvise,omitempty"`
	NotificationURL         string                 `json:"notificationUrl"`
}
