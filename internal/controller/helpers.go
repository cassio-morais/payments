package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

var validate = validator.New()

// errorMapping maps a domain error to an HTTP status code and error code.
type errorMapping struct {
	err    error
	status int
	code   string
}

// errorMappings is an ordered registry of domain errors to HTTP responses.
var errorMappings = []errorMapping{
	{domainErrors.ErrAccountNotFound, http.StatusNotFound, "not_found"},
	{domainErrors.ErrPaymentNotFound, http.StatusNotFound, "not_found"},
	{domainErrors.ErrInsufficientFunds, http.StatusUnprocessableEntity, "insufficient_funds"},
	{domainErrors.ErrAccountInactive, http.StatusUnprocessableEntity, "account_inactive"},
	{domainErrors.ErrInvalidCurrency, http.StatusBadRequest, "invalid_currency"},
	{domainErrors.ErrDuplicateIdempotencyKey, http.StatusConflict, "duplicate_request"},
	{domainErrors.ErrInvalidStateTransition, http.StatusConflict, "invalid_state_transition"},
	{domainErrors.ErrOptimisticLockFailed, http.StatusConflict, "conflict"},
	{domainErrors.ErrProviderUnavailable, http.StatusServiceUnavailable, "provider_unavailable"},
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError maps domain errors to HTTP error responses.
func writeError(w http.ResponseWriter, err error) {
	resp := ErrorResponse{Error: err.Error()}

	// Check for validation errors first.
	var validationErr *domainErrors.ValidationError
	if errors.As(err, &validationErr) {
		resp.Code = "validation_error"
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}

	// Check against registry.
	for _, m := range errorMappings {
		if errors.Is(err, m.err) {
			resp.Code = m.code
			if m.err == domainErrors.ErrOptimisticLockFailed {
				resp.Error = "concurrent modification, please retry"
			}
			writeJSON(w, m.status, resp)
			return
		}
	}

	// Check for generic domain error.
	var domainErr *domainErrors.DomainError
	if errors.As(err, &domainErr) {
		resp.Code = domainErr.Code
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	// Fallback: internal server error.
	log.Error().Err(err).Msg("unhandled error in handler")
	resp.Code = "internal_error"
	resp.Error = "internal server error"
	writeJSON(w, http.StatusInternalServerError, resp)
}

// decodeAndValidate decodes JSON body into dst and validates it.
func decodeAndValidate(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return domainErrors.NewValidationError("body", "invalid JSON: "+err.Error())
	}
	if err := validate.Struct(dst); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok && len(ve) > 0 {
			return domainErrors.NewValidationError(ve[0].Field(), ve[0].Tag()+" validation failed")
		}
		return domainErrors.NewValidationError("body", err.Error())
	}
	return nil
}
