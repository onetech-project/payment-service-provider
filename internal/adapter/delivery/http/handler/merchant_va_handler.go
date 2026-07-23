package handler

import (
	"errors"
	"net/http"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// MerchantVAHandler handles merchant VA HTTP requests
type MerchantVAHandler struct {
	merchantVAUsecase domain.MerchantVAUsecase
}

// NewMerchantVAHandler creates a new merchant VA handler
func NewMerchantVAHandler(merchantVAUsecase domain.MerchantVAUsecase) *MerchantVAHandler {
	return &MerchantVAHandler{merchantVAUsecase: merchantVAUsecase}
}

// CreateVA handles POST /v1.0/transfer-va/create-va
func (h *MerchantVAHandler) CreateVA(c echo.Context) error {
	var req domain.MerchantCreateVARequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.MerchantCreateVAResponse{
			ResponseCode:    "4002700",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountName == "" || req.TrxID == "" || req.NotificationURL == "" {
		return c.JSON(http.StatusBadRequest, domain.MerchantCreateVAResponse{
			ResponseCode:    "4002701",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.merchantVAUsecase.CreateVA(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.MerchantCreateVAResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.MerchantCreateVAResponse{
			ResponseCode:    "5002700",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// ListVA handles POST /v1.0/transfer-va/list
func (h *MerchantVAHandler) ListVA(c echo.Context) error {
	var req domain.MerchantListVARequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.MerchantListVAResponse{
			ResponseCode:    "4002400",
			ResponseMessage: "Invalid Field Format",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.merchantVAUsecase.ListVA(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.MerchantListVAResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.MerchantListVAResponse{
			ResponseCode:    "5002400",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// DeleteVA handles DELETE /v1.0/transfer-va/delete-va
func (h *MerchantVAHandler) DeleteVA(c echo.Context) error {
	var req domain.MerchantDeleteVARequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.MerchantDeleteVAResponse{
			ResponseCode:    "4003101",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" {
		return c.JSON(http.StatusBadRequest, domain.MerchantDeleteVAResponse{
			ResponseCode:    "4003101",
			ResponseMessage: "Invalid Mandatory Field",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.merchantVAUsecase.DeleteVA(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.MerchantDeleteVAResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.MerchantDeleteVAResponse{
			ResponseCode:    "5003100",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}
