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

// staticVATypeRuleProvider is an in-memory domain.VATypeRuleProvider
// reproducing the exact six rules that were previously hardcoded in
// internal/domain/va.go, so existing US1-3 tests exercise unchanged
// behavior now that CreateVA reads rules through this interface (feature
// 006-static-dynamic-va amendment, T040).
type staticVATypeRuleProvider struct {
	rules      map[string]domain.VATypeRule
	partnerIDs map[string]bool
}

func newTestVATypeRuleProvider() *staticVATypeRuleProvider {
	return &staticVATypeRuleProvider{
		rules: map[string]domain.VATypeRule{
			"01": {VAType: "01", PartnerServiceID: "15973", Dynamic: false, Billing: domain.VATypeBillingNone},
			"02": {VAType: "02", PartnerServiceID: "15974", Dynamic: false, Billing: domain.VATypeBillingVariable},
			"03": {VAType: "03", PartnerServiceID: "15975", Dynamic: false, Billing: domain.VATypeBillingFixed},
			"04": {VAType: "04", PartnerServiceID: "15973", Dynamic: true, Billing: domain.VATypeBillingNone},
			"05": {VAType: "05", PartnerServiceID: "15974", Dynamic: true, Billing: domain.VATypeBillingVariable},
			"06": {VAType: "06", PartnerServiceID: "15975", Dynamic: true, Billing: domain.VATypeBillingFixed},
		},
		partnerIDs: map[string]bool{"15973": true, "15974": true, "15975": true},
	}
}

func (p *staticVATypeRuleProvider) LookupVATypeRule(ctx context.Context, partnerServiceID, vaType string) (domain.VATypeRule, bool, error) {
	rule, ok := p.rules[vaType]
	if !ok || rule.PartnerServiceID != partnerServiceID {
		return domain.VATypeRule{}, false, nil
	}
	return rule, true, nil
}

func (p *staticVATypeRuleProvider) IsReservedPartnerServiceID(ctx context.Context, partnerServiceID string) (bool, error) {
	return p.partnerIDs[partnerServiceID], nil
}

// MockVATypeRuleProvider is a mockery-style mock for domain.VATypeRuleProvider,
// used (T040) to prove CreateVA is actually wired to the injected provider
// (constructor DI) rather than any package-level lookup, and that a
// provider-level failure surfaces as a system-unavailable domain error.
type MockVATypeRuleProvider struct {
	mock.Mock
}

func (m *MockVATypeRuleProvider) LookupVATypeRule(ctx context.Context, partnerServiceID, vaType string) (domain.VATypeRule, bool, error) {
	args := m.Called(ctx, partnerServiceID, vaType)
	return args.Get(0).(domain.VATypeRule), args.Bool(1), args.Error(2)
}

func (m *MockVATypeRuleProvider) IsReservedPartnerServiceID(ctx context.Context, partnerServiceID string) (bool, error) {
	args := m.Called(ctx, partnerServiceID)
	return args.Bool(0), args.Error(1)
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

func (m *MockMerchantVARepository) SaveBillDetails(ctx context.Context, transactionID string, bills []domain.BillDetail) error {
	args := m.Called(ctx, transactionID, bills)
	return args.Error(0)
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

func (m *MockMerchantVARepository) NextCustomerNoSequence(ctx context.Context, vaType string) (string, error) {
	args := m.Called(ctx, vaType)
	return args.String(0), args.Error(1)
}

func (m *MockMerchantVARepository) RegisterStaticCustomerNo(ctx context.Context, partnerServiceID, customerNo string) error {
	args := m.Called(ctx, partnerServiceID, customerNo)
	return args.Error(0)
}

func (m *MockMerchantVARepository) SaveVAPayment(ctx context.Context, transactionID string, amount string, referenceNo string) (string, string, error) {
	args := m.Called(ctx, transactionID, amount, referenceNo)
	return args.String(0), args.String(1), args.Error(2)
}

// --- CreateVA Tests ---

func TestMerchantVAUsecase_CreateVA_Success(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	amount := &domain.Amount{Value: "150000.00", Currency: "IDR"}
	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:      "088899",
		CustomerNo:            "12345678901234567890",
		VirtualAccountNo:      "08889912345678901234567890",
		VirtualAccountName:    "Jokul Doe",
		TrxID:                 "trx-001",
		TotalAmount:           amount,
		VirtualAccountTrxType: "C",
		// notificationUrl is not a spec field; per ASPI VAUpsertRequest it's
		// carried in additionalInfo.dbUrlProcess (aspi-open-api-va.yaml:317-320).
		AdditionalInfo: map[string]interface{}{"dbUrlProcess": "https://example.com/webhook"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.MatchedBy(func(r *domain.VAInquiryRecord) bool {
		return r.NotificationURL == "https://example.com/webhook"
	})).Return(nil)

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		// TrxID missing
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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, mock.Anything).Return(nil, domain.ErrMerchantVANotFound)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:      "088899",
		CustomerNo:            "12345678901234567890",
		VirtualAccountNo:      "08889912345678901234567890",
		VirtualAccountName:    "Jokul Doe",
		TrxID:                 "trx-002",
		VirtualAccountTrxType: "Z", // Invalid
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002700", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_PendingTransaction_Conflicts(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	existing := &domain.VAInquiryRecord{
		ID:               "existing-id",
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
		Status:           "03", // Still pending / unpaid — an active transaction
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(existing, nil)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-003",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4092700", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_ReusesVANumberAfterPaidTransaction(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	existing := &domain.VAInquiryRecord{
		ID:               "existing-id",
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
		Status:           "00", // Previous transaction already paid — number is free to reuse
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(existing, nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-new-cycle",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "2002700", resp.ResponseCode)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_NotificationURLOptional(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-004",
		// NotificationURL intentionally omitted: not part of ASPI VAUpsertRequest
		// (required: virtualAccountName, trxId only), so a spec-exact payload
		// without it must be accepted, not rejected.
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "2002700", resp.ResponseCode)
}

func TestMerchantVAUsecase_CreateVA_WithBillDetails(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	amount := &domain.Amount{Value: "150000.00", Currency: "IDR"}
	billAmount := &domain.Amount{Value: "150000.00", Currency: "IDR"}
	expiry := time.Now().Add(7 * 24 * time.Hour)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:      "088899",
		CustomerNo:            "12345678901234567890",
		VirtualAccountNo:      "08889912345678901234567890",
		VirtualAccountName:    "Jokul Doe",
		TrxID:                 "trx-005",
		TotalAmount:           amount,
		VirtualAccountTrxType: "C",
		ExpiredDate:           &expiry,
		BillDetails: []domain.BillDetail{
			{
				BillCode:   "01",
				BillNo:     "INV-001",
				BillName:   "Invoice Januari",
				BillAmount: billAmount,
			},
		},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)
	mockRepo.On("SaveBillDetails", mock.Anything, mock.Anything, req.BillDetails).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Len(t, resp.VirtualAccountData.BillDetails, 1)
	assert.NotNil(t, resp.VirtualAccountData.ExpiredDate)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_BillDetailsSaveFails(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-006",
		BillDetails: []domain.BillDetail{
			{BillNo: "INV-002"},
		},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)
	mockRepo.On("SaveBillDetails", mock.Anything, mock.Anything, req.BillDetails).Return(assert.AnError)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002700", domainErr.SNAPCode)
}

// --- Static/Dynamic VA Tests (feature 006-static-dynamic-va) ---

func TestMerchantVAUsecase_CreateVA_DynamicNoBill(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "15973000000000000001",
		VirtualAccountName: "Dynamic NoBill",
		TrxID:              "trx-dyn-01",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000001", nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.MatchedBy(func(r *domain.VAInquiryRecord) bool {
		return r.CustomerNo == "04000000000000000001" && r.VAType == "04"
	})).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Equal(t, "04000000000000000001", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_DynamicVariableBill(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15974",
		CustomerNo:         "",
		VirtualAccountNo:   "15974000000000000002",
		VirtualAccountName: "Dynamic Variable",
		TrxID:              "trx-dyn-02",
		TotalAmount:        &domain.Amount{Value: "100000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "05"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "05").Return("05000000000000000001", nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "05000000000000000001", resp.VirtualAccountData.CustomerNo)
	assert.Equal(t, "100000.00", resp.VirtualAccountData.TotalAmount.Value)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_DynamicFixedBill(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15975",
		CustomerNo:         "",
		VirtualAccountNo:   "15975000000000000003",
		VirtualAccountName: "Dynamic Fixed",
		TrxID:              "trx-dyn-03",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "06"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "06").Return("06000000000000000001", nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "06000000000000000001", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_DynamicConcurrent_DistinctCustomerNo(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, mock.Anything).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000001", nil).Once()
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000002", nil).Once()
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	req1 := &domain.MerchantCreateVARequest{
		PartnerServiceID: "15973", CustomerNo: "", VirtualAccountNo: "15973000000000000011",
		VirtualAccountName: "A", TrxID: "trx-c1", AdditionalInfo: map[string]interface{}{"vaType": "04"},
	}
	req2 := &domain.MerchantCreateVARequest{
		PartnerServiceID: "15973", CustomerNo: "", VirtualAccountNo: "15973000000000000012",
		VirtualAccountName: "B", TrxID: "trx-c2", AdditionalInfo: map[string]interface{}{"vaType": "04"},
	}

	resp1, err1 := uc.CreateVA(context.Background(), req1)
	resp2, err2 := uc.CreateVA(context.Background(), req2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, resp1.VirtualAccountData.CustomerNo, resp2.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_SequenceGeneratorUnavailable(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "15973000000000000099",
		VirtualAccountName: "Dynamic NoBill",
		TrxID:              "trx-dyn-unavail",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("", assert.AnError)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002702", domainErr.SNAPCode)
	assert.Contains(t, domainErr.Message, assert.AnError.Error())
}

func TestMerchantVAUsecase_CreateVA_StaticNoBill_EchoesCustomerNo(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "15973000012345670001",
		VirtualAccountName: "Static NoBill",
		TrxID:              "trx-static-01",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("RegisterStaticCustomerNo", mock.Anything, "15973", "0001234567").Return(nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "0001234567", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_StaticVariableBill_EchoesCustomerNo(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15974",
		CustomerNo:         "0002234567",
		VirtualAccountNo:   "15974000012345670002",
		VirtualAccountName: "Static Variable",
		TrxID:              "trx-static-02",
		TotalAmount:        &domain.Amount{Value: "200000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "02"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("RegisterStaticCustomerNo", mock.Anything, "15974", "0002234567").Return(nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "0002234567", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_StaticFixedBill_EchoesCustomerNo(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15975",
		CustomerNo:         "0003234567",
		VirtualAccountNo:   "15975000012345670003",
		VirtualAccountName: "Static Fixed",
		TrxID:              "trx-static-03",
		TotalAmount:        &domain.Amount{Value: "300000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "03"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("RegisterStaticCustomerNo", mock.Anything, "15975", "0003234567").Return(nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "0003234567", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_DuplicateStaticCustomerNo_Conflict(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "15973000012345670099",
		VirtualAccountName: "Static NoBill Dup",
		TrxID:              "trx-static-dup",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	mockRepo.On("RegisterStaticCustomerNo", mock.Anything, "15973", "0001234567").Return(domain.ErrVACustomerNoAlreadyRegistered)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4092701", domainErr.SNAPCode)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_MismatchedPartnerServiceIDAndVAType(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0009999999",
		VirtualAccountNo:   "15973000099999990001",
		VirtualAccountName: "Invalid Combo",
		TrxID:              "trx-invalid-combo",
		AdditionalInfo:     map[string]interface{}{"vaType": "02"}, // 02 belongs to 15974, not 15973
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002702", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_UnrecognizedVAType(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0009999998",
		VirtualAccountNo:   "15973000099999980002",
		VirtualAccountName: "Unrecognized Type",
		TrxID:              "trx-unrecognized",
		AdditionalInfo:     map[string]interface{}{"vaType": "99"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002702", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_NonEmptyCustomerNoOnDynamic_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001111111", // must be empty for dynamic vaType 04
		VirtualAccountNo:   "15973000011111110001",
		VirtualAccountName: "Dynamic With CustomerNo",
		TrxID:              "trx-dyn-with-customerno",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002703", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_EmptyCustomerNoOnStatic_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "", // required for static vaType 01
		VirtualAccountNo:   "15973000000000000002",
		VirtualAccountName: "Static Without CustomerNo",
		TrxID:              "trx-static-without-customerno",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002704", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_NoBillWithTotalAmount_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "15973000000000000003",
		VirtualAccountName: "Dynamic NoBill With Amount",
		TrxID:              "trx-nobill-with-amount",
		TotalAmount:        &domain.Amount{Value: "50000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002706", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_FixedBillMissingTotalAmount_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15975",
		CustomerNo:         "0004234567",
		VirtualAccountNo:   "15975000012345670004",
		VirtualAccountName: "Static Fixed Missing Amount",
		TrxID:              "trx-fixed-missing-amount",
		AdditionalInfo:     map[string]interface{}{"vaType": "03"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002705", domainErr.SNAPCode)
}

func TestMerchantVAUsecase_CreateVA_UsesInjectedVATypeRuleProvider(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	mockProvider := new(MockVATypeRuleProvider)
	uc := NewMerchantVAUsecase(mockRepo, mockProvider)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "15973000000000000042",
		VirtualAccountName: "Provider DI Test",
		TrxID:              "trx-provider-di",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	rule := domain.VATypeRule{VAType: "04", PartnerServiceID: "15973", Dynamic: true, Billing: domain.VATypeBillingNone}
	mockProvider.On("LookupVATypeRule", mock.Anything, "15973", "04").Return(rule, true, nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000042", nil)
	mockRepo.On("SaveInquiry", mock.Anything, mock.AnythingOfType("*domain.VAInquiryRecord")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "04000000000000000042", resp.VirtualAccountData.CustomerNo)
	mockProvider.AssertExpectations(t)
	mockProvider.AssertNotCalled(t, "IsReservedPartnerServiceID", mock.Anything, mock.Anything)
}

func TestMerchantVAUsecase_CreateVA_VATypeRuleProviderFailure_SystemUnavailable(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	mockProvider := new(MockVATypeRuleProvider)
	uc := NewMerchantVAUsecase(mockRepo, mockProvider)

	// Legacy (unmanaged) request — no additionalInfo.vaType — so the
	// provider is consulted only for IsReservedPartnerServiceID.
	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "15973000000000000099",
		VirtualAccountName: "Provider Failure Test",
		TrxID:              "trx-provider-fail",
	}

	mockProvider.On("IsReservedPartnerServiceID", mock.Anything, "15973").Return(false, assert.AnError)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002702", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

// --- ListVA Tests ---

func TestMerchantVAUsecase_ListVA_Success(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

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
