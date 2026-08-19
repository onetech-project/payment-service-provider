package e2e

import (
	"net/http"
	"testing"
	"time"

	"backbone-new/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end happy paths for every VA transaction type this gateway supports:
// fixed bill, variable bill (instalments) and no-bill (register once, pay
// many). Each walks inquiry → payment → status the way BCA does.

func seedFixedBill(s *server, partnerServiceID, customerNo, amount string) *domain.VAInquiryRecord {
	rec := &domain.VAInquiryRecord{
		ID:               "txn-fixed-" + customerNo,
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		CustomerName:     "Budi Manjo",
		VirtualAccountNo: partnerServiceID + customerNo,
		TrxID:            "trx-" + customerNo,
		NotificationURL:  "https://merchant.example/callback",
		Status:           "03",
		TotalAmount:      amount,
		Currency:         "IDR",
		VAType:           "06",
		SubCompany:       "00000",
	}
	s.repo.putTransaction(rec)
	return rec
}

func seedVariableBill(s *server, partnerServiceID, customerNo, amount string) *domain.VAInquiryRecord {
	rec := &domain.VAInquiryRecord{
		ID:               "txn-var-" + customerNo,
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		CustomerName:     "Budi Variable",
		VirtualAccountNo: partnerServiceID + customerNo,
		TrxID:            "trx-" + customerNo,
		NotificationURL:  "https://merchant.example/callback",
		Status:           "03",
		TotalAmount:      amount,
		Currency:         "IDR",
		VAType:           "05",
	}
	s.repo.putTransaction(rec)
	return rec
}

func seedNoBillAccount(s *server, partnerServiceID, customerNo string) *domain.VAAccount {
	acc := &domain.VAAccount{
		ID:               "acc-" + customerNo,
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		VirtualAccountNo: partnerServiceID + customerNo,
		VAType:           "04",
		Billing:          domain.VATypeBillingNone,
		CustomerName:     "Budi NoBill",
		TrxID:            "trx-" + customerNo,
		NotificationURL:  "https://merchant.example/callback",
		Status:           domain.VAAccountStatusActive,
	}
	s.repo.putAccount(acc)
	return acc
}

// --- fixed bill ---------------------------------------------------------

func TestE2E_FixedBill_InquiryPaymentStatus(t *testing.T) {
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890123", "250000.00")

	// Inquiry — note the payload carries no `amount` field, exactly as BCA
	// sends it.
	inq := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890123", "INQ-FIXED-1"))

	require.Equal(t, http.StatusOK, inq.status, inq.raw)
	assert.Equal(t, domain.CodeInquirySuccess, inq.code())
	inqData := inq.vaData(t)
	assert.Equal(t, domain.InquiryStatusSuccess, inqData["inquiryStatus"])
	assert.Equal(t, "Budi Manjo", inqData["virtualAccountName"])
	assert.Equal(t, "INQ-FIXED-1", inqData["inquiryRequestId"])
	assert.Equal(t, "250000.00", inqData["totalAmount"].(map[string]any)["value"])
	require.NotNil(t, inqData["inquiryReason"], "BCA rejects a response with an empty inquiryReason")

	// Payment for the exact bill amount.
	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890123", "PAY-FIXED-1", "250000.00"))

	require.Equal(t, http.StatusOK, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentSuccess, pay.code())
	payData := pay.vaData(t)
	assert.Equal(t, domain.PaymentFlagSuccess, payData["paymentFlagStatus"])
	assert.Equal(t, "Success", payData["paymentFlagReason"].(map[string]any)["english"])
	assert.Equal(t, "250000.00", payData["paidAmount"].(map[string]any)["value"])
	assert.Equal(t, 1, s.notifier.count(), "merchant callback must be enqueued")

	// Status reports the settled payment.
	st := s.call(t, statusPath, statusPayload(testPartnerServiceID, "678901234567890123", "PAY-FIXED-1"))

	require.Equal(t, http.StatusOK, st.status, st.raw)
	assert.Equal(t, domain.CodeStatusSuccess, st.code())
	assert.Equal(t, domain.PaymentFlagSuccess, st.vaData(t)["paymentFlagStatus"])
}

func TestE2E_FixedBill_AmountComparedAgainstStoredBill(t *testing.T) {
	// The bill is the source of truth. A payment that agrees with itself but
	// not with the bill must be rejected 4042513 — this is the check that was
	// entirely absent.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890124", "250000.00")

	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890124", "PAY-UNDER-1", "1.00"))

	require.Equal(t, http.StatusNotFound, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentInvalidAmt, pay.code())
	assert.Equal(t, domain.PaymentFlagReject, pay.vaData(t)["paymentFlagStatus"])
	assert.Equal(t, 0, s.notifier.count(), "a rejected payment must not notify the merchant")
}

func TestE2E_FixedBill_TrailingZerosAccepted(t *testing.T) {
	// BCA sends "250000" and "250000.00" interchangeably.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890125", "250000.00")

	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890125", "PAY-NUM-1", "250000"))

	require.Equal(t, http.StatusOK, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentSuccess, pay.code())
}

func TestE2E_FixedBill_PaidBillRejectedOnSecondPayment(t *testing.T) {
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890126", "250000.00")

	first := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890126", "PAY-FIRST", "250000.00"))
	require.Equal(t, http.StatusOK, first.status, first.raw)

	// A brand-new paymentRequestId against a settled bill: BCA's 4042514
	// "Paid Bill", not a Conflict.
	second := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890126", "PAY-SECOND", "250000.00"))

	require.Equal(t, http.StatusNotFound, second.status, second.raw)
	assert.Equal(t, domain.CodePaymentPaidBill, second.code())
	assert.Equal(t, "Paid Bill", second.body["responseMessage"])
	assert.Equal(t, domain.PaymentFlagReject, second.vaData(t)["paymentFlagStatus"])
}

func TestE2E_FixedBill_InquiryAfterPaymentReportsPaidBill(t *testing.T) {
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890127", "250000.00")

	require.Equal(t, http.StatusOK,
		s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890127", "PAY-PAID-1", "250000.00")).status)

	inq := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890127", "INQ-PAID-1"))

	require.Equal(t, http.StatusNotFound, inq.status, inq.raw)
	assert.Equal(t, domain.CodeInquiryPaidBill, inq.code())
	assert.Equal(t, domain.InquiryStatusFailed, inq.vaData(t)["inquiryStatus"])
	assert.Equal(t, "Bill has been paid", inq.vaData(t)["inquiryReason"].(map[string]any)["english"])
}

// --- variable bill ------------------------------------------------------

func TestE2E_VariableBill_InstalmentsFlagSuccessNotPending(t *testing.T) {
	// The regression: an accepted instalment used to report paymentFlagStatus
	// "03", which the payment service does not publish — BCA reads anything
	// outside 00/01/02 as 01 (rejected), so money that HAD been recorded came
	// back to the channel as a failure.
	s := newServer(t)
	seedVariableBill(s, testPartnerServiceID, "678901234567890130", "100000.00")

	first := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890130", "PAY-VAR-1", "60000.00"))

	require.Equal(t, http.StatusOK, first.status, first.raw)
	assert.Equal(t, domain.CodePaymentSuccess, first.code())
	firstData := first.vaData(t)
	assert.Equal(t, domain.PaymentFlagSuccess, firstData["paymentFlagStatus"])
	assert.NotEqual(t, domain.PaymentFlagPending, firstData["paymentFlagStatus"],
		`"03" is valid only on the inquiry-status service`)
	assert.Equal(t, "60000.00", firstData["paidAmount"].(map[string]any)["value"])

	// Second instalment settles the bill.
	second := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890130", "PAY-VAR-2", "40000.00"))

	require.Equal(t, http.StatusOK, second.status, second.raw)
	assert.Equal(t, domain.PaymentFlagSuccess, second.vaData(t)["paymentFlagStatus"])
	assert.Equal(t, "100000.00", second.vaData(t)["paidAmount"].(map[string]any)["value"],
		"paidAmount reports the cumulative total")
	assert.Equal(t, 2, s.notifier.count())
}

func TestE2E_VariableBill_NoExactAmountCheck(t *testing.T) {
	// A partial payment is expected and valid on a variable bill, so the
	// exact-amount rule that guards fixed bills must not apply here.
	s := newServer(t)
	seedVariableBill(s, testPartnerServiceID, "678901234567890131", "100000.00")

	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890131", "PAY-VAR-PART", "1000.00"))

	require.Equal(t, http.StatusOK, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentSuccess, pay.code())
}

// --- no-bill ------------------------------------------------------------

func TestE2E_NoBill_PayableRepeatedly(t *testing.T) {
	// A no-bill VA is a durable payment address: it stays inquirable and
	// payable after the first payment.
	s := newServer(t)
	seedNoBillAccount(s, testPartnerServiceID, "678901234567890140")

	inq := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890140", "INQ-NOBILL-1"))
	require.Equal(t, http.StatusOK, inq.status, inq.raw)
	assert.Equal(t, domain.InquiryStatusSuccess, inq.vaData(t)["inquiryStatus"])
	assert.Equal(t, "Budi NoBill", inq.vaData(t)["virtualAccountName"])

	first := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890140", "PAY-NOBILL-1", "12000.00"))
	require.Equal(t, http.StatusOK, first.status, first.raw)
	assert.Equal(t, domain.PaymentFlagSuccess, first.vaData(t)["paymentFlagStatus"])

	// Second, unrelated top-up.
	second := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890140", "PAY-NOBILL-2", "35000.00"))
	require.Equal(t, http.StatusOK, second.status, second.raw)
	assert.Equal(t, "35000.00", second.vaData(t)["paidAmount"].(map[string]any)["value"])

	// Still inquirable afterwards.
	inqAgain := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890140", "INQ-NOBILL-2"))
	require.Equal(t, http.StatusOK, inqAgain.status, inqAgain.raw)
	assert.Equal(t, domain.InquiryStatusSuccess, inqAgain.vaData(t)["inquiryStatus"])
}

func TestE2E_NoBill_ExpiredRegistrationRejected(t *testing.T) {
	s := newServer(t)
	acc := seedNoBillAccount(s, testPartnerServiceID, "678901234567890141")
	past := time.Now().Add(-time.Hour)
	acc.ExpiredDate = &past

	inq := s.call(t, inquiryPath, inquiryPayload(testPartnerServiceID, "678901234567890141", "INQ-EXP-1"))
	require.Equal(t, http.StatusNotFound, inq.status, inq.raw)
	assert.Equal(t, domain.CodeInquiryExpired, inq.code())
	assert.Equal(t, domain.InquiryStatusFailed, inq.vaData(t)["inquiryStatus"])

	pay := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890141", "PAY-EXP-1", "12000.00"))
	require.Equal(t, http.StatusNotFound, pay.status, pay.raw)
	assert.Equal(t, domain.CodePaymentExpired, pay.code())
	assert.Equal(t, domain.PaymentFlagReject, pay.vaData(t)["paymentFlagStatus"])
}

// --- double flagging / advice ------------------------------------------

func TestE2E_DoubleFlagging_ReturnsInconsistentRequest(t *testing.T) {
	// "If a system error occurs and causes BCA to send a double flagging
	// request with the same X-EXTERNAL-ID and paymentRequestId, then the
	// partner can send responseCode 4042518 ... with paymentFlagStatus and
	// paymentFlagReason according to the results of the first request."
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890150", "250000.00")

	first := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890150", "PAY-DF-1", "250000.00"))
	require.Equal(t, http.StatusOK, first.status, first.raw)

	// Same paymentRequestId, fresh X-EXTERNAL-ID so the idempotency cache
	// does not answer first.
	repeat := s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890150", "PAY-DF-1", "250000.00"))

	assert.Equal(t, domain.CodePaymentInconsistent, repeat.code(), repeat.raw)
	assert.Equal(t, "Inconsistent Request", repeat.body["responseMessage"])
	// The flag status is the ORIGINAL request's, so BCA reads the transaction
	// as the success it was.
	assert.Equal(t, domain.PaymentFlagSuccess, repeat.vaData(t)["paymentFlagStatus"])
	assert.Equal(t, 1, s.notifier.count(), "the replay must not fire a second callback")
}

func TestE2E_AdviceRetry_ReplaysOriginalSuccess(t *testing.T) {
	// flagAdvise "Y" is a deliberate retry, not a double-flag: BCA wants the
	// original outcome back as 2002500.
	s := newServer(t)
	seedFixedBill(s, testPartnerServiceID, "678901234567890151", "250000.00")

	payload := paymentPayload(testPartnerServiceID, "678901234567890151", "PAY-ADV-1", "250000.00")
	require.Equal(t, http.StatusOK, s.call(t, paymentPath, payload).status)

	retry := paymentPayload(testPartnerServiceID, "678901234567890151", "PAY-ADV-1", "250000.00")
	retry["flagAdvise"] = "Y"
	resp := s.call(t, paymentPath, retry)

	require.Equal(t, http.StatusOK, resp.status, resp.raw)
	assert.Equal(t, domain.CodePaymentSuccess, resp.code())
	assert.Equal(t, domain.PaymentFlagSuccess, resp.vaData(t)["paymentFlagStatus"])
	assert.Equal(t, 1, s.notifier.count())
}

// --- idempotency --------------------------------------------------------

// A repeat of both the X-EXTERNAL-ID and the paymentRequestId is BCA's
// double flag, not a conflict: it reaches the usecase, which answers 4042518
// against the stored payment. The middleware must not shortcut it — neither
// with a replayed 2002500 (which would read as a second settlement) nor with a
// 409 (which BCA counts as a failed transaction).
func TestE2E_DoubleFlag_AnsweredInconsistentRequest(t *testing.T) {
	s := newServer(t)
	seedNoBillAccount(s, testPartnerServiceID, "678901234567890160")

	payload := paymentPayload(testPartnerServiceID, "678901234567890160", "PAY-IDEM-1", "12000.00")
	first := s.call(t, paymentPath, payload, withExternalID("900000000000002"))
	require.Equal(t, http.StatusOK, first.status, first.raw)
	require.Equal(t, domain.CodePaymentSuccess, first.code())

	second := s.call(t, paymentPath, payload, withExternalID("900000000000002"))

	assert.Equal(t, http.StatusNotFound, second.status, second.raw)
	assert.Equal(t, domain.CodePaymentInconsistent, second.code())
	assert.Equal(t, 1, s.notifier.count(), "the double flag must not re-run the payment")
}

// One X-EXTERNAL-ID, one request. On inquiry and status there is no double-flag
// carve-out, so even a byte-identical repeat is 409 — the header is typed
// "unique in the same day".
func TestE2E_SameExternalIDRepeated_IsConflictOnInquiryAndStatus(t *testing.T) {
	for _, tc := range []struct {
		name, path, extID, wantCode string
		payload                     func(string, string, string) map[string]any
	}{
		{"inquiry", inquiryPath, "900000000000010", domain.CodeInquiryConflict, inquiryPayload},
		{"status", statusPath, "900000000000011", domain.CodeStatusConflict, statusPayload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newServer(t)
			seedFixedBill(s, testPartnerServiceID, "678901234567890161", "12000.00")

			payload := tc.payload(testPartnerServiceID, "678901234567890161", "INQ-IDEM-1")

			// What the first call answered is beside the point — the key is
			// spent either way, as long as the request was decided at all (a
			// 5xx is deliberately not recorded).
			first := s.call(t, tc.path, payload, withExternalID(tc.extID))
			require.Less(t, first.status, 500, first.raw)

			second := s.call(t, tc.path, payload, withExternalID(tc.extID))

			assert.Equal(t, http.StatusConflict, second.status, second.raw)
			assert.Equal(t, tc.wantCode, second.code())
		})
	}
}

func TestE2E_VariableBill_RepeatedInstalmentNotCreditedTwice(t *testing.T) {
	// An instalment retried under the same paymentRequestId must not move the
	// cumulative total. Before paymentRequestId became the dedup key on this
	// path, the second insert credited the same money again — enough to mark a
	// bill settled that the customer had only half paid.
	s := newServer(t)
	seedVariableBill(s, testPartnerServiceID, "678901234567890132", "100000.00")

	payload := paymentPayload(testPartnerServiceID, "678901234567890132", "PAY-VAR-DUP", "60000.00")

	first := s.call(t, paymentPath, payload)
	require.Equal(t, http.StatusOK, first.status, first.raw)
	assert.Equal(t, "60000.00", first.vaData(t)["paidAmount"].(map[string]any)["value"])

	// Fresh X-EXTERNAL-ID so the idempotency cache does not answer first.
	repeat := s.call(t, paymentPath, payload)

	assert.Equal(t, domain.CodePaymentInconsistent, repeat.code(), repeat.raw)
	assert.Equal(t, "60000.00", repeat.vaData(t)["paidAmount"].(map[string]any)["value"],
		"the cumulative total must not move on a replay")
	assert.Equal(t, 1, s.notifier.count(), "a replay must not re-notify the merchant")
}

func TestE2E_VariableBill_AdviceRetryReplaysWithoutDoubleCrediting(t *testing.T) {
	s := newServer(t)
	seedVariableBill(s, testPartnerServiceID, "678901234567890133", "100000.00")

	payload := paymentPayload(testPartnerServiceID, "678901234567890133", "PAY-VAR-ADV", "60000.00")
	require.Equal(t, http.StatusOK, s.call(t, paymentPath, payload).status)

	retry := paymentPayload(testPartnerServiceID, "678901234567890133", "PAY-VAR-ADV", "60000.00")
	retry["flagAdvise"] = "Y"
	resp := s.call(t, paymentPath, retry)

	require.Equal(t, http.StatusOK, resp.status, resp.raw)
	assert.Equal(t, domain.CodePaymentSuccess, resp.code())
	assert.Equal(t, "60000.00", resp.vaData(t)["paidAmount"].(map[string]any)["value"])
	assert.Equal(t, 1, s.notifier.count())
}

func TestE2E_Status_ResolvesByPaymentRequestID(t *testing.T) {
	// A no-bill VA's inquiry persists nothing — it is a durable address, not
	// a transaction — so its payments are reachable only by their own
	// paymentRequestId. Resolving status by inquiryRequestId alone reported
	// every no-bill payment as Transaction Not Found.
	s := newServer(t)
	seedNoBillAccount(s, testPartnerServiceID, "678901234567890170")

	require.Equal(t, http.StatusOK,
		s.call(t, paymentPath, paymentPayload(testPartnerServiceID, "678901234567890170", "PAY-ST-1", "12000.00")).status)

	byPaymentID := s.call(t, statusPath, map[string]any{
		"partnerServiceId": testPartnerServiceID,
		"customerNo":       "678901234567890170",
		"virtualAccountNo": testPartnerServiceID + "678901234567890170",
		"inquiryRequestId": "INQ-NEVER-PERSISTED",
		"paymentRequestId": "PAY-ST-1",
	})

	require.Equal(t, http.StatusOK, byPaymentID.status, byPaymentID.raw)
	assert.Equal(t, domain.CodeStatusSuccess, byPaymentID.code())
	assert.Equal(t, domain.PaymentFlagSuccess, byPaymentID.vaData(t)["paymentFlagStatus"])
}

func TestE2E_Status_UnknownIDsStillNotFound(t *testing.T) {
	// The paymentRequestId fallback must not turn a genuinely unknown
	// transaction into a success.
	s := newServer(t)

	resp := s.call(t, statusPath, map[string]any{
		"partnerServiceId": testPartnerServiceID,
		"customerNo":       "678901234567890171",
		"virtualAccountNo": testPartnerServiceID + "678901234567890171",
		"inquiryRequestId": "INQ-UNKNOWN",
		"paymentRequestId": "PAY-UNKNOWN",
	})

	require.Equal(t, http.StatusNotFound, resp.status, resp.raw)
	assert.Equal(t, domain.CodeStatusNotFound, resp.code())
}
