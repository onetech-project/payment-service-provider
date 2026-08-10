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

// Conformance tests for "Virtual Account untuk Biller" (Developer API BCA).
// Each one pins a field BCA marks Mandatory, or a value its Notes pin
// explicitly — the class of defect that does not fail locally but is rejected
// at the channel, in front of the customer.

// BCA: "subCompany ... Mandatory in BCA. Partner's product code (sub company
// code). Mandatory for non-multibills transaction." An empty string is dropped
// by omitempty, producing a response BCA rejects, so the documented default
// code stands in.
func TestInquiry_SubCompanyDefaultsToBCAsDefaultCode(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	// Neither the transaction nor any bill carries a sub-company code.
	record := &domain.VAInquiryRecord{
		ID:               "txn-1",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		CustomerName:     "Faris",
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(record, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, record.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "00000", resp.VirtualAccountData.SubCompany)
}

// BCA's inquiry Notes cap both arrays at 5 regardless of what the field table
// says ("billDetails should not be greater than 5", "The occurences for
// freeTexts field in inquiry bill should not be greater than 5"). A stored row
// that predates the create-va validation must still produce a presentable
// inquiry rather than one BCA fails wholesale.
func TestInquiry_CapsBillDetailsAndFreeTextsAtFive(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "INQ-overlong",
	}

	freeTexts := make([]domain.BilingualText, 8)
	for i := range freeTexts {
		freeTexts[i] = domain.BilingualText{English: "note", Indonesia: "catatan"}
	}
	record := &domain.VAInquiryRecord{
		ID:               "txn-overlong",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
		FreeTexts:        freeTexts,
	}

	bills := make([]domain.BillDetail, 7)
	for i := range bills {
		bills[i] = domain.BillDetail{BillNo: "INV", BillSubCompany: "00003"}
	}

	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(record, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, record.ID).Return(bills, nil)

	resp, err := usecase.Inquiry(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, resp.VirtualAccountData.BillDetails, domain.MaxInquiryBillDetails)
	assert.Len(t, resp.VirtualAccountData.FreeTexts, domain.MaxInquiryFreeTexts)
}

// BCA marks paidAmount, totalAmount, transactionDate and paymentRequestId
// Mandatory on the inquiry-status response. The pending branch has no payment
// to read them from, but "no payment yet" must still be expressed in fields
// BCA can parse — nil pointers render as JSON null, which it cannot.
func TestStatus_PendingBranchPopulatesMandatoryFields(t *testing.T) {
	mockRepo := new(MockVARepository)
	usecase := NewVAUsecase(mockRepo, nil)

	req := &domain.VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202202111031031234500001",
	}

	createdAt := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	inquiry := &domain.VAInquiryRecord{
		ID:               "txn-pending",
		PartnerServiceID: req.PartnerServiceID,
		CustomerNo:       req.CustomerNo,
		VirtualAccountNo: req.VirtualAccountNo,
		InquiryRequestID: req.InquiryRequestID,
		Status:           "03",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
		CreatedAt:        createdAt,
	}

	mockRepo.On("GetPayment", mock.Anything, req.InquiryRequestID).Return(nil, domain.ErrVAInvalidBill)
	mockRepo.On("GetInquiry", mock.Anything, req.InquiryRequestID).Return(inquiry, nil)
	mockRepo.On("GetVABillDetails", mock.Anything, inquiry.ID).Return([]domain.BillDetail(nil), nil)

	resp, err := usecase.Status(context.Background(), req)

	require.NoError(t, err)
	data := resp.VirtualAccountData
	assert.Equal(t, "Success", resp.ResponseMessage)
	assert.Equal(t, domain.PaymentFlagPending, data.PaymentFlagStatus)

	require.NotNil(t, data.PaidAmount, "paidAmount is Mandatory; nil renders as JSON null")
	assert.Equal(t, "0.00", data.PaidAmount.Value)
	assert.Equal(t, "IDR", data.PaidAmount.Currency)

	require.NotNil(t, data.TotalAmount)
	assert.Equal(t, "100000.00", data.TotalAmount.Value)

	require.NotNil(t, data.TransactionDate, "transactionDate is Mandatory; nil renders as JSON null")
	assert.Equal(t, createdAt, *data.TransactionDate)

	// "paymentRequestId ... This value must be same with inquiryRequestId".
	assert.Equal(t, req.InquiryRequestID, data.PaymentRequestID)
}

// BCA: "billerReferenceId ... This field value must be the same with
// billReferenceNo from payment request". BCA never sends billerReferenceId
// inbound, so the fallback is the only path that ever runs — and it must
// reach for billReferenceNo, not the partner's own billNo.
func TestEchoPaymentBillDetails_BillerReferenceIDMirrorsBillReferenceNo(t *testing.T) {
	out := echoPaymentBillDetails([]domain.VAPaymentBillDetail{{
		BillNo:          "INV-0001",
		BillReferenceNo: "98765432101",
		BillSubCompany:  "00001",
		BillAmount:      &domain.Amount{Value: "10000.00", Currency: "IDR"},
	}})

	require.Len(t, out, 1)
	assert.Equal(t, "98765432101", out[0].BillerReferenceID)
	assert.NotEqual(t, "INV-0001", out[0].BillerReferenceID)
	// status/reason still default for a successful payment.
	assert.Equal(t, "00", out[0].Status)
	require.NotNil(t, out[0].Reason)
}

// totalAmount is Mandatory on BCA's PaymentResponse, and this builder serves
// the 4042518 double-flag replay — the response BCA scrutinises most.
func TestPaymentResponseFromRecord_CarriesTotalAmount(t *testing.T) {
	resp := paymentResponseFromRecord(&domain.VAPaymentRecord{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		PaymentRequestID: "202202111031031234500001",
		PaidAmount:       "100000.00",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
		TransactionDate:  time.Now(),
	})

	require.NotNil(t, resp.VirtualAccountData.TotalAmount)
	assert.Equal(t, "100000.00", resp.VirtualAccountData.TotalAmount.Value)
	assert.Equal(t, "IDR", resp.VirtualAccountData.TotalAmount.Currency)
}

// A record written before total_amount was populated still has to answer with
// a parseable totalAmount rather than an empty one.
func TestPaymentResponseFromRecord_TotalAmountFallsBackToPaidAmount(t *testing.T) {
	resp := paymentResponseFromRecord(&domain.VAPaymentRecord{
		PaymentRequestID: "202202111031031234500002",
		PaidAmount:       "250000.00",
		Currency:         "IDR",
		TransactionDate:  time.Now(),
	})

	require.NotNil(t, resp.VirtualAccountData.TotalAmount)
	assert.Equal(t, "250000.00", resp.VirtualAccountData.TotalAmount.Value)
}

// A merchant-supplied static customerNo is held to the same issuance width as
// a generated one: BCA's payment/status tables cap customerNo at 18, and a VA
// number issued above that is inquirable but unpayable.
func TestCreateVA_RejectsCustomerNoWiderThanBCAAccepts(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "12345678901234567890", // 20 digits: over BCA's 18
		VirtualAccountNo:   "08889912345678901234567890",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-too-wide",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
	}

	resp, err := uc.CreateVA(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "customerNo too long")
	mockRepo.AssertNotCalled(t, "SaveInquiry")
}

// The boundary itself stays accepted.
func TestCreateVA_AcceptsCustomerNoAtBCAsLimit(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "123456789012345678", // exactly 18
		VirtualAccountNo:   "088899123456789012345678",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-at-limit",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).
		Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.Anything).Return(nil)

	resp, err := uc.CreateVA(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "2002700", resp.ResponseCode)
}

// BCA's inquiry Notes cap billDetails and freeTexts at 5. Refusing an
// over-long list at create-va is what stops the merchant from discovering the
// problem at the channel, in front of the customer.
func TestCreateVA_RejectsOverlongBillDetailsAndFreeTexts(t *testing.T) {
	newReq := func() *domain.MerchantCreateVARequest {
		return &domain.MerchantCreateVARequest{
			PartnerServiceID:   "088899",
			CustomerNo:         "123456789012345678",
			VirtualAccountNo:   "088899123456789012345678",
			VirtualAccountName: "Jokul Doe",
			TrxID:              "trx-arrays",
			TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
		}
	}

	t.Run("billDetails", func(t *testing.T) {
		uc := NewMerchantVAUsecase(new(MockMerchantVARepository), newTestVATypeRuleProvider())
		req := newReq()
		req.BillDetails = make([]domain.BillDetail, domain.MaxInquiryBillDetails+1)

		_, err := uc.CreateVA(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "billDetails")
	})

	t.Run("freeTexts", func(t *testing.T) {
		uc := NewMerchantVAUsecase(new(MockMerchantVARepository), newTestVATypeRuleProvider())
		req := newReq()
		req.FreeTexts = make([]domain.BilingualText, domain.MaxInquiryFreeTexts+1)

		_, err := uc.CreateVA(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "freeTexts")
	})
}

// freeTexts set at create-va must actually reach the stored transaction —
// they are what BCA displays on the channel screen. Previously they were
// echoed in the create-va response and then dropped on the floor.
func TestCreateVA_PersistsFreeTextsForInquiry(t *testing.T) {
	mockRepo := new(MockMerchantVARepository)
	uc := NewMerchantVAUsecase(mockRepo, newTestVATypeRuleProvider())

	freeTexts := []domain.BilingualText{{English: "Tuition Q3", Indonesia: "SPP Q3"}}
	req := &domain.MerchantCreateVARequest{
		PartnerServiceID:   "088899",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   "088899123456789012345678",
		VirtualAccountName: "Jokul Doe",
		TrxID:              "trx-freetexts",
		TotalAmount:        &domain.Amount{Value: "150000.00", Currency: "IDR"},
		FreeTexts:          freeTexts,
	}

	mockRepo.On("GetVAByVirtualAccountNo", mock.Anything, req.VirtualAccountNo).
		Return(nil, domain.ErrMerchantVANotFound)
	mockRepo.On("SaveInquiry", mock.Anything, mock.MatchedBy(func(r *domain.VAInquiryRecord) bool {
		return len(r.FreeTexts) == 1 && r.FreeTexts[0].Indonesia == "SPP Q3"
	})).Return(nil)

	_, err := uc.CreateVA(context.Background(), req)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
