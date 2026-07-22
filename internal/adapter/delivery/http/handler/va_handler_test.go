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

// MockVAUsecase is a mock implementation of domain.VAUsecase
type MockVAUsecase struct {
	mock.Mock
}

func (m *MockVAUsecase) Inquiry(ctx context.Context, req *domain.VAInquiryRequest) (*domain.VAInquiryResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.VAInquiryResponse), args.Error(1)
}

func (m *MockVAUsecase) Payment(ctx context.Context, req *domain.VAPaymentRequest) (*domain.VAPaymentResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.VAPaymentResponse), args.Error(1)
}

func (m *MockVAUsecase) Status(ctx context.Context, req *domain.VAStatusRequest) (*domain.VAStatusResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*domain.VAStatusResponse), args.Error(1)
}

func TestVAHandler_Inquiry_Success(t *testing.T) {
	e := echo.New()
	req := domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/inquiry", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Inquiry", mock.Anything, &req).Return(&domain.VAInquiryResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAAccountData{
			InquiryStatus: "00",
		},
	}, nil)

	h := NewVAHandler(mockUsecase)

	err := h.Inquiry(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockUsecase.AssertExpectations(t)
}

func TestVAHandler_Inquiry_MissingFields(t *testing.T) {
	e := echo.New()
	req := domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		// Missing required fields
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/inquiry", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	h := NewVAHandler(mockUsecase)

	err := h.Inquiry(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestVAHandler_Payment_Success(t *testing.T) {
	e := echo.New()
	txDate := time.Now()
	req := domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:      &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TransactionDate:  &txDate,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/payment", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Payment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRequest")).Return(&domain.VAPaymentResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PaymentFlagStatus: "00",
		},
	}, nil)

	h := NewVAHandler(mockUsecase)

	err := h.Payment(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockUsecase.AssertExpectations(t)
}

func TestVAHandler_Status_Success(t *testing.T) {
	e := echo.New()
	req := domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1.0/transfer-va/status", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Status", mock.Anything, &req).Return(&domain.VAStatusResponse{
		ResponseCode:    "2002600",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAStatusData{
			PaymentFlagStatus: "00",
		},
	}, nil)

	h := NewVAHandler(mockUsecase)

	err := h.Status(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	mockUsecase.AssertExpectations(t)
}

func TestMapSNAPCodeToHTTP(t *testing.T) {
	tests := []struct {
		code     string
		expected int
	}{
		{"4002400", http.StatusBadRequest},
		{"4012400", http.StatusUnauthorized},
		{"4032400", http.StatusForbidden},
		{"4042419", http.StatusNotFound},
		{"4092400", http.StatusConflict},
		{"5002400", http.StatusInternalServerError},
		{"5042400", http.StatusGatewayTimeout},
		{"", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		result := mapSNAPCodeToHTTP(tt.code)
		assert.Equal(t, tt.expected, result)
	}
}
