package handler

import (
	"errors"
	"net/http"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

type TokenHandler struct {
	tokenUsecase domain.TokenUsecase
}

func NewTokenHandler(tokenUsecase domain.TokenUsecase) *TokenHandler {
	return &TokenHandler{tokenUsecase: tokenUsecase}
}

// GetB2BAccessToken godoc
// @Tags Token
// @Summary Issue a SNAP B2B access token
// @Description Issues a bearer access token for a registered client, after verifying the asymmetric X-SIGNATURE against the client's registered public key. This endpoint ISSUES the token — the caller has no prior bearer token; auth is via the X-CLIENT-KEY/X-TIMESTAMP/X-SIGNATURE headers only. Use POST /api/v1/utilities/signature-auth to compute X-SIGNATURE.
// @Security SnapClientKey
// @Security SnapTimestamp
// @Security SnapSignature
// @Param X-CLIENT-KEY header string true "Client identifier issued at onboarding"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601 (e.g. 2026-07-24T10:00:00+07:00)"
// @Param X-SIGNATURE header string true "Asymmetric signature; compute via POST /api/v1/utilities/signature-auth"
// @Param Idempotency-Key header string true "Unique key for this request; enforced by IdempotencyMiddleware for all non-GET requests on this route. A repeated key with an identical payload replays the cached response; a repeated key with a different payload is rejected with 422."
// @Param request body domain.SNAPTokenRequest true "Token request payload"
// @Success 200 {object} domain.SNAPTokenResponse
// @Failure 400 {object} domain.SNAPTokenResponse "Invalid request payload, missing required SNAP headers, or missing Idempotency-Key"
// @Failure 401 {object} domain.SNAPTokenResponse "Signature verification failed"
// @Failure 409 {object} domain.SNAPTokenResponse "Conflict: request already in progress for this Idempotency-Key"
// @Failure 422 {object} domain.SNAPTokenResponse "Idempotency-Key reused with a different payload"
// @Failure 500 {object} domain.SNAPTokenResponse "Internal Server Error"
// @Router /openapi/v1.0/access-token/b2b [post]
func (h *TokenHandler) GetB2BAccessToken(c echo.Context) error {
	var req domain.SNAPTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.SNAPTokenResponse{
			ResponseCode:    "4007300",
			ResponseMessage: "Bad Request. Invalid request payload format.",
		})
	}

	clientID := c.Request().Header.Get("X-CLIENT-KEY")
	timestamp := c.Request().Header.Get("X-TIMESTAMP")
	signature := c.Request().Header.Get("X-SIGNATURE")

	if clientID == "" || timestamp == "" || signature == "" {
		return c.JSON(http.StatusBadRequest, domain.SNAPTokenResponse{
			ResponseCode:    "4007300",
			ResponseMessage: "Bad Request. Missing required SNAP headers (X-CLIENT-KEY, X-TIMESTAMP, X-SIGNATURE).",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.tokenUsecase.GenerateB2BToken(ctx, clientID, timestamp, signature, req.GrantType)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			statusCode := http.StatusInternalServerError
			switch domainErr.SNAPCode {
			case "4007300":
				statusCode = http.StatusBadRequest
			case "4017300":
				statusCode = http.StatusUnauthorized
			}
			return c.JSON(statusCode, domain.SNAPTokenResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.SNAPTokenResponse{
			ResponseCode:    "5007300",
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}
