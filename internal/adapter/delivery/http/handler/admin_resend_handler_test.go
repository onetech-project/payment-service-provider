package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockResendCallbackUsecase is a mock implementation of domain.ResendCallbackUsecase
type MockResendCallbackUsecase struct {
	mock.Mock
}

func (m *MockResendCallbackUsecase) Resend(ctx context.Context, virtualAccountNo string) (*domain.ResendCallbackResult, error) {
	args := m.Called(ctx, virtualAccountNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ResendCallbackResult), args.Error(1)
}

func newAdminResendRequest(vaNo string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	httpReq := httptest.NewRequest(http.MethodPost, "/admin/transactions/"+vaNo+"/resend-callback", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)
	c.SetParamNames("virtualAccountNo")
	c.SetParamValues(vaNo)
	return c, rec
}

// T024: POST /admin/transactions/:virtualAccountNo/resend-callback returns
// 200/404/422 per contracts/resend-callback.md. The 401 (missing
// X-Admin-API-Key) case is covered by AdminAuthMiddleware, which runs
// upstream of this handler in the route chain (cmd/api/main.go) and is
// exercised by admin_auth middleware tests — this handler itself has no
// knowledge of the auth header.
func TestAdminResendHandler_Resend(t *testing.T) {
	t.Run("200 OK on success", func(t *testing.T) {
		c, rec := newAdminResendRequest("7000108212221111")
		mockUsecase := new(MockResendCallbackUsecase)
		resentAt := time.Date(2026, 7, 28, 10, 15, 0, 0, time.UTC)
		mockUsecase.On("Resend", mock.Anything, "7000108212221111").Return(&domain.ResendCallbackResult{
			VirtualAccountNo: "7000108212221111",
			EventType:        "va.expired",
			ResentAt:         resentAt,
			DeliveryStatus:   "success",
		}, nil)

		h := NewAdminResendHandler(mockUsecase)
		err := h.Resend(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]interface{}
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "7000108212221111", body["virtualAccountNo"])
		assert.Equal(t, "va.expired", body["eventType"])
		assert.Equal(t, "success", body["deliveryStatus"])
	})

	t.Run("404 when transaction not found", func(t *testing.T) {
		c, rec := newAdminResendRequest("does-not-exist")
		mockUsecase := new(MockResendCallbackUsecase)
		mockUsecase.On("Resend", mock.Anything, "does-not-exist").Return(nil, domain.ErrMerchantVANotFound)

		h := NewAdminResendHandler(mockUsecase)
		err := h.Resend(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		var body map[string]string
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "transaction not found", body["error"])
	})

	t.Run("422 when no delivery record exists", func(t *testing.T) {
		c, rec := newAdminResendRequest("va-no-delivery")
		mockUsecase := new(MockResendCallbackUsecase)
		mockUsecase.On("Resend", mock.Anything, "va-no-delivery").Return(nil, domain.ErrResendNoDeliveryRecord)

		h := NewAdminResendHandler(mockUsecase)
		err := h.Resend(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var body map[string]string
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "no callback delivery on record for this transaction", body["error"])
	})

	t.Run("422 when no notification URL registered", func(t *testing.T) {
		c, rec := newAdminResendRequest("va-no-url")
		mockUsecase := new(MockResendCallbackUsecase)
		mockUsecase.On("Resend", mock.Anything, "va-no-url").Return(nil, domain.ErrResendNoNotificationURL)

		h := NewAdminResendHandler(mockUsecase)
		err := h.Resend(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var body map[string]string
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "transaction has no registered notification URL", body["error"])
	})
}
