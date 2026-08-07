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
// @Param request body domain.SNAPTokenRequest true "Token request payload"
// @Success 200 {object} domain.SNAPTokenResponse
// @Failure 400 {object} domain.SNAPTokenResponse "Invalid request payload, or missing required SNAP headers"
// @Failure 401 {object} domain.SNAPTokenResponse "Signature verification failed"
// @Failure 500 {object} domain.SNAPTokenResponse "Internal Server Error"
// @Router /openapi/v1.0/access-token/b2b [post]
func (h *TokenHandler) GetB2BAccessToken(c echo.Context) error {
	var req domain.SNAPTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.SNAPTokenResponse{
			ResponseCode:    domain.CodeTokenInvalidField,
			ResponseMessage: "Invalid field format [clientId/clientSecret/grantType]",
		})
	}

	// The headers are passed through to the usecase rather than pre-checked
	// here: BCA gives each one its own case code (4007302 for X-CLIENT-KEY,
	// 4007301 for X-TIMESTAMP), and a blanket "missing required SNAP headers"
	// at this layer answered 4007300 for all of them — losing the distinction
	// and naming no field.
	ctx := c.Request().Context()
	resp, err := h.tokenUsecase.GenerateB2BToken(ctx,
		c.Request().Header.Get("X-CLIENT-KEY"),
		c.Request().Header.Get("X-TIMESTAMP"),
		c.Request().Header.Get("X-SIGNATURE"),
		req.GrantType,
	)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			return c.JSON(tokenHTTPStatus(domainErr.SNAPCode), domain.SNAPTokenResponse{
				ResponseCode:    domainErr.SNAPCode,
				ResponseMessage: domainErr.Message,
			})
		}
		return c.JSON(http.StatusInternalServerError, domain.SNAPTokenResponse{
			ResponseCode:    domain.CodeTokenInternalError,
			ResponseMessage: "Internal Server Error",
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// tokenHTTPStatus maps an access-token responseCode to the HTTP status BCA's
// error table pairs it with. Derived from the code's own leading three digits
// rather than enumerated, so a code added to domain never silently defaults to
// 500 here — that is exactly how 4007301/4007302 would have been swallowed.
func tokenHTTPStatus(snapCode string) int {
	if len(snapCode) < 3 {
		return http.StatusInternalServerError
	}
	switch snapCode[:3] {
	case "400":
		return http.StatusBadRequest
	case "401":
		return http.StatusUnauthorized
	case "504":
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
