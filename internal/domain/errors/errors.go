package errors

import (
	"errors"
	"fmt"
)

var (
	// Account errors
	ErrAccountNotFound      = errors.New("account not found")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidCurrency      = errors.New("invalid currency")
	ErrAccountInactive      = errors.New("account is inactive")
	ErrOptimisticLockFailed = errors.New("optimistic lock conflict")

	// Payment errors
	ErrPaymentNotFound        = errors.New("payment not found")
	ErrInvalidPaymentType     = errors.New("invalid payment type")
	ErrInvalidAmount          = errors.New("invalid amount")
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrPaymentAlreadyProcessed = errors.New("payment already processed")
	ErrMaxRetriesExceeded     = errors.New("max retries exceeded")
	ErrPaymentCancelled       = errors.New("payment is cancelled")
	ErrPaymentExpired         = errors.New("payment has expired")

	// Provider errors
	ErrProviderNotFound       = errors.New("payment provider not found")
	ErrProviderUnavailable    = errors.New("payment provider unavailable")
	ErrProviderRejected       = errors.New("payment rejected by provider")
	ErrProviderTimeout        = errors.New("provider request timeout")

	// Idempotency errors
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")

	// Lock errors
	ErrLockAcquisitionFailed = errors.New("failed to acquire lock")
	ErrLockNotHeld           = errors.New("lock not held")

	// Validation errors
	ErrValidationFailed = errors.New("validation failed")
	ErrInvalidInput     = errors.New("invalid input")
)

// DomainError wraps errors with additional context
type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new domain error
func NewDomainError(code, message string, err error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
