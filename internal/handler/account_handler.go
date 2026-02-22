package handler

import (
	"net/http"
	"strconv"

	"github.com/cassiomorais/payments/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AccountHandler handles account-related HTTP requests.
type AccountHandler struct {
	accountService *service.AccountService
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler(accountService *service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

// Create handles POST /api/v1/accounts
func (h *AccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, err)
		return
	}

	acct, err := h.accountService.CreateAccount(r.Context(), service.CreateAccountRequest{
		UserID:         req.UserID,
		InitialBalance: floatToCents(req.InitialBalance),
		Currency:       req.Currency,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, FromAccount(acct))
}

// Get handles GET /api/v1/accounts/{id}
func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	acct, err := h.accountService.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromAccount(acct))
}

// GetBalance handles GET /api/v1/accounts/{id}/balance
func (h *AccountHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	balanceCents, currency, err := h.accountService.GetBalance(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, BalanceResponse{
		Balance:  centsToFloat(balanceCents),
		Currency: currency,
	})
}

// GetTransactions handles GET /api/v1/accounts/{id}/transactions
func (h *AccountHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	txns, err := h.accountService.GetTransactions(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]*TransactionResponse, 0, len(txns))
	for _, tx := range txns {
		resp = append(resp, FromTransaction(tx))
	}
	writeJSON(w, http.StatusOK, resp)
}
