package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/middleware"
	"github.com/cassiomorais/payments/internal/service"
	"github.com/cassiomorais/payments/internal/testutil"
)

func TestAccountController_Create(t *testing.T) {
	mockRepo := &testutil.MockAccountRepository{}
	accountService := service.NewAccountService(mockRepo)
	authzService := service.NewAuthzService(mockRepo)
	handler := NewAccountController(accountService, authzService)

	mockRepo.CreateFunc = func(ctx context.Context, acct *account.Account) error {
		return nil
	}

	reqBody := CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: 100.0,
		Currency:       "USD",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(body))

	// Add authenticated user context
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp AccountResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.UserID != "user123" {
		t.Errorf("expected user_id user123, got %s", resp.UserID)
	}
}
