package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	// Fixed rather than time.Now(): the monotonic reading time.Now() carries
	// does not survive the JSON round-trip, so the mock's argument comparison
	// would fail on a value that is otherwise identical.
	trxDateInit := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	req := domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		TrxDateInit:      &trxDateInit,
		ChannelCode:      6011,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", bytes.NewReader(body))
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

// A rejected inquiry goes over the wire with the full virtualAccountData the
// usecase resolved — identity, amount, bills — not an empty husk carrying only
// inquiryStatus/inquiryReason.
func TestVAHandler_Inquiry_PaidBill_ReturnsFullVAData(t *testing.T) {
	e := echo.New()
	trxDateInit := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	req := domain.VAInquiryRequest{
		PartnerServiceID: "   15974",
		CustomerNo:       "77121730326",
		VirtualAccountNo: "   1597477121730326",
		InquiryRequestID: "202607021545081597400051562507",
		TrxDateInit:      &trxDateInit,
		ChannelCode:      6011,
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Inquiry", mock.Anything, &req).Return((*domain.VAInquiryResponse)(nil),
		domain.NewInquiryError("4042414", "Paid Bill", domain.ErrVAPaidBill, &domain.VAAccountData{
			PartnerServiceID:   req.PartnerServiceID,
			CustomerNo:         req.CustomerNo,
			VirtualAccountNo:   req.VirtualAccountNo,
			VirtualAccountName: "budi manjo bill var",
			InquiryRequestID:   req.InquiryRequestID,
			TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
			SubCompany:         "00000",
			InquiryStatus:      "01",
			InquiryReason:      &domain.BilingualText{English: "Bill has been paid", Indonesia: "Tagihan telah dibayar"},
		}))

	h := NewVAHandler(mockUsecase)

	err := h.Inquiry(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"responseCode":"4042414","responseMessage":"Paid Bill","virtualAccountData":{"partnerServiceId":"   15974","customerNo":"77121730326","virtualAccountNo":"   1597477121730326","virtualAccountName":"budi manjo bill var","inquiryRequestId":"202607021545081597400051562507","totalAmount":{"value":"150000.00","currency":"IDR"},"subCompany":"00000","billDetails":[],"freeTexts":[],"inquiryStatus":"01","inquiryReason":{"english":"Bill has been paid","indonesia":"Tagihan telah dibayar"}},"additionalInfo":{}}`, rec.Body.String())
	mockUsecase.AssertExpectations(t)
}

func TestVAHandler_Inquiry_MissingFields(t *testing.T) {
	e := echo.New()
	req := domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		// Missing required fields
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/inquiry", bytes.NewReader(body))
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
	trxDateTime := time.Now()
	req := domain.VAPaymentRequest{
		PartnerServiceID:   " 12345",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   " 12345123456789012345678",
		VirtualAccountName: "Budi Manjo",
		TrxID:              "202607221000001234500001",
		PaymentRequestID:   "20260722100000123450",
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

// The wire shape the vendor actually parses: additionalInfo, billDetails and
// freeTexts are structurally guaranteed on every payment response, success or
// rejection, so a vendor can read them without nil/absent checks.
func TestVAHandler_Payment_ResponseAlwaysCarriesObjectAndArrays(t *testing.T) {
	e := echo.New()
	req := domain.VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		TrxID:            "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &domain.Amount{Value: "100000.00", Currency: "IDR"},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	// No BillDetails/FreeTexts/AdditionalInfo set anywhere — the marshallers
	// must still emit them.
	mockUsecase.On("Payment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRequest")).Return(&domain.VAPaymentResponse{
		ResponseCode:    "2002500",
		ResponseMessage: "Successful",
		VirtualAccountData: &domain.VAPaymentStatus{
			PaymentFlagStatus: "00",
		},
	}, nil)

	require.NoError(t, NewVAHandler(mockUsecase).Payment(c))

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.JSONEq(t, `{}`, string(raw["additionalInfo"]))

	var vaData map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["virtualAccountData"], &vaData))
	assert.JSONEq(t, `[]`, string(vaData["billDetails"]))
	assert.JSONEq(t, `[]`, string(vaData["freeTexts"]))
}

// `"billDetails": [null]` is a vendor saying "no bills" in the clumsiest way
// available: encoding/json leaves a one-element slice of blanks behind, which
// would otherwise be persisted and echoed back as a bill that never existed.
// The handler must hand the usecase an empty billDetails instead.
func TestVAHandler_Payment_NullBillDetailEntriesTreatedAsAbsent(t *testing.T) {
	for _, body := range []string{
		`{"partnerServiceId":" 12345","customerNo":"123456789012345678","virtualAccountNo":" 12345123456789012345678","virtualAccountName":"Budi Manjo","trxId":"202607221000001234500001","paymentRequestId":"202607221000001234500001","paidAmount":{"value":"100000.00","currency":"IDR"},"totalAmount":{"value":"100000.00","currency":"IDR"},"flagAdvise":"N","channelCode":6011,"trxDateTime":"2026-07-22T10:00:00+07:00","billDetails":[null]}`,
		`{"partnerServiceId":" 12345","customerNo":"123456789012345678","virtualAccountNo":" 12345123456789012345678","virtualAccountName":"Budi Manjo","trxId":"202607221000001234500001","paymentRequestId":"202607221000001234500001","paidAmount":{"value":"100000.00","currency":"IDR"},"totalAmount":{"value":"100000.00","currency":"IDR"},"flagAdvise":"N","channelCode":6011,"trxDateTime":"2026-07-22T10:00:00+07:00","billDetails":[null,null]}`,
		`{"partnerServiceId":" 12345","customerNo":"123456789012345678","virtualAccountNo":" 12345123456789012345678","virtualAccountName":"Budi Manjo","trxId":"202607221000001234500001","paymentRequestId":"202607221000001234500001","paidAmount":{"value":"100000.00","currency":"IDR"},"totalAmount":{"value":"100000.00","currency":"IDR"},"flagAdvise":"N","channelCode":6011,"trxDateTime":"2026-07-22T10:00:00+07:00","billDetails":[{}]}`,
	} {
		e := echo.New()
		httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", strings.NewReader(body))
		httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(httpReq, rec)

		mockUsecase := new(MockVAUsecase)
		mockUsecase.On("Payment", mock.Anything, mock.MatchedBy(func(r *domain.VAPaymentRequest) bool {
			return len(r.BillDetails) == 0
		})).Return(&domain.VAPaymentResponse{
			ResponseCode:       "2002500",
			ResponseMessage:    "Successful",
			VirtualAccountData: &domain.VAPaymentStatus{PaymentFlagStatus: "00"},
		}, nil)

		require.NoError(t, NewVAHandler(mockUsecase).Payment(c))
		mockUsecase.AssertExpectations(t)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
		var vaData map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw["virtualAccountData"], &vaData))
		assert.JSONEq(t, `[]`, string(vaData["billDetails"]), body)
	}
}

// A resubmitted paymentRequestId is rejected 404/4042518, and the rejection
// still carries the colliding payment's virtualAccountData.
func TestVAHandler_Payment_DuplicateReturnsInconsistentRequest(t *testing.T) {
	e := echo.New()
	dupTrxDateTime := time.Date(2026, 7, 7, 19, 10, 0, 0, time.FixedZone("WIB", 7*3600))
	req := domain.VAPaymentRequest{
		PartnerServiceID:   "   15975",
		CustomerNo:         "77121730326",
		VirtualAccountNo:   "   1597577121730326",
		VirtualAccountName: "budi manjo bill fixed",
		TrxID:              "202607071910007420381",
		PaymentRequestID:   "202607071910007420381204",
		ChannelCode:        6014,
		PaidAmount:         &domain.Amount{Value: "250000.00", Currency: "IDR"},
		TotalAmount:        &domain.Amount{Value: "250000.00", Currency: "IDR"},
		TrxDateTime:        &dupTrxDateTime,
		FlagAdvise:         "N",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Payment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRequest")).Return(
		(*domain.VAPaymentResponse)(nil),
		domain.NewPaymentError(domain.CodePaymentInconsistent, "Inconsistent Request", domain.ErrVAPaymentDuplicate, &domain.VAPaymentStatus{
			PartnerServiceID:   req.PartnerServiceID,
			CustomerNo:         req.CustomerNo,
			VirtualAccountNo:   req.VirtualAccountNo,
			VirtualAccountName: "budi manjo bill fixed",
			PaymentRequestID:   req.PaymentRequestID,
			PaidAmount:         &domain.Amount{Value: "250000.00", Currency: "IDR"},
			TotalAmount:        &domain.Amount{Value: "250000.00", Currency: "IDR"},
			ReferenceNo:        "05146121541",
			PaymentFlagStatus:  "00",
			PaymentFlagReason:  &domain.BilingualText{English: "Success", Indonesia: "Sukses"},
		}),
	)

	require.NoError(t, NewVAHandler(mockUsecase).Payment(c))

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp domain.VAPaymentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "4042518", resp.ResponseCode)
	assert.Equal(t, "Inconsistent Request", resp.ResponseMessage)
	require.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "budi manjo bill fixed", resp.VirtualAccountData.VirtualAccountName)
	assert.Equal(t, "250000.00", resp.VirtualAccountData.PaidAmount.Value)
	assert.Equal(t, "05146121541", resp.VirtualAccountData.ReferenceNo)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
}

// Locks the exact wire shape agreed for the not-found rejection, byte for byte
// against the contract sample (JSONEq ignores key order only).
func TestVAHandler_Payment_UnknownVAWireShape(t *testing.T) {
	const want = `{"responseCode":"4042512","responseMessage":"Invalid Bill/Virtual Account [Not Found]","virtualAccountData":{"paymentFlagReason":{"english":"Virtual Account Not Found","indonesia":"Virtual Account Tidak Ditemukan"},"partnerServiceId":"   15973","customerNo":"00000000000","virtualAccountNo":"   1597300000000000","virtualAccountName":"","paymentRequestId":"202606241142121597300051476279","paidAmount":{"value":"","currency":""},"totalAmount":{"value":"","currency":""},"trxDateTime":"2026-06-25T09:13:20+07:00","referenceNo":"05147220913","paymentFlagStatus":"01","billDetails":[],"freeTexts":[]},"additionalInfo":{}}`

	e := echo.New()
	trxDateTime := time.Date(2026, 6, 25, 9, 13, 20, 0, time.FixedZone("WIB", 7*3600))
	req := domain.VAPaymentRequest{
		PartnerServiceID:   "   15973",
		CustomerNo:         "00000000000",
		VirtualAccountNo:   "   1597300000000000",
		VirtualAccountName: "budi manjo",
		TrxID:              "TRX-unknown",
		PaymentRequestID:   "202606241142121597300051476279",
		ChannelCode:        6014,
		PaidAmount:         &domain.Amount{Value: "50000.00", Currency: "IDR"},
		TotalAmount:        &domain.Amount{Value: "50000.00", Currency: "IDR"},
		ReferenceNo:        "05147220913",
		TrxDateTime:        &trxDateTime,
		FlagAdvise:         "N",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/payment", bytes.NewReader(body))
	httpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)

	mockUsecase := new(MockVAUsecase)
	mockUsecase.On("Payment", mock.Anything, mock.AnythingOfType("*domain.VAPaymentRequest")).Return(
		(*domain.VAPaymentResponse)(nil),
		domain.NewPaymentError(domain.CodePaymentNotFound, "Invalid Bill/Virtual Account [Not Found]", domain.ErrVAInvalidBill,
			&domain.VAPaymentStatus{
				PartnerServiceID:  req.PartnerServiceID,
				CustomerNo:        req.CustomerNo,
				VirtualAccountNo:  req.VirtualAccountNo,
				PaymentRequestID:  req.PaymentRequestID,
				PaidAmount:        &domain.Amount{},
				TotalAmount:       &domain.Amount{},
				TrxDateTime:       req.TrxDateTime,
				ReferenceNo:       req.ReferenceNo,
				PaymentFlagStatus: "01",
				PaymentFlagReason: &domain.BilingualText{English: "Virtual Account Not Found", Indonesia: "Virtual Account Tidak Ditemukan"},
			}),
	)

	require.NoError(t, NewVAHandler(mockUsecase).Payment(c))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, want, rec.Body.String())
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
	httpReq := httptest.NewRequest(http.MethodPost, "/openapi/v1.0/transfer-va/status", bytes.NewReader(body))
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
