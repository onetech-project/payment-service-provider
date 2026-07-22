package usecase

import (
	"context"
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

func TestVAUsecase_Inquiry_Success(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		ChannelCode:      6011,
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, "Successful", resp.ResponseMessage)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "00", resp.VirtualAccountData.InquiryStatus)
	mockRepo.AssertExpectations(t)
}

func TestVAUsecase_Inquiry_Idempotent(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
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
	usecase := NewVAUsecase(mockRepo)

	txDate := time.Now()
	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TransactionDate:  &txDate,
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
	mockRepo.AssertExpectations(t)
}

func TestVAUsecase_Payment_AmountMismatch(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo)

	txDate := time.Now()
	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "200000.00", Currency: "IDR"},
		TransactionDate:  &txDate,
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002401", domainErr.SNAPCode)
}

func TestVAUsecase_Status_Success(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo)

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
	usecase := NewVAUsecase(mockRepo)

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
