package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"backbone-new/internal/domain"

	"github.com/hibiken/asynq"
)

const (
	TaskPaymentNotify = "merchant:payment:notify"
)

// PaymentNotificationWorker handles merchant payment notification delivery
type PaymentNotificationWorker struct {
	httpClient *http.Client
	secret     string
	recorder   domain.NotificationDeliveryResultRecorder
}

// NewPaymentNotificationWorker creates a new worker. recorder may be nil, in
// which case delivery outcomes are not written back to
// va_notification_deliveries.
func NewPaymentNotificationWorker(secret string, recorder domain.NotificationDeliveryResultRecorder) *PaymentNotificationWorker {
	return &PaymentNotificationWorker{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		secret:     secret,
		recorder:   recorder,
	}
}

// deliveryError is a delivery failure carrying the merchant's HTTP status,
// when it answered at all.
type deliveryError struct {
	httpStatus int
	err        error
}

func (e *deliveryError) Error() string { return e.err.Error() }
func (e *deliveryError) Unwrap() error { return e.err }

// HandlePaymentNotification processes payment notification tasks and records
// each attempt's outcome on the task's va_notification_deliveries row.
func (w *PaymentNotificationWorker) HandlePaymentNotification(ctx context.Context, t *asynq.Task) error {
	err := w.deliver(ctx, t)
	w.recordResult(ctx, t, err)
	return err
}

// recordResult writes the attempt's outcome onto the delivery row named by the
// task's delivery-id header. A task without one was enqueued before the header
// existed and has no row to update.
//
// A failure with retries left is "retrying", not "failed": Asynq will try
// again, and only the last attempt decides. A timeout is recorded with its
// error text ("context deadline exceeded" / "Client.Timeout exceeded") and no
// HTTP status, since the merchant never answered.
func (w *PaymentNotificationWorker) recordResult(ctx context.Context, t *asynq.Task, err error) {
	if w.recorder == nil {
		return
	}
	deliveryID := t.Headers()[domain.NotificationDeliveryIDHeader]
	if deliveryID == "" {
		return
	}

	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	result := domain.NotificationDeliveryResult{
		Status:       domain.NotificationDeliveryStatusSuccess,
		AttemptCount: retried + 1,
	}
	if err != nil {
		result.Status = domain.NotificationDeliveryStatusFailed
		if retried < maxRetry {
			result.Status = domain.NotificationDeliveryStatusRetrying
		}
		result.ErrorDetail = err.Error()
		var de *deliveryError
		if errors.As(err, &de) {
			result.HTTPStatus = de.httpStatus
		}
	} else {
		result.HTTPStatus = http.StatusOK
	}

	// The task context may already be past its deadline — that is exactly
	// the timeout case worth recording — so the write gets its own.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if rerr := w.recorder.UpdateDeliveryResult(recordCtx, deliveryID, result); rerr != nil {
		log.Printf("event=notification_delivery_record_failed delivery_id=%s err=%v", deliveryID, rerr)
	}
}

// deliver sends the callback once.
func (w *PaymentNotificationWorker) deliver(ctx context.Context, t *asynq.Task) error {
	var payload domain.PaymentNotificationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if payload.NotificationURL == "" {
		return fmt.Errorf("notification URL is empty")
	}

	// EventType defaults to "payment.received" for backward compatibility
	// with payloads enqueued before feature 007-merchant-expiry-callback
	// introduced the discriminator field.
	eventType := payload.EventType
	if eventType == "" {
		eventType = domain.NotificationEventPaymentReceived
	}

	// Build notification payload
	var notification map[string]interface{}
	if eventType == domain.NotificationEventVAExpired {
		notification = map[string]interface{}{
			"eventType": domain.NotificationEventVAExpired,
			"timestamp": time.Now().Format(time.RFC3339),
			"data": map[string]interface{}{
				"virtualAccountNo": payload.VirtualAccountNo,
				"customerNo":       payload.CustomerNo,
				"trxId":            payload.TrxID,
				"expiredAt":        payload.ExpiredAt,
				"status":           "02",
			},
		}
	} else {
		notification = map[string]interface{}{
			"eventType": domain.NotificationEventPaymentReceived,
			"timestamp": time.Now().Format(time.RFC3339),
			"data": map[string]interface{}{
				"virtualAccountNo": payload.VirtualAccountNo,
				"customerNo":       payload.CustomerNo,
				"trxId":            payload.TrxID,
				"paymentRequestId": payload.PaymentRequestID,
				"paidAmount":       payload.PaidAmount,
				"trxDateTime":      payload.TrxDateTime,
				"referenceNo":      payload.ReferenceNo,
				"status":           "00",
			},
		}
	}

	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.NotificationURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Generate HMAC-SHA512 signature
	timestamp := time.Now().Format(time.RFC3339)
	mac := hmac.New(sha512.New, []byte(w.secret))
	mac.Write(body)
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)

	// Execute request
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &deliveryError{
			httpStatus: resp.StatusCode,
			err:        fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	return nil
}

// RegisterWorker registers the payment notification worker with Asynq mux
func RegisterWorker(mux *asynq.ServeMux, worker *PaymentNotificationWorker) {
	mux.HandleFunc(TaskPaymentNotify, worker.HandlePaymentNotification)
}
