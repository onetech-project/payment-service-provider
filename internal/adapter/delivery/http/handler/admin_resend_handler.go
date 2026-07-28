package handler

import (
	"errors"
	"net/http"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// AdminResendHandler handles the admin-only manual callback resend endpoint
// (feature 007-merchant-expiry-callback, US2). Registered behind
// AdminAuthMiddleware on cmd/api/main.go's adminGroup.
type AdminResendHandler struct {
	resendUsecase domain.ResendCallbackUsecase
}

// NewAdminResendHandler creates a new admin resend handler.
func NewAdminResendHandler(resendUsecase domain.ResendCallbackUsecase) *AdminResendHandler {
	return &AdminResendHandler{resendUsecase: resendUsecase}
}

type resendCallbackResponse struct {
	VirtualAccountNo string `json:"virtualAccountNo"`
	EventType        string `json:"eventType"`
	ResentAt         string `json:"resentAt"`
	DeliveryStatus   string `json:"deliveryStatus"`
}

type resendCallbackErrorResponse struct {
	Error string `json:"error"`
}

// Resend godoc
// @Tags Admin
// @Summary Resend the most recent merchant callback for a transaction
// @Description Admin-only: redelivers the most recent callback event (payment.received or va.expired) on record for the given virtualAccountNo. Does not mutate transaction state.
// @Security AdminAPIKey
// @Param X-Admin-API-Key header string true "Admin API key"
// @Param virtualAccountNo path string true "Virtual account number"
// @Success 200 {object} resendCallbackResponse
// @Failure 401 {object} resendCallbackErrorResponse "Unauthorized"
// @Failure 404 {object} resendCallbackErrorResponse "Transaction not found"
// @Failure 422 {object} resendCallbackErrorResponse "No callback to resend / no notification URL"
// @Router /admin/transactions/{virtualAccountNo}/resend-callback [post]
func (h *AdminResendHandler) Resend(c echo.Context) error {
	virtualAccountNo := c.Param("virtualAccountNo")
	if virtualAccountNo == "" {
		return c.JSON(http.StatusNotFound, resendCallbackErrorResponse{Error: "transaction not found"})
	}

	ctx := c.Request().Context()
	result, err := h.resendUsecase.Resend(ctx, virtualAccountNo)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrMerchantVANotFound):
			return c.JSON(http.StatusNotFound, resendCallbackErrorResponse{Error: "transaction not found"})
		case errors.Is(err, domain.ErrResendNoDeliveryRecord):
			return c.JSON(http.StatusUnprocessableEntity, resendCallbackErrorResponse{Error: "no callback delivery on record for this transaction"})
		case errors.Is(err, domain.ErrResendNoNotificationURL):
			return c.JSON(http.StatusUnprocessableEntity, resendCallbackErrorResponse{Error: "transaction has no registered notification URL"})
		default:
			return c.JSON(http.StatusInternalServerError, resendCallbackErrorResponse{Error: "internal server error"})
		}
	}

	return c.JSON(http.StatusOK, resendCallbackResponse{
		VirtualAccountNo: result.VirtualAccountNo,
		EventType:        result.EventType,
		ResentAt:         result.ResentAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		DeliveryStatus:   result.DeliveryStatus,
	})
}
