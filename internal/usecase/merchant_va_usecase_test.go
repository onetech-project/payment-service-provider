package usecase

import (
	"context"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMerchantVARepository is a mock for testing merchant VA usecase
type MockMerchantVARepository struct {
	mock.Mock
}

func (m *MockMerchantVARepository) SaveInquiry(ctx context.Context, inquiry *domain.VAInquiryRecord) error {
	args := m.Called(ctx, inquiry)
	return args.Error(0)
}

func (m *MockMerchantVARepository) GetInquiry(ctx context.Context, inquiryRequestID string) (*domain.VAInquiryRecord, error) {
	args := m.Called(ctx, inquiryRequestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAInquiryRecord), args.Error(1)
}

func (m *MockMerchantVARepository) SavePayment(ctx context.Context, payment *domain.VAPaymentRecord) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *MockMerchantVARepository) GetPayment(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	args := m.Called(ctx, paymentRequestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAPaymentRecord), args.Error(1)
}

func (m *MockMerchantVARepository) UpdatePaymentStatus(ctx context.Context, paymentRequestID string, status string) error {
	args := m.Called(ctx, paymentRequestID, status)
	return args.Error(0)
}

func (m *MockMerchantVARepository) ListVA(ctx context.Context, filter *domain.VAListFilter) ([]domain.VAListItem, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.VAListItem), args.Int(1), args.Error(2)
}

func (m *MockMerchantVARepository) GetVABillDetails(ctx context.Context, transactionID string) ([]domain.BillDetail, error) {
	args := m.Called(ctx, transactionID)
	return args.Get(0).([]domain.BillDetail), args.Error(1)
}

func (m *MockMerchantVARepository) UpdateVAStatus(ctx context.Context, virtualAccountNo string, status string) error {
	args := m.Called(ctx, virtualAccountNo, status)
	return args.Error(0)
}

func (m *MockMerchantVARepository) GetVAByVirtualAccountNo(ctx context.Context, virtualAccountNo string) (*domain.VAInquiryRecord, error) {
	args := m.Called(ctx, virtualAccountNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAInquiryRecord), args.Error(1)
}

// --- CreateVA Tests ---

func TestMerchantVAUsecase_CreateVA_Success(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	amount := &domain.Amount{Value: "150000.00", Currency: "IDR"}
	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:    "088899",
		CustomerNo:          "12345678901234567890",
		VirtualAccountName:  "Jokul Doe",
		TrxID:               "trx-001",
		TotalAmount:         amount,
		VirtualAccountTrxType: "C",
		NotificationURL:     "https://example.com/webhook",
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Equal(t, "Success", resp.ResponseMessage)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "08889912345678901234567890", resp.VirtualAccountData.VirtualAccountNo)
	assert.Equal(t, "trx-001", resp.VirtualAccountData.TrxID)
	assert.Equal(t, "C", resp.VirtualAccountData.VirtualAccountTrxType)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_MissingTrxId(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountName: "Jokul Doe",
		// TrxID missing
		NotificationURL: "https://example.com/webhook",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002701", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_InvalidTrxType(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, mock.Anything).Return(nil, domain.ErrMerchantVANotFound)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:      "088899",
		CustomerNo:            "12345678901234567890",
		VirtualAccountName:    "Jokul Doe",
		TrxID:                 "trx-002",
		VirtualAccountTrxType: "Z", // Invalid
		NotificationURL:       "https://example.com/webhook",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002700", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_Idempotent(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	existing := &domain.VAInquiryRecord{
		ID:               "existing-id",
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
		Status:           "00", // Already paid (terminal)
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(existing, nil)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-003",
		NotificationURL:    "https://example.com/webhook",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4092700", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_MissingNotificationURL(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-004",
		// NotificationURL missing
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002701", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_WithBillDetails(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	amount := &domain.Amount{Value: "150000.00", Currency: "IDR"}
	billAmount := &domain.Amount{Value: "150000.00", Currency: "IDR"}
	expiry := time.Now().Add(7 * 24 * time.Hour)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:      "088899",
		CustomerNo:            "12345678901234567890",
		VirtualAccountName:    "Jokul Doe",
		TrxID:                 "trx-005",
		TotalAmount:           amount,
		VirtualAccountTrxType: "C",
		ExpiredDate:           &expiry,
		NotificationURL:       "https://example.com/webhook",
		BillDetails: []domain.BillDetail{
			{
				BillCode:  "01",
				BillNo:    "INV-001",
				BillName:  "Invoice Januari",
				BillAmount: billAmount,
			},
		},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Len(t, resp.VirtualAccountData.BillDetails, 1)
	assert.NotNil(t, resp.VirtualAccountData.ExpiredDate)
	mockRepo.AssertExpectations(t)
}

// --- ListVA Tests ---

func TestMerchantVAUsecase_ListVA_Success(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	items := []domain.VAListItem{
		{
			VirtualAccountNo: "08889912345678901234567890",
			CustomerNo:       "12345678901234567890",
			CustomerName:     "Jokul Doe",
			Status:           "03",
			CreatedAt:        time.Now(),
		},
	}

	mockRepo.On("ListVA", mock.Anything, mock.AnythingOfType("*domain.VAListFilter")).Return(items, 1, nil)

	req := &domain.MerchantListVARequest{
		PartnerServiceID: "088899",
		Page:             1,
		PageSize:         20,
	}

	resp, err := uc.ListVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Len(t, resp.Data, 1)
	assert.NotNil(t, resp.Pagination)
	assert.Equal(t, 1, resp.Pagination.TotalRows)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_ListVA_EmptyResults(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	mockRepo.On("ListVA", mock.Anything, mock.AnythingOfType("*domain.VAListFilter")).Return([]domain.VAListItem{}, 0, nil)

	req := &domain.MerchantListVARequest{
		PartnerServiceID: "088899",
		Page:             1,
		PageSize:         20,
	}

	resp, err := uc.ListVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Pagination.TotalRows)
	assert.Equal(t, 0, resp.Pagination.TotalPages)
}

func TestMerchantVAUsecase_ListVA_DefaultPagination(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	mockRepo.On("ListVA", mock.Anything, mock.AnythingOfType("*domain.VAListFilter")).Return([]domain.VAListItem{}, 0, nil)

	req := &domain.MerchantListVARequest{
		Page:     0, // Invalid, should default to 1
		PageSize: 0, // Invalid, should default to 20
	}

	resp, err := uc.ListVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 20, resp.Pagination.PageSize)
}

// --- DeleteVA Tests ---

func TestMerchantVAUsecase_DeleteVA_Success(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	existing := &domain.VAInquiryRecord{
		VirtualAccountNo: "08889912345678901234567890",
		Status:           "03", // Pending
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(existing, nil)
	mockRepo.On("UpdateVAStatus", mock.Anything, "08889912345678901234567890", "04").Return(nil)

	req := &domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
	}

	resp, err := uc.DeleteVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2003100", resp.ResponseCode)
	assert.Equal(t, "Success", resp.ResponseMessage)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_DeleteVA_AlreadyPaid(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	existing := &domain.VAInquiryRecord{
		VirtualAccountNo: "08889912345678901234567890",
		Status:           "00", // Success/Paid
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(existing, nil)

	req := &domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
	}

	resp, err := uc.DeleteVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4053101", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_DeleteVA_AlreadyDeleted(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	existing := &domain.VAInquiryRecord{
		VirtualAccountNo: "08889912345678901234567890",
		Status:           "04", // Already deleted
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(existing, nil)

	req := &domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
	}

	resp, err := uc.DeleteVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2003100", resp.ResponseCode) // Idempotent
}

func TestMerchantVAUsecase_DeleteVA_NotFound(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)

	req := &domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
	}

	resp, err := uc.DeleteVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4043112", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_DeleteVA_MissingFields(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo)

	req := &domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		// Missing CustomerNo and VirtualAccountNo
	}

	resp, err := uc.DeleteVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4003101", domainErr.SNAPCode)
}
