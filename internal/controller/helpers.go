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

type errorMapping struct {
	err    error
	status int
	code   string
}

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
	{domainErrors.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
	{domainErrors.ErrForbidden, http.StatusForbidden, "forbidden"},
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	resp := ErrorResponse{Error: err.Error()}

	var validationErr *domainErrors.ValidationError
	if errors.As(err, &validationErr) {
		resp.Code = "validation_error"
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}

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

	var domainErr *domainErrors.DomainError
	if errors.As(err, &domainErr) {
		resp.Code = domainErr.Code
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	log.Error().Err(err).Msg("unhandled error in handler")
	resp.Code = "internal_error"
	resp.Error = "internal server error"
	writeJSON(w, http.StatusInternalServerError, resp)
}

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
