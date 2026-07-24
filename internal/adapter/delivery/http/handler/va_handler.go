package handler

import (
	"errors"
	"net/http"

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
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-EXTERNAL-ID header string true "Unique external ID for this request"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.VAInquiryRequest true "VA inquiry request"
// @Success 200 {object} domain.VAInquiryResponse
// @Failure 400 {object} domain.VAInquiryResponse "Invalid Field Format / Invalid Mandatory Field / missing Idempotency-Key"
// @Failure 401 {object} domain.VAInquiryResponse "Unauthorized (mapped from downstream error)"
// @Failure 404 {object} domain.VAInquiryResponse "Not Found (mapped from downstream error)"
// @Failure 409 {object} domain.VAInquiryResponse "Conflict: request already in progress for this Idempotency-Key"
// @Failure 422 {object} domain.VAInquiryResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.VAInquiryResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/inquiry [post]
func (h *VAHandler) Inquiry(c echo.Context) error {
	var req domain.VAInquiryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.VAInquiryResponse{
			ResponseCode:    "4002401",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" || req.InquiryRequestID == "" {
		return c.JSON(http.StatusBadRequest, domain.VAInquiryResponse{
			ResponseCode:    "4002402",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.vaUsecase.Inquiry(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.VAInquiryResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
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
// @Description Vendor-initiated notification that a payment against a Virtual Account has been received. State-changing: records the payment.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-EXTERNAL-ID header string true "Unique external ID for this request"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.VAPaymentRequest true "VA payment notification"
// @Success 200 {object} domain.VAPaymentResponse
// @Failure 400 {object} domain.VAPaymentResponse "Invalid Field Format / Invalid Mandatory Field / missing Idempotency-Key"
// @Failure 401 {object} domain.VAPaymentResponse "Unauthorized (mapped from downstream error)"
// @Failure 404 {object} domain.VAPaymentResponse "Not Found (mapped from downstream error)"
// @Failure 409 {object} domain.VAPaymentResponse "Conflict (mapped from downstream error, or in-flight request with same Idempotency-Key)"
// @Failure 422 {object} domain.VAPaymentResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.VAPaymentResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/payment [post]
func (h *VAHandler) Payment(c echo.Context) error {
	var req domain.VAPaymentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.VAPaymentResponse{
			ResponseCode:    "4002401",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" ||
		req.InquiryRequestID == "" || req.PaymentRequestID == "" {
		return c.JSON(http.StatusBadRequest, domain.VAPaymentResponse{
			ResponseCode:    "4002402",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.vaUsecase.Payment(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.VAPaymentResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.VAPaymentResponse{
			ResponseCode:    "5002400",
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
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-EXTERNAL-ID header string true "Unique external ID for this request"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.VAStatusRequest true "VA status request"
// @Success 200 {object} domain.VAStatusResponse
// @Failure 400 {object} domain.VAStatusResponse "Invalid Field Format / Invalid Mandatory Field / missing Idempotency-Key"
// @Failure 401 {object} domain.VAStatusResponse "Unauthorized (mapped from downstream error)"
// @Failure 404 {object} domain.VAStatusResponse "Not Found (mapped from downstream error)"
// @Failure 409 {object} domain.VAStatusResponse "Conflict: request already in progress for this Idempotency-Key"
// @Failure 422 {object} domain.VAStatusResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.VAStatusResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/status [post]
func (h *VAHandler) Status(c echo.Context) error {
	var req domain.VAStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.VAStatusResponse{
			ResponseCode:    "4002401",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" || req.InquiryRequestID == "" {
		return c.JSON(http.StatusBadRequest, domain.VAStatusResponse{
			ResponseCode:    "4002402",
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
			ResponseCode:    "5002400",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
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
