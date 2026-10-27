package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// T019: resend succeeds for a VA with a prior delivery record, redelivers via
// NotificationEnqueuer, and records a new trigger="manual" row.
func TestResendCallbackUsecase_Resend_Success(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	expiredAt := time.Now().Add(-time.Hour)
	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		NotificationURL:  "https://merchant.example.com/callback",
		ExpiredDate:      &expiredAt,
	}
	latest := &domain.NotificationDelivery{
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		EventType:        domain.NotificationEventVAExpired,
		Trigger:          domain.NotificationTriggerAuto,
		Status:           domain.NotificationDeliveryStatusSuccess,
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(merchantVA, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(latest, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.EventType == domain.NotificationEventVAExpired && p.VirtualAccountNo == merchantVA.VirtualAccountNo
	})).Return(nil)
	mockDelivery.On("Create", mock.Anything, mock.MatchedBy(func(d *domain.NotificationDelivery) bool {
		return d.Trigger == domain.NotificationTriggerManual && d.Status == domain.NotificationDeliveryStatusQueued
	})).Return(nil)

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, merchantVA.VirtualAccountNo, result.VirtualAccountNo)
	assert.Equal(t, domain.NotificationEventVAExpired, result.EventType)
	assert.Equal(t, domain.NotificationDeliveryStatusQueued, result.DeliveryStatus)
	mockRepo.AssertExpectations(t)
	mockDelivery.AssertExpectations(t)
	mockNotifier.AssertExpectations(t)
}

// T020: resend returns not-found for a non-existent virtualAccountNo.
func TestResendCallbackUsecase_Resend_NotFound(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "nonexistent").Return(nil, domain.ErrMerchantVANotFound)

	result, err := uc.Resend(context.Background(), "nonexistent")

	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrMerchantVANotFound))
	mockDelivery.AssertNotCalled(t, "GetLatestByVirtualAccountNo")
}

// T021: resend returns a distinct error when no prior delivery record exists (FR-015).
func TestResendCallbackUsecase_Resend_NoDeliveryRecord(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		NotificationURL:  "https://merchant.example.com/callback",
	}
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(merchantVA, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(nil, nil)

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo)

	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrResendNoDeliveryRecord))
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
}

// T022: resend returns a distinct error when the VA has no notification_url (FR-016).
func TestResendCallbackUsecase_Resend_NoNotificationURL(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		NotificationURL:  "",
	}
	latest := &domain.NotificationDelivery{VirtualAccountNo: merchantVA.VirtualAccountNo, EventType: domain.NotificationEventPaymentReceived}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(merchantVA, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(latest, nil)

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo)

	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrResendNoNotificationURL))
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
}

// T023: resend never calls UpdateVAStatus (transaction state unchanged, FR-019).
func TestResendCallbackUsecase_Resend_NeverMutatesVAStatus(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		NotificationURL:  "https://merchant.example.com/callback",
	}
	latest := &domain.NotificationDelivery{VirtualAccountNo: merchantVA.VirtualAccountNo, EventType: domain.NotificationEventPaymentReceived}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(merchantVA, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(latest, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.Anything).Return(nil)
	mockDelivery.On("Create", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo)

	assert.NoError(t, err)
	mockRepo.AssertNotCalled(t, "UpdateVAStatus")
}

// A resent payment.received callback carries the paymentRequestId stored for
// the transaction, so the merchant can match it to the original payment.
func TestResendCallbackUsecase_Resend_PaymentReceivedCarriesPaymentRequestID(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		NotificationURL:  "https://merchant.example.com/callback",
		PaymentRequestID: "202610260001",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
	}
	latest := &domain.NotificationDelivery{VirtualAccountNo: merchantVA.VirtualAccountNo, EventType: domain.NotificationEventPaymentReceived}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(merchantVA, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(latest, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.PaymentRequestID == merchantVA.PaymentRequestID
	})).Return(nil)
	mockDelivery.On("Create", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo)

	assert.NoError(t, err)
	mockNotifier.AssertExpectations(t)
}

// An enqueue failure is recorded on the delivery row it was created under,
// as "failed" with the error, and reported back to the caller.
func TestResendCallbackUsecase_Resend_EnqueueFailureRecordedOnRow(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	merchantVA := &domain.VAInquiryRecord{VirtualAccountNo: "7000108212221111", NotificationURL: "https://merchant.example.com/callback"}
	latest := &domain.NotificationDelivery{VirtualAccountNo: merchantVA.VirtualAccountNo, EventType: domain.NotificationEventPaymentReceived}

	var createdID string
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(merchantVA, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, merchantVA.VirtualAccountNo).Return(latest, nil)
	mockDelivery.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		createdID = args.Get(1).(*domain.NotificationDelivery).ID
	}).Return(nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.Anything).Return(errors.New("redis down"))
	mockDelivery.On("UpdateDeliveryResult", mock.Anything, mock.Anything, domain.NotificationDeliveryResult{
		Status:      domain.NotificationDeliveryStatusFailed,
		ErrorDetail: "redis down",
	}).Return(nil)

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo)

	assert.NoError(t, err)
	assert.Equal(t, domain.NotificationDeliveryStatusFailed, result.DeliveryStatus)
	mockDelivery.AssertCalled(t, "UpdateDeliveryResult", mock.Anything, createdID, mock.Anything)
}

// Bulk resend redelivers each original with the payload it was first sent
// with — not the VA's current state — as a manual delivery carrying its own
// row id, and skips originals recorded without a payload that cannot be
// rebuilt.
func TestResendCallbackUsecase_ResendRange_ReusesOriginalPayload(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	from := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	filter := domain.NotificationDeliveryFilter{From: from, To: from.Add(24 * time.Hour), Statuses: []string{domain.NotificationDeliveryStatusFailed}}
	withPayload := &domain.NotificationDelivery{
		ID:               "orig-1",
		VirtualAccountNo: "7000108212221111",
		EventType:        domain.NotificationEventPaymentReceived,
		Trigger:          domain.NotificationTriggerAuto,
		Status:           domain.NotificationDeliveryStatusFailed,
		Payload: &domain.PaymentNotificationPayload{
			EventType:        domain.NotificationEventPaymentReceived,
			VirtualAccountNo: "7000108212221111",
			TrxID:            "TRX-OLD",
			PaymentRequestID: "PRQ-OLD",
			NotificationURL:  "https://merchant.example.com/callback",
		},
	}
	legacy := &domain.NotificationDelivery{ID: "orig-2", VirtualAccountNo: "7000108212222222", EventType: domain.NotificationEventVAExpired}

	mockDelivery.On("ListForResend", mock.Anything, mock.MatchedBy(func(f domain.NotificationDeliveryFilter) bool {
		return f.From.Equal(filter.From) && f.To.Equal(filter.To) && f.Limit == MaxBulkResend+1
	})).Return([]*domain.NotificationDelivery{withPayload, legacy}, nil)
	mockDelivery.On("ReconstructPayload", mock.Anything, legacy).Return(nil, domain.ErrNotificationPayloadUnrecoverable)
	mockDelivery.On("Create", mock.Anything, mock.MatchedBy(func(d *domain.NotificationDelivery) bool {
		return d.Trigger == domain.NotificationTriggerManual && d.Status == domain.NotificationDeliveryStatusQueued
	})).Return(nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.TrxID == "TRX-OLD" && p.PaymentRequestID == "PRQ-OLD" &&
			p.Trigger == domain.NotificationTriggerManual && p.DeliveryID != ""
	})).Return(nil)

	result, err := uc.ResendRange(context.Background(), filter)

	assert.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, domain.NotificationDeliveryStatusQueued, result.Items[0].Status)
	assert.Equal(t, "orig-1", result.Items[0].SourceDeliveryID)
	assert.NotEmpty(t, result.Items[0].DeliveryID)
	assert.Equal(t, "skipped", result.Items[1].Status)
	assert.Empty(t, withPayload.Payload.DeliveryID, "the original row's payload must not be mutated")
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo", mock.Anything, mock.Anything)
	mockNotifier.AssertNumberOfCalls(t, "EnqueuePaymentNotification", 1)
}

// A range matching more than MaxBulkResend callbacks is refused before
// anything is sent.
func TestResendCallbackUsecase_ResendRange_TooLarge(t *testing.T) {
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(new(MockVARepository), mockDelivery, mockNotifier)

	rows := make([]*domain.NotificationDelivery, MaxBulkResend+1)
	for i := range rows {
		rows[i] = &domain.NotificationDelivery{}
	}
	mockDelivery.On("ListForResend", mock.Anything, mock.Anything).Return(rows, nil)

	_, err := uc.ResendRange(context.Background(), domain.NotificationDeliveryFilter{})

	assert.ErrorIs(t, err, domain.ErrResendRangeTooLarge)
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification", mock.Anything, mock.Anything)
}

// An original recorded without its payload is rebuilt from transaction state
// and resent, marked as reconstructed. A repository error while rebuilding is
// a failure, not a skip.
func TestResendCallbackUsecase_ResendRange_ReconstructsMissingPayload(t *testing.T) {
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(new(MockVARepository), mockDelivery, mockNotifier)

	legacy := &domain.NotificationDelivery{ID: "old-1", VirtualAccountNo: "   15975123123121", EventType: domain.NotificationEventPaymentReceived}
	broken := &domain.NotificationDelivery{ID: "old-2", VirtualAccountNo: "15975999", EventType: domain.NotificationEventVAExpired}
	rebuilt := &domain.PaymentNotificationPayload{
		EventType:        domain.NotificationEventPaymentReceived,
		VirtualAccountNo: "15975123123121",
		TrxID:            "TRX-1",
		PaymentRequestID: "PRQ-1",
		NotificationURL:  "https://merchant.example.com/callback",
	}

	mockDelivery.On("ListForResend", mock.Anything, mock.Anything).Return([]*domain.NotificationDelivery{legacy, broken}, nil)
	mockDelivery.On("ReconstructPayload", mock.Anything, legacy).Return(rebuilt, nil)
	mockDelivery.On("ReconstructPayload", mock.Anything, broken).Return(nil, errors.New("db down"))
	mockDelivery.On("Create", mock.Anything, mock.Anything).Return(nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.TrxID == "TRX-1" && p.PaymentRequestID == "PRQ-1" && p.Trigger == domain.NotificationTriggerManual
	})).Return(nil)

	result, err := uc.ResendRange(context.Background(), domain.NotificationDeliveryFilter{})

	assert.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, domain.NotificationDeliveryStatusQueued, result.Items[0].Status)
	assert.True(t, result.Items[0].Reconstructed)
	assert.Equal(t, domain.NotificationDeliveryStatusFailed, result.Items[1].Status)
	assert.Equal(t, "db down", result.Items[1].Error)
	mockNotifier.AssertNumberOfCalls(t, "EnqueuePaymentNotification", 1)
}
