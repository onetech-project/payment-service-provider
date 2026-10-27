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
		return d.Trigger == domain.NotificationTriggerManual && d.Status == domain.NotificationDeliveryStatusSuccess
	})).Return(nil)

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, merchantVA.VirtualAccountNo, result.VirtualAccountNo)
	assert.Equal(t, domain.NotificationEventVAExpired, result.EventType)
	assert.Equal(t, domain.NotificationDeliveryStatusSuccess, result.DeliveryStatus)
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

	result, err := uc.Resend(context.Background(), "nonexistent", "")

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

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo, "")

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

	result, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo, "")

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

	_, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo, "")

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

	_, err := uc.Resend(context.Background(), merchantVA.VirtualAccountNo, "")

	assert.NoError(t, err)
	mockNotifier.AssertExpectations(t)
}

// With a trxId the resend targets that transaction cycle of a reused VA, not
// the cycle GetVAByVirtualAccountNo ranks first. A settled cycle is resent as
// payment.received even when the VA's latest delivery record belongs to
// another cycle's expiry.
func TestResendCallbackUsecase_Resend_WithTrxIDTargetsThatCycle(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	settledCycle := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		TrxID:            "TRX-OLD-001",
		Status:           "00",
		NotificationURL:  "https://merchant.example.com/callback",
		PaymentRequestID: "202610260001",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
	}
	latest := &domain.NotificationDelivery{VirtualAccountNo: settledCycle.VirtualAccountNo, EventType: domain.NotificationEventVAExpired}

	mockRepo.On("GetVAByVirtualAccountNoAndTrxID", mock.Anything, settledCycle.VirtualAccountNo, settledCycle.TrxID).Return(settledCycle, nil)
	mockDelivery.On("GetLatestByVirtualAccountNo", mock.Anything, settledCycle.VirtualAccountNo).Return(latest, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.EventType == domain.NotificationEventPaymentReceived &&
			p.TrxID == settledCycle.TrxID &&
			p.PaymentRequestID == settledCycle.PaymentRequestID
	})).Return(nil)
	mockDelivery.On("Create", mock.Anything, mock.MatchedBy(func(d *domain.NotificationDelivery) bool {
		return d.EventType == domain.NotificationEventPaymentReceived && d.Trigger == domain.NotificationTriggerManual
	})).Return(nil)

	result, err := uc.Resend(context.Background(), settledCycle.VirtualAccountNo, settledCycle.TrxID)

	assert.NoError(t, err)
	assert.Equal(t, settledCycle.TrxID, result.TrxID)
	assert.Equal(t, domain.NotificationEventPaymentReceived, result.EventType)
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo", mock.Anything, mock.Anything)
	mockNotifier.AssertExpectations(t)
	mockDelivery.AssertExpectations(t)
}

// An unknown trxId for the VA is reported as not found.
func TestResendCallbackUsecase_Resend_UnknownTrxIDNotFound(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	mockNotifier := new(MockNotifier)
	uc := NewResendCallbackUsecase(mockRepo, mockDelivery, mockNotifier)

	mockRepo.On("GetVAByVirtualAccountNoAndTrxID", mock.Anything, "7000108212221111", "TRX-NOPE").Return(nil, domain.ErrMerchantVANotFound)

	_, err := uc.Resend(context.Background(), "7000108212221111", "TRX-NOPE")

	assert.ErrorIs(t, err, domain.ErrMerchantVANotFound)
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification", mock.Anything, mock.Anything)
}
