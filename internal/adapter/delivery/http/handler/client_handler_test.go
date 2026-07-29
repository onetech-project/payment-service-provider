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

type MockClientUsecase struct {
	mock.Mock
}

func (m *MockClientUsecase) RegisterClient(ctx context.Context, client *domain.ClientApp, key *domain.ClientKey) error {
	args := m.Called(ctx, client, key)
	return args.Error(0)
}

func (m *MockClientUsecase) AddClientKey(ctx context.Context, key *domain.ClientKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockClientUsecase) RevokeClientKey(ctx context.Context, clientID, keyID string) error {
	args := m.Called(ctx, clientID, keyID)
	return args.Error(0)
}

func (m *MockClientUsecase) AddClientSecret(ctx context.Context, secret *domain.ClientSecret) error {
	args := m.Called(ctx, secret)
	return args.Error(0)
}

func (m *MockClientUsecase) RevokeClientSecret(ctx context.Context, clientID, secretID string) error {
	args := m.Called(ctx, clientID, secretID)
	return args.Error(0)
}

const testRSAPubPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtcbGGNSyv5wZjDNtEQIF
JSxUspXV5oO/g9u6ZGgo9rNOrf28Iu2mVONaPYmUlYoNFqkS1ljpLoz+6mrH3mpB
Q0YwKKnjRWYQScSQr1wr2cPbEyTX/vmnK5kBe9E86ox3E2l4gr+Ey4AcYAhWbZ7T
JUNsR8YUx7xB8XKO5V45EqQNmdRX9qJImNEMQW28gUV9xI1Ys52/hNmB1FA5vlLB
pS8qWOeWz5M+Rhme4Gk3MURSLvdDDLhvHJ6o+BlWhUCHOxbKp0BkOp5xi9Lu/HjC
VdwzwduwQAdNWoHcdyi+Wq0B2SL5AUtfs2j8vhv0QgcFrTAF1z3NB3Y/3SJf59Zk
fwIDAQAB
-----END PUBLIC KEY-----`

func TestClientHandler_RegisterClient(t *testing.T) {
	e := echo.New()

	t.Run("Success without key", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.RegisterClientRequest{ClientID: "c1", ClientName: "Client One"})
		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("RegisterClient", mock.Anything, mock.Anything, (*domain.ClientKey)(nil)).Return(nil).Once()

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		mockUC.AssertExpectations(t)
	})

	t.Run("Invalid payload", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader([]byte(`{"clientId":`)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Missing clientId/clientName", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.RegisterClientRequest{ClientID: "c1"})
		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("PublicKeyPEM without keyId", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.RegisterClientRequest{ClientID: "c1", ClientName: "Name", PublicKeyPEM: testRSAPubPEM})
		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Invalid publicKeyPem", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.RegisterClientRequest{ClientID: "c1", ClientName: "Name", KeyID: "k1", PublicKeyPEM: "not-a-pem"})
		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Success with valid key", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.RegisterClientRequest{ClientID: "c1", ClientName: "Name", KeyID: "k1", PublicKeyPEM: testRSAPubPEM})
		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("RegisterClient", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase error", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.RegisterClientRequest{ClientID: "c1", ClientName: "Name"})
		req := httptest.NewRequest(http.MethodPost, "/admin/clients", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockUC.On("RegisterClient", mock.Anything, mock.Anything, (*domain.ClientKey)(nil)).Return(assert.AnError).Once()

		err := h.RegisterClient(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		mockUC.AssertExpectations(t)
	})
}

func TestClientHandler_AddClientKey(t *testing.T) {
	e := echo.New()

	newCtx := func(body []byte, clientID string) (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodPost, "/admin/clients/"+clientID+"/keys", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("clientId")
		c.SetParamValues(clientID)
		return c, rec
	}

	t.Run("Missing clientId path param", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/admin/clients//keys", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.AddClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Invalid payload", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx([]byte(`{"keyId":`), "c1")
		err := h.AddClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Missing keyId/publicKeyPem", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientKeyRequest{KeyID: "k1"})
		c, rec := newCtx(body, "c1")
		err := h.AddClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Invalid publicKeyPem", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientKeyRequest{KeyID: "k1", PublicKeyPEM: "bad"})
		c, rec := newCtx(body, "c1")
		err := h.AddClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Success", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientKeyRequest{KeyID: "k1", PublicKeyPEM: testRSAPubPEM})
		c, rec := newCtx(body, "c1")

		mockUC.On("AddClientKey", mock.Anything, mock.Anything).Return(nil).Once()

		err := h.AddClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase error", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientKeyRequest{KeyID: "k1", PublicKeyPEM: testRSAPubPEM})
		c, rec := newCtx(body, "c1")

		mockUC.On("AddClientKey", mock.Anything, mock.Anything).Return(assert.AnError).Once()

		err := h.AddClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		mockUC.AssertExpectations(t)
	})
}

func TestClientHandler_RevokeClientKey(t *testing.T) {
	e := echo.New()

	newCtx := func(clientID, keyID string) (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/clients/"+clientID+"/keys/"+keyID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("clientId", "keyId")
		c.SetParamValues(clientID, keyID)
		return c, rec
	}

	t.Run("Missing path params", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx("", "")
		err := h.RevokeClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Success", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx("c1", "k1")
		mockUC.On("RevokeClientKey", mock.Anything, "c1", "k1").Return(nil).Once()

		err := h.RevokeClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase error", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx("c1", "k1")
		mockUC.On("RevokeClientKey", mock.Anything, "c1", "k1").Return(assert.AnError).Once()

		err := h.RevokeClientKey(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		mockUC.AssertExpectations(t)
	})
}

func TestClientHandler_AddClientSecret(t *testing.T) {
	e := echo.New()

	newCtx := func(body []byte, clientID string) (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodPost, "/admin/clients/"+clientID+"/secret", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("clientId")
		c.SetParamValues(clientID)
		return c, rec
	}

	t.Run("Missing clientId path param", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/admin/clients//secret", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.AddClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Invalid payload", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx([]byte(`{"secretId":`), "c1")
		err := h.AddClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Missing secretId/secretValue", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientSecretRequest{SecretID: "s1"})
		c, rec := newCtx(body, "c1")
		err := h.AddClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Success", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientSecretRequest{SecretID: "s1", SecretValue: "shh"})
		c, rec := newCtx(body, "c1")

		mockUC.On("AddClientSecret", mock.Anything, mock.Anything).Return(nil).Once()

		err := h.AddClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		mockUC.AssertExpectations(t)

		var resp map[string]interface{}
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.NotContains(t, rec.Body.String(), "shh", "secretValue must never be echoed back in the response")
	})

	t.Run("Usecase error", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		body, _ := json.Marshal(domain.AddClientSecretRequest{SecretID: "s1", SecretValue: "shh"})
		c, rec := newCtx(body, "c1")
		mockUC.On("AddClientSecret", mock.Anything, mock.Anything).Return(assert.AnError).Once()

		err := h.AddClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		mockUC.AssertExpectations(t)
	})
}

func TestClientHandler_RevokeClientSecret(t *testing.T) {
	e := echo.New()

	newCtx := func(clientID, secretID string) (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/clients/"+clientID+"/secret/"+secretID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("clientId", "secretId")
		c.SetParamValues(clientID, secretID)
		return c, rec
	}

	t.Run("Missing path params", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx("", "")
		err := h.RevokeClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Success", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx("c1", "s1")
		mockUC.On("RevokeClientSecret", mock.Anything, "c1", "s1").Return(nil).Once()

		err := h.RevokeClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		mockUC.AssertExpectations(t)
	})

	t.Run("Usecase error", func(t *testing.T) {
		mockUC := new(MockClientUsecase)
		h := handler.NewClientHandler(mockUC)

		c, rec := newCtx("c1", "s1")
		mockUC.On("RevokeClientSecret", mock.Anything, "c1", "s1").Return(assert.AnError).Once()

		err := h.RevokeClientSecret(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		mockUC.AssertExpectations(t)
	})
}
