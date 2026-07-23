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
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-001",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
		AdditionalInfo:     map[string]interface{}{"dbUrlProcess": "https://example.com/webhook"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/create-va", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("CreateVA", mock.Anything, &req).Return(&domain.MerchantCreateVAResponse{
		ResponseCode:    "2002700",
		ResponseMessage: "Success",
		VirtualAccountData: &domain.MerchantVAData{
			VirtualAccountNo: "08889912345678901234567890",
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
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/create-va", bytes.NewReader(body))
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
		CustomerNo:         "12345678901234567890",
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-err",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/create-va", bytes.NewReader(body))
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

// --- ListVA Handler Tests ---

func TestMerchantVAHandler_ListVA_Success(t *testing.T) {
	e := echo.New()
	req := domain.MerchantListVARequest{
		PartnerServiceID: "088899",
		Page:             1,
		PageSize:         20,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/list", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockMerchantVAUsecase)
	mockUsecase.On("ListVA", mock.Anything, &req).Return(&domain.MerchantListVAResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		Data:            []domain.VAListItem{},
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
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodDelete, "/v1.0/transfer-va/delete-va", bytes.NewReader(body))
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
	httpReq := httptest.NewRequest(http.MethodDelete, "/v1.0/transfer-va/delete-va", bytes.NewReader(body))
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
		CustomerNo:       "12345678901234567890",
		VirtualAccountNo: "08889912345678901234567890",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodDelete, "/v1.0/transfer-va/delete-va", bytes.NewReader(body))
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
