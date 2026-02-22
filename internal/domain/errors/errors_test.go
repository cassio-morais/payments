package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *DomainError
		expected string
	}{
		{
			name: "with wrapped error",
			err: &DomainError{
				Code:    "payment_failed",
				Message: "payment processing failed",
				Err:     errors.New("provider timeout"),
			},
			expected: "payment processing failed: provider timeout",
		},
		{
			name: "without wrapped error",
			err: &DomainError{
				Code:    "invalid_state",
				Message: "cannot process payment in current state",
				Err:     nil,
			},
			expected: "cannot process payment in current state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestDomainError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	domainErr := &DomainError{
		Code:    "test",
		Message: "test message",
		Err:     originalErr,
	}

	unwrapped := domainErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestNewDomainError(t *testing.T) {
	originalErr := errors.New("underlying error")
	err := NewDomainError("test_code", "test message", originalErr)

	assert.NotNil(t, err)
	assert.Equal(t, "test_code", err.Code)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, originalErr, err.Err)
}

func TestNewDomainError_NilWrappedError(t *testing.T) {
	err := NewDomainError("test_code", "test message", nil)

	assert.NotNil(t, err)
	assert.Equal(t, "test_code", err.Code)
	assert.Equal(t, "test message", err.Message)
	assert.Nil(t, err.Err)
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "email",
		Message: "must be a valid email address",
	}

	expected := "validation failed for field email: must be a valid email address"
	assert.Equal(t, expected, err.Error())
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("username", "cannot be empty")

	assert.NotNil(t, err)
	assert.Equal(t, "username", err.Field)
	assert.Equal(t, "cannot be empty", err.Message)
}

func TestErrorConstants(t *testing.T) {
	// Account errors
	assert.NotNil(t, ErrAccountNotFound)
	assert.NotNil(t, ErrInsufficientFunds)
	assert.NotNil(t, ErrInvalidCurrency)
	assert.NotNil(t, ErrAccountInactive)
	assert.NotNil(t, ErrOptimisticLockFailed)

	// Payment errors
	assert.NotNil(t, ErrPaymentNotFound)
	assert.NotNil(t, ErrInvalidPaymentType)
	assert.NotNil(t, ErrInvalidAmount)
	assert.NotNil(t, ErrInvalidStateTransition)
	assert.NotNil(t, ErrPaymentAlreadyProcessed)
	assert.NotNil(t, ErrMaxRetriesExceeded)
	assert.NotNil(t, ErrPaymentCancelled)
	assert.NotNil(t, ErrPaymentExpired)

	// Provider errors
	assert.NotNil(t, ErrProviderNotFound)
	assert.NotNil(t, ErrProviderUnavailable)
	assert.NotNil(t, ErrProviderRejected)
	assert.NotNil(t, ErrProviderTimeout)

	// Idempotency errors
	assert.NotNil(t, ErrDuplicateIdempotencyKey)

	// Lock errors
	assert.NotNil(t, ErrLockAcquisitionFailed)
	assert.NotNil(t, ErrLockNotHeld)

	// Validation errors
	assert.NotNil(t, ErrValidationFailed)
	assert.NotNil(t, ErrInvalidInput)
}

func TestErrorUnwrapping(t *testing.T) {
	baseErr := ErrProviderTimeout
	wrappedErr := NewDomainError("provider_error", "provider call failed", baseErr)

	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.ErrorIs(t, wrappedErr, ErrProviderTimeout)
}
