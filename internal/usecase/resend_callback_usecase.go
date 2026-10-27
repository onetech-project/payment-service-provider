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
		payload.PaymentRequestID = merchantVA.PaymentRequestID
	}

	// 5. Record (trigger="manual") and enqueue via the existing
	// NotificationEnqueuer — same signing/delivery path as auto-triggered
	// notifications (FR-018). The worker writes the outcome back onto the row.
	// Does NOT touch va_transactions.status (FR-019).
	resentAt := time.Now()
	// TrxDateTime is stamped with the resend time so this payload's bytes
	// differ from the original auto-triggered delivery (and from any earlier
	// resend) — kept from when manual resends shared the auto-triggered
	// uniqueness window; harmless now that they skip it.
	payload.TrxDateTime = resentAt.Format(time.RFC3339)
	_, deliveryStatus, _ := dispatchNotification(ctx, u.notifier, u.deliveryRepo, payload, virtualAccountNo, latest.EventType, domain.NotificationTriggerManual)

	log.Printf("event=resend_callback virtual_account_no=%s event_type=%s trigger=manual status=%s", virtualAccountNo, latest.EventType, deliveryStatus)

	return &domain.ResendCallbackResult{
		VirtualAccountNo: virtualAccountNo,
		EventType:        latest.EventType,
		ResentAt:         resentAt,
		DeliveryStatus:   deliveryStatus,
	}, nil
}

// MaxBulkResend caps how many callbacks one ResendRange call redelivers. A
// range matching more is refused outright rather than half-sent: the operator
// narrows it and nothing is left wondering which part went out.
const MaxBulkResend = 500

// ResendRange redelivers every original (trigger="auto") callback recorded in
// filter's time range, optionally for one VA.
//
// Each resend reuses the payload the original was enqueued with, not the VA's
// current state: a VA is reusable across transaction cycles, so rebuilding
// from current state would resend the wrong cycle's data. Rows recorded
// before payloads were stored (migration 000018) have nothing to resend from
// and are reported as skipped.
func (u *resendCallbackUsecase) ResendRange(ctx context.Context, filter domain.NotificationDeliveryFilter) (*domain.BulkResendResult, error) {
	filter.Limit = MaxBulkResend + 1
	originals, err := u.deliveryRepo.ListForResend(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(originals) > MaxBulkResend {
		return nil, domain.ErrResendRangeTooLarge
	}

	result := &domain.BulkResendResult{Items: make([]domain.BulkResendItem, 0, len(originals)), ResentAt: time.Now()}
	for _, original := range originals {
		item := domain.BulkResendItem{
			SourceDeliveryID: original.ID,
			VirtualAccountNo: original.VirtualAccountNo,
			EventType:        original.EventType,
		}
		switch {
		case original.Payload == nil:
			item.Status = "skipped"
			item.Error = "original payload not recorded"
		case original.Payload.NotificationURL == "":
			item.Status = "skipped"
			item.Error = domain.ErrResendNoNotificationURL.Error()
		default:
			payload := *original.Payload
			item.DeliveryID, item.Status, item.Error = dispatchNotification(ctx, u.notifier, u.deliveryRepo, &payload, original.VirtualAccountNo, original.EventType, domain.NotificationTriggerManual)
		}
		result.Items = append(result.Items, item)
	}

	log.Printf("event=resend_callback_range from=%s to=%s virtual_account_no=%s matched=%d trigger=manual",
		filter.From.Format(time.RFC3339), filter.To.Format(time.RFC3339), filter.VirtualAccountNo, len(originals))
	return result, nil
}

var _ domain.ResendCallbackUsecase = (*resendCallbackUsecase)(nil)
