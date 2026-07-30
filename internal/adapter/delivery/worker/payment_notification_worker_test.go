package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backbone-new/internal/domain"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
)

func newTask(t *testing.T, payload *domain.PaymentNotificationPayload) *asynq.Task {
	t.Helper()
	b, err := json.Marshal(payload)
	assert.NoError(t, err)
	return asynq.NewTask(TaskPaymentNotify, b)
}

func TestPaymentNotificationWorker_HandlePaymentNotification(t *testing.T) {
	t.Run("Invalid payload JSON", func(t *testing.T) {
		w := NewPaymentNotificationWorker("secret")
		task := asynq.NewTask(TaskPaymentNotify, []byte("not-json"))
		err := w.HandlePaymentNotification(context.Background(), task)
		assert.Error(t, err)
	})

	t.Run("Empty notification URL", func(t *testing.T) {
		w := NewPaymentNotificationWorker("secret")
		task := newTask(t, &domain.PaymentNotificationPayload{VirtualAccountNo: "va1"})
		err := w.HandlePaymentNotification(context.Background(), task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notification URL is empty")
	})

	t.Run("Success", func(t *testing.T) {
		var receivedSig, receivedTS string
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			receivedSig = r.Header.Get("X-Signature")
			receivedTS = r.Header.Get("X-Timestamp")
			rw.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		w := NewPaymentNotificationWorker("secret")
		task := newTask(t, &domain.PaymentNotificationPayload{
			VirtualAccountNo: "va1",
			NotificationURL:  srv.URL,
		})
		err := w.HandlePaymentNotification(context.Background(), task)
		assert.NoError(t, err)
		assert.NotEmpty(t, receivedSig)
		assert.NotEmpty(t, receivedTS)
		// Feature 012-base64-hash-encoding: X-Signature must be standard
		// base64 (HMAC-SHA512 -> 64 bytes -> 88 chars incl. padding), not hex.
		assert.Len(t, receivedSig, 88)
		_, decodeErr := base64.StdEncoding.DecodeString(receivedSig)
		assert.NoError(t, decodeErr, "X-Signature must be valid standard base64")
	})

	t.Run("Notification endpoint returns error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("boom"))
		}))
		defer srv.Close()

		w := NewPaymentNotificationWorker("secret")
		task := newTask(t, &domain.PaymentNotificationPayload{
			VirtualAccountNo: "va1",
			NotificationURL:  srv.URL,
		})
		err := w.HandlePaymentNotification(context.Background(), task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notification failed with status")
	})

	t.Run("Request creation fails for invalid URL", func(t *testing.T) {
		w := NewPaymentNotificationWorker("secret")
		task := newTask(t, &domain.PaymentNotificationPayload{
			VirtualAccountNo: "va1",
			NotificationURL:  "://bad-url",
		})
		err := w.HandlePaymentNotification(context.Background(), task)
		assert.Error(t, err)
	})
}

// T013: the worker correctly signs and delivers a va.expired-typed payload
// (HMAC-SHA512, X-Timestamp/X-Signature) with no paid-amount fields present.
func TestPaymentNotificationWorker_HandlePaymentNotification_VAExpiredEvent(t *testing.T) {
	var receivedSig, receivedTS string
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature")
		receivedTS = r.Header.Get("X-Timestamp")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := NewPaymentNotificationWorker("secret")
	task := newTask(t, &domain.PaymentNotificationPayload{
		EventType:        domain.NotificationEventVAExpired,
		VirtualAccountNo: "va-expired-1",
		CustomerNo:       "cust-1",
		TrxID:            "trx-1",
		ExpiredAt:        "2026-07-28T10:00:00Z",
		NotificationURL:  srv.URL,
	})
	err := w.HandlePaymentNotification(context.Background(), task)

	assert.NoError(t, err)
	assert.NotEmpty(t, receivedSig)
	assert.NotEmpty(t, receivedTS)
	assert.Equal(t, domain.NotificationEventVAExpired, receivedBody["eventType"])
	data, ok := receivedBody["data"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "va-expired-1", data["virtualAccountNo"])
	assert.Equal(t, "2026-07-28T10:00:00Z", data["expiredAt"])
	_, hasPaidAmount := data["paidAmount"]
	assert.False(t, hasPaidAmount, "va.expired payload must not carry paid-amount fields")
}

func TestRegisterWorker(t *testing.T) {
	mux := asynq.NewServeMux()
	w := NewPaymentNotificationWorker("secret")
	RegisterWorker(mux, w)
	// No panic means the handler was registered successfully.
}
