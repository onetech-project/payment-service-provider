package handler

import (
	"errors"
	"net/http"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// AdminReconcileHandler exposes the on-demand counterpart of the periodic
// status-reconciliation sweep (feature 014-vendor-status-reconciliation).
//
// The sweep is what catches stuck payments nobody has noticed; this is what an
// operator reaches for when a customer is on the phone saying they paid and
// the merchant says they did not. Waiting for the next sweep is not an answer
// in that conversation.
type AdminReconcileHandler struct {
	reconciler domain.VAStatusReconciler
}

// NewAdminReconcileHandler creates the handler. Registered behind
// AdminAuthMiddleware on cmd/api/main.go's adminGroup.
func NewAdminReconcileHandler(reconciler domain.VAStatusReconciler) *AdminReconcileHandler {
	return &AdminReconcileHandler{reconciler: reconciler}
}

type reconcileResponse struct {
	VirtualAccountNo string `json:"virtualAccountNo"`
	Outcome          string `json:"outcome"`
	Settled          bool   `json:"settled"`
	// The vendor's raw answer is echoed so an operator can see what the
	// outcome was inferred from rather than having to trust it.
	VendorResponseCode      string  `json:"vendorResponseCode,omitempty"`
	VendorPaymentFlagStatus string  `json:"vendorPaymentFlagStatus,omitempty"`
	PaidAmount              *amount `json:"paidAmount,omitempty"`
	ReconciledAt            string  `json:"reconciledAt"`
}

type amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type reconcileErrorResponse struct {
	Error string `json:"error"`
}

// Reconcile godoc
// @Tags Admin
// @Summary Ask the vendor what really happened to a pending transaction
// @Description Admin-only: calls the vendor's SNAP Inquiry Status service (code 26) for the given virtualAccountNo. If the vendor reports the payment as successful while this service still had it pending, the transaction is settled and the merchant payment callback is sent. Safe to call repeatedly — an already-settled transaction is reported back without a second callback.
// @Security AdminAPIKey
// @Param X-Admin-API-Key header string true "Admin API key"
// @Param virtualAccountNo path string true "Virtual account number"
// @Success 200 {object} reconcileResponse
// @Failure 401 {object} reconcileErrorResponse "Unauthorized"
// @Failure 404 {object} reconcileErrorResponse "Transaction not found"
// @Failure 503 {object} reconcileErrorResponse "Reconciliation is not configured for this deployment"
// @Router /admin/transactions/{virtualAccountNo}/reconcile [post]
func (h *AdminReconcileHandler) Reconcile(c echo.Context) error {
	// Fail closed and say why: without outbound vendor credentials this
	// endpoint cannot do anything, and a 500 would send whoever called it
	// looking for a bug instead of a config value.
	if h.reconciler == nil {
		return c.JSON(http.StatusServiceUnavailable, reconcileErrorResponse{
			Error: "vendor status reconciliation is not configured (see RECONCILE_ENABLED and the vendor's outbound credentials)",
		})
	}

	virtualAccountNo := c.Param("virtualAccountNo")
	if virtualAccountNo == "" {
		return c.JSON(http.StatusNotFound, reconcileErrorResponse{Error: "transaction not found"})
	}

	result, err := h.reconciler.Reconcile(c.Request().Context(), virtualAccountNo, domain.ReconcileTriggerAdmin)
	if err != nil {
		if errors.Is(err, domain.ErrMerchantVANotFound) || errors.Is(err, domain.ErrVAInvalidBill) {
			return c.JSON(http.StatusNotFound, reconcileErrorResponse{Error: "transaction not found"})
		}
		return c.JSON(http.StatusInternalServerError, reconcileErrorResponse{Error: "internal server error"})
	}

	resp := reconcileResponse{
		VirtualAccountNo:        result.VirtualAccountNo,
		Outcome:                 result.Outcome,
		Settled:                 result.Settled,
		VendorResponseCode:      result.VendorResponseCode,
		VendorPaymentFlagStatus: result.VendorPaymentFlagStatus,
		ReconciledAt:            result.ReconciledAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if result.PaidAmount != nil {
		resp.PaidAmount = &amount{Value: result.PaidAmount.Value, Currency: result.PaidAmount.Currency}
	}
	return c.JSON(http.StatusOK, resp)
}
