package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backbone-new/internal/adapter/delivery/http/handler"
	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSignatureUsecase struct {
	mock.Mock
}

func (m *MockSignatureUsecase) GenerateAccessTokenSignature(ctx context.Context, clientKey, timestamp, privateKey string) (*domain.SignatureAuthResponse, error) {
	args := m.Called(ctx, clientKey, timestamp, privateKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SignatureAuthResponse), args.Error(1)
}

func (m *MockSignatureUsecase) GenerateServiceSignature(ctx context.Context, clientSecret, httpMethod, endpointURL, accessToken, timestamp string, requestBody []byte) (*domain.SignatureServiceResponse, error) {
	args := m.Called(ctx, clientSecret, httpMethod, endpointURL, accessToken, timestamp, requestBody)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SignatureServiceResponse), args.Error(1)
}

func TestSignatureHandler_GenerateAccessTokenSignature(t *testing.T) {
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		mockUC := new(MockSignatureUsecase)
		h := handler.NewSignatureHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/signature-auth", nil)
		req.Header.Set("X-CLIENT-KEY", "client-1")
		req.Header.Set("X-TIMESTAMP", "2024-01-01T00:00:00Z")
		req.Header.Set("X-PRIVATE-KEY", "priv-key")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("GenerateAccessTokenSignature", mock.Anything, "client-1", "2024-01-01T00:00:00Z", "priv-key").
			Return(&domain.SignatureAuthResponse{ResponseCode: "2000000", ResponseMessage: "Successful", Signature: "sig"}, nil).Once()

		err := h.GenerateAccessTokenSignature(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp domain.SignatureAuthResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "sig", resp.Signature)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase Bad Request error", func(t *testing.T) {
		mockUC := new(MockSignatureUsecase)
		h := handler.NewSignatureHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/signature-auth", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("GenerateAccessTokenSignature", mock.Anything, "", "", "").
			Return(nil, domain.NewDomainError("4000000", "Bad Request. Missing required fields", domain.ErrMissingHeader)).Once()

		err := h.GenerateAccessTokenSignature(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var resp domain.SignatureAuthResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "4000000", resp.ResponseCode)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase generic error maps to 500", func(t *testing.T) {
		mockUC := new(MockSignatureUsecase)
		h := handler.NewSignatureHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/signature-auth", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("GenerateAccessTokenSignature", mock.Anything, "", "", "").
			Return(nil, assert.AnError).Once()

		err := h.GenerateAccessTokenSignature(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var resp domain.SignatureAuthResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "5000000", resp.ResponseCode)
		mockUC.AssertExpectations(t)
	})
}

func TestSignatureHandler_GenerateServiceSignature(t *testing.T) {
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		mockUC := new(MockSignatureUsecase)
		h := handler.NewSignatureHandler(mockUC)

		body := []byte(`{"a":1}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/signature-service", bytes.NewReader(body))
		req.Header.Set("X-CLIENT-SECRET", "secret")
		req.Header.Set("HttpMethod", "POST")
		req.Header.Set("EndpointUrl", "/openapi/v1.0/transfer-va/inquiry")
		req.Header.Set("AccessToken", "token")
		req.Header.Set("X-TIMESTAMP", "2024-01-01T00:00:00Z")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("GenerateServiceSignature", mock.Anything, "secret", "POST", "/openapi/v1.0/transfer-va/inquiry", "token", "2024-01-01T00:00:00Z", body).
			Return(&domain.SignatureServiceResponse{ResponseCode: "2000000", ResponseMessage: "Successful", Signature: "hmac-sig"}, nil).Once()

		err := h.GenerateServiceSignature(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp domain.SignatureServiceResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "hmac-sig", resp.Signature)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase error", func(t *testing.T) {
		mockUC := new(MockSignatureUsecase)
		h := handler.NewSignatureHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/signature-service", bytes.NewReader(nil))
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("GenerateServiceSignature", mock.Anything, "", "", "", "", "", []byte{}).
			Return(nil, domain.NewDomainError("4000000", "Bad Request. Missing required fields", domain.ErrMissingHeader)).Once()

		err := h.GenerateServiceSignature(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		mockUC.AssertExpectations(t)
	})
}
