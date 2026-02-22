package controller

import (
	"net/http"
	"strconv"

	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PaymentController handles payment-related HTTP requests.
type PaymentController struct {
	paymentService *service.PaymentService
	paymentRepo    payment.Repository
}

// NewPaymentController creates a new PaymentController.
func NewPaymentController(
	paymentService *service.PaymentService,
	paymentRepo payment.Repository,
) *PaymentController {
	return &PaymentController{
		paymentService: paymentService,
		paymentRepo:    paymentRepo,
	}
}

// CreatePayment handles POST /api/v1/payments
func (h *PaymentController) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, err)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	sourceID := parseUUID(*req.SourceAccountID)
	if sourceID == nil && req.SourceAccountID != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid source_account_id", Code: "invalid_id"})
		return
	}

	var destID *uuid.UUID
	if req.DestinationAccountID != nil {
		destID = parseUUID(*req.DestinationAccountID)
		if destID == nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid destination_account_id", Code: "invalid_id"})
			return
		}
	}

	var provider *payment.Provider
	if req.Provider != nil {
		p := payment.Provider(*req.Provider)
		provider = &p
	}

	resp, err := h.paymentService.CreatePayment(r.Context(), service.CreatePaymentRequest{
		IdempotencyKey:       idempotencyKey,
		PaymentType:          payment.PaymentType(req.PaymentType),
		SourceAccountID:      sourceID,
		DestinationAccountID: destID,
		Amount:               floatToCents(req.Amount),
		Currency:             req.Currency,
		Provider:             provider,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	status := http.StatusCreated
	if resp.IsAsync {
		status = http.StatusAccepted
	}
	writeJSON(w, status, FromPayment(resp.Payment))
}

// GetPayment handles GET /api/v1/payments/{id}
func (h *PaymentController) GetPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid payment id", Code: "invalid_id"})
		return
	}

	p, err := h.paymentRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromPayment(p))
}

// ListPayments handles GET /api/v1/payments
func (h *PaymentController) ListPayments(w http.ResponseWriter, r *http.Request) {
	filter := payment.ListFilter{}

	if s := r.URL.Query().Get("status"); s != "" {
		status := payment.PaymentStatus(s)
		filter.Status = &status
	}
	if s := r.URL.Query().Get("account_id"); s != "" {
		id, err := uuid.Parse(s)
		if err == nil {
			filter.AccountID = &id
		}
	}
	if s := r.URL.Query().Get("provider"); s != "" {
		prov := payment.Provider(s)
		filter.Provider = &prov
	}
	filter.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	filter.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	filter.SortBy = r.URL.Query().Get("sort_by")
	filter.SortOrder = r.URL.Query().Get("sort_order")

	payments, err := h.paymentRepo.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]*PaymentResponse, 0, len(payments))
	for _, p := range payments {
		resp = append(resp, FromPayment(p))
	}
	writeJSON(w, http.StatusOK, resp)
}

// RefundPayment handles POST /api/v1/payments/{id}/refund
func (h *PaymentController) RefundPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid payment id", Code: "invalid_id"})
		return
	}

	p, err := h.paymentService.RefundPayment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromPayment(p))
}

// CancelPayment handles POST /api/v1/payments/{id}/cancel
func (h *PaymentController) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid payment id", Code: "invalid_id"})
		return
	}

	p, err := h.paymentRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	if err := p.MarkCancelled(); err != nil {
		writeError(w, err)
		return
	}
	if err := h.paymentRepo.Update(r.Context(), p); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, FromPayment(p))
}

// Transfer handles POST /api/v1/transfers
func (h *PaymentController) Transfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, err)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	sourceID, err := uuid.Parse(req.SourceAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid source_account_id", Code: "invalid_id"})
		return
	}
	destID, err := uuid.Parse(req.DestinationAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid destination_account_id", Code: "invalid_id"})
		return
	}

	resp, err := h.paymentService.Transfer(r.Context(), service.TransferRequest{
		IdempotencyKey:       idempotencyKey,
		SourceAccountID:      sourceID,
		DestinationAccountID: destID,
		Amount:               floatToCents(req.Amount),
		Currency:             req.Currency,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, FromPayment(resp.Payment))
}
