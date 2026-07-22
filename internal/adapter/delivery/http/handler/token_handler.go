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
