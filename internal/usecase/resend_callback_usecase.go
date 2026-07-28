package usecase

import (
	"context"
	"log"
	"time"

	"backbone-new/internal/domain"
)

// resendCallbackUsecase implements domain.ResendCallbackUsecase (feature
// 007-merchant-expiry-callback, US2). Lets an operator redeliver the most
// recent merchant callback event for a transaction on demand, recording
// every attempt (trigger="manual") in the notification delivery audit trail.
// Never mutates va_transactions.status (FR-019).
type resendCallbackUsecase struct {
	repo         domain.VARepository
	deliveryRepo domain.VANotificationDeliveryRepository
	notifier     domain.NotificationEnqueuer
}

// NewResendCallbackUsecase creates a new resend-callback usecase.
func NewResendCallbackUsecase(repo domain.VARepository, deliveryRepo domain.VANotificationDeliveryRepository, notifier domain.NotificationEnqueuer) domain.ResendCallbackUsecase {
	return &resendCallbackUsecase{repo: repo, deliveryRepo: deliveryRepo, notifier: notifier}
}

// Resend redelivers the most recent callback event on record for
// virtualAccountNo. See contracts/resend-callback.md for the full behavior
// contract (404/422 preconditions, payload rebuild from current VA state).
func (u *resendCallbackUsecase) Resend(ctx context.Context, virtualAccountNo string) (*domain.ResendCallbackResult, error) {
	// 1. VA must exist (FR-014).
	merchantVA, err := u.repo.GetVAByVirtualAccountNo(ctx, virtualAccountNo)
	if err != nil {
		return nil, domain.ErrMerchantVANotFound
	}

	// 2. A prior delivery-attempt record must exist for this VA (FR-015).
	latest, err := u.deliveryRepo.GetLatestByVirtualAccountNo(ctx, virtualAccountNo)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, domain.ErrResendNoDeliveryRecord
	}

	// 3. VA must have a registered notification URL (FR-016).
	if merchantVA.NotificationURL == "" {
		return nil, domain.ErrResendNoNotificationURL
	}

	// 4. Rebuild the payload from CURRENT VA state (not the historical
	// payload), using the most recent record's event type.
	payload := &domain.PaymentNotificationPayload{
		EventType:        latest.EventType,
		PartnerServiceID: merchantVA.PartnerServiceID,
		CustomerNo:       merchantVA.CustomerNo,
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		TrxID:            merchantVA.TrxID,
		NotificationURL:  merchantVA.NotificationURL,
	}
	if latest.EventType == domain.NotificationEventVAExpired && merchantVA.ExpiredDate != nil {
		payload.ExpiredAt = merchantVA.ExpiredDate.Format(time.RFC3339)
	}
	if latest.EventType == domain.NotificationEventPaymentReceived {
		payload.PaidAmount = &domain.Amount{Value: merchantVA.TotalAmount, Currency: merchantVA.Currency}
	}

	// 5. Enqueue via the existing NotificationEnqueuer (same signing/delivery
	// path as auto-triggered notifications).
	resentAt := time.Now()
	// TrxDateTime is stamped with the resend time so this payload's bytes
	// differ from the original auto-triggered delivery (and from any earlier
	// resend). Without this, a resend of the same event within Asynq's
	// asynq.Unique(5*time.Minute) window on Client.EnqueuePaymentNotification
	// hashes identically to the prior enqueue and is silently rejected as a
	// duplicate ("task already exists") — resends must always go through
	// (spec.md Edge Cases: "no dedup/rate-limit... each call is a deliberate
	// resend").
	payload.TrxDateTime = resentAt.Format(time.RFC3339)
	deliveryStatus := domain.NotificationDeliveryStatusSuccess
	errorDetail := ""
	if u.notifier == nil {
		deliveryStatus = domain.NotificationDeliveryStatusFailed
		errorDetail = "notification enqueuer unavailable"
	} else if err := u.notifier.EnqueuePaymentNotification(ctx, payload); err != nil {
		deliveryStatus = domain.NotificationDeliveryStatusFailed
		errorDetail = err.Error()
	}

	// 6. Record this attempt (trigger="manual") for audit (FR-018). Does NOT
	// touch va_transactions.status (FR-019).
	_ = u.deliveryRepo.Create(ctx, &domain.NotificationDelivery{
		VirtualAccountNo: virtualAccountNo,
		EventType:        latest.EventType,
		Trigger:          domain.NotificationTriggerManual,
		Status:           deliveryStatus,
		AttemptedAt:      resentAt,
		ErrorDetail:      errorDetail,
	})

	log.Printf("event=resend_callback virtual_account_no=%s event_type=%s trigger=manual status=%s", virtualAccountNo, latest.EventType, deliveryStatus)

	return &domain.ResendCallbackResult{
		VirtualAccountNo: virtualAccountNo,
		EventType:        latest.EventType,
		ResentAt:         resentAt,
		DeliveryStatus:   deliveryStatus,
	}, nil
}

var _ domain.ResendCallbackUsecase = (*resendCallbackUsecase)(nil)
