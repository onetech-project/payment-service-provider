package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockVARepository is a mock implementation of domain.VARepository
type MockVARepository struct {
	mock.Mock
}

func (m *MockVARepository) SaveInquiry(ctx context.Context, inquiry *domain.VAInquiryRecord) error {
	args := m.Called(ctx, inquiry)
	return args.Error(0)
}

func (m *MockVARepository) GetInquiry(ctx context.Context, inquiryRequestID string) (*domain.VAInquiryRecord, error) {
	args := m.Called(ctx, inquiryRequestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAInquiryRecord), args.Error(1)
}

func (m *MockVARepository) SavePayment(ctx context.Context, payment *domain.VAPaymentRecord) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *MockVARepository) GetPayment(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	args := m.Called(ctx, paymentRequestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAPaymentRecord), args.Error(1)
}

func (m *MockVARepository) UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error {
	args := m.Called(ctx, paymentRequestID, status)
	return args.Error(0)
}

func (m *MockVARepository) ListVA(ctx context.Context, filter *domain.VAListFilter) ([]domain.VAListItem, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.VAListItem), args.Int(1), args.Error(2)
}

func (m *MockVARepository) GetVABillDetails(ctx context.Context, transactionID string) ([]domain.BillDetail, error) {
	args := m.Called(ctx, transactionID)
	return args.Get(0).([]domain.BillDetail), args.Error(1)
}

func (m *MockVARepository) SaveBillDetails(ctx context.Context, transactionID string, bills []domain.BillDetail) error {
	args := m.Called(ctx, transactionID, bills)
	return args.Error(0)
}

func (m *MockVARepository) UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error {
	args := m.Called(ctx, virtualAccountNo, status)
	return args.Error(0)
}

func (m *MockVARepository) GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.VAInquiryRecord, error) {
	args := m.Called(ctx, virtualAccountNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAInquiryRecord), args.Error(1)
}

func (m *MockVARepository) NextCustomerNoSequence(ctx context.Context, vaType string) (string, error) {
	args := m.Called(ctx, vaType)
	return args.String(0), args.Error(1)
}

func (m *MockVARepository) RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error {
	args := m.Called(ctx, partnerServiceID, customerNo)
	return args.Error(0)
}

func (m *MockVARepository) SaveVAPayment(ctx context.Context, transactionID string, amount string, referenceNo string) (string, string, error) {
	args := m.Called(ctx, transactionID, amount, referenceNo)
	return args.String(0), args.String(1), args.Error(2)
}

func TestVAUsecase_Inquiry_Success(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		ChannelCode:      6011,
		Amount:           &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, "Successful", resp.ResponseMessage)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "00", resp.VirtualAccountData.InquiryStatus)
	mockRepo.AssertExpectations(t)
}

func TestVAUsecase_Inquiry_ExistingMerchantVA_DoesNotDuplicateRecord(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		// Deliberately a fresh inquiryRequestId that was never used at create-va
		// time (e.g. a new inquiry attempt against an already-created VA) — this
		// must NOT create a second VAInquiryRecord for the same VA.
		InquiryRequestID: "INQ-brand-new-9999",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		ID:               "existing-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     "Faris",
		VirtualAccountNo: req.VirtualAccountNo,
		TrxID:            "TRX-original",
		Status:           "03",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
	}

	bills := []domain.BillDetail{
		{BillNo: "INV-001", BillName: "Invoice Januari", BillAmount: &domain.Amount{Value: "10000.00", Currency: "IDR"}},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return(bills, nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, req.VirtualAccountNo, resp.VirtualAccountData.VirtualAccountNo)
	assert.Equal(t, "Faris", resp.VirtualAccountData.VirtualAccountName)
	assert.Equal(t, "10000.00", resp.VirtualAccountData.TotalAmount.Value)
	assert.Len(t, resp.VirtualAccountData.BillDetails, 1)
	assert.Equal(t, "INV-001", resp.VirtualAccountData.BillDetails[0].BillNo)
	mockRepo.AssertNotCalled(t, "SaveInquiry")
}

func TestVAUsecase_Inquiry_MissingAmount(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002402", domainErr.SNAPCode)
}

func TestVAInquiryRequest_UnmarshalsSpecCompliantFields(t *testing.T) {
	body := []byte(`{
		"partnerServiceId": "12345",
		"customerNo": "123456789012345678",
		"virtualAccountNo": "12345123456789012345678",
		"txnDateInit": "2026-07-23T10:00:00+07:00",
		"amount": {"value": "100000.00", "currency": "IDR"},
		"inquiryRequestId": "202607221000001234500001"
	}`)

	var req domain.VAInquiryRequest
	err := json.Unmarshal(body, &req)

	assert.NoError(t, err)
	assert.NotNil(t, req.TrxDateInit)
	assert.NotNil(t, req.Amount)
	assert.Equal(t, "100000.00", req.Amount.Value)
}

func TestVAUsecase_Inquiry_Idempotent(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		Amount:           &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	existing := &domain.VAInquiryRecord{
		ID:               "existing-id",
		InquiryRequestID: req.InquiryRequestID,
		Status:           "00",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(existing, nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry")
}

func TestVAUsecase_Payment_Success(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002500", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, req.PartnerServiceID, resp.VirtualAccountData.PartnerServiceID)
	assert.Equal(t, req.CustomerNo, resp.VirtualAccountData.CustomerNo)
	assert.Equal(t, req.VirtualAccountNo, resp.VirtualAccountData.VirtualAccountNo)
	assert.Equal(t, req.PaymentRequestID, resp.VirtualAccountData.PaymentRequestID)
	assert.Equal(t, req.PaidAmount, resp.VirtualAccountData.PaidAmount)
	mockRepo.AssertExpectations(t)
}

func TestVAUsecase_Payment_Idempotent_EchoesPersistedFields(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	existing := &domain.VAPaymentRecord{
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		TrxID:            "TRX-001",
		PaymentRequestID: req.PaymentRequestID,
		PaidAmount:       "100000.00",
		Currency:         "IDR",
		ReferenceNo:      "R1234567890",
		TransactionDate:  time.Now(),
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(existing, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, existing.PartnerServiceID, resp.VirtualAccountData.PartnerServiceID)
	assert.Equal(t, existing.VirtualAccountNo, resp.VirtualAccountData.VirtualAccountNo)
	assert.Equal(t, existing.TrxID, resp.VirtualAccountData.TrxID)
	assert.Equal(t, existing.PaymentRequestID, resp.VirtualAccountData.PaymentRequestID)
	assert.Equal(t, existing.PaidAmount, resp.VirtualAccountData.PaidAmount.Value)
	mockRepo.AssertNotCalled(t, "SavePayment")
}

func TestVAUsecase_Payment_MissingPaymentRequestID(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002502", domainErr.SNAPCode)
}

func TestVAUsecase_Payment_MissingPaidAmount(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
	}

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002502", domainErr.SNAPCode)
}

func TestVAUsecase_Payment_AlreadyPaidVA_RejectsAndDoesNotOverwrite(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "70001082122221111",
		InquiryRequestID: "INQ-original",
		// A brand-new paymentRequestId the vendor has never sent before, so
		// the idempotency-by-PaymentRequestID lookup misses and this would
		// otherwise fall through to overwriting the already-paid transaction.
		PaymentRequestID: "PAY-second-attempt",
		PaidAmount:       &domain.Amount{Value: "999999.00", Currency: "IDR"},
	}

	alreadyPaidVA := &domain.VAInquiryRecord{
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "00", // Already paid
		TotalAmount:      "10000.00",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(alreadyPaidVA, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4092500", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SavePayment")
}

// MockNotifier is a mock implementation of domain.NotificationEnqueuer
type MockNotifier struct {
	mock.Mock
}

func (m *MockNotifier) EnqueuePaymentNotification(ctx context.Context, payload *domain.PaymentNotificationPayload) error {
	args := m.Called(ctx, payload)
	return args.Error(0)
}

func TestVAUsecase_Payment_NotifiesMerchant(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	usecase := NewVAUsecase(mockRepo, mockNotifier)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: req.VirtualAccountNo,
		TrxID:            "TRX-001",
		NotificationURL:  "https://merchant.example.com/callback",
		Status:           "03", // Pending — hasn't been paid yet
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.NotificationURL == merchantVA.NotificationURL &&
			p.TrxID == merchantVA.TrxID &&
			p.PaymentRequestID == req.PaymentRequestID &&
			p.PaidAmount == req.PaidAmount
	})).Return(nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002500", resp.ResponseCode)
	mockRepo.AssertExpectations(t)
	mockNotifier.AssertExpectations(t)
}

func TestVAUsecase_Payment_NoNotificationURL_SkipsCallback(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	usecase := NewVAUsecase(mockRepo, mockNotifier)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002500", resp.ResponseCode)
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
}

// MockVANotificationDeliveryRepository is a mock implementation of
// domain.VANotificationDeliveryRepository (feature 007-merchant-expiry-callback).
type MockVANotificationDeliveryRepository struct {
	mock.Mock
}

func (m *MockVANotificationDeliveryRepository) Create(ctx context.Context, delivery *domain.NotificationDelivery) error {
	args := m.Called(ctx, delivery)
	return args.Error(0)
}

func (m *MockVANotificationDeliveryRepository) GetLatestByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.NotificationDelivery, error) {
	args := m.Called(ctx, virtualAccountNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationDelivery), args.Error(1)
}

func (m *MockVANotificationDeliveryRepository) ExistsByVirtualAccountNoAndEventType(ctx context.Context, virtualAccountNo, eventType, trigger string) (bool, error) {
	args := m.Called(ctx, virtualAccountNo, eventType, trigger)
	return args.Bool(0), args.Error(1)
}

// T008: expired inquiry returns 4042419, transitions status, enqueues one va.expired notification.
func TestVAUsecase_Inquiry_Expired_ReturnsExpiredResponseAndNotifies(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	uc := NewVAUsecaseWithDeliveryRepo(mockRepo, mockNotifier, mockDelivery)

	expired := time.Now().Add(-1 * time.Hour)
	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		Status:           "03",
		ExpiredDate:      &expired,
		NotificationURL:  "https://merchant.example.com/callback",
	}

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		InquiryRequestID: "INQ-expired-0001",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("UpdateVAStatus", mock.Anything, merchantVA.VirtualAccountNo, "02").Return(nil)
	mockDelivery.On("ExistsByVirtualAccountNoAndEventType", mock.Anything, merchantVA.VirtualAccountNo, domain.NotificationEventVAExpired, domain.NotificationTriggerAuto).Return(false, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.EventType == domain.NotificationEventVAExpired && p.VirtualAccountNo == merchantVA.VirtualAccountNo
	})).Return(nil)
	mockDelivery.On("Create", mock.Anything, mock.MatchedBy(func(d *domain.NotificationDelivery) bool {
		return d.EventType == domain.NotificationEventVAExpired && d.Trigger == domain.NotificationTriggerAuto && d.Status == domain.NotificationDeliveryStatusSuccess
	})).Return(nil)

	resp, err := uc.Inquiry(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042419", domainErr.SNAPCode)
	mockRepo.AssertExpectations(t)
	mockNotifier.AssertExpectations(t)
	mockDelivery.AssertExpectations(t)
}

// T009: expired payment notify returns 4042519, transitions status, enqueues one va.expired notification.
func TestVAUsecase_Payment_Expired_ReturnsExpiredResponseAndNotifies(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	uc := NewVAUsecaseWithDeliveryRepo(mockRepo, mockNotifier, mockDelivery)

	expired := time.Now().Add(-1 * time.Hour)
	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		Status:           "03",
		ExpiredDate:      &expired,
		NotificationURL:  "https://merchant.example.com/callback",
	}

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		InquiryRequestID: "INQ-expired-0002",
		PaymentRequestID: "PAY-expired-0002",
		PaidAmount:       &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("UpdateVAStatus", mock.Anything, merchantVA.VirtualAccountNo, "02").Return(nil)
	mockDelivery.On("ExistsByVirtualAccountNoAndEventType", mock.Anything, merchantVA.VirtualAccountNo, domain.NotificationEventVAExpired, domain.NotificationTriggerAuto).Return(false, nil)
	mockNotifier.On("EnqueuePaymentNotification", mock.Anything, mock.MatchedBy(func(p *domain.PaymentNotificationPayload) bool {
		return p.EventType == domain.NotificationEventVAExpired
	})).Return(nil)
	mockDelivery.On("Create", mock.Anything, mock.Anything).Return(nil)

	resp, err := uc.Payment(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042519", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SavePayment")
	mockRepo.AssertExpectations(t)
}

// T010: repeated inquiry/notify calls on an already-expired VA do NOT enqueue
// a second va.expired notification.
func TestVAUsecase_Inquiry_AlreadyExpired_DoesNotDuplicateNotification(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	uc := NewVAUsecaseWithDeliveryRepo(mockRepo, mockNotifier, mockDelivery)

	expired := time.Now().Add(-1 * time.Hour)
	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		Status:           "02", // already expired by a prior call
		ExpiredDate:      &expired,
		NotificationURL:  "https://merchant.example.com/callback",
	}

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		InquiryRequestID: "INQ-expired-0003",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	// Status is already "02", so UpdateVAStatus's WHERE status='03' guard
	// no-ops (returns ErrMerchantVANotFound) — markExpiredAndNotify must stop
	// there and must NOT enqueue a second notification.
	mockRepo.On("UpdateVAStatus", mock.Anything, merchantVA.VirtualAccountNo, "02").Return(domain.ErrMerchantVANotFound)

	// spec.md User Story 1, Acceptance Scenario 4: a later inquiry on an
	// already-expired VA MUST keep returning the same 4042419 expired
	// response, not fall through to success — but must not send a duplicate
	// callback (the UpdateVAStatus no-op above prevents that).
	resp, err := uc.Inquiry(context.Background(), req)

	assert.Nil(t, resp)
	require.Error(t, err)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042419", domainErr.SNAPCode)
	mockRepo.AssertCalled(t, "UpdateVAStatus", mock.Anything, merchantVA.VirtualAccountNo, "02")
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
	mockDelivery.AssertNotCalled(t, "Create")
}

// T011: a VA with no notification_url still transitions to "02" but no
// notification is enqueued.
func TestVAUsecase_Inquiry_Expired_NoNotificationURL_StillTransitionsNoNotify(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	uc := NewVAUsecaseWithDeliveryRepo(mockRepo, mockNotifier, mockDelivery)

	expired := time.Now().Add(-1 * time.Hour)
	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		Status:           "03",
		ExpiredDate:      &expired,
		NotificationURL:  "", // no callback destination registered
	}

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		InquiryRequestID: "INQ-expired-0004",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("UpdateVAStatus", mock.Anything, merchantVA.VirtualAccountNo, "02").Return(nil)

	resp, err := uc.Inquiry(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042419", domainErr.SNAPCode)
	mockRepo.AssertExpectations(t)
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
	mockDelivery.AssertNotCalled(t, "Create")
}

// T012: a VA paid concurrently before expiry detection is NOT transitioned to
// expired and receives no va.expired callback (race precedence per FR-010).
// Simulated via UpdateVAStatus returning ErrMerchantVANotFound because the
// WHERE status='03' guard no longer matches (concurrent payment already
// moved the row to "00").
func TestVAUsecase_Inquiry_ConcurrentPaymentWinsRace_NoExpiredTransition(t *testing.T) {
	mockRepo := new(MockVARepository)
	mockNotifier := new(MockNotifier)
	mockDelivery := new(MockVANotificationDeliveryRepository)
	uc := NewVAUsecaseWithDeliveryRepo(mockRepo, mockNotifier, mockDelivery)

	expired := time.Now().Add(-1 * time.Hour)
	merchantVA := &domain.VAInquiryRecord{
		VirtualAccountNo: "7000108212221111",
		Status:           "03", // stale read: was pending as of this record's fetch
		ExpiredDate:      &expired,
		NotificationURL:  "https://merchant.example.com/callback",
	}

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: merchantVA.VirtualAccountNo,
		InquiryRequestID: "INQ-expired-0005",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	// Concurrent payment already flipped status away from "03" — the guarded
	// UPDATE affects 0 rows.
	mockRepo.On("UpdateVAStatus", mock.Anything, merchantVA.VirtualAccountNo, "02").Return(domain.ErrMerchantVANotFound)

	resp, err := uc.Inquiry(context.Background(), req)

	// The expired-response error is still returned to the vendor for THIS
	// request (the inline check saw an unpaid/expired snapshot), but no
	// notification is enqueued since the state transition did not apply.
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042419", domainErr.SNAPCode)
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
	mockDelivery.AssertNotCalled(t, "Create")
}

func TestVAUsecase_Payment_AmountMismatch(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "200000.00", Currency: "IDR"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002501", domainErr.SNAPCode)
}

func TestVAUsecase_Payment_OptionalTotalAmount_NoMismatchCheck(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Variable-bill multi-payment tests (feature 006-static-dynamic-va) ---

func TestVAUsecase_Payment_VariableBill_PartialPayment_StaysPending(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "15974",
		CustomerNo:       "05000000000000000001",
		VirtualAccountNo: "15974050000000000000001",
		InquiryRequestID: "inquiry-var-1",
		PaymentRequestID: "payment-var-1",
		PaidAmount:       &domain.Amount{Value: "60000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-var-1",
		PartnerServiceID: "15974",
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		VAType:           "05",
		TrxID:            "trx-var-1",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("SaveVAPayment", mock.Anything, "txn-var-1", "60000.00", req.ReferenceNo).Return("60000.00", "03", nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "03", resp.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, "60000.00", resp.VirtualAccountData.PaidAmount.Value)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "SavePayment", mock.Anything, mock.Anything)
}

func TestVAUsecase_Payment_VariableBill_CumulativeReachesTotal_MarksPaid(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "15974",
		CustomerNo:       "05000000000000000001",
		VirtualAccountNo: "15974050000000000000001",
		InquiryRequestID: "inquiry-var-2",
		PaymentRequestID: "payment-var-2",
		PaidAmount:       &domain.Amount{Value: "40000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-var-1",
		PartnerServiceID: "15974",
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		VAType:           "05",
		TrxID:            "trx-var-1",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("SaveVAPayment", mock.Anything, "txn-var-1", "40000.00", req.ReferenceNo).Return("100000.00", "00", nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, "100000.00", resp.VirtualAccountData.PaidAmount.Value)
}

func TestVAUsecase_Status_Success(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	txDate := time.Now()
	req := &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	trxDateTime := txDate.Add(-time.Minute)
	payment := &domain.VAPaymentRecord{
		ID:               "payment-id",
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: req.InquiryRequestID,
		PaymentRequestID: req.InquiryRequestID,
		PaidAmount:       "100000.00",
		TotalAmount:      "150000.00",
		Currency:         "IDR",
		Status:           "00",
		ReferenceNo:      "12345678901",
		PaymentType:      "1",
		FlagAdvise:       "Y",
		PaidBills:        "95000",
		TrxDateTime:      &trxDateTime,
		FreeTexts:        []domain.BilingualText{{English: "Free text", Indonesia: "Tulisan bebas"}},
		TransactionDate:  txDate,
	}

	bills := []domain.BillDetail{
		{BillNo: "123456789012345678", BillName: "Bill A for Jan", Status: "00"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.InquiryRequestID).Return(payment, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, payment.ID).Return(bills, nil)

	resp, err := usecase.Status(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002600", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	data := resp.VirtualAccountData
	assert.Equal(t, "00", data.PaymentFlagStatus)
	assert.Equal(t, "150000.00", data.TotalAmount.Value)
	assert.Equal(t, "100000.00", data.PaidAmount.Value)
	assert.Equal(t, "95000", data.PaidBills)
	assert.Equal(t, "1", data.PaymentType)
	assert.Equal(t, "Y", data.FlagAdvise)
	assert.Equal(t, &trxDateTime, data.TrxDateTime)
	assert.Equal(t, payment.FreeTexts, data.FreeTexts)
	assert.Len(t, data.BillDetails, 1)
	assert.Equal(t, "123456789012345678", data.BillDetails[0].BillNo)
	mockRepo.AssertExpectations(t)
}
func TestVAUsecase_Status_Pending(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	inquiry := &domain.VAInquiryRecord{
		ID:               "inquiry-id",
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: req.InquiryRequestID,
		Status:           "00",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(inquiry, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, inquiry.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Status(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002600", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "03", resp.VirtualAccountData.PaymentFlagStatus)
	mockRepo.AssertExpectations(t)
}
