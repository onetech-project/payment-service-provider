package usecase

import (
	"context"
	"encoding/json"
	"errors"
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

func (m *MockVARepository) ClaimInquiryRequestID(ctx context.Context, id string, inquiryRequestID string) error {
	args := m.Called(ctx, id, inquiryRequestID)
	return args.Error(0)
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

// GetPaymentByPaymentRequestID falls back to the "GetPayment" expectation when
// a test hasn't stubbed it explicitly — most tests predate the strict/lenient
// split and only care that a payment is or isn't already on file.
func (m *MockVARepository) GetPaymentByPaymentRequestID(ctx context.Context, paymentRequestID string) (*domain.VAPaymentRecord, error) {
	if !m.hasExpectation("GetPaymentByPaymentRequestID") {
		return m.GetPayment(ctx, paymentRequestID)
	}
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

func (m *MockVARepository) FindVAInstalment(ctx context.Context, paymentRequestID string) (string, string, bool, error) {
	if !m.hasExpectation("FindVAInstalment") {
		return "", "", false, nil
	}
	args := m.Called(ctx, paymentRequestID)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

func (m *MockVARepository) SaveVAPayment(ctx context.Context, transactionID, paymentRequestID, amount, referenceNo string) (string, string, bool, error) {
	args := m.Called(ctx, transactionID, paymentRequestID, amount, referenceNo)
	return args.String(0), args.String(1), args.Bool(2), args.Error(3)
}

// VA registry methods (feature 013-no-bill-payment-transaction).

// hasExpectation reports whether the current test stubbed method. Tests
// written before feature 013 don't stub the registry lookups, and testify
// panics on an un-stubbed call — so the registry methods below fall back to a
// "no registration" answer, which routes those tests down the unchanged legacy
// path exactly as they expect. Tests that care about the registry stub it
// explicitly and get normal mock behavior.
func (m *MockVARepository) hasExpectation(method string) bool {
	for _, call := range m.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func (m *MockVARepository) SaveVAAccount(ctx context.Context, account *domain.VAAccount) error {
	if !m.hasExpectation("SaveVAAccount") {
		return nil
	}
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockVARepository) GetVAAccount(ctx context.Context, virtualAccountNo string) (*domain.VAAccount, error) {
	if !m.hasExpectation("GetVAAccount") {
		return nil, domain.ErrVAAccountNotFound
	}
	args := m.Called(ctx, virtualAccountNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAAccount), args.Error(1)
}

func (m *MockVARepository) GetVAAccountByPartnerAndCustomer(ctx context.Context, partnerServiceID, customerNo string) (*domain.VAAccount, error) {
	if !m.hasExpectation("GetVAAccountByPartnerAndCustomer") {
		return nil, domain.ErrVAAccountNotFound
	}
	args := m.Called(ctx, partnerServiceID, customerNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VAAccount), args.Error(1)
}

func (m *MockVARepository) UpdateVAAccountStatus(ctx context.Context, virtualAccountNo string, status string) error {
	if !m.hasExpectation("UpdateVAAccountStatus") {
		return nil
	}
	args := m.Called(ctx, virtualAccountNo, status)
	return args.Error(0)
}

func (m *MockVARepository) ListVAAccounts(ctx context.Context, filter *domain.VAAccountListFilter) ([]domain.VAAccountListItem, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.VAAccountListItem), args.Int(1), args.Error(2)
}

func (m *MockVARepository) ListVATransactions(ctx context.Context, filter *domain.VAListFilter) ([]domain.VATransactionListItem, int, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domain.VATransactionListItem), args.Int(1), args.Error(2)
}

func (m *MockVARepository) SaveNoBillPayment(ctx context.Context, payment *domain.VAPaymentRecord) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

// An inquiry for a VA that neither an inquiryRequestId nor a virtualAccountNo
// lookup can find is answered 4042412 (404). It must NOT persist a row: only
// the merchant's create-va brings a VA into existence.
func TestVAUsecase_Inquiry_VANotFound(t *testing.T) {
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

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.Nil(t, resp)
	require.Error(t, err)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042412", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry")
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
	mockRepo.On("ClaimInquiryRequestID", mock.Anything, merchantVA.ID, req.InquiryRequestID).Return(nil)
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
	// The merchant row was created with no inquiryRequestId; this inquiry is
	// what supplies it, so it must be stamped onto that same row rather than
	// a second row being inserted.
	mockRepo.AssertCalled(t, "ClaimInquiryRequestID", mock.Anything, merchantVA.ID, req.InquiryRequestID)
}

// A row that already carries an inquiryRequestId must not have it rewritten by
// a later inquiry using a different id — Status and Payment resolve the
// transaction by the id stored at first claim.
func TestVAUsecase_Inquiry_AlreadyClaimedInquiryRequestID_NotReclaimed(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		InquiryRequestID: "INQ-second-attempt",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		ID:               "existing-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     "Faris",
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: "INQ-claimed-first",
		TrxID:            "TRX-original",
		Status:           "03",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return([]domain.BillDetail{}, nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	mockRepo.AssertNotCalled(t, "ClaimInquiryRequestID")
}

func TestVAUsecase_Inquiry_WithoutAmount_IsProcessed(t *testing.T) {
	// BCA's inquiry payload has no `amount` field at all. Requiring one
	// rejected every conformant inquiry with 4002400, so an inquiry that
	// omits it must reach the repository lookup like any other.
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	mockRepo.On("GetVAAccount", mock.Anything, req.VirtualAccountNo).
		Return(nil, domain.ErrVAAccountNotFound)
	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).
		Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).
		Return(nil, domain.ErrMerchantVANotFound)

	resp, err := usecase.Inquiry(context.Background(), req)

	// No such VA, so 4042412 — but crucially NOT a 400 about a missing amount.
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodeInquiryNotFound, domainErr.SNAPCode)
	mockRepo.AssertExpectations(t)
}

func TestVAInquiryRequest_UnmarshalsSpecCompliantFields(t *testing.T) {
	body := []byte(`{
		"partnerServiceId": "12345",
		"customerNo": "123456789012345678",
		"virtualAccountNo": "12345123456789012345678",
		"trxDateInit": "2026-07-23T10:00:00+07:00",
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
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     "Faris",
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
		SubCompany:       "00001",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(existing, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, existing.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	// The replay is answered from the stored row, not from constants.
	assert.Equal(t, "Faris", resp.VirtualAccountData.VirtualAccountName)
	assert.Equal(t, "100000.00", resp.VirtualAccountData.TotalAmount.Value)
	assert.Equal(t, "00001", resp.VirtualAccountData.SubCompany)
	mockRepo.AssertNotCalled(t, "SaveInquiry")
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo")
}

func TestVAUsecase_Inquiry_PaidBill_Rejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		InquiryRequestID: "INQ-after-payment",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	paid := &domain.VAInquiryRecord{
		ID:               "paid-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "budi manjo",
		Status:           "00",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
		SubCompany:       "00000",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(paid, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, paid.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042414", domainErr.SNAPCode)

	// The rejection reports the bill it refuses, not just that it refused one.
	require.NotNil(t, domainErr.InquiryData)
	assert.Equal(t, "01", domainErr.InquiryData.InquiryStatus)
	assert.Equal(t, &domain.BilingualText{English: "Bill has been paid", Indonesia: "Tagihan telah dibayar"}, domainErr.InquiryData.InquiryReason)
	assert.Equal(t, req.PartnerServiceID, domainErr.InquiryData.PartnerServiceID)
	assert.Equal(t, req.CustomerNo, domainErr.InquiryData.CustomerNo)
	assert.Equal(t, req.VirtualAccountNo, domainErr.InquiryData.VirtualAccountNo)
	assert.Equal(t, "budi manjo", domainErr.InquiryData.VirtualAccountName)
	assert.Equal(t, req.InquiryRequestID, domainErr.InquiryData.InquiryRequestID)
	assert.Equal(t, &domain.Amount{Value: "10000.00", Currency: "IDR"}, domainErr.InquiryData.TotalAmount)
	assert.Equal(t, "00000", domainErr.InquiryData.SubCompany)
	mockRepo.AssertNotCalled(t, "SaveInquiry")
}

// A rejected inquiry must still serialize the full SNAP shape: billDetails and
// freeTexts as [] rather than null, and a top-level additionalInfo object.
func TestVAInquiryResponse_RejectedJSONShape(t *testing.T) {
	resp := domain.VAInquiryResponse{
		ResponseCode:    "4042414",
		ResponseMessage: "Paid Bill",
		VirtualAccountData: &domain.VAAccountData{
			PartnerServiceID:   "   15974",
			CustomerNo:         "77121730326",
			VirtualAccountNo:   "   1597477121730326",
			VirtualAccountName: "budi manjo bill var",
			InquiryRequestID:   "202607021545081597400051562507",
			TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
			SubCompany:         "00000",
			InquiryStatus:      "01",
			InquiryReason:      &domain.BilingualText{English: "Bill has been paid", Indonesia: "Tagihan telah dibayar"},
		},
	}

	body, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.JSONEq(t, `{"responseCode":"4042414","responseMessage":"Paid Bill","virtualAccountData":{"partnerServiceId":"   15974","customerNo":"77121730326","virtualAccountNo":"   1597477121730326","virtualAccountName":"budi manjo bill var","inquiryRequestId":"202607021545081597400051562507","totalAmount":{"value":"150000.00","currency":"IDR"},"subCompany":"00000","billDetails":[],"freeTexts":[],"inquiryStatus":"01","inquiryReason":{"english":"Bill has been paid","indonesia":"Tagihan telah dibayar"}},"additionalInfo":{}}`, string(body))
}

func TestVAUsecase_Inquiry_DeletedVA_Rejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		InquiryRequestID: "INQ-after-delete",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	deleted := &domain.VAInquiryRecord{
		ID:               "deleted-transaction-id",
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "04",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(deleted, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, deleted.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042412", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry")
}

// A VA whose biller has no sub-company code still reports subCompany: BCA
// expects the field present, and "00000" is its "none" value.
func TestVAUsecase_Inquiry_NoSubCompany_ReportsDefault(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		InquiryRequestID: "INQ-no-subcompany",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	record := &domain.VAInquiryRecord{
		ID:               "no-subcompany-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "budi manjo",
		InquiryRequestID: req.InquiryRequestID,
		Status:           "03",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(record, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, record.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "00000", resp.VirtualAccountData.SubCompany)
}

func TestVAUsecase_Inquiry_SubCompanyFallsBackToBillDetails(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		InquiryRequestID: "INQ-subcompany",
		Amount:           &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	// No sub_company on the transaction itself — create-va carries the code on
	// the bill rows, which is where it must then be read from.
	merchantVA := &domain.VAInquiryRecord{
		ID:               "existing-transaction-id",
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "Faris",
		Status:           "03",
		TotalAmount:      "10000.00",
		Currency:         "IDR",
	}
	bills := []domain.BillDetail{
		{BillNo: "INV-001", BillSubCompany: "00002", BillAmount: &domain.Amount{Value: "10000.00", Currency: "IDR"}},
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("ClaimInquiryRequestID", mock.Anything, merchantVA.ID, req.InquiryRequestID).Return(nil)
	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return(bills, nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "00002", resp.VirtualAccountData.SubCompany)
}

// Rows written before create-va stopped filling inquiry_request_id carry a
// copy of trx_id there. That is a placeholder, not a vendor id, so the first
// real inquiry must replace it — otherwise those rows stay addressable only by
// the merchant's trxId, which the vendor never sends.
func TestVAUsecase_Inquiry_TrxIDPlaceholder_IsReplacedByRequestID(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "70001",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "7000108212221111",
		InquiryRequestID: "INQ-from-vendor-0001",
		Amount:           &domain.Amount{Value: "75000.00", Currency: "IDR"},
	}

	legacyVA := &domain.VAInquiryRecord{
		ID:               "legacy-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     "Faris",
		VirtualAccountNo: req.VirtualAccountNo,
		// The placeholder: identical to TrxID, as the old create-va wrote it.
		InquiryRequestID: "TRX-original",
		TrxID:            "TRX-original",
		Status:           "03",
		TotalAmount:      "75000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(legacyVA, nil)
	mockRepo.On("ClaimInquiryRequestID", mock.Anything, legacyVA.ID, req.InquiryRequestID).Return(nil)
	mockRepo.On("GetVABillDetails", mock.Anything, legacyVA.ID).Return([]domain.BillDetail{}, nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, req.InquiryRequestID, resp.VirtualAccountData.InquiryRequestID)
	mockRepo.AssertCalled(t, "ClaimInquiryRequestID", mock.Anything, legacyVA.ID, req.InquiryRequestID)
	mockRepo.AssertExpectations(t)
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

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-1",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "Budi",
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
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

func TestVAUsecase_Payment_DuplicatePaymentRequestID_EchoesPersistedFields(t *testing.T) {
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

	// A paymentRequestId that is already on file is an Inconsistent Request,
	// not a replayed success — but the rejection still echoes the colliding
	// payment so the vendor can see what it hit.
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042518", domainErr.SNAPCode)
	assert.Equal(t, "Inconsistent Request", domainErr.Message)

	require.NotNil(t, domainErr.PaymentData)
	// paymentFlagStatus describes the ORIGINAL payment, which did settle.
	assert.Equal(t, "00", domainErr.PaymentData.PaymentFlagStatus)
	assert.Equal(t, existing.PartnerServiceID, domainErr.PaymentData.PartnerServiceID)
	assert.Equal(t, existing.VirtualAccountNo, domainErr.PaymentData.VirtualAccountNo)
	assert.Equal(t, existing.PaymentRequestID, domainErr.PaymentData.PaymentRequestID)
	assert.Equal(t, existing.PaidAmount, domainErr.PaymentData.PaidAmount.Value)
	mockRepo.AssertNotCalled(t, "SavePayment")
}

// Mandatory-field rejection moved to domain.ValidatePaymentRequest, invoked
// by the handler, so that BCA's 4002502 (Invalid Mandatory Field) can be
// distinguished from 4002501 (Invalid Field Format). Coverage now lives in
// TestValidatePaymentRequest_* (internal/domain) and
// TestVAHandler_Payment_MissingMandatoryField (handler package).

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
	// BCA publishes 4042514 "Paid Bill" for this case. It used to answer
	// 4092500 Conflict, which tells the channel the X-EXTERNAL-ID was
	// duplicated rather than that the bill was already settled.
	assert.Equal(t, domain.CodePaymentPaidBill, domainErr.SNAPCode)
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

	// A VA with a transaction row but no registered notificationUrl. The VA
	// must exist: an unregistered VA number is now rejected 4042512 rather
	// than recorded as an orphan payment.
	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-no-url",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
		NotificationURL:  "",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)

	resp, err := usecase.Payment(context.Background(), req)

	require.NoError(t, err)
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

	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return([]domain.BillDetail(nil), nil)
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
	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return([]domain.BillDetail(nil), nil)
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

	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return([]domain.BillDetail(nil), nil)
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

	mockRepo.On("GetVABillDetails", mock.Anything, merchantVA.ID).Return([]domain.BillDetail(nil), nil)
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

// pendingBillFor is the open fixed-bill transaction a payment lands on. Every
// payment path needs one: a VA with no transaction at all is refused 4042512
// before any of the logic these tests exercise is reached.
func pendingBillFor(req *domain.VAPaymentRequest) *domain.VAInquiryRecord {
	return &domain.VAInquiryRecord{
		ID:               "pending-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		VAType:           "03",
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}
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

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-mismatch",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		TotalAmount:      "200000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	assert.ErrorAs(t, err, &domainErr)
	// BCA's code for "The amount in not as expected" is 4042513, not a
	// 400-class field-format error.
	assert.Equal(t, domain.CodePaymentInvalidAmt, domainErr.SNAPCode)
}

func TestVAUsecase_Payment_UnderpaysStoredBill_Rejected(t *testing.T) {
	// The regression this guards: paidAmount used to be compared against the
	// request's OWN totalAmount, so a payment of 1.00 against a 250000.00 bill
	// was accepted as long as the caller claimed totalAmount was 1.00 too.
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		PaymentRequestID: "PAY-UNDERPAY",
		PaidAmount:       &domain.Amount{Value: "1.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "1.00", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-underpay",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		TotalAmount:      "250000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentInvalidAmt, domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SavePayment", mock.Anything, mock.Anything)
}

func TestVAUsecase_Payment_AmountComparedNumerically(t *testing.T) {
	// BCA sends "250000" and "250000.00" interchangeably; a string compare
	// rejects the pair as a mismatch.
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		PaymentRequestID: "PAY-NUMERIC",
		PaidAmount:       &domain.Amount{Value: "250000", Currency: "IDR"},
	}

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-numeric",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		TotalAmount:      "250000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)

	resp, err := usecase.Payment(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, domain.CodePaymentSuccess, resp.ResponseCode)
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

	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-optional-total",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("SavePayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).Return(nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestVAUsecase_Payment_UnregisteredVA_Rejected(t *testing.T) {
	// Neither a registration nor a transaction: BCA's 4042512
	// "The inputted Virtual Account Number is Unregistered". This used to be
	// recorded as an orphan payment row that no merchant could reconcile.
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "999999999999999999",
		VirtualAccountNo: " 12345999999999999999999",
		PaymentRequestID: "PAY-UNKNOWN",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentNotFound, domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SavePayment", mock.Anything, mock.Anything)
}

// --- Variable-bill multi-payment tests (feature 006-static-dynamic-va) ---

func TestVAUsecase_Payment_VariableBill_PartialPayment_FlagsSuccess(t *testing.T) {
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
	mockRepo.On("SaveVAPayment", mock.Anything, "txn-var-1", req.PaymentRequestID, "60000.00", req.ReferenceNo).Return("60000.00", "03", true, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.NoError(t, err)
	// An accepted instalment is paymentFlagStatus "00". It used to report
	// "03", which is valid only on the inquiry-status service — BCA reads
	// anything outside 00/01/02 on payment as 01, i.e. rejected, so every
	// accepted partial payment looked to the channel like a failure.
	assert.Equal(t, domain.PaymentFlagSuccess, resp.VirtualAccountData.PaymentFlagStatus)
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
	mockRepo.On("SaveVAPayment", mock.Anything, "txn-var-1", req.PaymentRequestID, "40000.00", req.ReferenceNo).Return("100000.00", "00", true, nil)

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

// --- No-Bill VA payments (feature 013-no-bill-payment-transaction, US2) ---
//
// The defect these cover: a no-bill VA could be paid exactly once. The first
// payment settled the single pending transaction created at /create-va time,
// and every payment after that hit the "already paid or inactive" guard. A
// no-bill VA is a durable payment address — each payment is its own
// transaction, like an e-wallet top-up.

// noBillAccount builds an ACTIVE static no-bill registration.
func noBillAccount() *domain.VAAccount {
	return &domain.VAAccount{
		ID:               "acc-nobill-1",
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		VAType:           "01",
		Billing:          domain.VATypeBillingNone,
		CustomerName:     "NoBill Holder",
		CustomerEmail:    "holder@example.com",
		CustomerPhone:    "628123456789",
		TrxID:            "trx-nobill-1",
		NotificationURL:  "https://merchant.example.com/callback",
		Status:           domain.VAAccountStatusActive,
	}
}

// noBillPaymentReq builds a payment notification against the registration above.
func noBillPaymentReq(paymentRequestID, amount string) *domain.VAPaymentRequest {
	return &domain.VAPaymentRequest{
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		PaymentRequestID: paymentRequestID,
		PaidAmount:       &domain.Amount{Value: amount, Currency: "IDR"},
		ReferenceNo:      "REF" + paymentRequestID,
	}
}

// T031: FR-008 / FR-010 — the headline assertion. Two payments into ONE
// registration both succeed and produce two independent settled transactions.
// Before this feature the second returned 4092500.
func TestVAUsecase_Payment_NoBill_SecondPaymentSucceeds(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
	mockRepo.On("GetPayment", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)

	var saved []*domain.VAPaymentRecord
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).
		Run(func(args mock.Arguments) {
			saved = append(saved, args.Get(1).(*domain.VAPaymentRecord))
		}).Return(nil)

	resp1, err1 := uc.Payment(context.Background(), noBillPaymentReq("PAY-1", "10000.00"))
	resp2, err2 := uc.Payment(context.Background(), noBillPaymentReq("PAY-2", "20000.00"))

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, "2002500", resp1.ResponseCode)
	assert.Equal(t, "2002500", resp2.ResponseCode)
	assert.Equal(t, "00", resp1.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, "00", resp2.VirtualAccountData.PaymentFlagStatus)

	// Two independent rows, neither overwriting the other.
	require.Len(t, saved, 2)
	assert.Equal(t, "10000.00", saved[0].PaidAmount)
	assert.Equal(t, "20000.00", saved[1].PaidAmount)
	// The upsert path must not be used — it would settle a pending row instead
	// of creating a new one.
	mockRepo.AssertNotCalled(t, "SavePayment", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "SaveVAPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// T032: FR-009 / research.md R-003 — the persisted row's identity fields.
// inquiry_request_id is set to paymentRequestId so its existing UNIQUE index
// gives one row per payment plus database-level duplicate rejection.
func TestVAUsecase_Payment_NoBill_PersistedRecordFields(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
	mockRepo.On("GetPayment", mock.Anything, "PAY-3").Return(nil, domain.ErrVAInvalidBill)

	var saved *domain.VAPaymentRecord
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(*domain.VAPaymentRecord) }).Return(nil)

	req := noBillPaymentReq("PAY-3", "35000.00")
	// A vendor-supplied inquiryRequestId/trxId must NOT win: reusing either as
	// the row key would let two payments collide onto one row.
	req.InquiryRequestID = "some-shared-inquiry-id"
	req.TrxID = "some-shared-trx-id"

	_, err := uc.Payment(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "PAY-3", saved.InquiryRequestID)
	assert.Equal(t, "PAY-3", saved.PaymentRequestID)
	assert.Equal(t, "00", saved.Status)
	assert.Equal(t, "35000.00", saved.PaidAmount)
	// No bill exists, so the payment IS the total.
	assert.Equal(t, "35000.00", saved.TotalAmount)
	assert.Equal(t, "01", saved.VAType)
}

// T033: FR-013 — holder details and callback URL come from the registration
// when the payment notification doesn't carry them.
func TestVAUsecase_Payment_NoBill_InheritsHolderFromRegistration(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
	mockRepo.On("GetPayment", mock.Anything, "PAY-4").Return(nil, domain.ErrVAInvalidBill)

	var saved *domain.VAPaymentRecord
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRecord")).
		Run(func(args mock.Arguments) { saved = args.Get(1).(*domain.VAPaymentRecord) }).Return(nil)

	_, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-4", "5000.00"))

	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "NoBill Holder", saved.CustomerName)
	assert.Equal(t, "holder@example.com", saved.CustomerEmail)
	assert.Equal(t, "628123456789", saved.CustomerPhone)
	assert.Equal(t, "trx-nobill-1", saved.TrxID)
	assert.Equal(t, "https://merchant.example.com/callback", saved.NotificationURL)
}

// T034: FR-011 — an unregistered no-bill VA number is rejected. Note this only
// applies once a registration lookup succeeds; a VA with NO registration at all
// falls through to the legacy path (covered by the existing suite).
func TestVAUsecase_Payment_NoBill_InactiveRegistrationRejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	inactive := noBillAccount()
	inactive.Status = domain.VAAccountStatusInactive
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(inactive, nil)
	mockRepo.On("GetPayment", mock.Anything, "PAY-5").Return(nil, domain.ErrVAInvalidBill)

	resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-5", "5000.00"))

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentNotFound, domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveNoBillPayment", mock.Anything, mock.Anything)
}

// T035: FR-012 — a retried paymentRequestId is rejected as an Inconsistent
// Request via the GetPayment short-circuit, without creating a second row.
func TestVAUsecase_Payment_NoBill_RepeatPaymentRequestIDRejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	existing := &domain.VAPaymentRecord{
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		CustomerName:     "NoBill Holder",
		PaymentRequestID: "PAY-6",
		PaidAmount:       "12000.00",
		Currency:         "IDR",
		Status:           "00",
		TransactionDate:  time.Now(),
	}
	mockRepo.On("GetPayment", mock.Anything, "PAY-6").Return(existing, nil)

	resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-6", "12000.00"))

	// A repeat of the same paymentRequestId that is NOT an advice request is
	// BCA's double-flag case: 4042518 "Inconsistent Request", carrying the
	// paymentFlagStatus of the FIRST request. It is raised as a DomainError so
	// the HTTP status matches the 404-class code; the handler forwards
	// PaymentData verbatim, flag status included.
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentInconsistent, domainErr.SNAPCode)
	require.NotNil(t, domainErr.PaymentData)
	assert.Equal(t, domain.PaymentFlagSuccess, domainErr.PaymentData.PaymentFlagStatus)
	assert.Equal(t, "12000.00", domainErr.PaymentData.PaidAmount.Value)
	mockRepo.AssertNotCalled(t, "SaveNoBillPayment", mock.Anything, mock.Anything)
}

// T036: the concurrency case the short-circuit alone can't cover. Two
// simultaneous duplicates both read "no payment yet"; the loser is rejected by
// the UNIQUE index and must answer 4042518 against the winner's row rather
// than 500 or double-write.
func TestVAUsecase_Payment_NoBill_DuplicateRaceRejectsLoser(t *testing.T) {
	mockRepo := new(MockVARepository)
	notifier := new(MockNotifier)
	uc := NewVAUsecase(mockRepo, notifier)

	original := &domain.VAPaymentRecord{
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		CustomerName:     "NoBill Holder",
		PaymentRequestID: "PAY-7",
		PaidAmount:       "9000.00",
		Currency:         "IDR",
		Status:           "00",
		TransactionDate:  time.Now(),
	}

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
	// First read: not there yet (the race). Second read, after the insert
	// collides: the winner's row.
	mockRepo.On("GetPayment", mock.Anything, "PAY-7").Return(nil, domain.ErrVAInvalidBill).Once()
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.Anything).Return(domain.ErrVAPaymentDuplicate).Once()
	mockRepo.On("GetPayment", mock.Anything, "PAY-7").Return(original, nil).Once()

	resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-7", "9000.00"))

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042518", domainErr.SNAPCode)
	require.NotNil(t, domainErr.PaymentData)
	assert.Equal(t, "9000.00", domainErr.PaymentData.PaidAmount.Value)
	mockRepo.AssertExpectations(t)
	// The loser must not fire a second callback for a payment it didn't record.
	assert.Empty(t, notifier.Calls)
}

// T037: FR-014 — exactly one callback per settled payment, and none when the
// registration carries no notification URL.
func TestVAUsecase_Payment_NoBill_EmitsExactlyOneCallback(t *testing.T) {
	mockRepo := new(MockVARepository)
	notifier := new(MockNotifier)
	uc := NewVAUsecase(mockRepo, notifier)

	notifier.On("EnqueuePaymentNotification", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
	mockRepo.On("GetPayment", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.Anything).Return(nil)

	_, err1 := uc.Payment(context.Background(), noBillPaymentReq("PAY-8", "1000.00"))
	_, err2 := uc.Payment(context.Background(), noBillPaymentReq("PAY-9", "2000.00"))

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Len(t, notifier.Calls, 2, "one callback per settled payment")
}

func TestVAUsecase_Payment_NoBill_NoCallbackWithoutNotificationURL(t *testing.T) {
	mockRepo := new(MockVARepository)
	notifier := new(MockNotifier)
	uc := NewVAUsecase(mockRepo, notifier)

	silent := noBillAccount()
	silent.NotificationURL = ""
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(silent, nil)
	mockRepo.On("GetPayment", mock.Anything, "PAY-10").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-10", "1000.00"))

	require.NoError(t, err)
	assert.Empty(t, notifier.Calls)
}

// T041: a no-bill VA has no bill amount, so the vendor's totalAmount must not
// be compared against paidAmount — that check belongs to billed VAs only.
func TestVAUsecase_Payment_NoBill_TotalAmountMismatchAllowed(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
	mockRepo.On("GetPayment", mock.Anything, "PAY-11").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("SaveNoBillPayment", mock.Anything, mock.Anything).Return(nil)

	req := noBillPaymentReq("PAY-11", "5000.00")
	req.TotalAmount = &domain.Amount{Value: "999999.00", Currency: "IDR"}

	resp, err := uc.Payment(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "2002500", resp.ResponseCode)
}

// T042: a zero or negative payment is nonsense and must persist nothing.
func TestVAUsecase_Payment_NoBill_NonPositiveAmountRejected(t *testing.T) {
	for _, amount := range []string{"0.00", "-100.00"} {
		t.Run(amount, func(t *testing.T) {
			mockRepo := new(MockVARepository)
			uc := NewVAUsecase(mockRepo, nil)

			mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)
			mockRepo.On("GetPayment", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)

			resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-neg", amount))

			assert.Error(t, err)
			assert.Nil(t, resp)
			var domainErr *domain.DomainError
			require.ErrorAs(t, err, &domainErr)
			assert.Equal(t, "4002501", domainErr.SNAPCode)
			mockRepo.AssertNotCalled(t, "SaveNoBillPayment", mock.Anything, mock.Anything)
		})
	}
}

// --- No-Bill VA inquiry (feature 013-no-bill-payment-transaction, US3) ---
//
// With no transaction created at registration time, inquiry can no longer rely
// on a transaction row existing. Without this branch every first-time payment
// attempt would fail at the inquiry step.

func noBillInquiryReq(inquiryRequestID, amount string) *domain.VAInquiryRequest {
	req := &domain.VAInquiryRequest{
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		InquiryRequestID: inquiryRequestID,
	}
	if amount != "" {
		req.Amount = &domain.Amount{Value: amount, Currency: "IDR"}
	}
	return req
}

// T047: FR-015 / FR-016 — a registered, never-paid no-bill VA resolves from
// the registration and writes nothing.
func TestVAUsecase_Inquiry_NoBill_ResolvesFromRegistration(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetInquiry", mock.Anything, "INQ-1").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)

	resp, err := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-1", "50000.00"))

	require.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, "00", resp.VirtualAccountData.InquiryStatus)
	assert.Equal(t, "NoBill Holder", resp.VirtualAccountData.VirtualAccountName)
	assert.Equal(t, "0001234567", resp.VirtualAccountData.CustomerNo)
	// No transaction, and no transaction lookup either.
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo", mock.Anything, mock.Anything)
}

// T048: spec A-005 — a no-bill VA asserts no bill, so totalAmount is always
// zero and never the amount the caller sent.
func TestVAUsecase_Inquiry_NoBill_TotalAmountAlwaysZero(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetInquiry", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)

	// A request carrying a hefty amount must NOT come back as a bill for it.
	resp, err := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-2", "123456.78"))

	require.NoError(t, err)
	assert.Equal(t, "0.00", resp.VirtualAccountData.TotalAmount.Value)
	assert.Equal(t, "IDR", resp.VirtualAccountData.TotalAmount.Currency)
	// A no-bill VA has no bill breakdown to return.
	assert.Empty(t, resp.VirtualAccountData.BillDetails)
}

// The zero is unconditional: a request with no amount at all reports the same.
func TestVAUsecase_Inquiry_NoBill_TotalAmountZeroWithoutRequestAmount(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	req := noBillInquiryReq("INQ-2b", "0.00")
	req.Amount = &domain.Amount{Value: "", Currency: ""}

	mockRepo.On("GetInquiry", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)

	resp, err := uc.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "0.00", resp.VirtualAccountData.TotalAmount.Value)
	assert.Equal(t, "IDR", resp.VirtualAccountData.TotalAmount.Currency)
}

// T049: US3 AS2 — prior settled payments never block a new inquiry. This is
// the inquiry-side counterpart of "the second payment must succeed".
func TestVAUsecase_Inquiry_NoBill_SucceedsAfterPriorPayments(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	// The registration is untouched by payments, so an inquiry after N
	// payments looks exactly like an inquiry before the first one.
	mockRepo.On("GetInquiry", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(noBillAccount(), nil)

	resp1, err1 := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-3", "1000.00"))
	resp2, err2 := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-4", "2000.00"))

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, "2002400", resp1.ResponseCode)
	assert.Equal(t, "2002400", resp2.ResponseCode)
	mockRepo.AssertNotCalled(t, "SaveInquiry", mock.Anything, mock.Anything)
}

// T050: FR-022 — the fall-through that keeps pre-feature VAs working. A VA
// number with no registration must reach the unchanged legacy path.
func TestVAUsecase_Inquiry_NoRegistration_FallsThroughToLegacyPath(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	legacy := &domain.VAInquiryRecord{
		ID:               "legacy-id",
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
		CustomerName:     "Legacy Holder",
		Status:           "03",
		TotalAmount:      "150000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetInquiry", mock.Anything, "INQ-5").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "08889912345678901234567890").Return(nil, domain.ErrVAAccountNotFound)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "08889912345678901234567890").Return(legacy, nil)
	// The legacy row carries no vendor inquiryRequestId yet, so the legacy path
	// claims this one onto it — proving the fall-through reaches that path
	// intact rather than being short-circuited by the registry branch.
	mockRepo.On("ClaimInquiryRequestID", mock.Anything, "legacy-id", "INQ-5").Return(nil)
	mockRepo.On("GetVABillDetails", mock.Anything, "legacy-id").Return([]domain.BillDetail{}, nil)

	resp, err := uc.Inquiry(context.Background(), &domain.VAInquiryRequest{
		PartnerServiceID: "088899",
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
		InquiryRequestID: "INQ-5",
		Amount:           &domain.Amount{Value: "150000.00", Currency: "IDR"},
	})

	require.NoError(t, err)
	assert.Equal(t, "Legacy Holder", resp.VirtualAccountData.VirtualAccountName)
	// The legacy path reports the VA's own bill total, not the request amount.
	assert.Equal(t, "150000.00", resp.VirtualAccountData.TotalAmount.Value)
}

// T053: a broken registry query must surface as a 500, NOT be mistaken for
// "no registration" — that would silently route to the legacy path and produce
// a wrong answer instead of an error.
func TestVAUsecase_Inquiry_NoBill_RegistryQueryFailureIs500(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetInquiry", mock.Anything, "INQ-6").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(nil, errors.New("connection refused"))

	resp, err := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-6", "1000.00"))

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002400", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "GetVAByVirtualAccountNo", mock.Anything, mock.Anything)
}

func TestVAUsecase_Payment_NoBill_RegistryQueryFailureIs500(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetPayment", mock.Anything, "PAY-boom").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(nil, errors.New("connection refused"))

	resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-boom", "1000.00"))

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "5002500", domainErr.SNAPCode)
}

// --- No-Bill VA status (feature 013-no-bill-payment-transaction, US4) ---

// T055: FR-018 — with several payments on one VA, a status query must return
// THAT payment's own figures. Reconciliation is per top-up, not per VA.
func TestVAUsecase_Status_NoBill_ResolvesIndividualPayment(t *testing.T) {
	payments := map[string]*domain.VAPaymentRecord{
		"PAY-A": {
			ID:               "txn-a",
			PartnerServiceID: "15973",
			CustomerNo:       "0001234567",
			VirtualAccountNo: "159730001234567",
			// For a no-bill payment the two ids are the same by construction,
			// which is what makes a per-payment status query resolvable.
			InquiryRequestID: "PAY-A",
			PaymentRequestID: "PAY-A",
			PaidAmount:       "10000.00",
			TotalAmount:      "10000.00",
			Currency:         "IDR",
			Status:           "00",
			ReferenceNo:      "REFA",
			TransactionDate:  time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC),
		},
		"PAY-B": {
			ID:               "txn-b",
			PartnerServiceID: "15973",
			CustomerNo:       "0001234567",
			VirtualAccountNo: "159730001234567",
			InquiryRequestID: "PAY-B",
			PaymentRequestID: "PAY-B",
			PaidAmount:       "25000.00",
			TotalAmount:      "25000.00",
			Currency:         "IDR",
			Status:           "00",
			ReferenceNo:      "REFB",
			TransactionDate:  time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC),
		},
	}

	for id, want := range payments {
		t.Run(id, func(t *testing.T) {
			mockRepo := new(MockVARepository)
			uc := NewVAUsecase(mockRepo, nil)

			mockRepo.On("GetPayment", mock.Anything, id).Return(want, nil)
			mockRepo.On("GetVABillDetails", mock.Anything, want.ID).Return([]domain.BillDetail{}, nil)

			resp, err := uc.Status(context.Background(), &domain.VAStatusRequest{
				PartnerServiceID: "15973",
				CustomerNo:       "0001234567",
				VirtualAccountNo: "159730001234567",
				InquiryRequestID: id,
			})

			require.NoError(t, err)
			assert.Equal(t, "2002600", resp.ResponseCode)
			assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
			assert.Equal(t, want.PaidAmount, resp.VirtualAccountData.PaidAmount.Value)
			assert.Equal(t, want.ReferenceNo, resp.VirtualAccountData.ReferenceNo)
			assert.Equal(t, want.TransactionDate, *resp.VirtualAccountData.TransactionDate)
			// T058: no bill exists, so totalAmount mirrors the payment.
			assert.Equal(t, want.PaidAmount, resp.VirtualAccountData.TotalAmount.Value)
			assert.Empty(t, resp.VirtualAccountData.BillDetails)
		})
	}
}

// T056: US4 AS2 — an identifier with no matching payment is rejected. For a
// no-bill VA this is the "registered but never paid" case: the registration
// exists, but no transaction does.
func TestVAUsecase_Status_NoBill_UnknownPaymentRejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	mockRepo.On("GetPayment", mock.Anything, "PAY-missing").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetInquiry", mock.Anything, "PAY-missing").Return(nil, domain.ErrVAInvalidBill)

	resp, err := uc.Status(context.Background(), &domain.VAStatusRequest{
		PartnerServiceID: "15973",
		CustomerNo:       "0001234567",
		VirtualAccountNo: "159730001234567",
		InquiryRequestID: "PAY-missing",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodeStatusNotFound, domainErr.SNAPCode)
}

// --- No-Bill VA expiry and deactivation guards
// (feature 013-no-bill-payment-transaction, US6) ---

// T067: US6 AS2 / US3 AS3 — a deactivated registration stops accepting both
// payments and inquiries.
func TestVAUsecase_NoBill_InactiveRegistrationRejectsInquiryAndPayment(t *testing.T) {
	t.Run("inquiry", func(t *testing.T) {
		mockRepo := new(MockVARepository)
		uc := NewVAUsecase(mockRepo, nil)

		inactive := noBillAccount()
		inactive.Status = domain.VAAccountStatusInactive
		mockRepo.On("GetInquiry", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
		mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(inactive, nil)

		resp, err := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-inactive", "1000.00"))

		assert.Error(t, err)
		assert.Nil(t, resp)
		var domainErr *domain.DomainError
		require.ErrorAs(t, err, &domainErr)
		assert.Equal(t, "4042419", domainErr.SNAPCode)
	})

	t.Run("payment", func(t *testing.T) {
		mockRepo := new(MockVARepository)
		uc := NewVAUsecase(mockRepo, nil)

		inactive := noBillAccount()
		inactive.Status = domain.VAAccountStatusInactive
		mockRepo.On("GetPayment", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
		mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(inactive, nil)

		resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-inactive", "1000.00"))

		assert.Error(t, err)
		assert.Nil(t, resp)
		var domainErr *domain.DomainError
		require.ErrorAs(t, err, &domainErr)
		assert.Equal(t, domain.CodePaymentNotFound, domainErr.SNAPCode)
		mockRepo.AssertNotCalled(t, "SaveNoBillPayment", mock.Anything, mock.Anything)
	})
}

// T068: FR-017 — an expired registration is detected inline, transitions to
// EXPIRED, and emits EXACTLY ONE callback no matter how many times it is hit.
// The exactly-once guarantee comes from UpdateVAAccountStatus's
// WHERE status='ACTIVE' clause: only the caller that actually applied the
// transition proceeds to notify.
func TestVAUsecase_NoBill_ExpiredRegistrationNotifiesExactlyOnce(t *testing.T) {
	mockRepo := new(MockVARepository)
	notifier := new(MockNotifier)
	uc := NewVAUsecase(mockRepo, notifier)

	past := time.Now().Add(-1 * time.Hour)
	expired := noBillAccount()
	expired.ExpiredDate = &past

	mockRepo.On("GetInquiry", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(expired, nil)
	// First detection wins the guard; every later one finds no ACTIVE row.
	mockRepo.On("UpdateVAAccountStatus", mock.Anything, "159730001234567", domain.VAAccountStatusExpired).
		Return(nil).Once()
	mockRepo.On("UpdateVAAccountStatus", mock.Anything, "159730001234567", domain.VAAccountStatusExpired).
		Return(domain.ErrVAAccountNotFound)
	notifier.On("EnqueuePaymentNotification", mock.Anything, mock.Anything).Return(nil)

	for i := 0; i < 3; i++ {
		resp, err := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-exp", "1000.00"))
		assert.Error(t, err)
		assert.Nil(t, resp)
		var domainErr *domain.DomainError
		require.ErrorAs(t, err, &domainErr)
		assert.Equal(t, "4042419", domainErr.SNAPCode)
	}

	assert.Len(t, notifier.Calls, 1, "exactly one va.expired callback across repeated inquiries")
	payload := notifier.Calls[0].Arguments.Get(1).(*domain.PaymentNotificationPayload)
	assert.Equal(t, domain.NotificationEventVAExpired, payload.EventType)
	assert.Equal(t, past.Format(time.RFC3339), payload.ExpiredAt)
}

// The payment side of the same guard.
func TestVAUsecase_NoBill_ExpiredRegistrationRejectsPayment(t *testing.T) {
	mockRepo := new(MockVARepository)
	notifier := new(MockNotifier)
	uc := NewVAUsecase(mockRepo, notifier)

	past := time.Now().Add(-1 * time.Hour)
	expired := noBillAccount()
	expired.ExpiredDate = &past

	mockRepo.On("GetPayment", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(expired, nil)
	mockRepo.On("UpdateVAAccountStatus", mock.Anything, "159730001234567", domain.VAAccountStatusExpired).Return(nil)
	notifier.On("EnqueuePaymentNotification", mock.Anything, mock.Anything).Return(nil)

	resp, err := uc.Payment(context.Background(), noBillPaymentReq("PAY-exp", "1000.00"))

	assert.Error(t, err)
	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042519", domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveNoBillPayment", mock.Anything, mock.Anything)
}

// spec A-004 — a registration with no expiry date never expires, so it stays
// payable indefinitely.
func TestVAUsecase_NoBill_NoExpiryDateNeverExpires(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	account := noBillAccount()
	require.Nil(t, account.ExpiredDate)

	mockRepo.On("GetInquiry", mock.Anything, mock.Anything).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159730001234567").Return(account, nil)

	resp, err := uc.Inquiry(context.Background(), noBillInquiryReq("INQ-noexp", "1000.00"))

	require.NoError(t, err)
	assert.Equal(t, "2002400", resp.ResponseCode)
	mockRepo.AssertNotCalled(t, "UpdateVAAccountStatus", mock.Anything, mock.Anything, mock.Anything)
}

// --- Bill-bearing regression guard (US5, T061) ---
//
// The no-bill branch sits ahead of the legacy lookup in Payment. This proves a
// variable-bill VA that also carries a registration still routes past it to the
// cumulative SaveVAPayment path, unchanged (FR-021, SC-005).
func TestVAUsecase_Payment_VariableBillWithRegistration_StillUsesCumulativePath(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "15974",
		CustomerNo:       "0005550001",
		VirtualAccountNo: "159740005550001",
		PaymentRequestID: "payment-var-guard",
		PaidAmount:       &domain.Amount{Value: "60000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	// A registration exists (spec A-002 writes one for every managed type)...
	registration := &domain.VAAccount{
		ID:               "acc-var",
		PartnerServiceID: "15974",
		CustomerNo:       "0005550001",
		VirtualAccountNo: "159740005550001",
		VAType:           "02",
		Billing:          domain.VATypeBillingVariable,
		Status:           domain.VAAccountStatusActive,
	}
	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-var-guard",
		PartnerServiceID: "15974",
		VirtualAccountNo: req.VirtualAccountNo,
		Status:           "03",
		VAType:           "02",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, req.VirtualAccountNo).Return(registration, nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(merchantVA, nil)
	mockRepo.On("SaveVAPayment", mock.Anything, "txn-var-guard", req.PaymentRequestID, "60000.00", req.ReferenceNo).Return("60000.00", "03", true, nil)

	resp, err := uc.Payment(context.Background(), req)

	require.NoError(t, err)
	// ...but billing != none, so the no-bill branch must NOT claim it.
	// ...and the cumulative path is the one that ran. The flag is "00"
	// because the instalment was ACCEPTED — payment-side flag status has no
	// "pending" value; that lives on the inquiry-status service.
	assert.Equal(t, domain.PaymentFlagSuccess, resp.VirtualAccountData.PaymentFlagStatus)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "SaveNoBillPayment", mock.Anything, mock.Anything)
}

// The inquiry-side counterpart: a fixed-bill VA with a registration still
// reports its own bill total, not the request amount.
func TestVAUsecase_Inquiry_FixedBillWithRegistration_StillUsesTransactionPath(t *testing.T) {
	mockRepo := new(MockVARepository)
	uc := NewVAUsecase(mockRepo, nil)

	registration := &domain.VAAccount{
		VirtualAccountNo: "159750009876543",
		VAType:           "03",
		Billing:          domain.VATypeBillingFixed,
		Status:           domain.VAAccountStatusActive,
		CustomerName:     "Registration Name",
	}
	merchantVA := &domain.VAInquiryRecord{
		ID:               "txn-fixed",
		PartnerServiceID: "15975",
		CustomerNo:       "0009876543",
		VirtualAccountNo: "159750009876543",
		CustomerName:     "Transaction Name",
		Status:           "03",
		TotalAmount:      "75000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetInquiry", mock.Anything, "INQ-fixed").Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAAccount", mock.Anything, "159750009876543").Return(registration, nil)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, "159750009876543").Return(merchantVA, nil)
	// Reached only via the transaction path — the registry branch never claims
	// an inquiryRequestId, so this call is itself evidence the bill-bearing VA
	// took the unchanged route.
	mockRepo.On("ClaimInquiryRequestID", mock.Anything, "txn-fixed", "INQ-fixed").Return(nil)
	mockRepo.On("GetVABillDetails", mock.Anything, "txn-fixed").Return([]domain.BillDetail{}, nil)

	resp, err := uc.Inquiry(context.Background(), &domain.VAInquiryRequest{
		PartnerServiceID: "15975",
		CustomerNo:       "0009876543",
		VirtualAccountNo: "159750009876543",
		InquiryRequestID: "INQ-fixed",
		Amount:           &domain.Amount{Value: "1.00", Currency: "IDR"},
	})

	require.NoError(t, err)
	assert.Equal(t, "75000.00", resp.VirtualAccountData.TotalAmount.Value, "bill total, not the request amount")
	assert.Equal(t, "Transaction Name", resp.VirtualAccountData.VirtualAccountName)
}

// A variable-bill VA whose cumulative payments still fall short of totalAmount
// is an OPEN bill, however its status column reads. Answering 4042414 there
// would make the outstanding balance uncollectable.
func TestVAUsecase_Inquiry_VariableBillNotLunas_StillOpen(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: "15974",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "1597408212221111",
		InquiryRequestID: "INQ-variable-partial",
		Amount:           &domain.Amount{Value: "50000.00", Currency: "IDR"},
	}

	partial := &domain.VAInquiryRecord{
		ID:               "variable-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "budi manjo",
		VAType:           "05",
		Status:           "00",
		TotalAmount:      "200000.00",
		PaidAmount:       "50000.00",
		Currency:         "IDR",
		SubCompany:       "00000",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(partial, nil)
	mockRepo.On("ClaimInquiryRequestID", mock.Anything, partial.ID, req.InquiryRequestID).Return(nil).Maybe()
	mockRepo.On("GetVABillDetails", mock.Anything, partial.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, "00", resp.VirtualAccountData.InquiryStatus)
	assert.Equal(t, &domain.Amount{Value: "200000.00", Currency: "IDR"}, resp.VirtualAccountData.TotalAmount)
}

// Same record shape on the payment side: the guard must not reject the
// remaining balance as a settled bill.
func TestVAUsecase_Payment_VariableBillNotLunas_Accepted(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "15974",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "1597408212221111",
		TrxID:            "TRX-variable",
		PaymentRequestID: "PAY-variable-2",
		PaidAmount:       &domain.Amount{Value: "150000.00", Currency: "IDR"},
	}

	partial := &domain.VAInquiryRecord{
		ID:               "variable-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "budi manjo",
		VAType:           "05",
		Status:           "00",
		TotalAmount:      "200000.00",
		PaidAmount:       "50000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(partial, nil)
	// The cumulative payment reaches totalAmount, so the row settles for real.
	mockRepo.On("SaveVAPayment", mock.Anything, partial.ID, req.PaymentRequestID, req.PaidAmount.Value, req.ReferenceNo).
		Return("200000.00", "00", true, nil)

	resp, err := usecase.Payment(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "2002500", resp.ResponseCode)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, "200000.00", resp.VirtualAccountData.PaidAmount.Value)
	mockRepo.AssertExpectations(t)
}

// The guard still holds where it should: a fully settled bill is refused.
func TestVAUsecase_Payment_VariableBillLunas_Rejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "15974",
		CustomerNo:       "082122221111",
		VirtualAccountNo: "1597408212221111",
		TrxID:            "TRX-variable",
		PaymentRequestID: "PAY-variable-3",
		PaidAmount:       &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}

	settled := &domain.VAInquiryRecord{
		ID:               "variable-transaction-id",
		VirtualAccountNo: req.VirtualAccountNo,
		VAType:           "05",
		Status:           "00",
		TotalAmount:      "200000.00",
		PaidAmount:       "200000.00",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(settled, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentPaidBill, domainErr.SNAPCode)
	mockRepo.AssertNotCalled(t, "SaveVAPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A payment naming a VA that exists nowhere — no registration, no transaction
// — must be refused 4042512 rather than conjuring a settled row for it.
func TestVAUsecase_Payment_UnknownVA_Rejected(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	trxDateTime := time.Date(2026, 6, 25, 9, 13, 20, 0, time.FixedZone("WIB", 7*3600))
	req := &domain.VAPaymentRequest{
		PartnerServiceID: "   15973",
		CustomerNo:       "00000000000",
		VirtualAccountNo: "   1597300000000000",
		TrxID:            "TRX-unknown",
		PaymentRequestID: "202606241142121597300051476279",
		PaidAmount:       &domain.Amount{Value: "50000.00", Currency: "IDR"},
		ReferenceNo:      "05147220913",
		TrxDateTime:      &trxDateTime,
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(nil, domain.ErrMerchantVANotFound)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "4042512", domainErr.SNAPCode)
	assert.Equal(t, "Invalid Bill/Virtual Account [Not Found]", domainErr.Message)

	require.NotNil(t, domainErr.PaymentData)
	data := domainErr.PaymentData
	assert.Equal(t, "01", data.PaymentFlagStatus)
	assert.Equal(t, &domain.BilingualText{English: "Virtual Account Not Found", Indonesia: "Virtual Account Tidak Ditemukan"}, data.PaymentFlagReason)
	// The request's own keys come back; nothing is invented.
	assert.Equal(t, req.PartnerServiceID, data.PartnerServiceID)
	assert.Equal(t, req.CustomerNo, data.CustomerNo)
	assert.Equal(t, req.VirtualAccountNo, data.VirtualAccountNo)
	assert.Equal(t, req.PaymentRequestID, data.PaymentRequestID)
	assert.Equal(t, req.ReferenceNo, data.ReferenceNo)
	assert.Equal(t, req.TrxDateTime, data.TrxDateTime)
	// No bill was matched, so no amount was accepted and no holder is named.
	assert.Empty(t, data.VirtualAccountName)
	assert.Equal(t, &domain.Amount{}, data.PaidAmount)
	assert.Equal(t, &domain.Amount{}, data.TotalAmount)

	// The defect this closes: no phantom row for a VA no merchant ever issued.
	mockRepo.AssertNotCalled(t, "SavePayment", mock.Anything, mock.Anything)
	mockRepo.AssertNotCalled(t, "SaveVAPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A closed bill is refused, and the refusal reports WHICH bill it refused —
// the vendor cannot act on a bare code plus message.
func TestVAUsecase_Payment_PaidBill_RejectionCarriesVAData(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	trxDateTime := time.Date(2026, 6, 25, 9, 13, 20, 0, time.FixedZone("WIB", 7*3600))
	req := &domain.VAPaymentRequest{
		PartnerServiceID: "   15975",
		CustomerNo:       "06000000000000000004",
		VirtualAccountNo: "1597506000000000000000004",
		TrxID:            "TRX-paid",
		PaymentRequestID: "PAY-against-paid-bill",
		PaidAmount:       &domain.Amount{Value: "50000.00", Currency: "IDR"},
		ReferenceNo:      "05147220913",
		TrxDateTime:      &trxDateTime,
	}

	paid := &domain.VAInquiryRecord{
		ID:               "paid-transaction-id",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		CustomerName:     "budi manjo",
		TrxID:            "TRX-original",
		VAType:           "06",
		Status:           "00",
		TotalAmount:      "2000000.00",
		PaidAmount:       "2000000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(paid, nil)

	resp, err := usecase.Payment(context.Background(), req)

	assert.Nil(t, resp)
	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentPaidBill, domainErr.SNAPCode)

	require.NotNil(t, domainErr.PaymentData)
	data := domainErr.PaymentData
	assert.Equal(t, "01", data.PaymentFlagStatus)
	assert.Equal(t, &domain.BilingualText{English: "Bill has been paid", Indonesia: "Tagihan telah dibayar"}, data.PaymentFlagReason)
	assert.Equal(t, paid.VirtualAccountNo, data.VirtualAccountNo)
	assert.Equal(t, "budi manjo", data.VirtualAccountName)
	assert.Equal(t, req.PaymentRequestID, data.PaymentRequestID)
	assert.Equal(t, req.ReferenceNo, data.ReferenceNo)
	assert.Equal(t, req.TrxDateTime, data.TrxDateTime)
	// The stored bill is real; the tendered amount was never accepted.
	assert.Equal(t, &domain.Amount{Value: "2000000.00", Currency: "IDR"}, data.TotalAmount)
	assert.Equal(t, &domain.Amount{}, data.PaidAmount)

	mockRepo.AssertNotCalled(t, "SavePayment", mock.Anything, mock.Anything)
}

// A deleted bill is refused with the "not found" reason, not "already paid".
func TestVAUsecase_Payment_DeletedBill_ReportsInactiveReason(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAPaymentRequest{
		PartnerServiceID: "   15975",
		CustomerNo:       "06000000000000000004",
		VirtualAccountNo: "1597506000000000000000004",
		TrxID:            "TRX-deleted",
		PaymentRequestID: "PAY-against-deleted-bill",
		PaidAmount:       &domain.Amount{Value: "50000.00", Currency: "IDR"},
	}

	deleted := &domain.VAInquiryRecord{
		ID:               "deleted-transaction-id",
		VirtualAccountNo: req.VirtualAccountNo,
		VAType:           "06",
		Status:           "04",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetPayment", mock.Anything, req.PaymentRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).Return(deleted, nil)

	_, err := usecase.Payment(context.Background(), req)

	var domainErr *domain.DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, domain.CodePaymentNotFound, domainErr.SNAPCode)
	require.NotNil(t, domainErr.PaymentData)
	assert.Equal(t, domain.ReasonForCode(domain.CodePaymentNotFound), domainErr.PaymentData.PaymentFlagReason)
}
