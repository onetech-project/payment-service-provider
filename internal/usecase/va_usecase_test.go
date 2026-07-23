package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	assert.Equal(t, "2002400", resp.ResponseCode)
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
	assert.Equal(t, "4002402", domainErr.SNAPCode)
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
	assert.Equal(t, "4002402", domainErr.SNAPCode)
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
	assert.Equal(t, "2002400", resp.ResponseCode)
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
	assert.Equal(t, "2002400", resp.ResponseCode)
	mockNotifier.AssertNotCalled(t, "EnqueuePaymentNotification")
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

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002401", domainErr.SNAPCode)
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

	payment := &domain.VAPaymentRecord{
		ID:               "payment-id",
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: req.InquiryRequestID,
		PaymentRequestID: req.InquiryRequestID,
		PaidAmount:       "100000.00",
		Currency:         "IDR",
		Status:           "00",
		ReferenceNo:      "12345678901",
		TransactionDate:  txDate,
	}

	mockRepo.On("GetPayment", mock.Anything, req.InquiryRequestID).Return(payment, nil)

	resp, err := usecase.Status(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002600", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
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

	resp, err := usecase.Status(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002600", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "03", resp.VirtualAccountData.PaymentFlagStatus)
	mockRepo.AssertExpectations(t)
}
