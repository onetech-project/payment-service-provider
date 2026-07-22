package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backbone-new/internal/adapter/delivery/http/handler"
	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockTokenUsecase struct {
	mock.Mock
}

func (m *MockTokenUsecase) GenerateB2BToken(ctx context.Context, clientID, timestamp, signature, grantType string) (*domain.SNAPTokenResponse, error) {
	args := m.Called(ctx, clientID, timestamp, signature, grantType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SNAPTokenResponse), args.Error(1)
}

func (m *MockTokenUsecase) ValidateToken(ctx context.Context, tokenString string) (*domain.TokenClaims, error) {
	args := m.Called(ctx, tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenClaims), args.Error(1)
}

func TestTokenHandler_GetB2BAccessToken(t *testing.T) {
	e := echo.New()
	mockUC := new(MockTokenUsecase)
	h := handler.NewTokenHandler(mockUC)

	reqBody := []byte(`{"grantType":"client_credentials"}`)
	timestamp := time.Now().Format(time.RFC3339)

	t.Run("Missing Headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1.0/access-token/b2b", bytes.NewBuffer(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.GetB2BAccessToken(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp domain.SNAPTokenResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "4007300", resp.ResponseCode)
	})

	t.Run("Successful SNAP Token Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1.0/access-token/b2b", bytes.NewBuffer(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set("X-CLIENT-KEY", "client-001")
		req.Header.Set("X-TIMESTAMP", timestamp)
		req.Header.Set("X-SIGNATURE", "valid-signature")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("GenerateB2BToken", mock.Anything, "client-001", timestamp, "valid-signature", "client_credentials").Return(&domain.SNAPTokenResponse{
			ResponseCode:    "2007300",
			ResponseMessage: "Successful",
			AccessToken:     "jwt-token-sample",
			TokenType:       "Bearer",
			ExpiresIn:       "900",
		}, nil).Once()

		err := h.GetB2BAccessToken(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp domain.SNAPTokenResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "2007300", resp.ResponseCode)
		assert.Equal(t, "jwt-token-sample", resp.AccessToken)
	})
}
