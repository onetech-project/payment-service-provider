package usecase

import (
	"context"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func (m *MockMerchantVARepository) ClaimInquiryRequestID(ctx context.Context, id string, inquiryRequestID string) error {
	args := m.Called(ctx, id, inquiryRequestID)
	return args.Error(0)
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

func (m *MockMerchantVARepository) FindVAInstalment(ctx context.Context, paymentRequestID string) (string, string, bool, error) {
	return "", "", false, nil
}

func (m *MockMerchantVARepository) SaveVAPayment(ctx context.Context, transactionID, paymentRequestID, amount, referenceNo string) (string, string, bool, error) {
	args := m.Called(ctx, transactionID, paymentRequestID, amount, referenceNo)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

// VA registry methods (feature 013-no-bill-payment-transaction).

// hasExpectation reports whether the current test stubbed method. CreateVA now
// writes a registration for every managed VA type, so tests written before
// feature 013 would hit an un-stubbed SaveVAAccount and panic. Defaulting to
// success keeps those tests exercising exactly the behavior they assert, while
// tests that care about the registry stub it and get normal mock behavior.
func (m *MockMerchantVARepository) hasExpectation(method string) bool {
	for _, call := range m.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func (m *MockMerchantVARepository) SaveVAAccount(ctx context.Context, account *domain.VAAccount) error {
	if !m.hasExpectation("SaveVAAccount") {
		return nil
	}
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockMerchantVARepository) GetVAAccount(ctx context.Context, virtualAccountNo string) (*domain.VAAccount, error) {
	if !m.hasExpectation("GetVAAccount") {
		return nil, domain.ErrVAAccountNotFound
	}
	args := m.Called(ctx, virtualAccountNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAAccount), args.Error(1)
}

func (m *MockMerchantVARepository) GetVAAccountByPartnerAndCustomer(ctx context.Context, partnerServiceID, customerNo string) (*domain.VAAccount, error) {
	if !m.hasExpectation("GetVAAccountByPartnerAndCustomer") {
		return nil, domain.ErrVAAccountNotFound
	}
	args := m.Called(ctx, partnerServiceID, customerNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAAccount), args.Error(1)
}

func (m *MockMerchantVARepository) UpdateVAAccountStatus(ctx context.Context, virtualAccountNo string, status string) error {
	if !m.hasExpectation("UpdateVAAccountStatus") {
		return nil
	}
	args := m.Called(ctx, virtualAccountNo, status)
	return args.Error(0)
}

func (m *MockMerchantVARepository) ListVAAccounts(ctx context.Context, filter *domain.VAAccountListFilter) ([]domain.VAAccountListItem, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.VAAccountListItem), args.Int(1), args.Error(2)
}

func (m *MockMerchantVARepository) ListVATransactions(ctx context.Context, filter *domain.VAListFilter) ([]domain.VATransactionListItem, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.VATransactionListItem), args.Int(1), args.Error(2)
}

func (m *MockMerchantVARepository) SaveNoBillPayment(ctx context.Context, payment *domain.VAPaymentRecord) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
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

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000001", nil)
	// Since feature 013-no-bill-payment-transaction this lands in the VA
	// registry, not in a pending transaction — the customerNo generation and
	// echo behavior this test covers is otherwise unchanged.
	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.CustomerNo == "04000000000000000001" && a.VAType == "04"
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

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000001", nil).Once()
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000002", nil).Once()
	mockRepo.On("SaveVAAccount", mock.Anything, mock.AnythingOfType("*domain.VAAccount")).Return(nil)

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
		VirtualAccountNo:   "159730001234567",
		VirtualAccountName: "Static NoBill",
		TrxID:              "trx-static-01",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	// Since feature 013-no-bill-payment-transaction a static no-bill VA is
	// registered rather than transacted, and skips the one-shot
	// RegisterStaticCustomerNo check so a repeat call can update it (FR-005).
	// The customerNo echo behavior this test covers is unchanged.
	mockRepo.On("SaveVAAccount", mock.Anything, mock.AnythingOfType("*domain.VAAccount")).Return(nil)

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
		VirtualAccountNo:   "159740002234567",
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
		VirtualAccountNo:   "159750003234567",
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

	// Retargeted from vaType 01 to 02 by feature
	// 013-no-bill-payment-transaction: a repeat /create-va on a NO-BILL VA is
	// now an update, not a conflict (FR-005). Static BILL-bearing types keep
	// the one-shot customerNo rule unchanged (FR-021, research.md R-002), and
	// that is the behavior this test now guards.
	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15974",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "159740001234567",
		VirtualAccountName: "Static VariableBill Dup",
		TrxID:              "trx-static-dup",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "02"},
	}

	mockRepo.On("RegisterStaticCustomerNo", mock.Anything, "15974", "0001234567").Return(domain.ErrVACustomerNoAlreadyRegistered)

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

// --- VA Number Consistency Tests (feature 008-va-number-consistency) ---

func TestMerchantVAUsecase_CreateVA_StaticVirtualAccountNoMismatch_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "9999999999999999999999",
		VirtualAccountName: "Static Consistency Mismatch",
		TrxID:              "trx-static-mismatch",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002707", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "RegisterStaticCustomerNo", mock.Anything, mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

func TestMerchantVAUsecase_CreateVA_StaticVirtualAccountNoMatch_Succeeds(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "159730001234567",
		VirtualAccountName: "Static Consistency Match",
		TrxID:              "trx-static-match",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	// Registry, not transaction, since feature 013-no-bill-payment-transaction.
	// The partnerServiceId+customerNo consistency check this test covers is
	// unchanged.
	mockRepo.On("SaveVAAccount", mock.Anything, mock.AnythingOfType("*domain.VAAccount")).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Equal(t, "159730001234567", resp.VirtualAccountData.VirtualAccountNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_UnmanagedLegacyVirtualAccountNoMismatch_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, nil) // unmanaged/legacy mode (nil vaTypeRules)

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "00000000000000000000000000", // deliberately not partnerServiceId+customerNo
		VirtualAccountName: "Legacy Consistency Mismatch",
		TrxID:              "trx-legacy-mismatch",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002707", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

func TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoEmpty_AutoDerived(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "", // intentionally left empty
		VirtualAccountName: "Dynamic Auto-Derive",
		TrxID:              "trx-dyn-auto-derive",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000099", nil)
	// Registry, not transaction, since feature 013-no-bill-payment-transaction.
	// The VA-number derivation this test covers is unchanged.
	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.VirtualAccountNo == "1597304000000000000000099" && a.CustomerNo == "04000000000000000099"
	})).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "04000000000000000099", resp.VirtualAccountData.CustomerNo)
	assert.Equal(t, "1597304000000000000000099", resp.VirtualAccountData.VirtualAccountNo)
	mockRepo.AssertExpectations(t)
}

func TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoSupplied_UsedAsIs(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "1597304999999999999999", // merchant-chosen, not partnerServiceId+generated customerNo
		VirtualAccountName: "Dynamic Merchant Chosen",
		TrxID:              "trx-dyn-chosen",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000100", nil)
	// Registry, not transaction, since feature 013-no-bill-payment-transaction.
	// The "honor the merchant's own VA number" behavior is unchanged.
	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.VirtualAccountNo == "1597304999999999999999" && a.CustomerNo == "04000000000000000100"
	})).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "1597304999999999999999", resp.VirtualAccountData.VirtualAccountNo)
	assert.Equal(t, "04000000000000000100", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
}

// Retargeted from vaType 04 to 06 by feature 013-no-bill-payment-transaction:
// a no-bill VA no longer HAS a pending transaction to conflict with — that
// removal is the feature. Dynamic BILL-bearing types still create one at
// create-VA time and still conflict on it (FR-021), which is what this test
// now guards.
func TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoSupplied_ConflictOnPending(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15975",
		CustomerNo:         "",
		VirtualAccountNo:   "1597506888888888888888",
		VirtualAccountName: "Dynamic Merchant Chosen Conflict",
		TrxID:              "trx-dyn-conflict",
		TotalAmount:        &domain.Amount{Value: "75000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "06"},
	}

	existing := &domain.VAInquiryRecord{
		ID:               "existing-id",
		PartnerServiceID: "15975",
		VirtualAccountNo: "1597506888888888888888",
		Status:           "03", // active pending transaction
	}

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "06").Return("06000000000000000101", nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "1597506888888888888888").Return(existing, nil)

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4092700", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

func TestMerchantVAUsecase_CreateVA_StaticVirtualAccountNoTooLong_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, nil) // unmanaged/legacy mode

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "0888991234567890123456789012345", // > 28 chars
		VirtualAccountName: "Legacy Too Long",
		TrxID:              "trx-legacy-too-long",
	}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002700", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

func TestMerchantVAUsecase_CreateVA_DynamicDerivedVirtualAccountNoTooLong_Rejected(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	// partnerServiceId "15973" (5 chars) + a 24-char generated customerNo would
	// be 29 chars, exceeding the 28-char ASPI VAIdentity limit (FR-007). The
	// real sequence generator always returns 20-digit customerNo values (2
	// vaType + 18 sequence, feature 006), so this scenario is defensive/
	// future-proofing rather than reachable with today's generator — it
	// guards the derivation path regardless of generator implementation.
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "",
		VirtualAccountName: "Dynamic Derived Too Long",
		TrxID:              "trx-dyn-too-long",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("040000000000000000000001", nil) // 24 chars

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002700", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

func TestMerchantVAUsecase_InquiryVA_UsesServerDerivedVirtualAccountNo(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	// Create a dynamic VA leaving virtualAccountNo empty (server-derived, per
	// TestMerchantVAUsecase_CreateVA_DynamicVirtualAccountNoEmpty_AutoDerived).
	createReq := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "",
		VirtualAccountName: "Dynamic Inquiry Regression",
		TrxID:              "trx-dyn-inquiry-regress",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}
	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000200", nil)
	// Registry, not transaction, since feature 013-no-bill-payment-transaction;
	// CreateVA no longer probes GetVAByVirtualAccountNo for a no-bill VA, so
	// the only stub of it below belongs to this test's own lookup call.
	mockRepo.On("SaveVAAccount", mock.Anything, mock.AnythingOfType("*domain.VAAccount")).Return(nil)

	createResp, err := uc.CreateVA(context.Background(), createReq)
	assert.NoError(t, err)
	assert.Equal(t, "1597304000000000000000200", createResp.VirtualAccountData.VirtualAccountNo)
	assert.Equal(t, createResp.VirtualAccountData.PartnerServiceID+createResp.VirtualAccountData.CustomerNo, createResp.VirtualAccountData.VirtualAccountNo)

	// Confirm a subsequent lookup by that exact server-derived virtualAccountNo
	// resolves to the just-created record (regression: inquiry/payment lookups
	// are unaffected by how virtualAccountNo was produced).
	created := &domain.VAInquiryRecord{
		ID:               "created-id",
		PartnerServiceID: "15973",
		CustomerNo:       "04000000000000000200",
		VirtualAccountNo: "1597304000000000000000200",
		Status:           "03",
	}
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "1597304000000000000000200").Return(created, nil).Once()

	found, err := mockRepo.GetVAByVirtualAccountNo(context.Background(), "1597304000000000000000200")
	assert.NoError(t, err)
	assert.Equal(t, created, found)
	mockRepo.AssertExpectations(t)
}

// --- ListVA Tests ---

func TestMerchantVAUsecase_ListVA_Success(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	// Since feature 013-no-bill-payment-transaction, ListVA lists registered VA
	// numbers rather than transactions, so a VA with many payments shows once
	// with a transaction count (FR-023).
	items := []domain.VAAccountListItem{
		{
			VirtualAccountNo: "08889912345678901234567890",
			CustomerNo:       "12345678901234567890",
			CustomerName:     "Jokul Doe",
			VAType:           "01",
			Status:           domain.VAAccountStatusActive,
			TransactionCount: 3,
			TotalPaid:        &domain.Amount{Value: "60000.00", Currency: "IDR"},
			CreatedAt:        time.Now(),
		},
	}

	mockRepo.On("ListVAAccounts", mock.Anything, mock.AnythingOfType("*domain.VAAccountListFilter")).Return(items, 1, nil)

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

	mockRepo.On("ListVAAccounts", mock.Anything, mock.AnythingOfType("*domain.VAAccountListFilter")).Return([]domain.VAAccountListItem{}, 0, nil)

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

	mockRepo.On("ListVAAccounts", mock.Anything, mock.AnythingOfType("*domain.VAAccountListFilter")).Return([]domain.VAAccountListItem{}, 0, nil)

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

// --- No-Bill VA registration (feature 013-no-bill-payment-transaction, US1) ---
//
// The defect these cover: /create-va used to insert a PENDING transaction for
// a no-bill VA, which the first payment then consumed — making the VA payable
// exactly once and forcing the merchant to re-register before every payment.
// A no-bill VA is an address, not a transaction: registered once, paid many
// times, like an e-wallet top-up number.

// noBillCreateVAReq builds a minimal valid static no-bill (vaType 01) request.
func noBillCreateVAReq() *domain.MerchantCreateVARequest {
	return &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "159730001234567",
		VirtualAccountName: "NoBill Holder",
		TrxID:              "trx-nobill-1",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}
}

// T020: FR-001 / SC-002 — the headline assertion. A no-bill /create-va
// registers the VA and writes NO transaction.
func TestMerchantVAUsecase_CreateVA_StaticNoBill_WritesRegistrationNotTransaction(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := noBillCreateVAReq()
	req.VirtualAccountEmail = "holder@example.com"
	req.VirtualAccountPhone = "628123456789"
	req.AdditionalInfo["dbUrlProcess"] = "https://merchant.example.com/callback"

	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.PartnerServiceID == "15973" &&
			a.CustomerNo == "0001234567" &&
			a.VirtualAccountNo == "159730001234567" &&
			a.VAType == "01" &&
			a.CustomerName == "NoBill Holder" &&
			a.CustomerEmail == "holder@example.com" &&
			a.CustomerPhone == "628123456789" &&
			a.TrxID == "trx-nobill-1" &&
			a.NotificationURL == "https://merchant.example.com/callback" &&
			a.Status == domain.VAAccountStatusActive
	})).Return(nil).Once()

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Equal(t, "0001234567", resp.VirtualAccountData.CustomerNo)
	mockRepo.AssertExpectations(t)
	// The whole point of the feature: no transaction row, and no pending-VA
	// lookup that would gate one.
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo", mock.Anything, mock.Anything)
}

// T021: FR-003 — dynamic no-bill still generates a sequential customerNo and
// derives the VA number, but lands in the registry rather than a transaction.
func TestMerchantVAUsecase_CreateVA_DynamicNoBill_WritesRegistrationNotTransaction(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountName: "Dynamic NoBill Holder",
		TrxID:              "trx-dyn-nobill-1",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	mockRepo.On("NextCustomerNoSequence", mock.Anything, "04").Return("04000000000000000001", nil).Once()
	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.CustomerNo == "04000000000000000001" &&
			a.VAType == "04" &&
			// virtualAccountNo omitted by the merchant, so derived per SNAP.
			a.VirtualAccountNo == "1597304000000000000000001"
	})).Return(nil).Once()

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	assert.Equal(t, "04000000000000000001", resp.VirtualAccountData.CustomerNo)
	assert.Equal(t, "1597304000000000000000001", resp.VirtualAccountData.VirtualAccountNo)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

// T022: FR-005 — "create-va only needs to be called once per VA number". A
// repeat call updates the holder details instead of conflicting, because ASPI
// models this endpoint as an upsert (VAUpsertRequest).
func TestMerchantVAUsecase_CreateVA_NoBillRepeatCall_UpdatesRegistration(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	// No pre-read is stubbed on purpose: SaveVAAccount is an upsert keyed on
	// virtual_account_no, so a repeat call needs no "does it exist yet?" query.
	// That is precisely why the second call can't conflict.
	req := noBillCreateVAReq()
	req.VirtualAccountName = "Renamed Holder"
	req.TrxID = "trx-nobill-2"

	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.CustomerName == "Renamed Holder" && a.TrxID == "trx-nobill-2"
	})).Return(nil).Once()

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	mockRepo.AssertExpectations(t)
	// Explicitly NOT 4092700 / 4092701, and still no transaction.
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
	// No-bill VAs skip the one-shot customerNo registration entirely — that
	// check is what used to make a second call a conflict.
	mockRepo.AssertNotCalled(t, "RegisterStaticCustomerNo", mock.Anything, mock.Anything, mock.Anything)
}

// T023: FR-006 — a no-bill VA carries no bill, so totalAmount is rejected and
// nothing at all is persisted.
func TestMerchantVAUsecase_CreateVA_NoBillWithTotalAmount_PersistsNothing(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := noBillCreateVAReq()
	req.TotalAmount = &domain.Amount{Value: "50000.00", Currency: "IDR"}

	resp, err := uc.CreateVA(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4002706", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveVAAccount", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

// T027: FR-021 / spec A-002 — bill-bearing types gain a registration for
// identity but keep creating their pending transaction at create-VA time.
func TestMerchantVAUsecase_CreateVA_FixedBill_WritesBothRegistrationAndTransaction(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "15975",
		CustomerNo:         "0009876543",
		VirtualAccountNo:   "159750009876543",
		VirtualAccountName: "Fixed Bill Holder",
		TrxID:              "trx-fixed-1",
		TotalAmount:        &domain.Amount{Value: "75000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"vaType": "03"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("RegisterStaticCustomerNo", mock.Anything, "15975", "0009876543").Return(nil)
	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.VAType == "03" && a.Status == domain.VAAccountStatusActive
	})).Return(nil).Once()
	mockRepo.On("SaveInquiry", mock.Anything, mock.MatchedBy(func(r *domain.VAInquiryRecord) bool {
		return r.Status == "03" && r.TotalAmount == "75000.00" && r.VAType == "03"
	})).Return(nil).Once()

	resp, err := uc.CreateVA(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	mockRepo.AssertExpectations(t)
}

// T028: a registration write failure must surface as a 500, matching how a
// SaveInquiry failure is handled.
func TestMerchantVAUsecase_CreateVA_NoBillRegistrationFailure_InternalError(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("SaveVAAccount", mock.Anything, mock.Anything).Return(assert.AnError)

	resp, err := uc.CreateVA(context.Background(), noBillCreateVAReq())

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002700", domainErr.SNAPCode)
}

// --- No-Bill VA deactivation (feature 013-no-bill-payment-transaction, US6) ---

func noBillDeleteReq() *domain.MerchantDeleteVARequest {
	return &domain.MerchantDeleteVARequest{
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		TrxID:            "trx-del-1",
	}
}

func activeNoBillAccount() *domain.VAAccount {
	return &domain.VAAccount{
		ID:               "acc-nobill-del",
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		VAType:           "01",
		Billing:          domain.VATypeBillingNone,
		CustomerName:     "NoBill Holder",
		Status:           domain.VAAccountStatusActive,
	}
}

// T065 / T069: FR-019 / FR-020 — delete deactivates the registration, and
// touches no historical transaction. A no-bill VA has no pending transaction
// to cancel; its settled payments are history and must stay readable.
func TestMerchantVAUsecase_DeleteVA_NoBill_DeactivatesRegistration(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(activeNoBillAccount(), nil)
	mockRepo.On("UpdateVAAccountStatus", mock.Anything, "159730001234567", domain.VAAccountStatusInactive).Return(nil).Once()

	resp, err := uc.DeleteVA(context.Background(), noBillDeleteReq())

	assert.NoError(t, err)
	assert.Equal(t, "2003100", resp.ResponseCode)
	assert.Equal(t, "159730001234567", resp.VirtualAccountData.VirtualAccountNo)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "UpdateVAStatus", mock.Anything, mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo", mock.Anything, mock.Anything)
}

// T066: US6 AS4 — repeating the delete is a no-op success. The repository's
// WHERE status='ACTIVE' guard reports "no ACTIVE row" via ErrVAAccountNotFound,
// which here means "already deactivated", not "failed".
func TestMerchantVAUsecase_DeleteVA_NoBill_RepeatIsIdempotent(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	inactive := activeNoBillAccount()
	inactive.Status = domain.VAAccountStatusInactive
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(inactive, nil)
	mockRepo.On("UpdateVAAccountStatus", mock.Anything, "159730001234567", domain.VAAccountStatusInactive).
		Return(domain.ErrVAAccountNotFound)

	resp, err := uc.DeleteVA(context.Background(), noBillDeleteReq())

	assert.NoError(t, err)
	assert.Equal(t, "2003100", resp.ResponseCode)
}

// A genuine persistence failure must still surface, not be swallowed by the
// idempotency allowance above.
func TestMerchantVAUsecase_DeleteVA_NoBill_PersistFailureIs500(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(activeNoBillAccount(), nil)
	mockRepo.On("UpdateVAAccountStatus", mock.Anything, "159730001234567", domain.VAAccountStatusInactive).
		Return(assert.AnError)

	resp, err := uc.DeleteVA(context.Background(), noBillDeleteReq())

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5003100", domainErr.SNAPCode)
}

// T074: FR-005 — re-registering a deactivated VA reactivates it, because the
// registration upsert always writes status=ACTIVE.
func TestMerchantVAUsecase_CreateVA_NoBill_ReactivatesDeactivatedRegistration(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("SaveVAAccount", mock.Anything, mock.MatchedBy(func(a *domain.VAAccount) bool {
		return a.Status == domain.VAAccountStatusActive
	})).Return(nil).Once()

	resp, err := uc.CreateVA(context.Background(), noBillCreateVAReq())

	assert.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
	mockRepo.AssertExpectations(t)
}

// Bill-bearing VAs keep the transaction-cancellation delete path untouched
// (FR-021).
func TestMerchantVAUsecase_DeleteVA_BillBearing_UsesTransactionPath(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	billed := activeNoBillAccount()
	billed.VAType = "03"
	billed.Billing = domain.VATypeBillingFixed
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(billed, nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "159730001234567").
		Return(&domain.VAInquiryRecord{ID: "txn-1", VirtualAccountNo: "159730001234567", Status: "03"}, nil)
	mockRepo.On("UpdateVAStatus", mock.Anything, "159730001234567", "04").Return(nil).Once()

	resp, err := uc.DeleteVA(context.Background(), noBillDeleteReq())

	assert.NoError(t, err)
	assert.Equal(t, "2003100", resp.ResponseCode)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "UpdateVAAccountStatus", mock.Anything, mock.Anything, mock.Anything)
}

// T012/T077/T078: FR-023 / SC-007 — the two listings answer different
// questions. For a no-bill VA paid three times, ListVA returns 1 entry and
// ListTransactions returns 3.
func TestMerchantVAUsecase_ListVA_And_ListTransactions_SeparateViews(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("ListVAAccounts", mock.Anything, mock.AnythingOfType("*domain.VAAccountListFilter")).
		Return([]domain.VAAccountListItem{{
			VirtualAccountNo: "159730001234567",
			TransactionCount: 3,
			TotalPaid:        &domain.Amount{Value: "60000.00", Currency: "IDR"},
			Status:           domain.VAAccountStatusActive,
		}}, 1, nil)

	mockRepo.On("ListVATransactions", mock.Anything, mock.AnythingOfType("*domain.VAListFilter")).
		Return([]domain.VATransactionListItem{
			{VirtualAccountNo: "159730001234567", PaymentRequestID: "PAY-1", Status: "00"},
			{VirtualAccountNo: "159730001234567", PaymentRequestID: "PAY-2", Status: "00"},
			{VirtualAccountNo: "159730001234567", PaymentRequestID: "PAY-3", Status: "00"},
		}, 3, nil)

	req := &domain.MerchantListVARequest{VirtualAccountNo: "159730001234567", Page: 1, PageSize: 20}

	vaResp, err := uc.ListVA(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, vaResp.Data, 1, "one VA")
	assert.Equal(t, 3, vaResp.Data[0].TransactionCount)

	txnResp, err := uc.ListTransactions(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, txnResp.Data, 3, "three transactions")
	assert.Equal(t, 3, txnResp.Pagination.TotalRows)
	mockRepo.AssertExpectations(t)
}

// Paging is clamped identically on both listings.
func TestMerchantVAUsecase_ListTransactions_NormalizesPaging(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	var captured *domain.VAListFilter
	mockRepo.On("ListVATransactions", mock.Anything, mock.AnythingOfType("*domain.VAListFilter")).
		Run(func(args mock.Arguments) { captured = args.Get(1).(*domain.VAListFilter) }).
		Return([]domain.VATransactionListItem{}, 0, nil)

	resp, err := uc.ListTransactions(context.Background(), &domain.MerchantListVARequest{Page: 0, PageSize: 5000})

	require.NoError(t, err)
	assert.Equal(t, 1, resp.Pagination.Page)
	assert.Equal(t, 20, resp.Pagination.PageSize)
	assert.Equal(t, 0, captured.Offset)
	assert.Equal(t, 20, captured.Limit)
}

func TestMerchantVAUsecase_ListTransactions_RepositoryFailureIs500(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	mockRepo.On("ListVATransactions", mock.Anything, mock.Anything).
		Return([]domain.VATransactionListItem{}, 0, assert.AnError)

	resp, err := uc.ListTransactions(context.Background(), &domain.MerchantListVARequest{Page: 1, PageSize: 20})

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002400", domainErr.SNAPCode)
}
