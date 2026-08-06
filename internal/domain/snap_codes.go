package domain

import "strings"

// SNAP response codes follow BCA's AAABBCC format: AAA = HTTP status, BB =
// service code, CC = case code (Developer API BCA, "Virtual Account untuk
// Biller", Appendix A of each service). Every code this service emits on a
// transfer-va endpoint MUST carry that endpoint's own service code — BCA
// rejects a response whose code it does not recognise for the service it
// called, so a generic "4010000" is not merely untidy, it fails the
// transaction.
const (
	ServiceCodeInquiry = "24"
	ServiceCodePayment = "25"
	ServiceCodeStatus  = "26"
	ServiceCodeToken   = "73"
)

// Case codes shared by all three transfer-va services. Names mirror the
// Scenario column of BCA's Appendix A tables.
const (
	caseSuccess          = "00" // 200
	caseBadRequest       = "00" // 400 — general request/response parsing error
	caseInvalidFormat    = "01" // 400 — Invalid Field Format {field}
	caseMissingMandatory = "02" // 400 — Invalid Mandatory Field {field}
	caseUnauthorized     = "00" // 401 — Unauthorized. [reason]
	caseInvalidToken     = "01" // 401 — Invalid Token (B2B)
	caseConflict         = "00" // 409 — Cannot use the same X-EXTERNAL-ID
	caseInternalError    = "00" // 500
	caseTimeout          = "00" // 504
)

// ContextKeyVendor names the request-context entry where the SNAP auth
// middleware records which vendor authenticated the request. Declared here so
// the middleware that writes it and the handler that reads it share the key
// without either package depending on the other.
const ContextKeyVendor = "snap.vendor"

// VendorContext is the per-request vendor contract a handler needs. It carries
// only what varies per vendor at request time, so the delivery layer does not
// have to reach into infrastructure config.
type VendorContext struct {
	Vendor  string
	Channel string
	// StrictMandatoryFields enables the payment fields BCA marks Mandatory
	// that the wider SNAP standard leaves optional.
	StrictMandatoryFields bool
}

// SNAPCode assembles an AAABBCC response code from its three parts.
func SNAPCode(httpCode, serviceCode, caseCode string) string {
	return httpCode + serviceCode + caseCode
}

// ServiceCodeForPath resolves the SNAP service code from the request path, so
// middleware that runs before any handler still answers with the service code
// of the endpoint actually being called. Falls back to the token service code
// ("73"), which is what BCA's generic header/auth errors use.
func ServiceCodeForPath(path string) string {
	switch {
	case strings.Contains(path, "transfer-va/inquiry"):
		return ServiceCodeInquiry
	case strings.Contains(path, "transfer-va/payment"):
		return ServiceCodePayment
	case strings.Contains(path, "transfer-va/status"):
		return ServiceCodeStatus
	default:
		return ServiceCodeToken
	}
}

// Per-service code helpers. These exist so callers name the *condition* rather
// than paste a literal, which is how the service code stays correct when the
// same condition is reachable from more than one endpoint.

func CodeSuccess(service string) string    { return SNAPCode("200", service, caseSuccess) }
func CodeBadRequest(service string) string { return SNAPCode("400", service, caseBadRequest) }
func CodeInvalidField(service string) string {
	return SNAPCode("400", service, caseInvalidFormat)
}
func CodeMissingMandatory(service string) string {
	return SNAPCode("400", service, caseMissingMandatory)
}
func CodeUnauthorized(service string) string { return SNAPCode("401", service, caseUnauthorized) }
func CodeInvalidToken(service string) string { return SNAPCode("401", service, caseInvalidToken) }
func CodeConflict(service string) string     { return SNAPCode("409", service, caseConflict) }
func CodeInternalError(service string) string {
	return SNAPCode("500", service, caseInternalError)
}
func CodeTimeout(service string) string { return SNAPCode("504", service, caseTimeout) }

// Business-outcome codes. The case codes here are service-specific rather than
// shared (12/13/14/18/19 exist only on inquiry and payment, and differ between
// them), so they are spelled out per BCA's tables instead of derived.
const (
	// Inquiry (service 24)
	CodeInquirySuccess    = "2002400"
	CodeInquiryNotFound   = "4042412"
	CodeInquiryPaidBill   = "4042414"
	CodeInquiryExpired    = "4042419"
	CodeInquiryConflict   = "4092400"
	CodeInquiryBadRequest = "4002400"

	// Payment (service 25)
	CodePaymentSuccess      = "2002500"
	CodePaymentNotFound     = "4042512"
	CodePaymentInvalidAmt   = "4042513"
	CodePaymentPaidBill     = "4042514"
	CodePaymentInconsistent = "4042518"
	CodePaymentExpired      = "4042519"
	CodePaymentConflict     = "4092500"
	CodePaymentBadRequest   = "4002500"

	// Status (service 26)
	CodeStatusSuccess     = "2002600"
	CodeStatusNotFound    = "4042601"
	CodeStatusBadRequest  = "4002600"
	CodeStatusInternalErr = "5002601"
)

// SNAP transaction-status values.
//
// inquiryStatus (inquiry) and paymentFlagStatus (payment) are what BCA
// actually reads to decide the outcome — the responseCode alone does not
// determine it ("The final status of inquiry response is not determined by
// responseCode and responseMessage").
const (
	InquiryStatusSuccess = "00"
	InquiryStatusFailed  = "01"

	// PaymentFlagStatus values valid on the PAYMENT endpoint. BCA states
	// explicitly: "Payment flag status other than 00,01,02 will be considered
	// as 01" — so anything else here is read as a rejection.
	PaymentFlagSuccess = "00"
	PaymentFlagReject  = "01"
	PaymentFlagTimeout = "02"

	// PaymentFlagPending is valid ONLY on the inquiry-status endpoint
	// (service 26), where BCA documents "03 = Pending between BCA and the
	// partner". It must never appear in a payment (service 25) response.
	PaymentFlagPending = "03"
)

// snapReason maps a responseCode to the bilingual reason text BCA expects in
// inquiryReason / paymentFlagReason. BCA reads these fields to display the
// outcome on the channel screen, and treats an empty reason as a failed
// transaction, so every business code needs an entry here.
var snapReason = map[string]BilingualText{
	CodeInquirySuccess:  {English: "Success", Indonesia: "Sukses"},
	CodePaymentSuccess:  {English: "Success", Indonesia: "Sukses"},
	CodeStatusSuccess:   {English: "Success", Indonesia: "Sukses"},
	CodeInquiryNotFound: {English: "Virtual Account Not Found", Indonesia: "Virtual Account Tidak Ditemukan"},
	CodePaymentNotFound: {English: "Virtual Account Not Found", Indonesia: "Virtual Account Tidak Ditemukan"},
	CodeInquiryPaidBill: {English: "Bill has been paid", Indonesia: "Tagihan telah dibayar"},
	CodePaymentPaidBill: {English: "Bill has been paid", Indonesia: "Tagihan telah dibayar"},
	// The expired wording is contract-fixed by
	// specs/007-merchant-expiry-callback/contracts/{inquiry,notify}-expired.md
	// — do not reword it without updating those contracts.
	CodeInquiryExpired: {English: "expired transaction", Indonesia: "transaksi kadaluarsa"},
	CodePaymentExpired: {English: "expired transaction", Indonesia: "transaksi kadaluarsa"},
	CodePaymentInvalidAmt: {
		English:   "Invalid Amount",
		Indonesia: "Nominal pembayaran tidak sesuai",
	},
	CodeInquiryConflict: {
		English:   "Cannot use the same X-EXTERNAL-ID",
		Indonesia: "Tidak bisa menggunakan X-EXTERNAL-ID yang sama",
	},
	CodePaymentConflict: {
		English:   "Cannot use the same X-EXTERNAL-ID",
		Indonesia: "Tidak bisa menggunakan X-EXTERNAL-ID yang sama",
	},
	CodeStatusNotFound: {English: "Transaction Not Found", Indonesia: "Transaksi Tidak Ditemukan"},
}

// ReasonForCode returns the bilingual reason for a responseCode. Unknown codes
// fall back to a generic rejection rather than an empty object: BCA rejects a
// response whose reason fields are empty, so "no entry" must never render as
// "no reason".
func ReasonForCode(code string) *BilingualText {
	if r, ok := snapReason[code]; ok {
		reason := r
		return &reason
	}
	return &BilingualText{English: "Rejected", Indonesia: "Ditolak"}
}

// FlagStatusForCode reports the inquiryStatus / paymentFlagStatus that
// accompanies a responseCode. Only the success codes carry "00"; every other
// outcome BCA models as a rejected flag.
//
// CodePaymentInconsistent is the documented exception and is handled by its
// caller, not here: on a double-flag replay the partner must echo the status
// of the ORIGINAL request, which may well have been "00".
func FlagStatusForCode(code string) string {
	switch code {
	case CodeInquirySuccess, CodePaymentSuccess, CodeStatusSuccess:
		return InquiryStatusSuccess
	default:
		return InquiryStatusFailed
	}
}
