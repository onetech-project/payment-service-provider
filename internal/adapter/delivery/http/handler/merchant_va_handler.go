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

// CreateVA godoc
// @Tags Merchant VA Dashboard
// @Summary Create or update a Virtual Account
// @Description Merchant-initiated upsert of a Virtual Account (ASPI VAUpsertRequest). This performs a real state-changing action: it creates or updates a persistent Virtual Account record.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-EXTERNAL-ID header string true "Unique external ID for this request"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.MerchantCreateVARequest true "VA create/update request"
// @Success 200 {object} domain.MerchantCreateVAResponse
// @Failure 400 {object} domain.MerchantCreateVAResponse "Invalid Field Format / Invalid Mandatory Field / missing Idempotency-Key"
// @Failure 401 {object} domain.MerchantCreateVAResponse "Unauthorized (mapped from downstream error)"
// @Failure 409 {object} domain.MerchantCreateVAResponse "Conflict: request already in progress for this Idempotency-Key"
// @Failure 422 {object} domain.MerchantCreateVAResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.MerchantCreateVAResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/create-va [post]
func (h *MerchantVAHandler) CreateVA(c echo.Context) error {
	var req domain.MerchantCreateVARequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.MerchantCreateVAResponse{
			ResponseCode:    "4002700",
			ResponseMessage: "Invalid Field Format",
		})
	}

	// Validate required fields per ASPI VAUpsertRequest (required: virtualAccountName,
	// trxId, plus VAIdentity's partnerServiceId/customerNo/virtualAccountNo).
	// notificationUrl is a proprietary extension, not part of the spec, so it's optional here.
	if req.PartnerServiceID == "" || req.CustomerNo == "" || req.VirtualAccountNo == "" || req.VirtualAccountName == "" || req.TrxID == "" {
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

// ListVA godoc
// @Tags Merchant VA Dashboard
// @Summary List Virtual Account transactions
// @Description Merchant-initiated paginated listing of Virtual Account transactions, filterable by date range, status and VA number. Read-only.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-EXTERNAL-ID header string true "Unique external ID for this request"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware since ListVA is called via POST. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.MerchantListVARequest true "VA list filter/pagination request"
// @Success 200 {object} domain.MerchantListVAResponse
// @Failure 400 {object} domain.MerchantListVAResponse "Invalid Field Format / missing Idempotency-Key"
// @Failure 401 {object} domain.MerchantListVAResponse "Unauthorized (mapped from downstream error)"
// @Failure 409 {object} domain.MerchantListVAResponse "Conflict: request already in progress for this Idempotency-Key"
// @Failure 422 {object} domain.MerchantListVAResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.MerchantListVAResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/list [post]
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

// DeleteVA godoc
// @Tags Merchant VA Dashboard
// @Summary Delete a Virtual Account
// @Description Merchant-initiated deletion of a Virtual Account (ASPI DeleteVARequest). This performs a real state-changing action: it permanently removes/deactivates the Virtual Account record.
// @Security SnapTimestamp
// @Security SnapSignature
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param X-SIGNATURE header string true "Symmetric signature; compute via POST /api/v1/utilities/signature-service"
// @Param X-EXTERNAL-ID header string true "Unique external ID for this request"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.MerchantDeleteVARequest true "VA delete request"
// @Success 200 {object} domain.MerchantDeleteVAResponse
// @Failure 400 {object} domain.MerchantDeleteVAResponse "Invalid Field Format / Invalid Mandatory Field / missing Idempotency-Key"
// @Failure 401 {object} domain.MerchantDeleteVAResponse "Unauthorized (mapped from downstream error)"
// @Failure 409 {object} domain.MerchantDeleteVAResponse "Conflict: request already in progress for this Idempotency-Key"
// @Failure 422 {object} domain.MerchantDeleteVAResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.MerchantDeleteVAResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/delete-va [delete]
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
