package usecase

import (
	"context"
	"errors"
	"time"

	"backbone-new/internal/domain"

	"github.com/google/uuid"
)

// dispatchNotification records a merchant callback in
// va_notification_deliveries and enqueues it, returning the delivery row id
// and the status the row was left in.
//
// The row is written BEFORE the enqueue, as "queued" and carrying the full
// payload: the worker writes each attempt's outcome back onto it by id
// (domain.NotificationDeliveryIDHeader), and could otherwise race ahead of an
// insert that has not happened yet. Only an enqueue failure is settled here;
// "success" is the worker's word, never the enqueue's.
//
// Best-effort like every callback in this service — a failed insert does not
// stop the enqueue, and nothing here is returned as an error.
func dispatchNotification(
	ctx context.Context,
	notifier domain.NotificationEnqueuer,
	deliveryRepo domain.VANotificationDeliveryRepository,
	payload *domain.PaymentNotificationPayload,
	virtualAccountNo, eventType, trigger string,
) (deliveryID, status, errorDetail string) {
	deliveryID = uuid.New().String()
	payload.DeliveryID = deliveryID
	payload.Trigger = trigger

	if deliveryRepo != nil {
		_ = deliveryRepo.Create(ctx, &domain.NotificationDelivery{
			ID:               deliveryID,
			VirtualAccountNo: virtualAccountNo,
			EventType:        eventType,
			Trigger:          trigger,
			Status:           domain.NotificationDeliveryStatusQueued,
			AttemptedAt:      time.Now(),
			Payload:          payload,
		})
	}

	status = domain.NotificationDeliveryStatusQueued
	var err error
	if notifier == nil {
		err = errors.New("notification enqueuer unavailable")
	} else {
		err = notifier.EnqueuePaymentNotification(ctx, payload)
	}
	if err == nil {
		return deliveryID, status, ""
	}

	status = domain.NotificationDeliveryStatusFailed
	if errors.Is(err, domain.ErrNotificationDuplicate) {
		status = domain.NotificationDeliveryStatusDuplicate
	}
	errorDetail = err.Error()
	if deliveryRepo != nil {
		_ = deliveryRepo.UpdateDeliveryResult(ctx, deliveryID, domain.NotificationDeliveryResult{
			Status:      status,
			ErrorDetail: errorDetail,
		})
	}
	return deliveryID, status, errorDetail
}
