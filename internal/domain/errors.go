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
	ErrMerchantVANotFound      = errors.New("merchant VA not found")
	ErrMerchantVAAlreadyPaid   = errors.New("merchant VA already paid")
	ErrMerchantVAAlreadyDeleted = errors.New("merchant VA already deleted")
	ErrMerchantVAExpired       = errors.New("merchant VA expired")
	ErrMerchantNotificationFailed = errors.New("merchant notification failed")
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
