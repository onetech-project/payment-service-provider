package domain

import (
	"errors"
	"fmt"
)

var (
	ErrClientNotFound      = errors.New("client not found")
	ErrClientRevoked       = errors.New("client is revoked or inactive")
	ErrClientAlreadyExists = errors.New("client already exists")
	ErrClientKeyInvalid    = errors.New("invalid public key")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrInvalidTimestamp    = errors.New("invalid or expired timestamp")
	ErrMissingHeader       = errors.New("missing required SNAP header")
	ErrInvalidGrantType    = errors.New("invalid grant type")
	ErrTokenExpired        = errors.New("token expired")
	ErrTokenInvalid        = errors.New("token invalid")
	ErrIdempotencyMissing  = errors.New("idempotency key is required")
	ErrIdempotencyConflict = errors.New("request in progress for this idempotency key")
	ErrPayloadMismatch     = errors.New("payload mismatch for idempotency key")

	// vendor VA Error Constants
	ErrVAInvalidBill      = errors.New("invalid bill/virtual account")
	ErrVAPaidBill         = errors.New("paid bill")
	ErrVAInvalidPartner   = errors.New("invalid bill/virtual account partner")
	ErrVADuplicateExtID   = errors.New("conflict: duplicate external ID")
	ErrVAMissingMandatory = errors.New("missing mandatory field")
	ErrVAInvalidField     = errors.New("invalid field format")
	ErrVAUnauthorized     = errors.New("unauthorized")
	ErrVAInternalError    = errors.New("internal server error")

	// Merchant VA Error Constants
	ErrMerchantVANotFound         = errors.New("merchant VA not found")
	ErrMerchantVAAlreadyPaid      = errors.New("merchant VA already paid")
	ErrMerchantVAAlreadyDeleted   = errors.New("merchant VA already deleted")
	ErrMerchantVAExpired          = errors.New("merchant VA expired")
	ErrMerchantNotificationFailed = errors.New("merchant notification failed")

	// Expired VA SNAP Error Constants (feature 007-merchant-expiry-callback)
	ErrVAExpiredInquiry = errors.New("expired transaction (inquiry)")
	ErrVAExpiredPayment = errors.New("expired transaction (payment notification)")

	// Resend Callback Error Constants (feature 007-merchant-expiry-callback)
	ErrResendNoDeliveryRecord  = errors.New("no callback delivery on record for this transaction")
	ErrResendNoNotificationURL = errors.New("transaction has no registered notification URL")

	// Static/Dynamic VA Error Constants (feature 006-static-dynamic-va)
	ErrVACustomerNoAlreadyRegistered = errors.New("customerNo already registered for this partnerServiceId")

	// VA Registry Error Constants (feature 013-no-bill-payment-transaction)

	// ErrVAAccountNotFound reports that no va_accounts registration exists for
	// the virtual account number. Callers MUST treat this as "fall through to
	// the legacy transaction-based path", not as a hard failure — that
	// fall-through is what keeps VAs created before this feature working.
	ErrVAAccountNotFound = errors.New("VA registration not found")
	// ErrVAAccountInactive reports that the registration exists but has been
	// deactivated via delete-VA.
	ErrVAAccountInactive = errors.New("VA registration is inactive")
	// ErrVAPaymentDuplicate reports that a payment with this paymentRequestId
	// is already recorded. Raised by the unique index on
	// va_transactions.inquiry_request_id when a duplicate races past the
	// GetPayment short-circuit; the caller replays the original response.
	ErrVAPaymentDuplicate = errors.New("payment already recorded for this paymentRequestId")
)

type DomainError struct {
	SNAPCode string
	Message  string
	Err      error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.SNAPCode, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.SNAPCode, e.Message)
}

func NewDomainError(snapCode, message string, err error) *DomainError {
	return &DomainError{
		SNAPCode: snapCode,
		Message:  message,
		Err:      err,
	}
}
