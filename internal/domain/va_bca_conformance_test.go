package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BCA enumerates channelCode rather than leaving it free-form: "Channel code
// field may vary with these values: 6000 = Others, 6010 = Teller ... 6020 =
// Shared Biller". A four-digit number outside that set is not a channel BCA
// generates.
func TestValidatePaymentRequest_ChannelCodeMustBeEnumerated(t *testing.T) {
	for _, code := range []int{6000, 6010, 6011, 6012, 6013, 6014, 6015, 6016, 6017, 6018, 6019, 6020} {
		req := validPaymentRequest()
		req.ChannelCode = code
		assert.Nil(t, ValidatePaymentRequest(req, true), "channelCode %d is documented by BCA", code)
	}

	for _, code := range []int{1, 5999, 6021, 7000, 9999} {
		req := validPaymentRequest()
		req.ChannelCode = code

		v := ValidatePaymentRequest(req, true)
		require.NotNil(t, v, "channelCode %d is not in BCA's table", code)
		assert.Equal(t, ViolationFormat, v.Kind)
		assert.Equal(t, "channelCode", v.Field)
	}
}

// channelCode is Optional (N) on inquiry, so absent stays valid — but a value
// that IS sent is held to the same table.
func TestValidateInquiryRequest_ChannelCodeEnumerated(t *testing.T) {
	// Absent is a mandatory-field outcome for a BCA-configured vendor, and no
	// outcome at all for one that does not follow BCA's table — that split is
	// covered by TestValidateInquiryRequest_BCAMandatoryFieldsGatedByStrictness.
	// What this test pins is the ENUMERATION: a value that is sent must be one
	// BCA generates, whichever mode is in force.
	req := validInquiryRequest()
	req.ChannelCode = 0
	assert.Nil(t, ValidateInquiryRequest(req, false))

	req = validInquiryRequest()
	req.ChannelCode = 6021

	for _, strict := range []bool{true, false} {
		v := ValidateInquiryRequest(req, strict)
		require.NotNil(t, v, "strict=%v", strict)
		assert.Equal(t, ViolationFormat, v.Kind)
		assert.Equal(t, "channelCode", v.Field)
	}
}

// BCA defines virtualAccountNo as "partnerServiceId (8 digit left padding
// space) + customerNo", so it must agree with both halves. A request naming
// one biller in partnerServiceId and another inside the VA number contradicts
// itself.
func TestRequireVAIdentity_VirtualAccountNoMustMatchBothHalves(t *testing.T) {
	t.Run("partnerServiceId is not the prefix", func(t *testing.T) {
		req := validPaymentRequest()
		// Suffix still matches customerNo, so only the prefix check can catch it.
		req.VirtualAccountNo = "   99999123456789012345678"

		v := ValidatePaymentRequest(req, true)
		require.NotNil(t, v)
		assert.Equal(t, ViolationFormat, v.Kind)
		assert.Equal(t, "virtualAccountNo", v.Field)
	})

	t.Run("customerNo is not the suffix", func(t *testing.T) {
		req := validPaymentRequest()
		req.VirtualAccountNo = "   12345000000000000000000"

		v := ValidatePaymentRequest(req, true)
		require.NotNil(t, v)
		assert.Equal(t, ViolationFormat, v.Kind)
		assert.Equal(t, "virtualAccountNo", v.Field)
	})

	t.Run("both halves agree", func(t *testing.T) {
		assert.Nil(t, ValidatePaymentRequest(validPaymentRequest(), true))
		assert.Nil(t, ValidateInquiryRequest(validInquiryRequest(), true))
	})
}

// The inquiry Notes cap both arrays at 5, below what the field table alone
// suggests (24 billDetails / 9 freeTexts).
func TestInquiryArrayCapsMatchBCAsNotes(t *testing.T) {
	assert.Equal(t, 5, MaxInquiryBillDetails)
	assert.Equal(t, 5, MaxInquiryFreeTexts)
}

// What this gateway issues must satisfy BCA's NARROWEST table, not its widest.
// Inquiry allows customerNo(20)/virtualAccountNo(28); payment and status allow
// (18)/(26). Issuing at 20/28 produces a VA the customer can look up and then
// cannot pay.
func TestIssuanceWidthsUseBCAsNarrowestLimits(t *testing.T) {
	assert.Equal(t, 18, MaxIssuedCustomerNo)
	assert.Equal(t, 26, MaxIssuedVirtualAccountNo)

	// The issued widths must be self-consistent: an 8-character
	// partnerServiceId plus the maximum customerNo is exactly the maximum
	// virtualAccountNo.
	assert.Equal(t, MaxIssuedVirtualAccountNo, maxPartnerServiceID+MaxIssuedCustomerNo)

	// And they must stay at or below what we accept inbound, or we would issue
	// VA numbers our own payment endpoint rejects.
	assert.LessOrEqual(t, MaxIssuedCustomerNo, maxPaymentCustomerNo)
	assert.LessOrEqual(t, MaxIssuedVirtualAccountNo, maxPaymentVirtualAccountNo)
}

// The v2.4 inquiry payload carries four Optional fields beyond the identity
// set. They were absent from the request model, so a conformant BCA inquiry
// that sent them was silently missing them from the bound request — and
// nothing bounded their length either.
func TestValidateInquiryRequest_OptionalV24Fields(t *testing.T) {
	req := validInquiryRequest()
	req.Language = "id"
	req.HashedSourceAccountNo = strings.Repeat("a", 32)
	req.SourceBankCode = "014"
	req.PassApp = strings.Repeat("k", 64)

	assert.Nil(t, ValidateInquiryRequest(req, true), "every value is exactly at its limit")

	for _, tc := range []struct {
		name   string
		mutate func(*VAInquiryRequest)
		field  string
	}{
		{"language over 2", func(r *VAInquiryRequest) { r.Language = "idn" }, "language"},
		{"hashedSourceAccountNo over 32", func(r *VAInquiryRequest) {
			r.HashedSourceAccountNo = strings.Repeat("a", 33)
		}, "hashedSourceAccountNo"},
		{"sourceBankCode over 3", func(r *VAInquiryRequest) { r.SourceBankCode = "0141" }, "sourceBankCode"},
		{"passApp over 64", func(r *VAInquiryRequest) { r.PassApp = strings.Repeat("k", 65) }, "passApp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := validInquiryRequest()
			tc.mutate(bad)

			v := ValidateInquiryRequest(bad, true)
			require.NotNil(t, v)
			assert.Equal(t, ViolationFormat, v.Kind)
			assert.Equal(t, tc.field, v.Field)
		})
	}
}

// passApp is "Key for 3rd party to access API like client secret". It is bound
// so the field exists on the contract, but must never be treated as a
// credential: this service authenticates on X-SIGNATURE, which covers the body
// but is verified before it. A request whose passApp is empty, wrong, or
// absent is accepted or rejected purely on the other fields.
func TestValidateInquiryRequest_PassAppIsNotACredential(t *testing.T) {
	for _, passApp := range []string{"", "wrong-secret", "anything at all"} {
		req := validInquiryRequest()
		req.PassApp = passApp
		assert.Nil(t, ValidateInquiryRequest(req, true), "passApp %q must not gate the request", passApp)
	}
}

// The inquiry request model must bind every field name BCA publishes, or a
// value BCA sends lands nowhere.
func TestVAInquiryRequest_BindsEveryDocumentedField(t *testing.T) {
	body := []byte(`{
		"partnerServiceId": "   12345",
		"customerNo": "123456789012345678",
		"virtualAccountNo": "   12345123456789012345678",
		"trxDateInit": "2022-02-12T17:29:57+07:00",
		"channelCode": 6011,
		"language": "id",
		"amount": {"value": "100000.00", "currency": "IDR"},
		"hashedSourceAccountNo": "ABCDEF0123456789",
		"sourceBankCode": "014",
		"additionalInfo": {},
		"passApp": "secret",
		"inquiryRequestId": "202202110909311234500001136962"
	}`)

	var req VAInquiryRequest
	require.NoError(t, json.Unmarshal(body, &req))

	assert.Equal(t, "   12345", req.PartnerServiceID)
	assert.Equal(t, "123456789012345678", req.CustomerNo)
	assert.Equal(t, "   12345123456789012345678", req.VirtualAccountNo)
	require.NotNil(t, req.TrxDateInit)
	assert.Equal(t, 6011, req.ChannelCode)
	assert.Equal(t, "id", req.Language)
	require.NotNil(t, req.Amount)
	assert.Equal(t, "100000.00", req.Amount.Value)
	assert.Equal(t, "ABCDEF0123456789", req.HashedSourceAccountNo)
	assert.Equal(t, "014", req.SourceBankCode)
	assert.Equal(t, "secret", req.PassApp)
	assert.Equal(t, "202202110909311234500001136962", req.InquiryRequestID)
	assert.NotNil(t, req.AdditionalInfo)
}
