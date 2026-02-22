package handlers

import (
	"net/http"
	"strconv"

	accountApp "github.com/cassiomorais/payments/internal/serviceaccount"
	"github.com/cassiomorais/payments/internal/handler/dto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AccountHandler handles account-related HTTP requests.
type AccountHandler struct {
	createUC       *accountApp.CreateAccountUseCase
	getUC          *accountApp.GetAccountUseCase
	getBalanceUC   *accountApp.GetBalanceUseCase
	getTransactionsUC *accountApp.GetTransactionsUseCase
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler(
	createUC *accountApp.CreateAccountUseCase,
	getUC *accountApp.GetAccountUseCase,
	getBalanceUC *accountApp.GetBalanceUseCase,
	getTransactionsUC *accountApp.GetTransactionsUseCase,
) *AccountHandler {
	return &AccountHandler{
		createUC:       createUC,
		getUC:          getUC,
		getBalanceUC:   getBalanceUC,
		getTransactionsUC: getTransactionsUC,
	}
}

// Create handles POST /api/v1/accounts
func (h *AccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAccountRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, err)
		return
	}

	acct, err := h.createUC.Execute(r.Context(), accountApp.CreateAccountRequest{
		UserID:         req.UserID,
		InitialBalance: floatToCents(req.InitialBalance),
		Currency:       req.Currency,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, dto.FromAccount(acct))
}

// Get handles GET /api/v1/accounts/{id}
func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	acct, err := h.getUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.FromAccount(acct))
}

// GetBalance handles GET /api/v1/accounts/{id}/balance
func (h *AccountHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	balanceCents, currency, err := h.getBalanceUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.BalanceResponse{
		AccountID: id,
		Balance:   float64(balanceCents) / 100.0,
		Currency:  currency,
	})
}

// GetTransactions handles GET /api/v1/accounts/{id}/transactions
func (h *AccountHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	txns, err := h.getTransactionsUC.Execute(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]*dto.TransactionResponse, 0, len(txns))
	for _, tx := range txns {
		resp = append(resp, dto.FromTransaction(tx))
	}
	writeJSON(w, http.StatusOK, resp)
}
