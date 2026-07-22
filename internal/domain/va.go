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

// BillDetail represents a single bill item
type BillDetail struct {
	BillNo          string                 `json:"billNo"`
	BillDescription *BilingualText         `json:"billDescription,omitempty"`
	BillSubCompany  string                 `json:"billSubCompany,omitempty"`
	BillAmount      *Amount                `json:"billAmount,omitempty"`
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
