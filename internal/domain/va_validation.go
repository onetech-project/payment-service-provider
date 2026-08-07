package domain

import (
	"regexp"
	"strings"
)

// SNAP request validation, transcribed from the field tables in Developer API
// BCA, "Virtual Account untuk Biller".
//
// The limits genuinely differ between services — inquiry allows customerNo(20)
// / virtualAccountNo(28) / inquiryRequestId(128) while payment and status
// allow (18) / (26) / (30) — so they are spelled out per service rather than
// shared. Getting this wrong is silent: an over-long value is accepted here and
// then truncated or rejected downstream at BCA.

// Field length limits per BCA's tables.
const (
	// partnerServiceId is documented as String(8) Fixed, "using space padding
	// on the left if it doesn't reach 8 characters". We enforce it as a MAX
	// rather than an exact width: BCA itself always sends the padded 8-char
	// form, so nothing is lost, while merchants that registered VAs under an
	// unpadded 5-character company code keep working.
	maxPartnerServiceID = 8

	// Inquiry (service 24)
	maxInquiryCustomerNo       = 20
	maxInquiryVirtualAccountNo = 28
	maxInquiryRequestID        = 128

	// Payment (service 25) and status (service 26).
	//
	// BCA's payment/status tables say customerNo(18)/virtualAccountNo(26)
	// while its inquiry table says (20)/(28) for the same two fields. INBOUND
	// we accept the wider pair, purely for backward compatibility: the dynamic
	// sequence used to emit 20-digit customerNo values, and VA numbers already
	// in customers' hands from that era must stay payable. Rejecting them here
	// would break VAs this very system issued.
	//
	// This is a receive-side allowance only. Issuance is now 18/26 — see
	// NextCustomerNoSequence — so every NEW VA number fits BCA's narrower
	// payment/status limit. Being more permissive than BCA on receive is safe
	// (BCA never sends more than it generated); being more permissive on issue
	// was not, and that is the half that was fixed.
	maxPaymentCustomerNo       = 20
	maxPaymentVirtualAccountNo = 28
	maxPaymentRequestID        = 30
	maxVirtualAccountName      = 30
	maxVirtualAccountEmail     = 255
	maxVirtualAccountPhone     = 30
	maxTrxID                   = 64
	maxHashedSourceAccountNo   = 32
	maxSourceBankCode          = 3
	maxPaidBills               = 6
	maxReferenceNo             = 11
	maxJournalNum              = 6
	maxPaymentType             = 1
	maxFlagAdvise              = 1
	maxSubCompany              = 5
	maxPaymentBillDetails      = 5
	maxFreeTexts               = 9

	// MaxInquiryBillDetails and MaxInquiryFreeTexts bound what the INQUIRY
	// response may carry. BCA's inquiry field table nominally allows 24
	// billDetails and 9 freeTexts, but its Notes override both downward:
	// "billDetails should not be greater than 5" and "The occurences for
	// freeTexts field in inquiry bill should not be greater than 5". Exceeding
	// either makes BCA treat the inquiry as failed, so these are enforced when
	// a merchant creates the VA and again when the response is built.
	MaxInquiryBillDetails = 5
	MaxInquiryFreeTexts   = 5

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

// ValidateInquiryRequest checks an InquiryRequest against BCA's service-24
// field table. Note what is NOT here: `amount`. BCA's inquiry payload has no
// such field, so requiring one rejects every conformant inquiry.
func ValidateInquiryRequest(req *VAInquiryRequest) *FieldViolation {
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
	// channelCode is Optional (N) on inquiry, so 0/absent is fine; a value that
	// IS sent must still be one BCA generates.
	if req.ChannelCode != 0 && !allowedPaymentChannelCodes[req.ChannelCode] {
		return badFormat("channelCode")
	}
	// amount is not part of BCA's inquiry payload, but a vendor that sends it
	// anyway must still send it well-formed.
	if req.Amount != nil {
		if v := validateAmount(req.Amount, "amount"); v != nil {
			return v
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
		{"virtualAccountName", req.VirtualAccountName, maxVirtualAccountName},
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
