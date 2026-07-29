package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backbone-new/internal/domain"

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

func TestMerchantAuthMiddleware_MissingAuthorizationHeader_Rejected(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/create-va", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)

	middleware := MerchantAuthMiddleware(mockIssuer)
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

	middleware := MerchantAuthMiddleware(mockIssuer)
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

	middleware := MerchantAuthMiddleware(mockIssuer)
	handler := middleware(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockIssuer.AssertExpectations(t)
}

func TestMerchantAuthMiddleware_ValidToken_PassesThrough(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/create-va", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockIssuer := new(MockJWTIssuer)
	mockIssuer.On("ValidateToken", "good-token").Return(&domain.TokenClaims{ClientID: "test-client"}, nil)

	middleware := MerchantAuthMiddleware(mockIssuer)
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
}
