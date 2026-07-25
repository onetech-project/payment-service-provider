package worker

import (
	"context"
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

func TestRegisterWorker(t *testing.T) {
	mux := asynq.NewServeMux()
	w := NewPaymentNotificationWorker("secret")
	RegisterWorker(mux, w)
	// No panic means the handler was registered successfully.
}
