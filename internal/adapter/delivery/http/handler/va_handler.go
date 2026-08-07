package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// VAHandler handles vendor Virtual Account HTTP requests
type VAHandler struct {
	vaUsecase domain.VAUsecase
}

// NewVAHandler creates a new VA handler
func NewVAHandler(vaUsecase domain.VAUsecase) *VAHandler {
	return &VAHandler{vaUsecase: vaUsecase}
}

// Inquiry godoc
// @Tags Virtual Account
// @Summary VA bill inquiry
// @Description Vendor-initiated inquiry for Virtual Account bill/customer details prior to payment. Read-only.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b. Required for vendors onboarded with a VENDOR_CLIENT_ID; the token is also bound into the AccessToken component of stringToSign"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-PARTNER-ID header string true "Partner identifier, max 36 chars. Enforced whenever the vendor config sets VENDOR_PARTNER_ID"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param CHANNEL-ID header string true "PJP channel id, 5 chars. Mandatory per the ASPI security standard, and enforced whenever the vendor config sets VENDOR_CHANNEL_ID"
// @Param request body domain.VAInquiryRequest true "VA inquiry request"
// @Success 200 {object} domain.VAInquiryResponse
// @Failure 400 {object} domain.VAInquiryResponse "4002400 Bad Request: unparseable body, missing mandatory field, or invalid field format"
// @Failure 401 {object} domain.VAInquiryResponse "Unauthorized: invalid HMAC signature or X-TIMESTAMP outside the ±5 minute freshness window"
// @Failure 404 {object} domain.VAInquiryResponse "Not Found, all with virtualAccountData.inquiryStatus=01: 4042412 Invalid Bill/Virtual Account (no such VA, or a deleted one), 4042419 Expired Transaction (feature 007-merchant-expiry-callback), 4042414 Paid Bill"
// @Failure 409 {object} domain.VAInquiryResponse "Conflict: request already in progress for this X-EXTERNAL-ID"
// @Failure 422 {object} domain.VAInquiryResponse "X-EXTERNAL-ID reused with a different payload"
// @Failure 500 {object} domain.VAInquiryResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/inquiry [post]
func (h *VAHandler) Inquiry(c echo.Context) error {
	// Both rejected-input cases answer 4002400: an unparseable body and a
	// missing mandatory field are the same outcome to the vendor — the request
	// was not accepted — and this endpoint publishes one 400 code, not two.
	var req domain.VAInquiryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.VAInquiryResponse{
			ResponseCode:    "4002400",
			ResponseMessage: "Bad Request",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" || req.InquiryRequestID == "" {
		return c.JSON(http.StatusBadRequest, domain.VAInquiryResponse{
			ResponseCode:    "4002400",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.vaUsecase.Inquiry(ctx, &req)
	if err != nil {
		logInquiryFailure(&req, err)
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			// A rejected inquiry carries the VA it refused in
			// virtualAccountData, with inquiryStatus "01" + inquiryReason
			// (contracts/inquiry-expired.md for the expired case), so the
			// vendor learns WHICH bill is not payable and WHY, not merely that
			// one isn't. The usecase resolves it — validation/auth/server
			// errors have no VA behind them and leave it nil.
			return c.JSON(statusCode, domain.VAInquiryResponse{
				ResponseCode:       domainErr.SNAPCode,
				ResponseMessage:    domainErr.Message,
				VirtualAccountData: domainErr.InquiryData,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.VAInquiryResponse{
			ResponseCode:    "5002400",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// Payment godoc
// @Tags Virtual Account
// @Summary VA payment notification
// @Description Vendor-initiated notification that a payment against a Virtual Account has been received. State-changing: records the payment. Mandatory body fields per ASPI: partnerServiceId, customerNo, virtualAccountNo, trxId, paymentRequestId, paidAmount.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b. Required for vendors onboarded with a VENDOR_CLIENT_ID; the token is also bound into the AccessToken component of stringToSign"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-PARTNER-ID header string true "Partner identifier, max 36 chars. Enforced whenever the vendor config sets VENDOR_PARTNER_ID"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param CHANNEL-ID header string true "PJP channel id, 5 chars. Mandatory per the ASPI security standard, and enforced whenever the vendor config sets VENDOR_CHANNEL_ID"
// @Param request body domain.VAPaymentRequest true "VA payment notification"
// @Success 200 {object} domain.VAPaymentResponse
// @Failure 400 {object} domain.VAPaymentResponse "Invalid Field Format / Invalid Mandatory Field"
// @Failure 401 {object} domain.VAPaymentResponse "Unauthorized: invalid HMAC signature or X-TIMESTAMP outside the ±5 minute freshness window"
// @Failure 404 {object} domain.VAPaymentResponse "4042512 Invalid Bill/Virtual Account [Not Found] when the VA exists in neither the registry nor any transaction (virtualAccountData echoes the request keys, paymentFlagStatus=01, empty paidAmount/totalAmount); 4042518 Inconsistent Request when X-EXTERNAL-ID and paymentRequestId are both reused (virtualAccountData echoes the payment it collided with); or 4042519 Expired Transaction (virtualAccountData.paymentFlagStatus=01, feature 007-merchant-expiry-callback)"
// @Failure 409 {object} domain.VAPaymentResponse "Conflict (mapped from downstream error, or in-flight request with same X-EXTERNAL-ID)"
// @Failure 422 {object} domain.VAPaymentResponse "X-EXTERNAL-ID reused with a different payload"
// @Failure 500 {object} domain.VAPaymentResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/payment [post]
func (h *VAHandler) Payment(c echo.Context) error {
	var req domain.VAPaymentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.VAPaymentResponse{
			ResponseCode:    "4002501",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" ||
		req.TrxID == "" || req.PaymentRequestID == "" {
		return c.JSON(http.StatusBadRequest, domain.VAPaymentResponse{
			ResponseCode:    "4002502",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.vaUsecase.Payment(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			paymentResp := domain.VAPaymentResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			}
			// Every payment rejection that has a VA behind it reports that VA:
			// the not-found echo (4042512), the payment it collided with
			// (4042518), the expired transaction (4042519, contracts/
			// notify-expired.md) and the closed bill (4092500). The usecase
			// builds the block — it is the layer that knows which VA was
			// resolved — and the handler only forwards it.
			if domainErr.PaymentData != nil {
				paymentResp.VirtualAccountData = domainErr.PaymentData
			}
			return c.JSON(statusCode, paymentResp)
		}
		return c.JSON(http.StatusInternalServerError, domain.VAPaymentResponse{
			ResponseCode:    "5002500",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// Status godoc
// @Tags Virtual Account
// @Summary VA payment status inquiry
// @Description Vendor-initiated inquiry of the current payment status of a Virtual Account transaction. Read-only.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b. Required for vendors onboarded with a VENDOR_CLIENT_ID; the token is also bound into the AccessToken component of stringToSign"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-PARTNER-ID header string true "Partner identifier, max 36 chars. Enforced whenever the vendor config sets VENDOR_PARTNER_ID"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param CHANNEL-ID header string true "PJP channel id, 5 chars. Mandatory per the ASPI security standard, and enforced whenever the vendor config sets VENDOR_CHANNEL_ID"
// @Param request body domain.VAStatusRequest true "VA status request"
// @Success 200 {object} domain.VAStatusResponse
// @Failure 400 {object} domain.VAStatusResponse "Invalid Field Format / Invalid Mandatory Field"
// @Failure 401 {object} domain.VAStatusResponse "Unauthorized: invalid HMAC signature or X-TIMESTAMP outside the ±5 minute freshness window"
// @Failure 404 {object} domain.VAStatusResponse "Not Found (mapped from downstream error)"
// @Failure 409 {object} domain.VAStatusResponse "Conflict: request already in progress for this X-EXTERNAL-ID"
// @Failure 422 {object} domain.VAStatusResponse "X-EXTERNAL-ID reused with a different payload"
// @Failure 500 {object} domain.VAStatusResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/status [post]
func (h *VAHandler) Status(c echo.Context) error {
	var req domain.VAStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.VAStatusResponse{
			ResponseCode:    "4002601",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" || req.InquiryRequestID == "" {
		return c.JSON(http.StatusBadRequest, domain.VAStatusResponse{
			ResponseCode:    "4002602",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.vaUsecase.Status(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.VAStatusResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.VAStatusResponse{
			ResponseCode:    "5002600",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
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

// mapSNAPCodeToHTTP maps SNAP response codes to HTTP status codes
func mapSNAPCodeToHTTP(snapCode string) int {
	if len(snapCode) < 3 {
		return http.StatusInternalServerError
	}
	switch snapCode[:3] {
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
