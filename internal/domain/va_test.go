package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAmount(t *testing.T) {
	amount := &Amount{
		Value:    "100000.00",
		Currency: "IDR",
	}

	assert.Equal(t, "100000.00", amount.Value)
	assert.Equal(t, "IDR", amount.Currency)
}

func TestBilingualText(t *testing.T) {
	text := &BilingualText{
		English:   "Success",
		Indonesia: "Sukses",
	}

	assert.Equal(t, "Success", text.English)
	assert.Equal(t, "Sukses", text.Indonesia)
}

func TestBillDetail(t *testing.T) {
	bill := &BillDetail{
		BillNo: "123456789012345678",
		BillDescription: &BilingualText{
			English:   "Maintenance",
			Indonesia: "Pemeliharaan",
		},
		BillSubCompany: "00000",
		BillAmount: &Amount{
			Value:    "100000.00",
			Currency: "IDR",
		},
	}

	assert.Equal(t, "123456789012345678", bill.BillNo)
	assert.Equal(t, "00000", bill.BillSubCompany)
	assert.NotNil(t, bill.BillDescription)
	assert.NotNil(t, bill.BillAmount)
}

func TestVAInquiryRequest(t *testing.T) {
	req := &VAInquiryRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		ChannelCode:      6011,
		Amount:           &Amount{Value: "100000.00", Currency: "IDR"},
	}

	assert.Equal(t, " 12345", req.PartnerServiceID)
	assert.Equal(t, "123456789012345678", req.CustomerNo)
	assert.Equal(t, 6011, req.ChannelCode)
	assert.Equal(t, "100000.00", req.Amount.Value)
}

func TestVAInquiryRequest_UnmarshalsTxnDateInit(t *testing.T) {
	body := []byte(`{"txnDateInit": "2026-07-23T10:00:00+07:00"}`)

	var req VAInquiryRequest
	err := json.Unmarshal(body, &req)

	assert.NoError(t, err)
	assert.NotNil(t, req.TrxDateInit)
}

func TestVAInquiryResponse(t *testing.T) {
	resp := &VAInquiryResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &VAAccountData{
			InquiryStatus: "00",
			InquiryReason: &BilingualText{
				English:   "Success",
				Indonesia: "Sukses",
			},
		},
	}

	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.NotNil(t, resp.VirtualAccountData)
	assert.Equal(t, "00", resp.VirtualAccountData.InquiryStatus)
}

func TestVAPaymentRequest(t *testing.T) {
	req := &VAPaymentRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       &Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:      &Amount{Value: "100000.00", Currency: "IDR"},
	}

	assert.Equal(t, "100000.00", req.PaidAmount.Value)
	assert.Equal(t, "100000.00", req.TotalAmount.Value)
}

func TestVAPaymentResponse(t *testing.T) {
	resp := &VAPaymentResponse{
		ResponseCode:    "2002400",
		ResponseMessage: "Successful",
		VirtualAccountData: &VAPaymentStatus{
			PaymentFlagStatus: "00",
			PaymentFlagReason: &BilingualText{
				English:   "Success",
				Indonesia: "Sukses",
			},
		},
	}

	assert.Equal(t, "2002400", resp.ResponseCode)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
}

func TestVAStatusRequest(t *testing.T) {
	req := &VAStatusRequest{
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
	}

	assert.Equal(t, " 12345", req.PartnerServiceID)
}

func TestVAStatusResponse(t *testing.T) {
	resp := &VAStatusResponse{
		ResponseCode:    "2002600",
		ResponseMessage: "Successful",
		VirtualAccountData: &VAStatusData{
			PaymentFlagStatus: "00",
		},
	}

	assert.Equal(t, "2002600", resp.ResponseCode)
	assert.Equal(t, "00", resp.VirtualAccountData.PaymentFlagStatus)
}

func TestVAInquiryRecord(t *testing.T) {
	record := &VAInquiryRecord{
		ID:               "test-id",
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		Status:           "00",
		TotalAmount:      "100000.00",
		Currency:         "IDR",
	}

	assert.Equal(t, "test-id", record.ID)
	assert.Equal(t, "00", record.Status)
}

func TestVAPaymentRecord(t *testing.T) {
	record := &VAPaymentRecord{
		ID:               "test-id",
		PartnerServiceID: " 12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: " 12345123456789012345678",
		InquiryRequestID: "202607221000001234500001",
		PaymentRequestID: "202607221000001234500001",
		PaidAmount:       "100000.00",
		Currency:         "IDR",
		Status:           "00",
	}

	assert.Equal(t, "test-id", record.ID)
	assert.Equal(t, "00", record.Status)
}
