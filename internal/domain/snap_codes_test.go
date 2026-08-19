package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every code asserted here is transcribed from an Appendix A table in
// Developer API BCA, "Virtual Account untuk Biller". BCA matches responseCode
// against the service it called, so a code carrying the wrong service digits
// fails the transaction even when the HTTP status is right.

func TestServiceCodeForPath(t *testing.T) {
	for path, want := range map[string]string{
		"/openapi/v1.0/transfer-va/inquiry": ServiceCodeInquiry,
		"/openapi/v1.0/transfer-va/payment": ServiceCodePayment,
		"/openapi/v2.0/transfer-va/status":  ServiceCodeStatus,
		"/v1.0/transfer-va/inquiry":         ServiceCodeInquiry,
		"/openapi/v1.0/access-token/b2b":    ServiceCodeToken,
		"/health":                           ServiceCodeToken,
	} {
		assert.Equal(t, want, ServiceCodeForPath(path), "path %s", path)
	}
}

func TestPerServiceCodes_MatchBCAAppendixA(t *testing.T) {
	for _, tc := range []struct {
		service string
		// {badRequest, invalidField, missingMandatory, unauthorized,
		//  invalidToken, conflict, internalError}
		want [7]string
	}{
		{ServiceCodeInquiry, [7]string{"4002400", "4002401", "4002402", "4012400", "4012401", "4092400", "5002400"}},
		{ServiceCodePayment, [7]string{"4002500", "4002501", "4002502", "4012500", "4012501", "4092500", "5002500"}},
		{ServiceCodeStatus, [7]string{"4002600", "4002601", "4002602", "4012600", "4012601", "4092600", "5002600"}},
	} {
		t.Run(tc.service, func(t *testing.T) {
			assert.Equal(t, tc.want[0], CodeBadRequest(tc.service))
			assert.Equal(t, tc.want[1], CodeInvalidField(tc.service))
			assert.Equal(t, tc.want[2], CodeMissingMandatory(tc.service))
			assert.Equal(t, tc.want[3], CodeUnauthorized(tc.service))
			assert.Equal(t, tc.want[4], CodeInvalidToken(tc.service))
			assert.Equal(t, tc.want[5], CodeConflict(tc.service))
			assert.Equal(t, tc.want[6], CodeInternalError(tc.service))
		})
	}
}

func TestBusinessOutcomeCodes(t *testing.T) {
	// Inquiry (service 24)
	assert.Equal(t, "2002400", CodeInquirySuccess)
	assert.Equal(t, "4042412", CodeInquiryNotFound)
	assert.Equal(t, "4042414", CodeInquiryPaidBill)
	assert.Equal(t, "4042419", CodeInquiryExpired)
	assert.Equal(t, "4092400", CodeInquiryConflict)

	// Payment (service 25)
	assert.Equal(t, "2002500", CodePaymentSuccess)
	assert.Equal(t, "4042512", CodePaymentNotFound)
	assert.Equal(t, "4042513", CodePaymentInvalidAmt)
	assert.Equal(t, "4042514", CodePaymentPaidBill)
	assert.Equal(t, "4042518", CodePaymentInconsistent)
	assert.Equal(t, "4042519", CodePaymentExpired)
	assert.Equal(t, "4092500", CodePaymentConflict)

	// Status (service 26)
	assert.Equal(t, "2002600", CodeStatusSuccess)
	assert.Equal(t, "4042601", CodeStatusNotFound)
	assert.Equal(t, "5002601", CodeStatusInternalErr)
}

func TestReasonForCode_NeverEmpty(t *testing.T) {
	// BCA treats a response whose reason fields are empty as a failed
	// transaction, so even an unmapped code must render a reason.
	for _, code := range []string{
		CodeInquirySuccess, CodeInquiryNotFound, CodeInquiryPaidBill, CodeInquiryExpired,
		CodeInquiryConflict, CodePaymentSuccess, CodePaymentNotFound, CodePaymentInvalidAmt,
		CodePaymentPaidBill, CodePaymentExpired, CodePaymentConflict, CodeStatusNotFound,
		"9999999",
	} {
		reason := ReasonForCode(code)
		require.NotNil(t, reason, "code %s", code)
		assert.NotEmpty(t, reason.English, "code %s", code)
		assert.NotEmpty(t, reason.Indonesia, "code %s", code)
	}
}

func TestFlagStatusForCode(t *testing.T) {
	assert.Equal(t, InquiryStatusSuccess, FlagStatusForCode(CodeInquirySuccess))
	assert.Equal(t, InquiryStatusSuccess, FlagStatusForCode(CodePaymentSuccess))
	assert.Equal(t, InquiryStatusSuccess, FlagStatusForCode(CodeStatusSuccess))

	for _, code := range []string{
		CodeInquiryNotFound, CodeInquiryPaidBill, CodeInquiryExpired, CodeInquiryConflict,
		CodePaymentNotFound, CodePaymentInvalidAmt, CodePaymentPaidBill, CodePaymentExpired,
		CodePaymentConflict, CodeStatusNotFound, "4002501",
	} {
		assert.Equal(t, InquiryStatusFailed, FlagStatusForCode(code), "code %s", code)
	}
}

func TestPaymentFlagStatusValues(t *testing.T) {
	// The payment service publishes only 00/01/02. "03 = Pending" exists on
	// the inquiry-status service and must never appear in a payment response.
	assert.Equal(t, "00", PaymentFlagSuccess)
	assert.Equal(t, "01", PaymentFlagReject)
	assert.Equal(t, "02", PaymentFlagTimeout)
	assert.Equal(t, "03", PaymentFlagPending)
}

func TestErrorEnvelopes_CarryStatusAndReason(t *testing.T) {
	echoData := VAIdentityEcho{
		PartnerServiceID: "   12345",
		CustomerNo:       "123456789012345678",
		VirtualAccountNo: "   12345123456789012345678",
		InquiryRequestID: "INQ-1",
		PaymentRequestID: "PAY-1",
	}

	inquiry := NewInquiryErrorResponse(CodeInquiryNotFound, "Invalid Bill/Virtual Account [Not Found]", echoData)
	require.NotNil(t, inquiry.VirtualAccountData)
	assert.Equal(t, InquiryStatusFailed, inquiry.VirtualAccountData.InquiryStatus)
	assert.Equal(t, "Virtual Account Not Found", inquiry.VirtualAccountData.InquiryReason.English)
	assert.Equal(t, echoData.PartnerServiceID, inquiry.VirtualAccountData.PartnerServiceID)
	assert.Equal(t, "INQ-1", inquiry.VirtualAccountData.InquiryRequestID)

	payment := NewPaymentErrorResponse(CodePaymentPaidBill, "Paid Bill", echoData)
	require.NotNil(t, payment.VirtualAccountData)
	assert.Equal(t, PaymentFlagReject, payment.VirtualAccountData.PaymentFlagStatus)
	assert.Equal(t, "Bill has been paid", payment.VirtualAccountData.PaymentFlagReason.English)
	assert.Equal(t, "PAY-1", payment.VirtualAccountData.PaymentRequestID)

	status := NewStatusErrorResponse(CodeStatusNotFound, "Transaction Not Found", echoData)
	require.NotNil(t, status.VirtualAccountData)
	assert.Equal(t, "Transaction Not Found", status.VirtualAccountData.PaymentFlagReason.English)
}

func TestNewSNAPErrorBody_PicksShapeFromService(t *testing.T) {
	assert.IsType(t, VAInquiryResponse{},
		NewSNAPErrorBody(ServiceCodeInquiry, CodeInquiryConflict, "Conflict", VAIdentityEcho{}))
	assert.IsType(t, VAPaymentResponse{},
		NewSNAPErrorBody(ServiceCodePayment, CodePaymentConflict, "Conflict", VAIdentityEcho{}))
	assert.IsType(t, VAStatusResponse{},
		NewSNAPErrorBody(ServiceCodeStatus, "4092600", "Conflict", VAIdentityEcho{}))
	// Outside the transfer-va services there is no virtualAccountData to send.
	assert.IsType(t, SNAPErrorResponse{},
		NewSNAPErrorBody(ServiceCodeToken, "4007300", "Bad Request", VAIdentityEcho{}))
}

func TestBillingForVAType(t *testing.T) {
	// Mirrors the seeded master_va_type rows.
	assert.Equal(t, VATypeBillingNone, BillingForVAType("01"))
	assert.Equal(t, VATypeBillingNone, BillingForVAType("04"))
	assert.Equal(t, VATypeBillingVariable, BillingForVAType("02"))
	assert.Equal(t, VATypeBillingVariable, BillingForVAType("05"))
	assert.Equal(t, VATypeBillingFixed, BillingForVAType("03"))
	assert.Equal(t, VATypeBillingFixed, BillingForVAType("06"))
	// Unknown/empty predates the classification: left unclassified rather
	// than assumed fixed, which would impose an exact-amount check the VA was
	// never issued under.
	assert.Equal(t, VATypeBilling(""), BillingForVAType(""))
	assert.Equal(t, VATypeBilling(""), BillingForVAType("99"))
}

// BCA parameterises 400xx01 / 400xx02 by field name — Appendix A spells them
// "Invalid Field Format {field name}" / "Invalid Mandatory Field {field name}"
// — so the reason must carry that name rather than collapse to the generic
// rejection, and its Indonesian half must be a translation of the same text.
func TestReasonForCodeMessage_FieldViolationsMirrorTheMessage(t *testing.T) {
	cases := []struct {
		code, message  string
		wantEN, wantID string
	}{
		{CodeInvalidField(ServiceCodeInquiry), "Invalid Field Format [customerNo]",
			"Invalid Field Format [customerNo]", "Format Field Tidak Valid [customerNo]"},
		{CodeMissingMandatory(ServiceCodeInquiry), "Invalid Mandatory Field [inquiryRequestId]",
			"Invalid Mandatory Field [inquiryRequestId]", "Field Wajib Tidak Valid [inquiryRequestId]"},
		{CodeInvalidField(ServiceCodePayment), "Invalid Field Format [billDetails.billAmount.currency]",
			"Invalid Field Format [billDetails.billAmount.currency]", "Format Field Tidak Valid [billDetails.billAmount.currency]"},
		{CodeMissingMandatory(ServiceCodePayment), "Invalid Mandatory Field [virtualAccountName]",
			"Invalid Mandatory Field [virtualAccountName]", "Field Wajib Tidak Valid [virtualAccountName]"},
		{CodeMissingMandatory(ServiceCodeStatus), "Invalid Mandatory Field [X-EXTERNAL-ID]",
			"Invalid Mandatory Field [X-EXTERNAL-ID]", "Field Wajib Tidak Valid [X-EXTERNAL-ID]"},
	}
	for _, c := range cases {
		got := ReasonForCodeMessage(c.code, c.message)
		assert.Equal(t, c.wantEN, got.English, c.code)
		assert.Equal(t, c.wantID, got.Indonesia, c.code)
	}
}

// Every other code keeps the code-keyed table: their message is fixed, and the
// table's wording is what the contracts and BCA's samples pin.
func TestReasonForCodeMessage_NonFieldCodesKeepTheTable(t *testing.T) {
	cases := []struct{ code, message string }{
		{CodeInquiryNotFound, "Invalid Bill/Virtual Account [Not Found]"},
		{CodeInquiryPaidBill, "Paid Bill"},
		{CodeInquiryExpired, "Invalid Bill/Virtual Account"},
		{CodeInquiryConflict, "Conflict"},
		{CodeInquiryBadRequest, "Bad Request"},
		{CodeInternalError(ServiceCodeInquiry), "Internal Server Error"},
		{CodePaymentInconsistent, "Inconsistent Request"},
	}
	for _, c := range cases {
		assert.Equal(t, ReasonForCode(c.code), ReasonForCodeMessage(c.code, c.message), c.code)
	}
}

// A 400xx01/02 whose wording is not one of the two Appendix A templates has no
// translation available, so it falls back rather than emitting an English
// string in the Indonesian field.
func TestReasonForCodeMessage_UntranslatableFieldViolationFallsBack(t *testing.T) {
	got := ReasonForCodeMessage(CodeInvalidField(ServiceCodeInquiry), "Something Else Entirely")
	assert.Equal(t, ReasonForCode(CodeInvalidField(ServiceCodeInquiry)), got)
}

// BCA types both reason halves String(64) Max. The longest field name the
// validators can name must still fit, in both languages.
func TestReasonForCodeMessage_StaysWithinBCAsSixtyFourCharLimit(t *testing.T) {
	const longestField = "billDetails.billAmount.currency"
	for _, c := range []struct{ code, message string }{
		{CodeInvalidField(ServiceCodePayment), "Invalid Field Format [" + longestField + "]"},
		{CodeMissingMandatory(ServiceCodePayment), "Invalid Mandatory Field [" + longestField + "]"},
	} {
		got := ReasonForCodeMessage(c.code, c.message)
		assert.LessOrEqual(t, len(got.English), maxReasonLength, c.message)
		assert.LessOrEqual(t, len(got.Indonesia), maxReasonLength, c.message)
		assert.Contains(t, got.Indonesia, longestField, "the field name must survive translation")
	}
}

// An over-length message would put an out-of-contract reason on the wire; the
// generic one is still valid, a truncated field name is not.
func TestReasonForCodeMessage_OverLongMessageFallsBack(t *testing.T) {
	code := CodeInvalidField(ServiceCodeInquiry)
	got := ReasonForCodeMessage(code, "Invalid Field Format ["+strings.Repeat("x", maxReasonLength)+"]")
	assert.Equal(t, ReasonForCode(code), got)
}

func TestIsFieldViolationCode(t *testing.T) {
	for _, code := range []string{"4002401", "4002402", "4002501", "4002502", "4002601", "4002602"} {
		assert.True(t, isFieldViolationCode(code), code)
	}
	for _, code := range []string{"4002400", "2002400", "4042412", "4092400", "5002400", "4012400", "", "400240"} {
		assert.False(t, isFieldViolationCode(code), code)
	}
}

// The three 409xx00 codes describe one condition — a reused X-EXTERNAL-ID —
// and all three endpoints sit behind the same idempotency middleware, so all
// three must answer with the same reason. 4092600 was the one missing from the
// table, and fell through to the generic "Rejected" that says nothing about
// what the vendor did wrong.
func TestReasonForCode_ConflictIsCoveredOnEveryService(t *testing.T) {
	want := &BilingualText{
		English:   "Cannot use the same X-EXTERNAL-ID",
		Indonesia: "Tidak bisa menggunakan X-EXTERNAL-ID yang sama",
	}
	generic := &BilingualText{English: "Rejected", Indonesia: "Ditolak"}

	for _, service := range []string{ServiceCodeInquiry, ServiceCodePayment, ServiceCodeStatus} {
		code := CodeConflict(service)
		got := ReasonForCode(code)
		assert.Equal(t, want, got, code)
		assert.NotEqual(t, generic, got, "%s must not fall back to the generic reason", code)
		assert.Equal(t, InquiryStatusFailed, FlagStatusForCode(code), code)
	}
}

// Transcribed from the Appendix A / response-code tables of the three BCA
// tech docs.
func TestConflictCodesMatchBCAsTables(t *testing.T) {
	assert.Equal(t, CodeInquiryConflict, CodeConflict(ServiceCodeInquiry))
	assert.Equal(t, CodePaymentConflict, CodeConflict(ServiceCodePayment))
	assert.Equal(t, CodeStatusConflict, CodeConflict(ServiceCodeStatus))
	assert.Equal(t, "4092400", CodeInquiryConflict)
	assert.Equal(t, "4092500", CodePaymentConflict)
	assert.Equal(t, "4092600", CodeStatusConflict)
}

// 409xx00 carries a fixed message, so it stays on the code-keyed table rather
// than mirroring responseMessage the way the 400xx01/02 codes do.
func TestReasonForCodeMessage_ConflictKeepsTheTable(t *testing.T) {
	for _, service := range []string{ServiceCodeInquiry, ServiceCodePayment, ServiceCodeStatus} {
		code := CodeConflict(service)
		assert.Equal(t, ReasonForCode(code), ReasonForCodeMessage(code, "Conflict"), code)
	}
}
