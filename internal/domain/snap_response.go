package domain

// SNAP error envelopes.
//
// BCA marks virtualAccountData as Mandatory on every transfer-va response, and
// states that a response whose status/reason fields are empty is treated as a
// failed transaction. A bare {responseCode, responseMessage} is therefore not
// a valid rejection — it is an unparseable one. These builders produce the
// full envelope for each service so no error path can accidentally omit it.
//
// The 401 cases are the documented exception: BCA's Appendix A tables show "-"
// in the status column for "Unauthorized" and "Invalid Token (B2B)", so those
// carry no virtualAccountData. Callers signal that by using the bare
// SNAPErrorResponse below.

// SNAPErrorResponse is the body used for authentication failures on the
// transfer-va services.
//
// It carries a `data` object as well as the code and message. BCA API OAuth &
// Signature v1.1 shows the HMAC-mismatch response as
//
//	{"responseCode": "401xx00", "responseMessage": "Unauthorized. [Signature]", "data": {}}
//
// and notes "xx -> customize to each service code" — so the shape belongs to
// every transfer-va 401, not only to the access-token endpoint. The object is
// always empty; it exists so a client that dereferences `data` unconditionally
// does not fault on a rejection.
type SNAPErrorResponse struct {
	ResponseCode    string         `json:"responseCode"`
	ResponseMessage string         `json:"responseMessage"`
	Data            map[string]any `json:"data"`
}

// NewSNAPErrorResponse builds the SNAP authentication-failure body.
func NewSNAPErrorResponse(code, message string) SNAPErrorResponse {
	return SNAPErrorResponse{ResponseCode: code, ResponseMessage: message, Data: map[string]any{}}
}

// VAIdentityEcho carries the request identity fields a rejection echoes back.
// All fields are optional: a body that failed to parse has none of them, and
// BCA accepts empty strings there (its own reference responses use them).
type VAIdentityEcho struct {
	PartnerServiceID string
	CustomerNo       string
	VirtualAccountNo string
	// InquiryRequestID is echoed by inquiry and status; PaymentRequestID by
	// payment. Only the one belonging to the service is used.
	InquiryRequestID string
	PaymentRequestID string
}

// NewInquiryErrorResponse builds a rejected InquiryResponse (service 24) with
// inquiryStatus "01" and the bilingual reason for the code.
func NewInquiryErrorResponse(code, message string, echo VAIdentityEcho) VAInquiryResponse {
	return VAInquiryResponse{
		ResponseCode:    code,
		ResponseMessage: message,
		VirtualAccountData: &VAAccountData{
			InquiryStatus:    FlagStatusForCode(code),
			InquiryReason:    ReasonForCode(code),
			PartnerServiceID: echo.PartnerServiceID,
			CustomerNo:       echo.CustomerNo,
			VirtualAccountNo: echo.VirtualAccountNo,
			InquiryRequestID: echo.InquiryRequestID,
			TotalAmount:      &Amount{},
		},
	}
}

// NewPaymentErrorResponse builds a rejected PaymentResponse (service 25) with
// paymentFlagStatus "01" and the bilingual reason for the code.
func NewPaymentErrorResponse(code, message string, echo VAIdentityEcho) VAPaymentResponse {
	return VAPaymentResponse{
		ResponseCode:    code,
		ResponseMessage: message,
		VirtualAccountData: &VAPaymentStatus{
			PaymentFlagStatus: FlagStatusForCode(code),
			PaymentFlagReason: ReasonForCode(code),
			PartnerServiceID:  echo.PartnerServiceID,
			CustomerNo:        echo.CustomerNo,
			VirtualAccountNo:  echo.VirtualAccountNo,
			PaymentRequestID:  echo.PaymentRequestID,
			PaidAmount:        &Amount{},
			TotalAmount:       &Amount{},
		},
	}
}

// NewStatusErrorResponse builds a rejected StatusResponse (service 26).
func NewStatusErrorResponse(code, message string, echo VAIdentityEcho) VAStatusResponse {
	return VAStatusResponse{
		ResponseCode:    code,
		ResponseMessage: message,
		VirtualAccountData: &VAStatusData{
			PaymentFlagStatus: FlagStatusForCode(code),
			PaymentFlagReason: ReasonForCode(code),
			PartnerServiceID:  echo.PartnerServiceID,
			CustomerNo:        echo.CustomerNo,
			VirtualAccountNo:  echo.VirtualAccountNo,
			InquiryRequestID:  echo.InquiryRequestID,
			PaidAmount:        &Amount{},
			TotalAmount:       &Amount{},
		},
	}
}

// NewSNAPErrorBody returns the correctly-shaped rejection body for whichever
// transfer-va service the path belongs to. Used by middleware, which rejects
// before any handler runs and so must pick the shape from the path alone.
func NewSNAPErrorBody(service, code, message string, echo VAIdentityEcho) any {
	switch service {
	case ServiceCodeInquiry:
		return NewInquiryErrorResponse(code, message, echo)
	case ServiceCodePayment:
		return NewPaymentErrorResponse(code, message, echo)
	case ServiceCodeStatus:
		return NewStatusErrorResponse(code, message, echo)
	default:
		return NewSNAPErrorResponse(code, message)
	}
}
