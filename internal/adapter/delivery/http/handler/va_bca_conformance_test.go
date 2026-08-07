package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The double-flag replay is not an error — BCA counts 4042518 "Inconsistent
// Request" among the codes it treats as a successful transaction, so the
// usecase returns it as a response. It is still a 404-class code, and BCA
// pairs responseCode prefixes with the matching HTTP status throughout
// Appendix A, so the handler must not staple an HTTP 200 onto it.
func TestVAHandler_Payment_InconsistentRequestUsesMatchingHTTPStatus(t *testing.T) {
	e := echo.New()
	trxDateTime := time.Date(2026, 8, 6, 10, 5, 0, 0, time.UTC)
	req := domain.VAPaymentRequest{
		PartnerServiceID:   " 12345",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   " 12345123456789012345678",
		VirtualAccountName: "Budi Manjo",
		PaymentRequestID:   "202202111031031234500001",
		ChannelCode:        6011,
		PaidAmount:         &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:        &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TrxDateTime:        &trxDateTime,
		FlagAdvise:         "N",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Payment", mock.Anything, mock.Anything).Return(&domain.VAPaymentResponse{
		ResponseCode:    domain.CodePaymentInconsistent,
		ResponseMessage: "Inconsistent Request",
		VirtualAccountData: &domain.VAPaymentStatus{
			// "with paymentFlagStatus and paymentFlagReason according to the
			// results of the first request" — the original succeeded.
			PaymentFlagStatus: domain.PaymentFlagSuccess,
			PaymentFlagReason: domain.ReasonForCode(domain.CodePaymentSuccess),
		},
	}, nil)

	handler := NewVAHandler(mockUsecase)
	require.NoError(t, handler.Payment(c))

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp domain.VAPaymentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "4042518", resp.ResponseCode)
	// The replayed flag status is preserved, not rewritten to a rejection.
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
}

// A plain successful payment still travels as HTTP 200.
func TestVAHandler_Payment_SuccessStaysHTTP200(t *testing.T) {
	e := echo.New()
	trxDateTime := time.Date(2026, 8, 6, 10, 5, 0, 0, time.UTC)
	req := domain.VAPaymentRequest{
		PartnerServiceID:   " 12345",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   " 12345123456789012345678",
		VirtualAccountName: "Budi Manjo",
		PaymentRequestID:   "202202111031031234500002",
		ChannelCode:        6011,
		PaidAmount:         &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:        &domain.Amount{Value: "100000.00", Currency: "IDR"},
		TrxDateTime:        &trxDateTime,
		FlagAdvise:         "N",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Payment", mock.Anything, mock.Anything).Return(&domain.VAPaymentResponse{
		ResponseCode:    domain.CodePaymentSuccess,
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PaymentFlagStatus: domain.PaymentFlagSuccess,
			PaymentFlagReason: domain.ReasonForCode(domain.CodePaymentSuccess),
		},
	}, nil)

	handler := NewVAHandler(mockUsecase)
	require.NoError(t, handler.Payment(c))

	assert.Equal(t, http.StatusOK, rec.Code)
}
