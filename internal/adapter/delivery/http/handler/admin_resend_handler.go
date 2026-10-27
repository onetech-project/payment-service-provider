package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

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

// maxResendRange bounds one bulk resend's time window, alongside the
// usecase's per-call row cap.
const maxResendRange = 31 * 24 * time.Hour

// resendRangeRequest selects original callbacks to resend. startTime is
// inclusive and endTime exclusive, both RFC 3339 with an offset.
// virtualAccountNo narrows to one VA. status filters on the original's
// delivery status ("failed", "retrying", "queued", "success", "duplicate");
// empty means every status.
type resendRangeRequest struct {
	StartTime        string   `json:"startTime" example:"2026-10-27T00:00:00+07:00"`
	EndTime          string   `json:"endTime" example:"2026-10-28T00:00:00+07:00"`
	VirtualAccountNo string   `json:"virtualAccountNo,omitempty"`
	Status           []string `json:"status,omitempty"`
}

type resendRangeItem struct {
	SourceDeliveryID string `json:"sourceDeliveryId"`
	DeliveryID       string `json:"deliveryId,omitempty"`
	VirtualAccountNo string `json:"virtualAccountNo"`
	EventType        string `json:"eventType"`
	Status           string `json:"status"`
	Error            string `json:"error,omitempty"`
	// Reconstructed marks a resend whose original was recorded without its
	// payload and was rebuilt from transaction state.
	Reconstructed bool `json:"reconstructed,omitempty"`
}

type resendRangeResponse struct {
	Matched  int               `json:"matched"`
	Queued   int               `json:"queued"`
	Failed   int               `json:"failed"`
	Skipped  int               `json:"skipped"`
	ResentAt string            `json:"resentAt"`
	Items    []resendRangeItem `json:"items"`
}

var resendRangeStatuses = map[string]bool{
	domain.NotificationDeliveryStatusQueued:    true,
	domain.NotificationDeliveryStatusRetrying:  true,
	domain.NotificationDeliveryStatusSuccess:   true,
	domain.NotificationDeliveryStatusFailed:    true,
	domain.NotificationDeliveryStatusDuplicate: true,
}

// ResendRange godoc
// @Tags Admin
// @Summary Bulk resend merchant callbacks in a time range
// @Description Admin-only: redelivers every original (auto-triggered) callback recorded in [startTime, endTime), optionally for one virtualAccountNo and/or only those whose delivery ended in the given statuses (e.g. ["failed"] for undelivered ones, timeouts included). Each callback is resent with the payload it was first sent with. Callbacks recorded before payloads were stored are rebuilt from the transaction recorded alongside them (marked reconstructed), or skipped when none is found. Range at most 31 days and 500 callbacks per call. Does not mutate transaction state.
// @Security AdminAPIKey
// @Param X-Admin-API-Key header string true "Admin API key"
// @Param request body resendRangeRequest true "Time range and optional filters"
// @Success 200 {object} resendRangeResponse
// @Failure 400 {object} resendCallbackErrorResponse "Invalid payload or range"
// @Failure 401 {object} resendCallbackErrorResponse "Unauthorized"
// @Failure 422 {object} resendCallbackErrorResponse "Too many callbacks in range"
// @Router /admin/callbacks/resend [post]
func (h *AdminResendHandler) ResendRange(c echo.Context) error {
	var req resendRangeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, resendCallbackErrorResponse{Error: "invalid request payload"})
	}

	from, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return c.JSON(http.StatusBadRequest, resendCallbackErrorResponse{Error: "startTime must be RFC 3339, e.g. 2026-10-27T00:00:00+07:00"})
	}
	to, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return c.JSON(http.StatusBadRequest, resendCallbackErrorResponse{Error: "endTime must be RFC 3339, e.g. 2026-10-28T00:00:00+07:00"})
	}
	if !from.Before(to) {
		return c.JSON(http.StatusBadRequest, resendCallbackErrorResponse{Error: "startTime must be before endTime"})
	}
	if to.Sub(from) > maxResendRange {
		return c.JSON(http.StatusBadRequest, resendCallbackErrorResponse{Error: "range must not exceed 31 days"})
	}
	for _, status := range req.Status {
		if !resendRangeStatuses[status] {
			return c.JSON(http.StatusBadRequest, resendCallbackErrorResponse{Error: "unknown status: " + status})
		}
	}

	result, err := h.resendUsecase.ResendRange(c.Request().Context(), domain.NotificationDeliveryFilter{
		From:             from,
		To:               to,
		VirtualAccountNo: strings.TrimSpace(req.VirtualAccountNo),
		Statuses:         req.Status,
	})
	if err != nil {
		if errors.Is(err, domain.ErrResendRangeTooLarge) {
			return c.JSON(http.StatusUnprocessableEntity, resendCallbackErrorResponse{Error: "more than 500 callbacks in range; narrow the range or filter by status/virtualAccountNo"})
		}
		return c.JSON(http.StatusInternalServerError, resendCallbackErrorResponse{Error: "internal server error"})
	}

	resp := resendRangeResponse{
		Matched:  len(result.Items),
		ResentAt: result.ResentAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Items:    make([]resendRangeItem, 0, len(result.Items)),
	}
	for _, item := range result.Items {
		switch item.Status {
		case domain.NotificationDeliveryStatusQueued:
			resp.Queued++
		case "skipped":
			resp.Skipped++
		default:
			resp.Failed++
		}
		resp.Items = append(resp.Items, resendRangeItem{
			SourceDeliveryID: item.SourceDeliveryID,
			DeliveryID:       item.DeliveryID,
			VirtualAccountNo: item.VirtualAccountNo,
			EventType:        item.EventType,
			Status:           item.Status,
			Error:            item.Error,
			Reconstructed:    item.Reconstructed,
		})
	}
	return c.JSON(http.StatusOK, resp)
}
