package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validInquiryRequest() *VAInquiryRequest {
	trxDateInit := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	return &VAInquiryRequest{
		PartnerServiceID: "   12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: "   12345123456789012345678",
		InquiryRequestID: "202202110909311234500001136962",
		TrxDateInit:      &trxDateInit,
		ChannelCode:      6011,
	}
}

func validPaymentRequest() *VAPaymentRequest {
	trxDateTime := time.Date(2026, 8, 6, 10, 5, 0, 0, time.UTC)
	return &VAPaymentRequest{
		PartnerServiceID:   "   12345",
		CustomerNo:         "123456789012345678",
		VirtualAccountNo:   "   12345123456789012345678",
		VirtualAccountName: "Budi Manjo",
		PaymentRequestID:   "202202111031031234500001",
		ChannelCode:        6011,
		PaidAmount:         &Amount{Value: "100000.00", Currency: "IDR"},
		TotalAmount:        &Amount{Value: "100000.00", Currency: "IDR"},
		TrxDateTime:        &trxDateTime,
		FlagAdvise:         "N",
		ReferenceNo:        "12345678901",
	}
}

// --- Inquiry ------------------------------------------------------------

func TestValidateInquiryRequest_Valid(t *testing.T) {
	assert.Nil(t, ValidateInquiryRequest(validInquiryRequest(), true))
}

func TestValidateInquiryRequest_AmountIsNotRequired(t *testing.T) {
	// The whole point: BCA's inquiry payload has no `amount` field, so a
	// conformant inquiry that omits it must pass validation.
	req := validInquiryRequest()
	req.Amount = nil

	assert.Nil(t, ValidateInquiryRequest(req, true))
}

func TestValidateInquiryRequest_MandatoryFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*VAInquiryRequest)
		field  string
	}{
		{"partnerServiceId", func(r *VAInquiryRequest) { r.PartnerServiceID = "" }, "partnerServiceId"},
		{"customerNo", func(r *VAInquiryRequest) { r.CustomerNo = "" }, "customerNo"},
		{"virtualAccountNo", func(r *VAInquiryRequest) { r.VirtualAccountNo = "" }, "virtualAccountNo"},
		{"inquiryRequestId", func(r *VAInquiryRequest) { r.InquiryRequestID = "" }, "inquiryRequestId"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validInquiryRequest()
			tc.mutate(req)

			v := ValidateInquiryRequest(req, true)

			require.NotNil(t, v)
			assert.Equal(t, ViolationMandatory, v.Kind)
			assert.Equal(t, tc.field, v.Field)
		})
	}
}

func TestValidate_TwentyDigitCustomerNoAcceptedOnEveryService(t *testing.T) {
	// This gateway issues 20-digit customerNo values of its own (feature
	// 006-static-dynamic-va dynamic sequences). Enforcing BCA's narrower
	// payment-table limit of 18 would reject a payment for a VA number this
	// system handed the customer, so all three services take the wider pair.
	const (
		partnerServiceID = "15973"
		// 20 digits, the width the dynamic customerNo sequence emits.
		customerNo = "04000000000000000001"
	)
	virtualAccountNo := partnerServiceID + customerNo
	require.Len(t, customerNo, 20)

	inquiry := validInquiryRequest()
	inquiry.PartnerServiceID = partnerServiceID
	inquiry.CustomerNo = customerNo
	inquiry.VirtualAccountNo = virtualAccountNo
	assert.Nil(t, ValidateInquiryRequest(inquiry, true))

	payment := validPaymentRequest()
	payment.PartnerServiceID = partnerServiceID
	payment.CustomerNo = customerNo
	payment.VirtualAccountNo = virtualAccountNo
	assert.Nil(t, ValidatePaymentRequest(payment, true))

	status := &VAStatusRequest{
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		VirtualAccountNo: virtualAccountNo,
		InquiryRequestID: "INQ-1",
	}
	assert.Nil(t, ValidateStatusRequest(status))
}

func TestValidateInquiryRequest_OverLongCustomerNo(t *testing.T) {
	req := validInquiryRequest()
	req.CustomerNo = "123456789012345678901"
	req.VirtualAccountNo = "   12345123456789012345678901"

	v := ValidateInquiryRequest(req, true)

	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "customerNo", v.Field)
}

// --- Payment ------------------------------------------------------------

func TestValidatePaymentRequest_Valid(t *testing.T) {
	assert.Nil(t, ValidatePaymentRequest(validPaymentRequest(), true))
}

func TestValidatePaymentRequest_TrxIDIsNotMandatory(t *testing.T) {
	// BCA marks trxId "N — Mandatory if payment comes from the Create VA
	// Request". Requiring it unconditionally rejected valid
	// channel-originated payments.
	req := validPaymentRequest()
	req.TrxID = ""

	assert.Nil(t, ValidatePaymentRequest(req, true))
}

func TestValidatePaymentRequest_StrictMandatoryFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*VAPaymentRequest)
		field  string
	}{
		{"virtualAccountName", func(r *VAPaymentRequest) { r.VirtualAccountName = "" }, "virtualAccountName"},
		{"channelCode", func(r *VAPaymentRequest) { r.ChannelCode = 0 }, "channelCode"},
		{"totalAmount", func(r *VAPaymentRequest) { r.TotalAmount = nil }, "totalAmount"},
		{"trxDateTime", func(r *VAPaymentRequest) { r.TrxDateTime = nil }, "trxDateTime"},
		{"flagAdvise", func(r *VAPaymentRequest) { r.FlagAdvise = "" }, "flagAdvise"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validPaymentRequest()
			tc.mutate(req)

			v := ValidatePaymentRequest(req, true)

			require.NotNil(t, v, "BCA marks %s Mandatory", tc.field)
			assert.Equal(t, ViolationMandatory, v.Kind)
			assert.Equal(t, tc.field, v.Field)

			// The same request passes for a vendor that does not follow BCA's
			// extended mandatory set — one vendor's field table must not be
			// forced onto the others.
			assert.Nil(t, ValidatePaymentRequest(req, false))
		})
	}
}

func TestValidatePaymentRequest_CoreMandatoryFieldsApplyInBothModes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*VAPaymentRequest)
		field  string
	}{
		{"paymentRequestId", func(r *VAPaymentRequest) { r.PaymentRequestID = "" }, "paymentRequestId"},
		{"paidAmount", func(r *VAPaymentRequest) { r.PaidAmount = nil }, "paidAmount"},
		{"partnerServiceId", func(r *VAPaymentRequest) { r.PartnerServiceID = "" }, "partnerServiceId"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validPaymentRequest()
			tc.mutate(req)

			for _, strict := range []bool{true, false} {
				v := ValidatePaymentRequest(req, strict)
				require.NotNil(t, v, "strict=%v", strict)
				assert.Equal(t, ViolationMandatory, v.Kind)
				assert.Equal(t, tc.field, v.Field)
			}
		})
	}
}

func TestValidatePaymentRequest_FieldLengths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*VAPaymentRequest)
		field  string
	}{
		{"partnerServiceId over 8", func(r *VAPaymentRequest) { r.PartnerServiceID = "123456789" }, "partnerServiceId"},
		{"paymentRequestId over 30", func(r *VAPaymentRequest) { r.PaymentRequestID = "1234567890123456789012345678901" }, "paymentRequestId"},
		{"virtualAccountName over 30", func(r *VAPaymentRequest) { r.VirtualAccountName = "1234567890123456789012345678901" }, "virtualAccountName"},
		{"referenceNo over 11", func(r *VAPaymentRequest) { r.ReferenceNo = "123456789012" }, "referenceNo"},
		{"journalNum over 6", func(r *VAPaymentRequest) { r.JournalNum = "1234567" }, "journalNum"},
		{"paidBills over 6", func(r *VAPaymentRequest) { r.PaidBills = "1234567" }, "paidBills"},
		{"subCompany over 5", func(r *VAPaymentRequest) { r.SubCompany = "123456" }, "subCompany"},
		{"sourceBankCode over 3", func(r *VAPaymentRequest) { r.SourceBankCode = "1234" }, "sourceBankCode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validPaymentRequest()
			tc.mutate(req)

			v := ValidatePaymentRequest(req, true)

			require.NotNil(t, v)
			assert.Equal(t, ViolationFormat, v.Kind)
			assert.Equal(t, tc.field, v.Field)
		})
	}
}

func TestValidatePaymentRequest_FlagAdviseValues(t *testing.T) {
	for _, value := range []string{"N", "Y", "n", "y"} {
		req := validPaymentRequest()
		req.FlagAdvise = value
		assert.Nil(t, ValidatePaymentRequest(req, true), "flagAdvise %q is documented", value)
	}

	req := validPaymentRequest()
	req.FlagAdvise = "X"
	v := ValidatePaymentRequest(req, true)
	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "flagAdvise", v.Field)
}

func TestValidatePaymentRequest_AmountFormat(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		valid bool
	}{
		{"integer", "250000", true},
		{"two decimals", "250000.00", true},
		{"one decimal", "250000.0", true},
		{"13 integer digits", "1234567890123.00", true},
		{"14 integer digits", "12345678901234.00", false},
		{"three decimals", "250000.000", false},
		{"negative", "-1.00", false},
		{"not a number", "abc", false},
		{"thousand separators", "250,000.00", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validPaymentRequest()
			req.PaidAmount = &Amount{Value: tc.value, Currency: "IDR"}
			req.TotalAmount = &Amount{Value: tc.value, Currency: "IDR"}

			v := ValidatePaymentRequest(req, true)

			if tc.valid {
				assert.Nil(t, v)
				return
			}
			require.NotNil(t, v)
			assert.Equal(t, ViolationFormat, v.Kind)
		})
	}
}

func TestValidatePaymentRequest_Currency(t *testing.T) {
	for _, currency := range []string{"IDR", "USD", "SGD"} {
		req := validPaymentRequest()
		req.PaidAmount.Currency = currency
		req.TotalAmount.Currency = currency
		assert.Nil(t, ValidatePaymentRequest(req, true), "%s is documented for VA", currency)
	}

	req := validPaymentRequest()
	req.PaidAmount.Currency = "EUR"
	req.TotalAmount.Currency = "EUR"
	v := ValidatePaymentRequest(req, true)
	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "paidAmount.currency", v.Field)
}

func TestValidatePaymentRequest_CurrencyMustAgreeAcrossAmounts(t *testing.T) {
	// "currency must be the same for totalAmount, billAmount, and paidAmount".
	req := validPaymentRequest()
	req.TotalAmount.Currency = "USD"

	v := ValidatePaymentRequest(req, true)

	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "totalAmount.currency", v.Field)
}

func TestValidatePaymentRequest_BillDetailsCap(t *testing.T) {
	req := validPaymentRequest()
	req.BillDetails = make([]VAPaymentBillDetail, 6)

	v := ValidatePaymentRequest(req, true)

	require.NotNil(t, v)
	assert.Equal(t, "billDetails", v.Field)
}

func TestValidatePaymentRequest_CustomerNoMustBeSuffixOfVA(t *testing.T) {
	// virtualAccountNo is documented as partnerServiceId + customerNo, so a
	// customerNo that is not its suffix means the two disagree about which VA
	// is being paid.
	req := validPaymentRequest()
	req.CustomerNo = "999999999999999999"

	v := ValidatePaymentRequest(req, true)

	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "virtualAccountNo", v.Field)
}

func TestValidatePaymentRequest_UnpaddedPartnerServiceIDAccepted(t *testing.T) {
	// partnerServiceId is enforced as a max width, not an exact one, so
	// merchants that registered VAs under an unpadded company code keep
	// working while BCA's own padded 8-char form still validates.
	req := validPaymentRequest()
	req.PartnerServiceID = "15973"
	req.CustomerNo = "77121730326"
	req.VirtualAccountNo = "1597377121730326"

	assert.Nil(t, ValidatePaymentRequest(req, true))
}

// --- Status -------------------------------------------------------------

func TestValidateStatusRequest_Valid(t *testing.T) {
	req := &VAStatusRequest{
		PartnerServiceID: "   12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: "   12345123456789012345678",
		InquiryRequestID: "202202111031031234500001",
	}

	assert.Nil(t, ValidateStatusRequest(req))
}

func TestValidateStatusRequest_InquiryRequestIDLimitIs30(t *testing.T) {
	// Status uses payment's limits, not inquiry's 128.
	req := &VAStatusRequest{
		PartnerServiceID: "   12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: "   12345123456789012345678",
		InquiryRequestID: "1234567890123456789012345678901",
	}

	v := ValidateStatusRequest(req)

	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "inquiryRequestId", v.Field)
}

// --- Alignment with the current BCA technical documentation ---
// (VA-BillPresentment v2.4, VA-Payment-Flag v2.3, VA-Payment-Status V2 v1.0)

// v2.4 puts inquiryRequestId at String(30), down from the 128 the older
// documentation gave it. The cap is not cosmetic: paymentRequestId "must be
// the same with inquiryRequestId" and is itself capped at 30, so a longer id
// accepted here would only fail at the payment that follows.
func TestValidateInquiryRequest_InquiryRequestIDCappedAt30(t *testing.T) {
	req := validInquiryRequest()
	req.InquiryRequestID = strings.Repeat("9", 30)
	assert.Nil(t, ValidateInquiryRequest(req, true), "30 characters is exactly the limit")

	req.InquiryRequestID = strings.Repeat("9", 31)
	v := ValidateInquiryRequest(req, true)
	if assert.NotNil(t, v) {
		assert.Equal(t, "inquiryRequestId", v.Field)
		assert.Equal(t, ViolationFormat, v.Kind)
	}
}

// trxDateInit and channelCode are Mandatory (Y) on the v2.4 inquiry payload,
// and are required of a vendor configured for BCA's field tables — the same
// per-vendor switch payment's extended mandatory set already runs behind.
//
// They were previously never required, on the reasoning that being more
// permissive on receive cannot produce a wrong answer. True as far as it goes,
// but it also meant a BCA request that dropped a mandatory field was answered
// as if nothing were missing, and it left inquiry inconsistent with payment,
// where the identical fields are enforced. Gating rather than dropping the
// check keeps the permissive behaviour available to the vendors it was for.
func TestValidateInquiryRequest_BCAMandatoryFieldsGatedByStrictness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*VAInquiryRequest)
		field  string
	}{
		{"trxDateInit", func(r *VAInquiryRequest) { r.TrxDateInit = nil }, "trxDateInit"},
		{"channelCode", func(r *VAInquiryRequest) { r.ChannelCode = 0 }, "channelCode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validInquiryRequest()
			tc.mutate(req)

			v := ValidateInquiryRequest(req, true)
			if assert.NotNil(t, v, "BCA marks %s Mandatory", tc.field) {
				assert.Equal(t, ViolationMandatory, v.Kind)
				assert.Equal(t, tc.field, v.Field)
			}

			assert.Nil(t, ValidateInquiryRequest(req, false),
				"a vendor that does not follow BCA's field table is unaffected")
		})
	}
}

// Only the COUNT of freeTexts was checked before, so an over-long entry was
// accepted and then echoed into the response, where BCA fails the whole
// transaction on it.
func TestValidatePaymentRequest_FreeTextEntryLength(t *testing.T) {
	req := validPaymentRequest()
	req.FreeTexts = []BilingualText{{English: strings.Repeat("a", MaxFreeTextLength), Indonesia: "ok"}}
	assert.Nil(t, ValidatePaymentRequest(req, true), "32 characters is exactly the limit")

	req.FreeTexts = []BilingualText{{English: strings.Repeat("a", MaxFreeTextLength+1), Indonesia: "ok"}}
	v := ValidatePaymentRequest(req, true)
	if assert.NotNil(t, v) {
		assert.Equal(t, "freeTexts", v.Field)
	}

	// The Indonesian side is bounded by the same limit, not just the English.
	req.FreeTexts = []BilingualText{{English: "ok", Indonesia: strings.Repeat("b", MaxFreeTextLength+1)}}
	assert.NotNil(t, ValidatePaymentRequest(req, true))
}
