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
//
// trxID optionally pins the transaction cycle: a virtualAccountNo is reusable
// (MerchantVAUsecase.CreateVA), and without it the payload is rebuilt from
// whichever cycle GetVAByVirtualAccountNo ranks first — the open bill, not the
// settled one whose callback the merchant missed.
func (u *resendCallbackUsecase) Resend(ctx context.Context, virtualAccountNo, trxID string) (*domain.ResendCallbackResult, error) {
	// 1. VA must exist (FR-014).
	var merchantVA *domain.VAInquiryRecord
	var err error
	if trxID != "" {
		merchantVA, err = u.repo.GetVAByVirtualAccountNoAndTrxID(ctx, virtualAccountNo, trxID)
	} else {
		merchantVA, err = u.repo.GetVAByVirtualAccountNo(ctx, virtualAccountNo)
	}
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
	eventType := resendEventType(latest.EventType, merchantVA, trxID)
	payload := &domain.PaymentNotificationPayload{
		EventType:        eventType,
		PartnerServiceID: merchantVA.PartnerServiceID,
		CustomerNo:       merchantVA.CustomerNo,
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		TrxID:            merchantVA.TrxID,
		NotificationURL:  merchantVA.NotificationURL,
	}
	if eventType == domain.NotificationEventVAExpired && merchantVA.ExpiredDate != nil {
		payload.ExpiredAt = merchantVA.ExpiredDate.Format(time.RFC3339)
	}
	if eventType == domain.NotificationEventPaymentReceived {
		payload.PaidAmount = &domain.Amount{Value: merchantVA.TotalAmount, Currency: merchantVA.Currency}
		payload.PaymentRequestID = merchantVA.PaymentRequestID
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
		EventType:        eventType,
		Trigger:          domain.NotificationTriggerManual,
		Status:           deliveryStatus,
		AttemptedAt:      resentAt,
		ErrorDetail:      errorDetail,
	})

	log.Printf("event=resend_callback virtual_account_no=%s trx_id=%s event_type=%s trigger=manual status=%s", virtualAccountNo, merchantVA.TrxID, eventType, deliveryStatus)

	return &domain.ResendCallbackResult{
		VirtualAccountNo: virtualAccountNo,
		TrxID:            merchantVA.TrxID,
		EventType:        eventType,
		ResentAt:         resentAt,
		DeliveryStatus:   deliveryStatus,
	}, nil
}

// resendEventType picks the event to redeliver. Without trxID it is the most
// recent delivery on record for the VA (contract step 4). With trxID that
// record may belong to a different cycle of the same reused VA — delivery rows
// carry no trxId — so a cycle whose own status already says paid ('00') or
// expired ('02') is resent as that event instead; any other status falls back
// to the most recent record.
func resendEventType(latestEventType string, merchantVA *domain.VAInquiryRecord, trxID string) string {
	if trxID == "" {
		return latestEventType
	}
	switch merchantVA.Status {
	case "00":
		return domain.NotificationEventPaymentReceived
	case "02":
		return domain.NotificationEventVAExpired
	}
	return latestEventType
}

var _ domain.ResendCallbackUsecase = (*resendCallbackUsecase)(nil)
