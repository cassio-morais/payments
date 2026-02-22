package handlers

import (
	"math"
	"net/http"
	"strconv"

	paymentApp "github.com/cassiomorais/payments/internal/application/payment"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/interfaces/http/dto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PaymentHandler handles payment-related HTTP requests.
type PaymentHandler struct {
	createUC    *paymentApp.CreatePaymentUseCase
	refundUC    *paymentApp.RefundPaymentUseCase
	paymentRepo payment.Repository
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(
	createUC *paymentApp.CreatePaymentUseCase,
	refundUC *paymentApp.RefundPaymentUseCase,
	paymentRepo payment.Repository,
) *PaymentHandler {
	return &PaymentHandler{
		createUC:    createUC,
		refundUC:    refundUC,
		paymentRepo: paymentRepo,
	}
}

// floatToCents converts a float64 amount to int64 cents.
func floatToCents(f float64) int64 {
	return int64(math.Round(f * 100))
}

// CreatePayment handles POST /api/v1/payments
func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePaymentRequest
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
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid source_account_id", Code: "invalid_id"})
		return
	}

	var destID *uuid.UUID
	if req.DestinationAccountID != "" {
		d, err := uuid.Parse(req.DestinationAccountID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid destination_account_id", Code: "invalid_id"})
			return
		}
		destID = &d
	}

	var provider *payment.Provider
	if req.Provider != "" {
		p := payment.Provider(req.Provider)
		provider = &p
	}

	resp, err := h.createUC.Execute(r.Context(), paymentApp.CreatePaymentRequest{
		IdempotencyKey:       idempotencyKey,
		PaymentType:          payment.PaymentType(req.PaymentType),
		SourceAccountID:      &sourceID,
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
	writeJSON(w, status, dto.FromPayment(resp.Payment))
}

// GetPayment handles GET /api/v1/payments/{id}
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid payment id", Code: "invalid_id"})
		return
	}

	p, err := h.paymentRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.FromPayment(p))
}

// ListPayments handles GET /api/v1/payments
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
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

	resp := make([]*dto.PaymentResponse, 0, len(payments))
	for _, p := range payments {
		resp = append(resp, dto.FromPayment(p))
	}
	writeJSON(w, http.StatusOK, resp)
}

// RefundPayment handles POST /api/v1/payments/{id}/refund
func (h *PaymentHandler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid payment id", Code: "invalid_id"})
		return
	}

	p, err := h.refundUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dto.FromPayment(p))
}

// CancelPayment handles POST /api/v1/payments/{id}/cancel
func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid payment id", Code: "invalid_id"})
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

	writeJSON(w, http.StatusOK, dto.FromPayment(p))
}

// Transfer handles POST /api/v1/transfers
func (h *PaymentHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req dto.TransferRequest
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
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid source_account_id", Code: "invalid_id"})
		return
	}
	destID, err := uuid.Parse(req.DestinationAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid destination_account_id", Code: "invalid_id"})
		return
	}

	resp, err := h.createUC.Transfer(r.Context(), paymentApp.TransferRequest{
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

	writeJSON(w, http.StatusCreated, dto.FromPayment(resp.Payment))
}
