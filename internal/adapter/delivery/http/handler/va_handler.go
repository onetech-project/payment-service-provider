package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// headerExternalID is SNAP's per-request identifier. The middleware package
// keeps its own unexported copy for the idempotency key; this one exists
// because the payment handler must pass the value on to the usecase, where it
// forms half of BCA's double-flagging key.
const headerExternalID = "X-EXTERNAL-ID"

// VAHandler handles vendor Virtual Account HTTP requests
type VAHandler struct {
	vaUsecase domain.VAUsecase
	// strictMandatory enables the field set BCA marks Mandatory but the wider
	// SNAP standard leaves optional (virtualAccountName, channelCode,
	// totalAmount, trxDateTime, flagAdvise). Defaults to on via
	// NewVAHandler; NewVAHandlerWithOptions lets a deployment fronting a
	// non-BCA vendor relax it without changing every other vendor's contract.
	strictMandatory bool
}

// NewVAHandler creates a new VA handler with BCA-conformant strictness.
func NewVAHandler(vaUsecase domain.VAUsecase) *VAHandler {
	return &VAHandler{vaUsecase: vaUsecase, strictMandatory: true}
}

// NewVAHandlerWithOptions creates a VA handler with an explicit default
// strictness, used when no vendor config is resolved for the request.
func NewVAHandlerWithOptions(vaUsecase domain.VAUsecase, strictMandatory bool) *VAHandler {
	return &VAHandler{vaUsecase: vaUsecase, strictMandatory: strictMandatory}
}

// strictMandatoryFor resolves field strictness for THIS request from the
// vendor that authenticated it. One handler serves every vendor — the router
// cannot fan out per vendor, since echo would keep only the last route
// registered for a path — so the per-vendor contract is read from the context
// the auth middleware populated, falling back to the handler's own default
// when the route is not vendor-scoped.
func (h *VAHandler) strictMandatoryFor(c echo.Context) bool {
	if vendor, ok := c.Get(domain.ContextKeyVendor).(domain.VendorContext); ok {
		return vendor.StrictMandatoryFields
	}
	return h.strictMandatory
}

// Inquiry godoc
// @Tags Virtual Account
// @Summary VA bill inquiry
// @Description Vendor-initiated inquiry for Virtual Account bill/customer details prior to payment. Read-only. Mandatory body fields per BCA: partnerServiceId, customerNo, virtualAccountNo, inquiryRequestId. There is no `amount` field on this request.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b. Required for vendors onboarded with a VENDOR_CLIENT_ID; the token is also bound into the AccessToken component of stringToSign"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-PARTNER-ID header string true "Partner ID using Company Code VA, max 8 chars (BCA tech docs v2.3/v2.4). Value-checked whenever the vendor config sets VENDOR_PARTNER_ID"
// @Param X-EXTERNAL-ID header string true "Numeric string, max 36 chars, unique per calendar day. Doubles as the idempotency key"
// @Param CHANNEL-ID header string true "PJP channel id (BCA VA: 95231). Value-checked whenever the vendor config sets VENDOR_CHANNEL_ID"
// @Param request body domain.VAInquiryRequest true "VA inquiry request"
// @Success 200 {object} domain.VAInquiryResponse
// @Failure 400 {object} domain.VAInquiryResponse "4002400 Bad Request (unparseable body), 4002401 Invalid Field Format {field}, 4002402 Invalid Mandatory Field {field}"
// @Failure 401 {object} domain.SNAPErrorResponse "4012400 Unauthorized. [reason], 4012401 Invalid Token (B2B)"
// @Failure 404 {object} domain.VAInquiryResponse "All with virtualAccountData.inquiryStatus=01: 4042412 Invalid Bill/Virtual Account [Not Found], 4042414 Paid Bill, 4042419 Invalid Bill/Virtual Account (expired)"
// @Failure 409 {object} domain.VAInquiryResponse "4092400 Conflict — X-EXTERNAL-ID reused"
// @Failure 500 {object} domain.VAInquiryResponse "5002400 Internal Server Error"
// @Router /openapi/v1.0/transfer-va/inquiry [post]
func (h *VAHandler) Inquiry(c echo.Context) error {
	var req domain.VAInquiryRequest
	if err := c.Bind(&req); err != nil {
		// BCA distinguishes a body it could not parse ("Request Parsing Error"
		// → 4002400 Bad Request) from a parsed body with a bad field
		// (4002401/4002402). Collapsing them loses the distinction BCA's own
		// Appendix A draws.
		return c.JSON(http.StatusBadRequest, domain.NewInquiryErrorResponse(
			domain.CodeInquiryBadRequest, "Bad Request", domain.VAIdentityEcho{}))
	}

	echoData := inquiryEcho(&req)
	if v := domain.ValidateInquiryRequest(&req, h.strictMandatoryFor(c)); v != nil {
		code, message := violationCode(domain.ServiceCodeInquiry, v)
		return c.JSON(http.StatusBadRequest,
			domain.NewInquiryErrorResponse(code, message, echoData))
	}

	resp, err := h.vaUsecase.Inquiry(c.Request().Context(), &req)
	if err != nil {
		logInquiryFailure(&req, err)
		return h.inquiryError(c, err, echoData)
	}

	return c.JSON(http.StatusOK, resp)
}

// inquiryError renders a usecase failure as a full SNAP InquiryResponse. BCA
// treats a response whose inquiryStatus/inquiryReason are empty as a failed
// transaction regardless of the code, so every rejection carries them.
func (h *VAHandler) inquiryError(c echo.Context, err error, echoData domain.VAIdentityEcho) error {
	var domainErr *domain.DomainError
	if !errors.As(err, &domainErr) {
		return c.JSON(http.StatusInternalServerError, domain.NewInquiryErrorResponse(
			domain.CodeInternalError(domain.ServiceCodeInquiry), "Internal Server Error", echoData))
	}

	resp := domain.NewInquiryErrorResponse(domainErr.SNAPCode, domainErr.Message, echoData)
	// When the usecase got far enough to resolve the VA it refused, report that
	// VA rather than the bare echo of the request keys: the vendor then learns
	// WHICH bill is not payable (name, totalAmount, billDetails) and not merely
	// that one isn't. Validation, auth and server errors have no VA behind them
	// and leave InquiryData nil.
	if domainErr.InquiryData != nil {
		resolved := *domainErr.InquiryData
		// The envelope's outcome pair is the fallback, not the override: both
		// come from the same code tables, so they agree, and deferring to the
		// usecase leaves room for a rejection that needs a status of its own.
		if resolved.InquiryStatus == "" && resp.VirtualAccountData != nil {
			resolved.InquiryStatus = resp.VirtualAccountData.InquiryStatus
			resolved.InquiryReason = resp.VirtualAccountData.InquiryReason
		}
		resp.VirtualAccountData = &resolved
	}
	return c.JSON(mapSNAPCodeToHTTP(domainErr.SNAPCode), resp)
}

// Payment godoc
// @Tags Virtual Account
// @Summary VA payment notification
// @Description Vendor-initiated notification that a payment against a Virtual Account has been received. State-changing: records the payment. Mandatory body fields per BCA: partnerServiceId, customerNo, virtualAccountNo, virtualAccountName, paymentRequestId, channelCode, paidAmount, totalAmount, trxDateTime, flagAdvise. trxId is conditional (mandatory only when the payment originates from a Create VA request).
// @Security SnapTimestamp
// @Security SnapSignature
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b. Required for vendors onboarded with a VENDOR_CLIENT_ID; the token is also bound into the AccessToken component of stringToSign"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-PARTNER-ID header string true "Partner ID using Company Code VA, max 8 chars (BCA tech docs v2.3/v2.4). Value-checked whenever the vendor config sets VENDOR_PARTNER_ID"
// @Param X-EXTERNAL-ID header string true "Numeric string, max 36 chars, unique per calendar day. Doubles as the idempotency key"
// @Param CHANNEL-ID header string true "PJP channel id (BCA VA: 95231). Value-checked whenever the vendor config sets VENDOR_CHANNEL_ID"
// @Param request body domain.VAPaymentRequest true "VA payment notification"
// @Success 200 {object} domain.VAPaymentResponse "2002500 Successful, paymentFlagStatus=00"
// @Failure 400 {object} domain.VAPaymentResponse "4002500 Bad Request (unparseable body), 4002501 Invalid Field Format {field}, 4002502 Invalid Mandatory Field {field}"
// @Failure 401 {object} domain.SNAPErrorResponse "4012500 Unauthorized. [reason] (invalid HMAC signature, or X-TIMESTAMP outside the ±5 minute freshness window), 4012501 Invalid Token (B2B)"
// @Failure 404 {object} domain.VAPaymentResponse "All with virtualAccountData.paymentFlagStatus=01 unless noted: 4042512 Invalid Bill/Virtual Account [Not Found] (the VA exists in neither the registry nor any transaction; virtualAccountData echoes the request keys with empty paidAmount/totalAmount), 4042513 Invalid Amount, 4042514 Paid Bill, 4042518 Inconsistent Request (double-flag replay — same X-EXTERNAL-ID and paymentRequestId; echoes the FIRST request's paymentFlagStatus and paymentFlagReason, whether that was 00 settled or 01 rejected), 4042519 Invalid Bill/Virtual Account (expired, feature 007-merchant-expiry-callback)"
// @Failure 409 {object} domain.VAPaymentResponse "4092500 Conflict — same X-EXTERNAL-ID with a different paymentRequestId, or an in-flight request still holding the key"
// @Failure 500 {object} domain.VAPaymentResponse "5002500 Internal Server Error"
// @Router /openapi/v1.0/transfer-va/payment [post]
func (h *VAHandler) Payment(c echo.Context) error {
	var req domain.VAPaymentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.NewPaymentErrorResponse(
			domain.CodePaymentBadRequest, "Bad Request", domain.VAIdentityEcho{}))
	}

	// X-EXTERNAL-ID is a header, so binding cannot have populated it — but the
	// usecase needs it: BCA's double-flagging rule is keyed on the PAIR
	// (X-EXTERNAL-ID, paymentRequestId), and without the header half the
	// duplicate check can only see payments that succeeded.
	req.ExternalID = c.Request().Header.Get(headerExternalID)

	echoData := paymentEcho(&req)
	if v := domain.ValidatePaymentRequest(&req, h.strictMandatoryFor(c)); v != nil {
		code, message := violationCode(domain.ServiceCodePayment, v)
		return c.JSON(http.StatusBadRequest,
			domain.NewPaymentErrorResponse(code, message, echoData))
	}

	resp, err := h.vaUsecase.Payment(c.Request().Context(), &req)
	if err != nil {
		return h.paymentError(c, err, &req, echoData)
	}

	// Not every non-error outcome is an HTTP 200. The double-flag replay
	// answers 4042518 "Inconsistent Request", which BCA counts as a successful
	// transaction — hence it travels as a response rather than an error — but
	// its code is 404-class, and BCA pairs responseCode prefixes with the
	// matching HTTP status throughout Appendix A. Hardcoding 200 here shipped
	// a 404-class code over an HTTP 200.
	return c.JSON(mapSNAPCodeToHTTP(resp.ResponseCode), resp)
}

// paymentError renders a usecase failure as a full SNAP PaymentResponse,
// echoing the amounts from the request so the vendor can reconcile which
// payment was rejected.
func (h *VAHandler) paymentError(c echo.Context, err error, req *domain.VAPaymentRequest, echoData domain.VAIdentityEcho) error {
	var domainErr *domain.DomainError
	if !errors.As(err, &domainErr) {
		return c.JSON(http.StatusInternalServerError, domain.NewPaymentErrorResponse(
			domain.CodeInternalError(domain.ServiceCodePayment), "Internal Server Error", echoData))
	}

	resp := domain.NewPaymentErrorResponse(domainErr.SNAPCode, domainErr.Message, echoData)

	// When the usecase resolved a stored payment behind the rejection it wins
	// outright — it is the layer that knows which VA was matched. 4042518 is
	// why: the double-flag replay must echo the ORIGINAL payment's flag status,
	// not the "01" a fresh rejection would carry, so PaymentData's own
	// status/reason are kept whenever it sets them.
	if domainErr.PaymentData != nil {
		stored := *domainErr.PaymentData
		if stored.PaymentFlagStatus == "" && resp.VirtualAccountData != nil {
			stored.PaymentFlagStatus = resp.VirtualAccountData.PaymentFlagStatus
			stored.PaymentFlagReason = resp.VirtualAccountData.PaymentFlagReason
		}
		resp.VirtualAccountData = &stored
		return c.JSON(mapSNAPCodeToHTTP(domainErr.SNAPCode), resp)
	}

	// Otherwise echo the request's own identity/amount fields onto the
	// rejection — BCA's PaymentResponse marks these Mandatory, and a rejection
	// that omits them gives the channel nothing to display.
	resp.VirtualAccountData.VirtualAccountName = req.VirtualAccountName
	resp.VirtualAccountData.TrxDateTime = req.TrxDateTime
	resp.VirtualAccountData.ReferenceNo = req.ReferenceNo
	if req.PaidAmount != nil {
		resp.VirtualAccountData.PaidAmount = req.PaidAmount
	}
	if req.TotalAmount != nil {
		resp.VirtualAccountData.TotalAmount = req.TotalAmount
	}
	return c.JSON(mapSNAPCodeToHTTP(domainErr.SNAPCode), resp)
}

// Status godoc
// @Tags Virtual Account
// @Summary VA payment status inquiry
// @Description Vendor-initiated inquiry of the current payment status of a Virtual Account transaction. Read-only. Registered under /openapi/v2.0 as well as v1.0 — BCA calls this service at v2.0.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b. Required for vendors onboarded with a VENDOR_CLIENT_ID; the token is also bound into the AccessToken component of stringToSign"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-PARTNER-ID header string true "Partner ID using Company Code VA, max 8 chars (BCA tech docs v2.3/v2.4). Value-checked whenever the vendor config sets VENDOR_PARTNER_ID"
// @Param X-EXTERNAL-ID header string true "Numeric string, max 36 chars, unique per calendar day. Doubles as the idempotency key"
// @Param CHANNEL-ID header string true "PJP channel id (BCA VA: 95231). Value-checked whenever the vendor config sets VENDOR_CHANNEL_ID"
// @Param request body domain.VAStatusRequest true "VA status request"
// @Success 200 {object} domain.VAStatusResponse "2002600 Success. paymentFlagStatus 00 settled / 01 rejected / 02 timeout / 03 pending"
// @Failure 400 {object} domain.VAStatusResponse "4002600 Bad Request (unparseable body), 4002601 Invalid Field Format {field}, 4002602 Invalid Mandatory Field {field}"
// @Failure 401 {object} domain.SNAPErrorResponse "4012600 Unauthorized. [reason], 4012601 Invalid Token (B2B)"
// @Failure 404 {object} domain.VAStatusResponse "4042601 Transaction Not Found"
// @Failure 409 {object} domain.VAStatusResponse "4092600 Conflict — X-EXTERNAL-ID reused"
// @Failure 500 {object} domain.VAStatusResponse "5002601 Internal Server Error"
// @Router /openapi/v2.0/transfer-va/status [post]
func (h *VAHandler) Status(c echo.Context) error {
	var req domain.VAStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.NewStatusErrorResponse(
			domain.CodeStatusBadRequest, "Bad Request", domain.VAIdentityEcho{}))
	}

	echoData := statusEcho(&req)
	if v := domain.ValidateStatusRequest(&req); v != nil {
		code, message := violationCode(domain.ServiceCodeStatus, v)
		return c.JSON(http.StatusBadRequest,
			domain.NewStatusErrorResponse(code, message, echoData))
	}

	resp, err := h.vaUsecase.Status(c.Request().Context(), &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			return c.JSON(mapSNAPCodeToHTTP(domainErr.SNAPCode),
				domain.NewStatusErrorResponse(domainErr.SNAPCode, domainErr.Message, echoData))
		}
		return c.JSON(http.StatusInternalServerError, domain.NewStatusErrorResponse(
			domain.CodeStatusInternalErr, "Internal Server Error", echoData))
	}

	return c.JSON(http.StatusOK, resp)
}

// violationCode maps a field violation to the service's Invalid Mandatory
// Field / Invalid Field Format code and BCA's message wording, which embeds
// the offending field name.
func violationCode(service string, v *domain.FieldViolation) (code, message string) {
	if v.Kind == domain.ViolationMandatory {
		return domain.CodeMissingMandatory(service), "Invalid Mandatory Field [" + v.Field + "]"
	}
	return domain.CodeInvalidField(service), "Invalid Field Format [" + v.Field + "]"
}

func inquiryEcho(req *domain.VAInquiryRequest) domain.VAIdentityEcho {
	return domain.VAIdentityEcho{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
	}
}

// logInquiryFailure records why an inquiry was not answered 2002400. Without
// it a 5002400 reaches the vendor with the underlying cause discarded — the
// SNAP body deliberately says nothing beyond "Internal Server Error", so the
// log is the only place the real error can survive. Rejections (4xx) are logged
// at Warn: they are ordinary outcomes, not faults.
func logInquiryFailure(req *domain.VAInquiryRequest, err error) {
	if middleware.Logger == nil {
		return
	}

	attrs := []any{
		slog.String("endpoint", "inquiry"),
		slog.String("virtualAccountNo", req.VirtualAccountNo),
		slog.String("inquiryRequestId", req.InquiryRequestID),
		slog.String("error", err.Error()),
	}

	var domainErr *domain.DomainError
	if errors.As(err, &domainErr) {
		attrs = append(attrs, slog.String("responseCode", domainErr.SNAPCode))
		if mapSNAPCodeToHTTP(domainErr.SNAPCode) < http.StatusInternalServerError {
			middleware.Logger.Warn("va_inquiry_rejected", attrs...)
			return
		}
	}
	middleware.Logger.Error("va_inquiry_failed", attrs...)
}

func paymentEcho(req *domain.VAPaymentRequest) domain.VAIdentityEcho {
	return domain.VAIdentityEcho{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		PaymentRequestID: req.PaymentRequestID,
	}
}

func statusEcho(req *domain.VAStatusRequest) domain.VAIdentityEcho {
	return domain.VAIdentityEcho{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		PaymentRequestID: req.PaymentRequestID,
	}
}

// mapSNAPCodeToHTTP maps SNAP response codes to HTTP status codes.
func mapSNAPCodeToHTTP(snapCode string) int {
	if len(snapCode) < 3 {
		return http.StatusInternalServerError
	}
	switch snapCode[:3] {
	case "200":
		return http.StatusOK
	case "400":
		return http.StatusBadRequest
	case "401":
		return http.StatusUnauthorized
	case "403":
		return http.StatusForbidden
	case "404":
		return http.StatusNotFound
	case "405":
		return http.StatusMethodNotAllowed
	case "409":
		return http.StatusConflict
	case "422":
		return http.StatusUnprocessableEntity
	case "500":
		return http.StatusInternalServerError
	case "504":
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
