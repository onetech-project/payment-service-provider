package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func (m *MockResendCallbackUsecase) ResendRange(ctx context.Context, filter domain.NotificationDeliveryFilter) (*domain.BulkResendResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.BulkResendResult), args.Error(1)
}

func newAdminResendRangeRequest(body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	httpReq := httptest.NewRequest(http.MethodPost, "/admin/callbacks/resend", strings.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(httpReq, rec), rec
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

func TestAdminResendHandler_ResendRange(t *testing.T) {
	t.Run("200 passes the parsed filter and summarises the outcome", func(t *testing.T) {
		c, rec := newAdminResendRangeRequest(`{"startTime":"2026-10-27T00:00:00+07:00","endTime":"2026-10-28T00:00:00+07:00","virtualAccountNo":"7000108212221111","status":["failed"]}`)
		mockUsecase := new(MockResendCallbackUsecase)
		mockUsecase.On("ResendRange", mock.Anything, mock.MatchedBy(func(f domain.NotificationDeliveryFilter) bool {
			return f.From.Equal(time.Date(2026, 10, 26, 17, 0, 0, 0, time.UTC)) &&
				f.To.Equal(time.Date(2026, 10, 27, 17, 0, 0, 0, time.UTC)) &&
				f.VirtualAccountNo == "7000108212221111" &&
				len(f.Statuses) == 1 && f.Statuses[0] == "failed"
		})).Return(&domain.BulkResendResult{
			ResentAt: time.Now(),
			Items: []domain.BulkResendItem{
				{SourceDeliveryID: "a", DeliveryID: "a2", Status: "queued"},
				{SourceDeliveryID: "b", Status: "skipped", Error: "original payload not recorded"},
				{SourceDeliveryID: "c", DeliveryID: "c2", Status: "failed", Error: "redis down"},
			},
		}, nil)

		err := NewAdminResendHandler(mockUsecase).ResendRange(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]interface{}
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.EqualValues(t, 3, body["matched"])
		assert.EqualValues(t, 1, body["queued"])
		assert.EqualValues(t, 1, body["skipped"])
		assert.EqualValues(t, 1, body["failed"])
		mockUsecase.AssertExpectations(t)
	})

	for _, tc := range []struct{ name, body, wantErr string }{
		{"400 on malformed body", `{"startTime":`, "invalid request payload"},
		{"400 on missing startTime", `{"endTime":"2026-10-28T00:00:00+07:00"}`, "startTime must be RFC 3339"},
		{"400 on reversed range", `{"startTime":"2026-10-28T00:00:00+07:00","endTime":"2026-10-27T00:00:00+07:00"}`, "startTime must be before endTime"},
		{"400 on range over 31 days", `{"startTime":"2026-01-01T00:00:00Z","endTime":"2026-03-01T00:00:00Z"}`, "range must not exceed 31 days"},
		{"400 on unknown status", `{"startTime":"2026-10-27T00:00:00Z","endTime":"2026-10-28T00:00:00Z","status":["timeout"]}`, "unknown status: timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newAdminResendRangeRequest(tc.body)
			mockUsecase := new(MockResendCallbackUsecase)

			err := NewAdminResendHandler(mockUsecase).ResendRange(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.wantErr)
			mockUsecase.AssertNotCalled(t, "ResendRange", mock.Anything, mock.Anything)
		})
	}

	t.Run("422 when the range matches too many callbacks", func(t *testing.T) {
		c, rec := newAdminResendRangeRequest(`{"startTime":"2026-10-27T00:00:00Z","endTime":"2026-10-28T00:00:00Z"}`)
		mockUsecase := new(MockResendCallbackUsecase)
		mockUsecase.On("ResendRange", mock.Anything, mock.Anything).Return(nil, domain.ErrResendRangeTooLarge)

		err := NewAdminResendHandler(mockUsecase).ResendRange(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}
