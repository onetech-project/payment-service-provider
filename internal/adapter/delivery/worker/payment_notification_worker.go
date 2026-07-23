package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
}

// NewPaymentNotificationWorker creates a new worker
func NewPaymentNotificationWorker(secret string) *PaymentNotificationWorker {
	return &PaymentNotificationWorker{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		secret:     secret,
	}
}

// HandlePaymentNotification processes payment notification tasks
func (w *PaymentNotificationWorker) HandlePaymentNotification(ctx context.Context, t *asynq.Task) error {
	var payload domain.PaymentNotificationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if payload.NotificationURL == "" {
		return fmt.Errorf("notification URL is empty")
	}

	// Build notification payload
	notification := map[string]interface{}{
		"eventType": "payment.received",
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
	signature := hex.EncodeToString(mac.Sum(nil))

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
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RegisterWorker registers the payment notification worker with Asynq mux
func RegisterWorker(mux *asynq.ServeMux, worker *PaymentNotificationWorker) {
	mux.HandleFunc(TaskPaymentNotify, worker.HandlePaymentNotification)
}
