package domain

import (
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
func TestValidateInquiryRequest_ChannelCodeOptionalButEnumerated(t *testing.T) {
	req := validInquiryRequest()
	req.ChannelCode = 0
	assert.Nil(t, ValidateInquiryRequest(req))

	req = validInquiryRequest()
	req.ChannelCode = 6021

	v := ValidateInquiryRequest(req)
	require.NotNil(t, v)
	assert.Equal(t, ViolationFormat, v.Kind)
	assert.Equal(t, "channelCode", v.Field)
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
		assert.Nil(t, ValidateInquiryRequest(validInquiryRequest()))
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
