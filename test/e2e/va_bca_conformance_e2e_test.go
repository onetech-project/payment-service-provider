package e2e

import (
	"net/http"
	"testing"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BCA: "paymentRequestId ... If payment comes from the Inquiry process, this
// value must be the same with inquiryRequestId."
//
// That is the canonical flow — inquire, then pay with the same id — and it was
// broken end to end. The inquiry stamps the id onto the transaction's
// inquiry_request_id; the payment's already-recorded check then resolved the
// id with a query matching EITHER column, found that very transaction, and
// concluded the payment had already been flagged. BCA received 4042518
// "Inconsistent Request" for a first-time payment, nothing was persisted, the
// transaction stayed pending and the merchant was never notified.
func TestE2E_PaymentReusingInquiryRequestID_IsAFirstPaymentNotADoubleFlag(t *testing.T) {
	s := newServer(t)

	const (
		customerNo      = "678901234567890900"
		sharedRequestID = "202202111031031234500900"
	)
	partnerServiceID := testPartnerServiceID
	seedFixedBill(s, partnerServiceID, customerNo, "150000.00")

	// 1. Inquiry claims sharedRequestID onto the transaction.
	inq := s.call(t, inquiryPath, inquiryPayload(partnerServiceID, customerNo, sharedRequestID))
	require.Equal(t, http.StatusOK, inq.status, inq.raw)
	require.Equal(t, domain.CodeInquirySuccess, inq.code())

	// 2. Payment reuses it as paymentRequestId, exactly as BCA specifies.
	pay := s.call(t, paymentPath,
		paymentPayload(partnerServiceID, customerNo, sharedRequestID, "150000.00"))

	assert.Equal(t, http.StatusOK, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentSuccess, pay.code(),
		"a first payment that reuses inquiryRequestId must be 2002500, not 4042518")
	assert.Equal(t, domain.PaymentFlagSuccess, pay.vaData(t)["paymentFlagStatus"])

	// The money must actually be recorded, and the merchant told about it.
	assert.Equal(t, 1, s.notifier.count(), "the merchant callback must fire for a first payment")

	// 3. A genuine repeat of that same paymentRequestId IS a double-flag.
	repeat := s.call(t, paymentPath,
		paymentPayload(partnerServiceID, customerNo, sharedRequestID, "150000.00"))
	assert.Equal(t, domain.CodePaymentInconsistent, repeat.code(),
		"the second flag of the same paymentRequestId is the real 4042518 case")
	assert.Equal(t, 1, s.notifier.count(), "a replay must not notify the merchant twice")
}
