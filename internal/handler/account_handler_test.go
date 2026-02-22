package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	accountApp "github.com/cassiomorais/payments/internal/service"
	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	
	
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func newAccountRouter(h *handlers.AccountHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/v1/accounts", h.Create)
	r.Get("/api/v1/accounts/{id}", h.Get)
	r.Get("/api/v1/accounts/{id}/balance", h.GetBalance)
	r.Get("/api/v1/accounts/{id}/transactions", h.GetTransactions)
	return r
}

func newTestAccountHandler(accountRepo *testutil.MockAccountRepository) *handlers.AccountHandler {
	return handlers.NewAccountHandler(
		accountApp.NewCreateAccountUseCase(accountRepo),
		accountApp.NewGetAccountUseCase(accountRepo),
		accountApp.NewGetBalanceUseCase(accountRepo),
		accountApp.NewGetTransactionsUseCase(accountRepo),
	)
}

func TestCreateAccount_Success(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":         "user-1",
		"initial_balance": 100.0,
		"currency":        "USD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp AccountResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.UserID != "user-1" {
		t.Errorf("expected user_id user-1, got %s", resp.UserID)
	}
	if resp.Balance != 100.0 {
		t.Errorf("expected balance 100.0, got %f", resp.Balance)
	}
	if resp.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", resp.Currency)
	}
}

func TestCreateAccount_MissingFields(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"initial_balance": 100.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetAccount_InvalidUUID(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetAccount_Success(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	acct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(acct)
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acct.ID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp AccountResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID != acct.ID {
		t.Errorf("expected account ID %s, got %s", acct.ID, resp.ID)
	}
	if resp.Balance != 100.0 {
		t.Errorf("expected balance 100.0, got %f", resp.Balance)
	}
}

func TestGetBalance_Success(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	acct := testutil.NewTestAccount("user-1", 123_45, "USD")
	accountRepo.AddAccount(acct)
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acct.ID.String()+"/balance", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp BalanceResponse
	json.NewDecoder(w.Body).Decode(&resp)
	// 12345 cents = $123.45
	if resp.Balance != 123.45 {
		t.Errorf("expected balance 123.45, got %f", resp.Balance)
	}
	if resp.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", resp.Currency)
	}
}

func TestGetBalance_InvalidUUID(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/bad/balance", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetTransactions_DefaultLimit(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	acct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(acct)
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acct.ID.String()+"/transactions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTransactions_InvalidUUID(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/bad/transactions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateAccount_InvalidJSON(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString("{broken"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetBalance_NotFound(t *testing.T) {
	accountRepo := testutil.NewMockAccountRepository()
	accountRepo.GetByIDFunc = func(_ context.Context, _ uuid.UUID) (*account.Account, error) {
		return nil, domainErrors.ErrAccountNotFound
	}
	h := newTestAccountHandler(accountRepo)
	router := newAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+uuid.New().String()+"/balance", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
