package handler

import (
	"errors"
	"io"
	"net/http"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
)

type SignatureHandler struct {
	signatureUsecase domain.SignatureUsecase
}

func NewSignatureHandler(signatureUsecase domain.SignatureUsecase) *SignatureHandler {
	return &SignatureHandler{signatureUsecase: signatureUsecase}
}

func mapSignatureError(c echo.Context, err error) error {
	var domainErr *domain.DomainError
	if errors.As(err, &domainErr) {
		statusCode := http.StatusInternalServerError
		switch domainErr.SNAPCode {
		case "4000000":
			statusCode = http.StatusBadRequest
		case "4010000":
			statusCode = http.StatusUnauthorized
		}
		return c.JSON(statusCode, domain.SignatureAuthResponse{
			ResponseCode:    domainErr.SNAPCode,
			ResponseMessage: domainErr.Message,
		})
	}
	return c.JSON(http.StatusInternalServerError, domain.SignatureAuthResponse{
		ResponseCode:    "5000000",
		ResponseMessage: "Internal Server Error",
	})
}

// GenerateAccessTokenSignature implements POST /api/v1/utilities/signature-auth:
// generates the asymmetric (SHA256withRSA) signature used to sign
// Access Token B2B requests.
// GenerateAccessTokenSignature godoc
// @Tags Signature Utilities
// @Summary Generate the asymmetric access-token signature
// @Description Generates the SHA256withRSA signature required as X-SIGNATURE when calling POST /openapi/v1.0/access-token/b2b.
// @Param X-CLIENT-KEY header string true "Client identifier"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param Private_Key header string true "RSA private key (PEM) used to sign the string-to-sign"
// @Success 200 {object} domain.SignatureAuthResponse
// @Failure 400 {object} domain.SignatureAuthResponse "Bad Request"
// @Failure 401 {object} domain.SignatureAuthResponse "Unauthorized"
// @Failure 500 {object} domain.SignatureAuthResponse "Internal Server Error"
// @Router /api/v1/utilities/signature-auth [post]
func (h *SignatureHandler) GenerateAccessTokenSignature(c echo.Context) error {
	clientKey := c.Request().Header.Get("X-CLIENT-KEY")
	timestamp := c.Request().Header.Get("X-TIMESTAMP")
	privateKey := c.Request().Header.Get("Private_Key")

	ctx := c.Request().Context()
	resp, err := h.signatureUsecase.GenerateAccessTokenSignature(ctx, clientKey, timestamp, privateKey)
	if err != nil {
		return mapSignatureError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GenerateServiceSignature implements POST /api/v1/utilities/signature-service:
// generates the symmetric (HMAC-SHA512) signature used to sign transaction
// API calls made with an issued access token.
// GenerateServiceSignature godoc
// @Tags Signature Utilities
// @Summary Generate the symmetric service (transaction) signature
// @Description Generates the HMAC-SHA512 signature required as X-SIGNATURE when calling SNAP-protected transaction endpoints (transfer-VA, merchant VA dashboard) with an issued access token.
// @Param X-CLIENT-SECRET header string true "Client secret shared with the partner"
// @Param HttpMethod header string true "HTTP method of the target request, e.g. POST"
// @Param EndpointUrl header string true "Relative path of the target request, e.g. /openapi/v1.0/transfer-va/inquiry"
// @Param AccessToken header string true "Bearer access token issued by POST /openapi/v1.0/access-token/b2b"
// @Param X-TIMESTAMP header string true "Request timestamp, ISO 8601"
// @Param request body object true "Raw request body of the target request, used verbatim in the string-to-sign"
// @Success 200 {object} domain.SignatureServiceResponse
// @Failure 400 {object} domain.SignatureServiceResponse "Bad Request"
// @Failure 401 {object} domain.SignatureServiceResponse "Unauthorized"
// @Failure 500 {object} domain.SignatureServiceResponse "Internal Server Error"
// @Router /api/v1/utilities/signature-service [post]
func (h *SignatureHandler) GenerateServiceSignature(c echo.Context) error {
	clientSecret := c.Request().Header.Get("X-CLIENT-SECRET")
	httpMethod := c.Request().Header.Get("HttpMethod")
	endpointURL := c.Request().Header.Get("EndpointUrl")
	accessToken := c.Request().Header.Get("AccessToken")
	timestamp := c.Request().Header.Get("X-TIMESTAMP")

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, domain.SignatureServiceResponse{
			ResponseCode:    "4000000",
			ResponseMessage: "Bad Request. Invalid request payload format.",
		})
	}

	ctx := c.Request().Context()
	resp, err := h.signatureUsecase.GenerateServiceSignature(ctx, clientSecret, httpMethod, endpointURL, accessToken, timestamp, body)
	if err != nil {
		return mapSignatureError(c, err)
	}
	return c.JSON(http.StatusOK, resp)
}
