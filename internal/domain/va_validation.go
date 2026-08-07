package domain

import (
	"regexp"
	"strings"
)

// SNAP request validation, transcribed from the field tables in the current
// BCA technical documentation: VA-BillPresentment v2.4 (service 24),
// VA-Payment-Flag v2.3 (25) and VA-Payment-Status V2 v1.0 (26).
//
// The three services once published DIFFERENT limits for the same fields —
// inquiry allowed customerNo(20)/virtualAccountNo(28)/inquiryRequestId(128)
// where payment and status allowed (18)/(26)/(30). v2.4 converged them all
// onto the narrower set, so the split below is no longer a spec difference:
// what remains of it is a deliberate receive-side allowance, documented at
// each constant. Getting this wrong is silent — an over-long value is accepted
// here and then truncated or rejected downstream at BCA.

// Field length limits per BCA's tables.
const (
	// partnerServiceId is documented as String(8) Fixed, "using space padding
	// on the left if it doesn't reach 8 characters". We enforce it as a MAX
	// rather than an exact width: BCA itself always sends the padded 8-char
	// form, so nothing is lost, while merchants that registered VAs under an
	// unpadded 5-character company code keep working.
	maxPartnerServiceID = 8

	// Inquiry (service 24).
	//
	// inquiryRequestId is String(30) Fixed as of v2.4, down from the 128 the
	// older documentation gave it. 30 is enforced: the two are not independent
	// — "If payment comes from the Inquiry process, paymentRequestId must be
	// the same with inquiryRequestId", and paymentRequestId is capped at 30 on
	// the payment service. Accepting a 40-character id at inquiry only defers
	// the rejection to the payment that follows it, by which point the
	// customer has already been shown a bill.
	maxInquiryCustomerNo       = 20
	maxInquiryVirtualAccountNo = 28
	maxInquiryRequestID        = 30
	// language is String(2) ISO-639-1 and passApp String(64); both Optional,
	// both inquiry-only (VA-BillPresentment v2.4).
	maxLanguage = 2
	maxPassApp  = 64

	// Payment (service 25) and status (service 26).
	//
	// Every current table — inquiry included, since v2.4 — says
	// customerNo(18)/virtualAccountNo(26). INBOUND we still accept (20)/(28),
	// purely for backward compatibility: the dynamic sequence used to emit
	// 20-digit customerNo values, and VA numbers already in customers' hands
	// from that era must stay payable. Rejecting them here would break VAs
	// this very system issued.
	//
	// This is a receive-side allowance only. Issuance is now 18/26 — see
	// NextCustomerNoSequence — so every NEW VA number fits BCA's limit. Being
	// more permissive than BCA on receive is safe (BCA never sends more than
	// it generated); being more permissive on issue was not, and that is the
	// half that was fixed.
	maxPaymentCustomerNo       = 20
	maxPaymentVirtualAccountNo = 28
	maxPaymentRequestID        = 30
	// MaxVirtualAccountName is String(30) Max on the payment REQUEST and on
	// the inquiry RESPONSE alike (VA-Payment-Flag v2.3, VA-BillPresentment
	// v2.4). Exported because create-va must enforce it too: a name accepted
	// there is echoed on every later inquiry, so the limit has to bite at
	// registration rather than at the channel.
	MaxVirtualAccountName    = 30
	maxVirtualAccountEmail   = 255
	maxVirtualAccountPhone   = 30
	maxTrxID                 = 64
	maxHashedSourceAccountNo = 32
	maxSourceBankCode        = 3
	maxPaidBills             = 6
	maxReferenceNo           = 11
	maxJournalNum            = 6
	maxPaymentType           = 1
	maxFlagAdvise            = 1
	maxSubCompany            = 5
	maxPaymentBillDetails    = 5
	maxFreeTexts             = 9

	// MaxInquiryBillDetails and MaxInquiryFreeTexts bound what the INQUIRY
	// response may carry. BCA's inquiry field table nominally allows 24
	// billDetails and 9 freeTexts, but its Notes override both downward:
	// "billDetails should not be greater than 5" and "The occurences for
	// freeTexts field in inquiry bill should not be greater than 5". Exceeding
	// either makes BCA treat the inquiry as failed, so these are enforced when
	// a merchant creates the VA and again when the response is built.
	MaxInquiryBillDetails = 5
	MaxInquiryFreeTexts   = 5

	// MaxFreeTextLength bounds each freeTexts entry's english/indonesia string.
	// Exported because create-va enforces it too: a freeText stored there is
	// echoed verbatim into every inquiry response for the VA, so an over-long
	// entry fails the inquiry at BCA rather than at the point it was accepted.
	MaxFreeTextLength = 32

	// MaxIssuedCustomerNo and MaxIssuedVirtualAccountNo bound what this
	// gateway may ISSUE, as opposed to what it will accept inbound.
	//
	// They are BCA's payment/status limits (18/26), deliberately the narrower
	// of the two sets it publishes — its inquiry table allows 20/28. A VA
	// number issued at the wider limit is inquirable and then unpayable: the
	// channel accepts it right up to the moment the customer tries to pay.
	// Issuing at the narrower limit satisfies all three services at once, so
	// there is no reason to ever hand out the wider form.
	MaxIssuedCustomerNo       = 18
	MaxIssuedVirtualAccountNo = 26
	// Amounts are String(13.2): at most 13 digits before the decimal point and
	// two after. BCA states an over-long amount is treated as a failed
	// transaction.
	maxAmountIntegerDigits = 13
)

// amountPattern accepts the ISO-4217 style amount BCA documents: digits, with
// an optional 1-2 digit fractional part. Length of the integer part is checked
// separately so the error can name the actual problem.
var amountPattern = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

// allowedCurrencies is BCA's documented set for Virtual Account ("currency
// field may vary with these values: IDR, SGD, USD").
var allowedCurrencies = map[string]bool{"IDR": true, "USD": true, "SGD": true}

// allowedPaymentChannelCodes is BCA's enumerated channelCode set on the
// payment service (ISO 18245). The inquiry service documents only the 6010 /
// 6011 subset, but the two are checked against the same table on purpose: a
// code outside this set is one BCA does not generate, and accepting it would
// persist an unmappable channel onto the transaction.
//
// Zero is not in the table and is not accepted here either — it is the Go
// zero value, i.e. "field absent", which the mandatory check reports as a
// missing field rather than a malformed one.
var allowedPaymentChannelCodes = map[int]bool{
	6000: true, // Others
	6010: true, // Teller
	6011: true, // ATM (eBanking on the inquiry table)
	6012: true, // EDC
	6013: true, // Autodebet
	6014: true, // Internet Banking
	6015: true, // Oneklik
	6016: true, // myBCA
	6017: true, // Mobile Banking
	6018: true, // Other Bank
	6019: true, // Cardless
	6020: true, // Shared Biller
}

// ViolationKind distinguishes the two 400-class outcomes BCA separates:
// a field that is absent (Invalid Mandatory Field, case code 02) from one that
// is present but malformed (Invalid Field Format, case code 01).
type ViolationKind int

const (
	ViolationMandatory ViolationKind = iota
	ViolationFormat
)

// FieldViolation names the single field that caused a request to be rejected.
// BCA's messages embed the field name ("Invalid Mandatory Field {field}"), so
// the name is carried rather than flattened into a generic message.
type FieldViolation struct {
	Kind  ViolationKind
	Field string
}

func mandatory(field string) *FieldViolation {
	return &FieldViolation{Kind: ViolationMandatory, Field: field}
}

func badFormat(field string) *FieldViolation {
	return &FieldViolation{Kind: ViolationFormat, Field: field}
}

// ValidateInquiryRequest checks an InquiryRequest against the service-24 field
// table in Tech. Doc. OpenAPI VA-BillPresentment v2.4.
//
// trxDateInit and channelCode are marked Mandatory (Y) there but are NOT
// required here. Both are BCA-generated and always present on a real BCA
// inquiry; requiring them would only reject other vendors that omit what BCA
// happens to send. Being more permissive than the spec on RECEIVE cannot
// produce a wrong answer — the reverse would.
func ValidateInquiryRequest(req *VAInquiryRequest, strictMandatory bool) *FieldViolation {
	if v := requireVAIdentity(req.PartnerServiceID, req.CustomerNo, req.VirtualAccountNo,
		maxInquiryCustomerNo, maxInquiryVirtualAccountNo); v != nil {
		return v
	}
	if strings.TrimSpace(req.InquiryRequestID) == "" {
		return mandatory("inquiryRequestId")
	}
	if len(req.InquiryRequestID) > maxInquiryRequestID {
		return badFormat("inquiryRequestId")
	}
	// A value that IS sent must be one BCA generates, whether or not the field
	// was obligatory.
	if req.ChannelCode != 0 && !allowedPaymentChannelCodes[req.ChannelCode] {
		return badFormat("channelCode")
	}
	// amount IS part of the inquiry payload as of v2.4 (Object, Mandatory N).
	// It is still never required — the customer-entered amount belongs to the
	// payment — but when sent it must be well-formed.
	if req.Amount != nil {
		if v := validateAmount(req.Amount, "amount"); v != nil {
			return v
		}
	}
	if strictMandatory {
		// trxDateInit and channelCode are both Mandatory (Y) on the v2.4
		// inquiry payload. They are gated behind the same per-vendor switch as
		// payment's extended mandatory set: BCA always sends them, but this
		// gateway fronts other vendors whose field tables do not, and one
		// vendor's contract must not be imposed on the rest.
		if req.TrxDateInit == nil {
			return mandatory("trxDateInit")
		}
		if req.ChannelCode == 0 {
			return mandatory("channelCode")
		}
	}
	// The remaining Optional fields of the v2.4 payload. They are inert here —
	// nothing in the bill this inquiry presents depends on them — but a value
	// that IS sent still has to fit, or it is accepted at inquiry and rejected
	// at the payment that quotes it back.
	for _, check := range []struct {
		field string
		value string
		max   int
	}{
		{"language", req.Language, maxLanguage},
		{"hashedSourceAccountNo", req.HashedSourceAccountNo, maxHashedSourceAccountNo},
		{"sourceBankCode", req.SourceBankCode, maxSourceBankCode},
		{"passApp", req.PassApp, maxPassApp},
	} {
		if len(check.value) > check.max {
			return badFormat(check.field)
		}
	}
	return nil
}

// ValidatePaymentRequest checks a PaymentRequest against BCA's service-25
// field table.
//
// strictMandatory controls the fields BCA marks Mandatory that the wider SNAP
// standard leaves optional (virtualAccountName, channelCode, totalAmount,
// trxDateTime, flagAdvise). It is on by default so BCA conformance is the
// out-of-the-box behaviour, and can be turned off per vendor for non-BCA
// integrations that legitimately omit them — this service fronts several
// vendors and must not force one vendor's field table onto the others.
//
// trxId is deliberately NOT mandatory in either mode: BCA marks it "N —
// Mandatory if payment comes from the Create VA Request", so requiring it
// rejects valid channel-originated payments.
func ValidatePaymentRequest(req *VAPaymentRequest, strictMandatory bool) *FieldViolation {
	if v := requireVAIdentity(req.PartnerServiceID, req.CustomerNo, req.VirtualAccountNo,
		maxPaymentCustomerNo, maxPaymentVirtualAccountNo); v != nil {
		return v
	}
	if strings.TrimSpace(req.PaymentRequestID) == "" {
		return mandatory("paymentRequestId")
	}
	if len(req.PaymentRequestID) > maxPaymentRequestID {
		return badFormat("paymentRequestId")
	}
	if req.PaidAmount == nil {
		return mandatory("paidAmount")
	}
	if v := validateAmount(req.PaidAmount, "paidAmount"); v != nil {
		return v
	}

	if v := validatePaymentOptionalLengths(req); v != nil {
		return v
	}
	if v := validatePaymentAmountsAndBills(req); v != nil {
		return v
	}
	if !strictMandatory {
		return nil
	}
	return validatePaymentStrictMandatory(req)
}

// validatePaymentStrictMandatory enforces the fields BCA marks Mandatory (Y)
// on top of the SNAP-common set.
func validatePaymentStrictMandatory(req *VAPaymentRequest) *FieldViolation {
	if strings.TrimSpace(req.VirtualAccountName) == "" {
		return mandatory("virtualAccountName")
	}
	if req.ChannelCode == 0 {
		return mandatory("channelCode")
	}
	if req.TotalAmount == nil {
		return mandatory("totalAmount")
	}
	if req.TrxDateTime == nil {
		return mandatory("trxDateTime")
	}
	if strings.TrimSpace(req.FlagAdvise) == "" {
		return mandatory("flagAdvise")
	}
	return nil
}

func validatePaymentOptionalLengths(req *VAPaymentRequest) *FieldViolation {
	for _, check := range []struct {
		field string
		value string
		max   int
	}{
		{"virtualAccountName", req.VirtualAccountName, MaxVirtualAccountName},
		{"virtualAccountEmail", req.VirtualAccountEmail, maxVirtualAccountEmail},
		{"virtualAccountPhone", req.VirtualAccountPhone, maxVirtualAccountPhone},
		{"trxId", req.TrxID, maxTrxID},
		{"hashedSourceAccountNo", req.HashedSourceAccountNo, maxHashedSourceAccountNo},
		{"sourceBankCode", req.SourceBankCode, maxSourceBankCode},
		{"paidBills", req.PaidBills, maxPaidBills},
		{"referenceNo", req.ReferenceNo, maxReferenceNo},
		{"journalNum", req.JournalNum, maxJournalNum},
		{"paymentType", req.PaymentType, maxPaymentType},
		{"subCompany", req.SubCompany, maxSubCompany},
	} {
		if len(check.value) > check.max {
			return badFormat(check.field)
		}
	}
	if len(req.FlagAdvise) > maxFlagAdvise {
		return badFormat("flagAdvise")
	}
	// BCA documents exactly two values: N = new request, Y = advice (retry).
	if req.FlagAdvise != "" && !strings.EqualFold(req.FlagAdvise, "N") && !strings.EqualFold(req.FlagAdvise, "Y") {
		return badFormat("flagAdvise")
	}
	// channelCode is Mandatory on payment and enumerated, not free-form: only
	// 0 (absent, reported as a missing field by the strict check) skips the
	// membership test.
	if req.ChannelCode != 0 && !allowedPaymentChannelCodes[req.ChannelCode] {
		return badFormat("channelCode")
	}
	return nil
}

// validateFreeTextLengths enforces the per-entry limit on freeTexts. Only the
// COUNT was checked before, so an over-long entry passed here and was then
// echoed into the inquiry/payment response, where BCA fails the whole
// transaction on it ("The length of the characters sent must not exceed the
// number stated in the technical documentation").
//
// 32 is the current figure (VA-BillPresentment v2.4, VA-Payment-Flag v2.3);
// the older documentation said 18, so this is a widening — nothing that used
// to pass starts failing.
func validateFreeTextLengths(texts []BilingualText) *FieldViolation {
	for _, t := range texts {
		if len(t.English) > MaxFreeTextLength || len(t.Indonesia) > MaxFreeTextLength {
			return badFormat("freeTexts")
		}
	}
	return nil
}

func validatePaymentAmountsAndBills(req *VAPaymentRequest) *FieldViolation {
	if req.TotalAmount != nil {
		if v := validateAmount(req.TotalAmount, "totalAmount"); v != nil {
			return v
		}
		// "currency must be the same for totalAmount, billAmount, and
		// paidAmount".
		if !strings.EqualFold(req.TotalAmount.Currency, req.PaidAmount.Currency) {
			return badFormat("totalAmount.currency")
		}
	}
	if req.CumulativePaymentAmount != nil {
		if v := validateAmount(req.CumulativePaymentAmount, "cumulativePaymentAmount"); v != nil {
			return v
		}
	}
	if len(req.BillDetails) > maxPaymentBillDetails {
		return badFormat("billDetails")
	}
	if len(req.FreeTexts) > maxFreeTexts {
		return badFormat("freeTexts")
	}
	if v := validateFreeTextLengths(req.FreeTexts); v != nil {
		return v
	}
	for _, bill := range req.BillDetails {
		if bill.BillAmount == nil {
			continue
		}
		if v := validateAmount(bill.BillAmount, "billDetails.billAmount"); v != nil {
			return v
		}
		if !strings.EqualFold(bill.BillAmount.Currency, req.PaidAmount.Currency) {
			return badFormat("billDetails.billAmount.currency")
		}
	}
	return nil
}

// ValidateStatusRequest checks a StatusRequest against BCA's service-26 field
// table, whose limits match payment's rather than inquiry's.
func ValidateStatusRequest(req *VAStatusRequest) *FieldViolation {
	if v := requireVAIdentity(req.PartnerServiceID, req.CustomerNo, req.VirtualAccountNo,
		maxPaymentCustomerNo, maxPaymentVirtualAccountNo); v != nil {
		return v
	}
	if strings.TrimSpace(req.InquiryRequestID) == "" {
		return mandatory("inquiryRequestId")
	}
	if len(req.InquiryRequestID) > maxPaymentRequestID {
		return badFormat("inquiryRequestId")
	}
	return nil
}

// requireVAIdentity validates the three-field VA identity every transfer-va
// service shares. maxCustomerNo/maxVirtualAccountNo are passed in because they
// differ between inquiry and payment/status.
func requireVAIdentity(partnerServiceID, customerNo, virtualAccountNo string, maxCustomerNo, maxVANo int) *FieldViolation {
	if strings.TrimSpace(partnerServiceID) == "" {
		return mandatory("partnerServiceId")
	}
	if len(partnerServiceID) > maxPartnerServiceID {
		return badFormat("partnerServiceId")
	}
	if strings.TrimSpace(customerNo) == "" {
		return mandatory("customerNo")
	}
	if len(customerNo) > maxCustomerNo {
		return badFormat("customerNo")
	}
	if strings.TrimSpace(virtualAccountNo) == "" {
		return mandatory("virtualAccountNo")
	}
	if len(virtualAccountNo) > maxVANo {
		return badFormat("virtualAccountNo")
	}
	// virtualAccountNo is documented as partnerServiceId + customerNo, so it
	// must agree with BOTH halves: a customerNo that is not its suffix, or a
	// partnerServiceId that is not its prefix, means the request disagrees with
	// itself about which VA this is. Checking only the suffix let a payment
	// name one biller in partnerServiceId and a different one inside the VA
	// number.
	//
	// Compared trimmed throughout because partnerServiceId carries the 8-char
	// left space padding that the concatenated VA number keeps.
	trimmedVANo := strings.TrimSpace(virtualAccountNo)
	if !strings.HasSuffix(trimmedVANo, strings.TrimSpace(customerNo)) {
		return badFormat("virtualAccountNo")
	}
	if !strings.HasPrefix(trimmedVANo, strings.TrimSpace(partnerServiceID)) {
		return badFormat("virtualAccountNo")
	}
	return nil
}

// validateAmount enforces the String(13.2) + ISO-4217 contract shared by every
// amount object in the VA services.
func validateAmount(amount *Amount, field string) *FieldViolation {
	if strings.TrimSpace(amount.Value) == "" {
		return mandatory(field + ".value")
	}
	if !amountPattern.MatchString(amount.Value) {
		return badFormat(field + ".value")
	}
	integerPart := amount.Value
	if idx := strings.Index(amount.Value, "."); idx >= 0 {
		integerPart = amount.Value[:idx]
	}
	if len(integerPart) > maxAmountIntegerDigits {
		return badFormat(field + ".value")
	}
	if strings.TrimSpace(amount.Currency) == "" {
		return mandatory(field + ".currency")
	}
	if !allowedCurrencies[strings.ToUpper(strings.TrimSpace(amount.Currency))] {
		return badFormat(field + ".currency")
	}
	return nil
}
