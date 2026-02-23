package controller

import (
	"net/http"
	"strconv"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/middleware"
	"github.com/cassiomorais/payments/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AccountController struct {
	accountService *service.AccountService
	authzService   *service.AuthzService
}

func NewAccountController(accountService *service.AccountService, authzService *service.AuthzService) *AccountController {
	return &AccountController{
		accountService: accountService,
		authzService:   authzService,
	}
}

func (h *AccountController) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, err)
		return
	}

	// Override user_id from authenticated context
	authenticatedUserID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, domainErrors.ErrUnauthorized)
		return
	}
	req.UserID = authenticatedUserID

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

func (h *AccountController) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	// Authorization check
	if err := h.authzService.VerifyAccountOwnership(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}

	acct, err := h.accountService.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromAccount(acct))
}

func (h *AccountController) GetBalance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	// Authorization check
	if err := h.authzService.VerifyAccountOwnership(r.Context(), id); err != nil {
		writeError(w, err)
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

func (h *AccountController) GetTransactions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account id", Code: "invalid_id"})
		return
	}

	// Authorization check
	if err := h.authzService.VerifyAccountOwnership(r.Context(), id); err != nil {
		writeError(w, err)
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
