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
	CodeStatusSuccess    = "2002600"
	CodeStatusNotFound   = "4042601"
	CodeStatusBadRequest = "4002600"
	// CodeStatusConflict completes the 409xx00 set. The status endpoint sits
	// behind the same idempotency middleware as inquiry and payment
	// (cmd/api/main.go), so CodeConflict("26") is reachable there too, and
	// VA-Payment-Status V2 v1.0 lists it: 409 / 4092600 / "Conflict" /
	// "Cannot use same X-EXTERNAL-ID in the same day".
	CodeStatusConflict    = "4092600"
	CodeStatusInternalErr = "5002601"

	// Access token (service 73). These come from the Errors table in
	// "BCA API - OAuth & Signature OpenAPI" v1.1, which — unlike the
	// transfer-va services — gives the failing FIELD its own case code rather
	// than one code for the whole endpoint. Collapsing them tells the caller
	// only "something in your request was wrong".
	CodeTokenSuccess = "2007300"
	// CodeTokenInvalidField and CodeTokenInvalidTimestamp share 4007301: v1.1
	// lists "Invalid field format [clientId/clientSecret/grantType]" and
	// "Invalid field format [X-TIMESTAMP]" under the SAME code, which is
	// consistent with case code 01 meaning "invalid field format" throughout
	// SNAP. The earlier Developer API BCA doc had the first row at 4007300;
	// v1.1 drops 4007300 from the list entirely, so it is deliberately not
	// declared here — the message is what distinguishes the two.
	CodeTokenInvalidField     = "4007301"
	CodeTokenInvalidTimestamp = "4007301"
	// CodeTokenMissingClientKey is "Invalid mandatory field [X-CLIENT-KEY]".
	CodeTokenMissingClientKey = "4007302"
	// CodeTokenUnauthorized is "Unauthorized. [Signature]" / "[Unknown
	// client]" / "[Connection not allowed]". v1.1 moved the last of those from
	// HTTP 400 to 401, so every reason on this code is now a 401.
	CodeTokenUnauthorized  = "4017300"
	CodeTokenInternalError = "5007300"
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

// MsgInvalidAmount is the responseMessage every 4042513 carries. BCA's
// Appendix A spells the code "Invalid Amount"; the reason text in snapReason
// below is kept identical in English so responseMessage and
// paymentFlagReason.english never disagree about the same rejection.
const MsgInvalidAmount = "Invalid Amount"

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
		Indonesia: "Jumlah tidak valid",
	},
	CodeInquiryConflict: {
		English:   "Cannot use the same X-EXTERNAL-ID",
		Indonesia: "Tidak bisa menggunakan X-EXTERNAL-ID yang sama",
	},
	CodePaymentConflict: {
		English:   "Cannot use the same X-EXTERNAL-ID",
		Indonesia: "Tidak bisa menggunakan X-EXTERNAL-ID yang sama",
	},
	// Same wording as the other two: all three 409xx00 codes describe one
	// condition, and a vendor reading the reason should not have to notice
	// which endpoint it came back from.
	CodeStatusConflict: {
		English:   "Cannot use the same X-EXTERNAL-ID",
		Indonesia: "Tidak bisa menggunakan X-EXTERNAL-ID yang sama",
	},
	CodeStatusNotFound: {English: "Transaction Not Found", Indonesia: "Transaksi Tidak Ditemukan"},
}

// maxReasonLength is the tightest cap the three services put on their reason
// text, applied uniformly. The tables disagree: VA-BillPresentment v2.4 types
// inquiryReason.english/.indonesia String(64) Max, while VA-Payment-Flag v2.3
// and VA-Payment-Status V2 v1.0 allow String(200) for paymentFlagReason. 64
// satisfies all three, and the longest reason this service can build — the
// deepest field name, "billDetails.billAmount.currency" — comes to 58, so the
// cap is a backstop rather than something a real rejection meets.
//
// A reason built from a message that would exceed it is abandoned in favour of
// the generic one: an over-length reason is out of contract, and truncating
// would cut the field name mid-word, which is the one part the vendor needs
// intact.
const maxReasonLength = 64

// fieldViolationReasons translates the two field-level rejection messages BCA
// defines in Appendix A ("Invalid Field Format {field name}" and "Invalid
// Mandatory Field {field name}") into Indonesian. Only the fixed prefix is
// translated — what follows is the offending field's JSON name, which is an
// identifier and stays verbatim in both languages.
var fieldViolationReasons = []struct{ english, indonesia string }{
	{"Invalid Field Format", "Format Field Tidak Valid"},
	{"Invalid Mandatory Field", "Field Wajib Tidak Valid"},
}

// ReasonForCodeMessage returns the bilingual reason for a rejection, using the
// responseMessage when the code is one BCA parameterises by field name.
//
// The 400xx01 / 400xx02 codes are the only ones whose message is not fixed:
// Appendix A spells them "Invalid Field Format {field name}" and "Invalid
// Mandatory Field {field name}", so the field name — the single piece of
// information that tells the vendor WHAT to fix — lives in the message alone.
// Routing those through the code-keyed table collapsed every one of them to
// "Rejected"/"Ditolak" and threw that field name away, while BCA's own sample
// rejection mirrors the message into the reason. Every other code has exactly
// one message, so the table remains the right source for them.
func ReasonForCodeMessage(code, message string) *BilingualText {
	if !isFieldViolationCode(code) {
		return ReasonForCode(code)
	}
	for _, t := range fieldViolationReasons {
		rest, ok := strings.CutPrefix(message, t.english)
		if !ok {
			continue
		}
		reason := BilingualText{English: message, Indonesia: t.indonesia + rest}
		if len(reason.English) > maxReasonLength || len(reason.Indonesia) > maxReasonLength {
			return ReasonForCode(code)
		}
		return &reason
	}
	// A 400xx01/02 carrying some other wording has no translation to offer;
	// the generic reason is still a valid one, and a half-translated pair
	// would not be.
	return ReasonForCode(code)
}

// isFieldViolationCode reports whether code is one of BCA's field-level
// rejections — 400xx01 (invalid format) or 400xx02 (missing mandatory) — for
// any service code xx.
func isFieldViolationCode(code string) bool {
	if len(code) != 7 || !strings.HasPrefix(code, "400") {
		return false
	}
	switch code[5:] {
	case caseInvalidFormat, caseMissingMandatory:
		return true
	default:
		return false
	}
}

// ReasonForCode returns the bilingual reason for a responseCode. Unknown codes
// fall back to a generic rejection rather than an empty object: BCA rejects a
// response whose reason fields are empty, so "no entry" must never render as
// "no reason".
//
// Prefer ReasonForCodeMessage at any call site that has the responseMessage to
// hand: it answers identically here and additionally carries the field name on
// the 400xx01/400xx02 codes.
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
