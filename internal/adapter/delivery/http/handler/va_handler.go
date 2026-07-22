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

// GetB2BAccessToken handles VA inquiry requests
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

// Payment handles VA payment notification requests
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

// Status handles VA status inquiry requests
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
