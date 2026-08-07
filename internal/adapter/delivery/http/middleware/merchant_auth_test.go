package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/crypto"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJWTIssuer is a local mock of domain.JWTIssuer for middleware tests
// (mirrors internal/usecase/token_usecase_test.go's MockJWTIssuer; not
// shared across packages since Go test files aren't importable).
type MockJWTIssuer struct {
	mock.Mock
}

func (m *MockJWTIssuer) GenerateB2BToken(clientID string, ttl time.Duration) (string, string, error) {
	args := m.Called(clientID, ttl)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockJWTIssuer) ValidateToken(tokenString string) (*domain.TokenClaims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenClaims), args.Error(1)
}

// MockMerchantClientRepository is a local mock of domain.ClientRepository
// for merchant_auth_test.go (feature 010-merchant-hmac-signature) — only
// GetActiveClientSecret is exercised by MerchantAuthMiddleware, but the full
// interface must be implemented.
type MockMerchantClientRepository struct {
	mock.Mock
}

func (m *MockMerchantClientRepository) GetClientByID(ctx context.Context, clientID string) (*domain.ClientApp, error) {
	panic("not used by these tests")
}

func (m *MockMerchantClientRepository) GetActiveClientPublicKey(ctx context.Context, clientID string) (string, error) {
	panic("not used by these tests")
}

func (m *MockMerchantClientRepository) CreateClient(ctx context.Context, client *domain.ClientApp) error {
	panic("not used by these tests")
}

func (m *MockMerchantClientRepository) CreateClientKey(ctx context.Context, key *domain.ClientKey) error {
	panic("not used by these tests")
}

func (m *MockMerchantClientRepository) RevokeClientKey(ctx context.Context, clientID, keyID string) error {
	panic("not used by these tests")
}

func (m *MockMerchantClientRepository) GetActiveClientSecret(ctx context.Context, clientID string) (string, error) {
	args := m.Called(ctx, clientID)
	return args.String(0), args.Error(1)
}

func (m *MockMerchantClientRepository) CreateClientSecret(ctx context.Context, secret *domain.ClientSecret) error {
	panic("not used by these tests")
}

func (m *MockMerchantClientRepository) RevokeClientSecret(ctx context.Context, clientID, secretID string) error {
	panic("not used by these tests")
}

// newSignedMerchantRequest builds a POST request carrying a Bearer token and
// a correctly-computed X-SIGNATURE per this feature's stringToSign
// convention — unlike the vendor side (snap_auth_test.go's
// newSignedRequest), the AccessToken component here is the REAL token, not
// an empty string.
func newSignedMerchantRequest(t *testing.T, path, body, token, secret, timestamp string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	// Minified body, matching the vendor side and SNAP.
	bodyHash := crypto.HashRequestBody([]byte(body), crypto.BodyHashBase64)
	stringToSign := crypto.BuildStringToSign(http.MethodPost, path, token, bodyHash, timestamp)
	signature := crypto.NewHMACSigner(secret, "HMAC-SHA512").Sign(stringToSign)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signature)
	return req
}

func TestMerchantAuthMiddleware_MissingAuthorizationHeader_Rejected(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/create-va", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockRepo := new(MockMerchantClientRepository)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockIssuer.AssertNotCalled(t, "ValidateToken", mock.Anything)
}

func TestMerchantAuthMiddleware_MalformedAuthorizationHeader_Rejected(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/create-va", nil)
	req.Header.Set("Authorization", "Basic abc123")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockRepo := new(MockMerchantClientRepository)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockIssuer.AssertNotCalled(t, "ValidateToken", mock.Anything)
}

func TestMerchantAuthMiddleware_InvalidToken_Rejected(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/create-va", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "bad-token").Return(nil, assert.AnError)
	mockRepo := new(MockMerchantClientRepository)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockIssuer.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "GetActiveClientSecret", mock.Anything, mock.Anything)
}

// --- Signature, Fail-Closed & Timestamp Freshness Tests (feature 010-merchant-hmac-signature) ---

func TestMerchantAuthMiddleware_ValidTokenAndSignature_PassesThrough(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "merchant-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
	mockIssuer.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestMerchantAuthMiddleware_ValidTokenInvalidSignature_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "merchant-secret", timestamp)
	req.Header.Set("X-SIGNATURE", "clearly-wrong-signature")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMerchantAuthMiddleware_HexEncodedSignature_Rejected(t *testing.T) {
	// Feature 012-base64-hash-encoding: a request signed with the old hex
	// convention (both bodyHash and HMAC signature) must be rejected now
	// that the server only accepts base64.
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	token := "good-token"
	secret := "merchant-secret"

	bodyHashHex := hex.EncodeToString(sha256Sum(body))
	stringToSign := crypto.BuildStringToSign(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", token, bodyHashHex, timestamp)
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	signatureHex := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", signatureHex)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", token).Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return(secret, nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called, "handler must not be invoked for a hex-signed (pre-migration) request")
}

func TestMerchantAuthMiddleware_ValidTokenMissingSignature_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("X-TIMESTAMP", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMerchantAuthMiddleware_NoProvisionedSecret_FailsClosed(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	// Signed against a secret the server doesn't have — proves fail-closed
	// triggers before signature verification is even attempted.
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "some-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "unprovisioned-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "unprovisioned-client").Return("", assert.AnError)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMerchantAuthMiddleware_StaleTimestamp_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "merchant-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMerchantAuthMiddleware_FutureTimestamp_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "merchant-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMerchantAuthMiddleware_TimestampWithinWindow_PassesThrough(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(4 * time.Minute).Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "merchant-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

func TestMerchantAuthMiddleware_SkewCheckSkippedWhenFlagSet(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "good-token", "merchant-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
	mockRepo := new(MockMerchantClientRepository)
	mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, true, false)
	called := false
	handler := middleware(func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

// --- Dual-Enforcement Combination Tests (US4) ---

func TestMerchantAuthMiddleware_InvalidTokenValidSignature_Rejected(t *testing.T) {
	e := echo.New()
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`
	// A signature that WOULD be valid if the token check passed — proves the
	// token check still short-circuits first, independent of signature.
	req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, "bad-token", "merchant-secret", timestamp)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "bad-token").Return(nil, assert.AnError)
	mockRepo := new(MockMerchantClientRepository)

	middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockRepo.AssertNotCalled(t, "GetActiveClientSecret", mock.Anything, mock.Anything)
}

func TestMerchantAuthMiddleware_AllFourCombinations(t *testing.T) {
	timestamp := time.Now().Format(time.RFC3339)
	body := `{"partnerServiceId":"088899"}`

	tests := []struct {
		name           string
		tokenValid     bool
		signatureValid bool
		wantStatus     int
	}{
		{"valid token, valid signature", true, true, http.StatusOK},
		{"valid token, invalid signature", true, false, http.StatusUnauthorized},
		{"invalid token, valid-looking signature", false, true, http.StatusUnauthorized},
		{"invalid token, invalid signature", false, false, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			token := "good-token"
			if !tt.tokenValid {
				token = "bad-token"
			}
			req := newSignedMerchantRequest(t, "/openapi/v1.0/transfer-va/create-va", body, token, "merchant-secret", timestamp)
			if !tt.signatureValid {
				req.Header.Set("X-SIGNATURE", "clearly-wrong-signature")
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			mockIssuer := new(MockJWTIssuer)
			mockRepo := new(MockMerchantClientRepository)
			if tt.tokenValid {
				mockIssuer.On("ValidateToken", token).Return(&domain.TokenClaims{ClientID: "test-client"}, nil)
				mockRepo.On("GetActiveClientSecret", mock.Anything, "test-client").Return("merchant-secret", nil)
			} else {
				mockIssuer.On("ValidateToken", token).Return(nil, assert.AnError)
			}

			middleware := MerchantAuthMiddleware(mockIssuer, mockRepo, false, false)
			handler := middleware(func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			err := handler(c)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
