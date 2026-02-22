package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		payload      any
		expectedBody string
	}{
		{
			name:         "simple map",
			status:       http.StatusOK,
			payload:      map[string]string{"message": "hello"},
			expectedBody: `{"message":"hello"}`,
		},
		{
			name:         "struct",
			status:       http.StatusCreated,
			payload:      struct{ ID string }{ID: "123"},
			expectedBody: `{"ID":"123"}`,
		},
		{
			name:         "error response",
			status:       http.StatusBadRequest,
			payload:      ErrorResponse{Error: "bad request", Code: "invalid_input"},
			expectedBody: `{"error":"bad request","code":"invalid_input"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeJSON(w, tt.status, tt.payload)

			assert.Equal(t, tt.status, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestWriteError_ValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	err := domainErrors.NewValidationError("email", "must be valid email")

	writeError(w, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, "validation_error", response.Code)
	assert.Contains(t, response.Error, "email")
}

func TestWriteError_DomainErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "account not found",
			err:            domainErrors.ErrAccountNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "payment not found",
			err:            domainErrors.ErrPaymentNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "insufficient funds",
			err:            domainErrors.ErrInsufficientFunds,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "insufficient_funds",
		},
		{
			name:           "account inactive",
			err:            domainErrors.ErrAccountInactive,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "account_inactive",
		},
		{
			name:           "invalid currency",
			err:            domainErrors.ErrInvalidCurrency,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_currency",
		},
		{
			name:           "duplicate idempotency key",
			err:            domainErrors.ErrDuplicateIdempotencyKey,
			expectedStatus: http.StatusConflict,
			expectedCode:   "duplicate_request",
		},
		{
			name:           "invalid state transition",
			err:            domainErrors.ErrInvalidStateTransition,
			expectedStatus: http.StatusConflict,
			expectedCode:   "invalid_state_transition",
		},
		{
			name:           "optimistic lock failed",
			err:            domainErrors.ErrOptimisticLockFailed,
			expectedStatus: http.StatusConflict,
			expectedCode:   "conflict",
		},
		{
			name:           "provider unavailable",
			err:            domainErrors.ErrProviderUnavailable,
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   "provider_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, response.Code)
		})
	}
}

func TestWriteError_OptimisticLockFailed_CustomMessage(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, domainErrors.ErrOptimisticLockFailed)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response ErrorResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, "concurrent modification, please retry", response.Error)
	assert.Equal(t, "conflict", response.Code)
}

func TestWriteError_GenericDomainError(t *testing.T) {
	w := httptest.NewRecorder()
	err := domainErrors.NewDomainError("custom_error", "custom error message", nil)

	writeError(w, err)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var response ErrorResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, "custom_error", response.Code)
	assert.Equal(t, "custom error message", response.Error)
}

func TestWriteError_UnknownError_FallbackToInternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	err := errors.New("unexpected error")

	writeError(w, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, "internal_error", response.Code)
	assert.Equal(t, "internal server error", response.Error)
}

func TestDecodeAndValidate_Success(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	body := `{"name":"John","email":"john@example.com"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))

	var result TestStruct
	err := decodeAndValidate(req, &result)

	require.NoError(t, err)
	assert.Equal(t, "John", result.Name)
	assert.Equal(t, "john@example.com", result.Email)
}

func TestDecodeAndValidate_InvalidJSON(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))

	var result TestStruct
	err := decodeAndValidate(req, &result)

	assert.Error(t, err)
	var validationErr *domainErrors.ValidationError
	assert.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "body", validationErr.Field)
	assert.Contains(t, validationErr.Message, "invalid JSON")
}

func TestDecodeAndValidate_ValidationFailure_RequiredField(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name" validate:"required"`
	}

	body := `{"name":""}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))

	var result TestStruct
	err := decodeAndValidate(req, &result)

	assert.Error(t, err)
	var validationErr *domainErrors.ValidationError
	assert.True(t, errors.As(err, &validationErr))
	assert.Contains(t, validationErr.Message, "validation failed")
}

func TestDecodeAndValidate_ValidationFailure_EmailFormat(t *testing.T) {
	type TestStruct struct {
		Email string `json:"email" validate:"required,email"`
	}

	body := `{"email":"not-an-email"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))

	var result TestStruct
	err := decodeAndValidate(req, &result)

	assert.Error(t, err)
	var validationErr *domainErrors.ValidationError
	assert.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "Email", validationErr.Field)
	assert.Contains(t, validationErr.Message, "validation failed")
}

func TestDecodeAndValidate_EmptyBody(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name" validate:"required"`
	}

	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte{}))

	var result TestStruct
	err := decodeAndValidate(req, &result)

	assert.Error(t, err)
}
