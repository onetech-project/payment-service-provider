package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMerchantVAUsecase is a mock for merchant VA usecase
type MockMerchantVAUsecase struct {
	mock.Mock
}

func (m *MockMerchantVAUsecase) CreateVA(ctx context.Context, req *domain.MerchantCreateVARequest) (*domain.MerchantCreateVAResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MerchantCreateVAResponse), args.Error(1)
}

func (m *MockMerchantVAUsecase) ListVA(ctx context.Context, req *domain.MerchantListVARequest) (*domain.MerchantListVAResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MerchantListVAResponse), args.Error(1)
}

func (m *MockMerchantVAUsecase) ListTransactions(ctx context.Context, req *domain.MerchantListVARequest) (*domain.MerchantListTransactionsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MerchantListTransactionsResponse), args.Error(1)
}

func (m *MockMerchantVAUsecase) DeleteVA(ctx context.Context, req *domain.MerchantDeleteVARequest) (*domain.MerchantDeleteVAResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MerchantDeleteVAResponse), args.Error(1)
}

// --- CreateVA Handler Tests ---

func TestMerchantVAHandler_CreateVA_Success(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   "088899123456789012345678",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-001",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"dbUrlProcess": "https://example.com/webhook"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(&domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			VirtualAccountNo: "088899123456789012345678",
			TrxID:            "trx-001",
		},
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockUsecase.AssertExpectations(t)
}

func TestMerchantVAHandler_CreateVA_MissingFields(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID: "088899",
		// Missing required fields
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	h := NewMerchantVAHandler(mockUsecase)

	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMerchantVAHandler_CreateVA_UsecaseError(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   "088899123456789012345678",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-err",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(nil, domain.NewDomainError("4002701", "Invalid Mandatory Field", nil))

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMerchantVAHandler_CreateVA_DynamicVA_EmptyCustomerNoAccepted(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "15973000000000000001",
		VirtualAccountName: "Dynamic NoBill",
		TrxID:              "trx-dyn-handler-01",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(&domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			PartnerServiceID: "15973",
			CustomerNo:       "04000000000000000001",
			VirtualAccountNo: req.VirtualAccountNo,
			TrxID:            req.TrxID,
		},
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp domain.MerchantCreateVAResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "04000000000000000001", resp.VirtualAccountData.CustomerNo)
	mockUsecase.AssertExpectations(t)
}

func TestMerchantVAHandler_CreateVA_StaticVA_EchoesCustomerNo(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "15973000012345670001",
		VirtualAccountName: "Static NoBill",
		TrxID:              "trx-static-handler-01",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(&domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			PartnerServiceID: "15973",
			CustomerNo:       "0001234567",
			VirtualAccountNo: req.VirtualAccountNo,
			TrxID:            req.TrxID,
		},
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp domain.MerchantCreateVAResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "0001234567", resp.VirtualAccountData.CustomerNo)
	mockUsecase.AssertExpectations(t)
}

func TestMerchantVAHandler_CreateVA_DuplicateStaticCustomerNo_Returns409(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0001234567",
		VirtualAccountNo:   "15973000012345670099",
		VirtualAccountName: "Static NoBill Dup",
		TrxID:              "trx-static-handler-dup",
		AdditionalInfo:     map[string]interface{}{"vaType": "01"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(nil, domain.NewDomainError("4092701", "Conflict: customerNo already registered for this partnerServiceId", nil))

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestMerchantVAHandler_CreateVA_InvalidCombination_Returns400(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "0009999999",
		VirtualAccountNo:   "15973000099999990001",
		VirtualAccountName: "Invalid Combo",
		TrxID:              "trx-handler-invalid-combo",
		AdditionalInfo:     map[string]interface{}{"vaType": "02"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(nil, domain.NewDomainError("4002702", "Invalid Field Format [partnerServiceId/additionalInfo.vaType combination]", nil))

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp domain.MerchantCreateVAResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.ResponseMessage)
}

func TestMerchantVAHandler_CreateVA_DynamicVA_EmptyVirtualAccountNoAccepted(t *testing.T) {
	e := echo.New()
	req := domain.MerchantCreateVARequest{
		PartnerServiceID:   "15973",
		CustomerNo:         "",
		VirtualAccountNo:   "", // feature 008-va-number-consistency: optional for dynamic vaType
		VirtualAccountName: "Dynamic NoBill Auto Derive",
		TrxID:              "trx-dyn-handler-auto-derive",
		AdditionalInfo:     map[string]interface{}{"vaType": "04"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(&domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			PartnerServiceID: "15973",
			CustomerNo:       "04000000000000000099",
			VirtualAccountNo: "1597304000000000000099",
			TrxID:            req.TrxID,
		},
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.CreateVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp domain.MerchantCreateVAResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "1597304000000000000099", resp.VirtualAccountData.VirtualAccountNo)
	mockUsecase.AssertExpectations(t)
}

// --- ListVA Handler Tests ---

func TestMerchantVAHandler_ListVA_Success(t *testing.T) {
	e := echo.New()
	req := domain.MerchantListVARequest{
		PartnerServiceID: "088899",
		Page:             1,
		PageSize:         20,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/list", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("ListVA", mock.Anything, &req).Return(&domain.MerchantListVAResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		Data:            []domain.VAAccountListItem{},
		Pagination:      &domain.Pagination{Page: 1, PageSize: 20, TotalRows: 0, TotalPages: 0},
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.ListVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockUsecase.AssertExpectations(t)
}

// --- DeleteVA Handler Tests ---

func TestMerchantVAHandler_DeleteVA_Success(t *testing.T) {
	e := echo.New()
	req := domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: "088899123456789012345678",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodDelete, "/openapi/v1.0/transfer-va/delete-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("DeleteVA", mock.Anything, &req).Return(&domain.MerchantDeleteVAResponse{
		ResponseCode:    "2003100",
		ResponseMessage: "Success",
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.DeleteVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockUsecase.AssertExpectations(t)
}

func TestMerchantVAHandler_DeleteVA_MissingFields(t *testing.T) {
	e := echo.New()
	req := domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		// Missing CustomerNo and VirtualAccountNo
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodDelete, "/openapi/v1.0/transfer-va/delete-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	h := NewMerchantVAHandler(mockUsecase)

	err := h.DeleteVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMerchantVAHandler_DeleteVA_AlreadyPaid(t *testing.T) {
	e := echo.New()
	txDate := time.Now()
	req := domain.MerchantDeleteVARequest{
		PartnerServiceID: "088899",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: "088899123456789012345678",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodDelete, "/openapi/v1.0/transfer-va/delete-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	_ = txDate
	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("DeleteVA", mock.Anything, &req).Return(nil, domain.NewDomainError("4053101", "Requested Operation Is Not Allowed", nil))

	h := NewMerchantVAHandler(mockUsecase)
	err := h.DeleteVA(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- ListTransactions Handler Tests (feature 013-no-bill-payment-transaction) ---

func TestMerchantVAHandler_ListTransactions_Success(t *testing.T) {
	e := echo.New()
	req := domain.MerchantListVARequest{
		VirtualAccountNo: "159730001234567",
		Page:             1,
		PageSize:         20,
	}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/list-transactions", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("ListTransactions", mock.Anything, &req).Return(&domain.MerchantListTransactionsResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		Data: []domain.VATransactionListItem{
			{VirtualAccountNo: "159730001234567", PaymentRequestID: "PAY-1", Status: "00"},
			{VirtualAccountNo: "159730001234567", PaymentRequestID: "PAY-2", Status: "00"},
		},
		Pagination: &domain.Pagination{Page: 1, PageSize: 20, TotalRows: 2, TotalPages: 1},
	}, nil)

	h := NewMerchantVAHandler(mockUsecase)
	err := h.ListTransactions(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp domain.MerchantListTransactionsResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
	mockUsecase.AssertExpectations(t)
}

func TestMerchantVAHandler_ListTransactions_InvalidBody(t *testing.T) {
	e := echo.New()
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/list-transactions", bytes.NewReader([]byte("{not json")))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	h := NewMerchantVAHandler(new(MockMerchantVAUsecase))
	err := h.ListTransactions(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMerchantVAHandler_ListTransactions_UsecaseError(t *testing.T) {
	e := echo.New()
	req := domain.MerchantListVARequest{Page: 1, PageSize: 20}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/list-transactions", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("ListTransactions", mock.Anything, &req).
		Return(nil, domain.NewDomainError("5002400", "Internal Server Error", nil))

	h := NewMerchantVAHandler(mockUsecase)
	err := h.ListTransactions(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
