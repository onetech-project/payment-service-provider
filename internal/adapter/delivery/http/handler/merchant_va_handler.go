package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"backbone-new/internal/adapter/delivery/http/middleware"
	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

// logCreateVAFailure records why a create-va was refused. The SNAP body carries
// only the code and a terse message, so without this the reason a merchant's
// request bounced — which field, which VA-type rule — survives nowhere. The
// request's Content-Type is logged alongside the bound fields because the two
// failure modes look identical from outside: echo's binder silently leaves the
// struct zeroed for a non-JSON content type, which then trips the very same
// "missing mandatory field" check a genuinely incomplete body does.
func logCreateVAFailure(c echo.Context, req *domain.MerchantCreateVARequest, snapCode, message string) {
	if middleware.Logger == nil {
		return
	}

	middleware.Logger.Warn("create_va_rejected",
		slog.String("responseCode", snapCode),
		slog.String("responseMessage", message),
		slog.String("contentType", c.Request().Header.Get(echo.HeaderContentType)),
		slog.Int64("contentLength", c.Request().ContentLength),
		slog.String("partnerServiceId", req.PartnerServiceID),
		slog.String("customerNo", req.CustomerNo),
		slog.String("virtualAccountNo", req.VirtualAccountNo),
		slog.String("trxId", req.TrxID),
		slog.Bool("hasTotalAmount", req.TotalAmount != nil),
		slog.Any("additionalInfo", req.AdditionalInfo),
	)
}

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
// @Description To register a payment-notification callback URL, set additionalInfo.dbUrlProcess (e.g. {"additionalInfo": {"dbUrlProcess": "https://merchant.example.com/webhook/payment-callback"}}) — per ASPI's VAUpsertRequest, this is the only defined key under additionalInfo; it is not a top-level request field.
// @Security BearerAuth
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601, must be within ±5 minutes of server time"
// @Param X-SIGNATURE header string true "HMAC-SHA512(merchantSecret, \"POST:<path>:<accessToken>:<base64(sha256(body))>:<timestamp>\"), base64-encoded — merchantSecret provisioned via POST /admin/clients/{clientId}/secret"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param X-PARTNER-ID header string false "Partner identifier. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param CHANNEL-ID header string false "PJP channel id, 5 chars. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param request body domain.MerchantCreateVARequest true "VA create/update request. additionalInfo.dbUrlProcess carries the merchant payment-callback URL (see description)."
// @Success 200 {object} domain.MerchantCreateVAResponse "additionalInfo.dbUrlProcess is echoed back in virtualAccountData.additionalInfo"
// @Failure 400 {object} domain.MerchantCreateVAResponse "Invalid Field Format / Invalid Mandatory Field"
// @Failure 401 {object} domain.MerchantCreateVAResponse "Unauthorized: missing/invalid/expired accessToken, invalid/missing X-SIGNATURE, X-TIMESTAMP outside the ±5 minute window, or no signing secret provisioned for this client"
// @Failure 409 {object} domain.MerchantCreateVAResponse "Conflict: request already in progress for this X-EXTERNAL-ID"
// @Failure 422 {object} domain.MerchantCreateVAResponse "X-EXTERNAL-ID reused with a different payload"
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
	// trxId, plus VAIdentity's partnerServiceId).
	// notificationUrl is a proprietary extension, not part of the spec, so it's optional here.
	// customerNo and virtualAccountNo are intentionally NOT required here: per
	// feature 006-static-dynamic-va, dynamic VA types require customerNo to be
	// empty, and per feature 008-va-number-consistency, dynamic VA types leave
	// virtualAccountNo optional (server-derived) too — those mode-dependent
	// checks are performed in the usecase layer instead.
	if req.PartnerServiceID == "" || req.VirtualAccountName == "" || req.TrxID == "" {
		logCreateVAFailure(c, &req, "4002701", "Invalid Mandatory Field")
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
			logCreateVAFailure(c, &req, domainErr.SNAPCode, domainErr.Error())
			statusCode := mapSNAPCodeToHTTP(domainErr.SNAPCode)
			return c.JSON(statusCode, domain.MerchantCreateVAResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		logCreateVAFailure(c, &req, "5002700", err.Error())
		return c.JSON(http.StatusInternalServerError, domain.MerchantCreateVAResponse{
			ResponseCode:    "5002700",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// ListVA godoc
// @Tags Merchant VA Dashboard
// @Summary List registered Virtual Accounts
// @Description Merchant-initiated paginated listing of registered Virtual Account numbers — ONE entry per VA — filterable by date range, registration status and VA number. Read-only.
// @Description Each entry carries transactionCount and totalPaid aggregated over that VA's settled payments. Note: status filters on the REGISTRATION state (ACTIVE / INACTIVE / EXPIRED), not the transaction state. For per-payment detail use POST /openapi/v1.0/transfer-va/list-transactions.
// @Security BearerAuth
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601, must be within ±5 minutes of server time"
// @Param X-SIGNATURE header string true "HMAC-SHA512(merchantSecret, \"POST:<path>:<accessToken>:<base64(sha256(body))>:<timestamp>\"), base64-encoded — merchantSecret provisioned via POST /admin/clients/{clientId}/secret"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param X-PARTNER-ID header string false "Partner identifier. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param CHANNEL-ID header string false "PJP channel id, 5 chars. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param request body domain.MerchantListVARequest true "VA list filter/pagination request"
// @Success 200 {object} domain.MerchantListVAResponse
// @Failure 400 {object} domain.MerchantListVAResponse "Invalid Field Format"
// @Failure 401 {object} domain.MerchantListVAResponse "Unauthorized: missing/invalid/expired accessToken, invalid/missing X-SIGNATURE, X-TIMESTAMP outside the ±5 minute window, or no signing secret provisioned for this client"
// @Failure 409 {object} domain.MerchantListVAResponse "Conflict: request already in progress for this X-EXTERNAL-ID"
// @Failure 422 {object} domain.MerchantListVAResponse "X-EXTERNAL-ID reused with a different payload"
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

// ListTransactions godoc
// @Tags Merchant VA Dashboard
// @Summary List Virtual Account payment transactions
// @Description Merchant-initiated paginated listing of individual payment/transaction events, filterable by date range, transaction status ("00" paid, "02" expired, "03" pending, "04" deleted) and VA number. Read-only.
// @Description This is the per-payment counterpart of POST /openapi/v1.0/transfer-va/list — a no-bill VA paid N times returns N entries here and 1 entry there.
// @Security BearerAuth
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601, must be within ±5 minutes of server time"
// @Param X-SIGNATURE header string true "HMAC-SHA512(merchantSecret, \"POST:<path>:<accessToken>:<base64(sha256(body))>:<timestamp>\"), base64-encoded — merchantSecret provisioned via POST /admin/clients/{clientId}/secret"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param X-PARTNER-ID header string false "Partner identifier. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param CHANNEL-ID header string false "PJP channel id, 5 chars. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param request body domain.MerchantListVARequest true "Transaction list filter/pagination request"
// @Success 200 {object} domain.MerchantListTransactionsResponse
// @Failure 400 {object} domain.MerchantListTransactionsResponse "Invalid Field Format"
// @Failure 401 {object} domain.MerchantListTransactionsResponse "Unauthorized: missing/invalid/expired accessToken, invalid/missing X-SIGNATURE, X-TIMESTAMP outside the ±5 minute window, or no signing secret provisioned for this client"
// @Failure 409 {object} domain.MerchantListTransactionsResponse "Conflict: request already in progress for this X-EXTERNAL-ID"
// @Failure 422 {object} domain.MerchantListTransactionsResponse "X-EXTERNAL-ID reused with a different payload"
// @Failure 500 {object} domain.MerchantListTransactionsResponse "Internal Server Error"
// @Router /openapi/v1.0/transfer-va/list-transactions [post]
func (h *MerchantVAHandler) ListTransactions(c echo.Context) error {
	var req domain.MerchantListVARequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.MerchantListTransactionsResponse{
			ResponseCode:    "4002400",
			ResponseMessage: "Invalid Field Format",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.merchantVAUsecase.ListTransactions(ctx, &req)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			return c.JSON(mapSNAPCodeToHTTP(domainErr.SNAPCode), domain.MerchantListTransactionsResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.MerchantListTransactionsResponse{
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
// @Security BearerAuth
// @Param Authorization header string true "Bearer accessToken issued by POST /openapi/v1.0/access-token/b2b"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601, must be within ±5 minutes of server time"
// @Param X-SIGNATURE header string true "HMAC-SHA512(merchantSecret, \"DELETE:<path>:<accessToken>:<base64(sha256(body))>:<timestamp>\"), base64-encoded — merchantSecret provisioned via POST /admin/clients/{clientId}/secret"
// @Param X-EXTERNAL-ID header string true "Numeric string, unique per calendar day. Doubles as the idempotency key"
// @Param X-PARTNER-ID header string false "Partner identifier. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param CHANNEL-ID header string false "PJP channel id, 5 chars. Mandatory per the ASPI spec but NOT enforced on merchant routes — send it for SNAP conformance"
// @Param request body domain.MerchantDeleteVARequest true "VA delete request"
// @Success 200 {object} domain.MerchantDeleteVAResponse
// @Failure 400 {object} domain.MerchantDeleteVAResponse "Invalid Field Format / Invalid Mandatory Field"
// @Failure 401 {object} domain.MerchantDeleteVAResponse "Unauthorized: missing/invalid/expired accessToken, invalid/missing X-SIGNATURE, X-TIMESTAMP outside the ±5 minute window, or no signing secret provisioned for this client"
// @Failure 409 {object} domain.MerchantDeleteVAResponse "Conflict: request already in progress for this X-EXTERNAL-ID"
// @Failure 422 {object} domain.MerchantDeleteVAResponse "X-EXTERNAL-ID reused with a different payload"
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
