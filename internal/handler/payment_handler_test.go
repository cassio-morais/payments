package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	paymentApp "github.com/cassiomorais/payments/internal/service"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/payment"
	
	
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// mockOutboxWriter implements paymentApp.OutboxWriter.
type mockOutboxWriter struct{}

func (m *mockOutboxWriter) Insert(_ context.Context, _ *paymentApp.OutboxEntry) error { return nil }

func newTestRouter(h *handlers.PaymentHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/v1/payments", h.CreatePayment)
	r.Get("/api/v1/payments/{id}", h.GetPayment)
	r.Get("/api/v1/payments", h.ListPayments)
	r.Post("/api/v1/payments/{id}/refund", h.RefundPayment)
	r.Post("/api/v1/payments/{id}/cancel", h.CancelPayment)
	r.Post("/api/v1/transfers", h.Transfer)
	return r
}

func TestGetPayment_InvalidUUID(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	h := handlers.NewPaymentHandler(nil, nil, paymentRepo)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/not-a-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != "invalid_id" {
		t.Errorf("expected code invalid_id, got %s", resp.Code)
	}
}

func TestGetPayment_NotFound(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	paymentRepo.GetByIDFunc = func(_ context.Context, _ uuid.UUID) (*payment.Payment, error) {
		return nil, domainErrors.ErrPaymentNotFound
	}
	h := handlers.NewPaymentHandler(nil, nil, paymentRepo)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetPayment_Success(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	sourceID := uuid.New()
	p := testutil.NewTestPayment(payment.InternalTransfer, &sourceID, nil, 50_00, "USD")
	paymentRepo.Create(context.Background(), p)

	h := handlers.NewPaymentHandler(nil, nil, paymentRepo)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+p.ID.String(), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp PaymentResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID != p.ID {
		t.Errorf("expected payment ID %s, got %s", p.ID, resp.ID)
	}
	if resp.Amount != 50.0 {
		t.Errorf("expected amount 50.0, got %f", resp.Amount)
	}
}

func TestCreatePayment_InvalidJSON(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}
	createUC := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	h := handlers.NewPaymentHandler(createUC, nil, paymentRepo)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreatePayment_MissingRequiredFields(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}
	createUC := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	h := handlers.NewPaymentHandler(createUC, nil, paymentRepo)
	router := newTestRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"payment_type": "internal_transfer",
		// missing source_account_id, amount, currency
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreatePayment_InvalidSourceUUID(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}
	createUC := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	h := handlers.NewPaymentHandler(createUC, nil, paymentRepo)
	router := newTestRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"payment_type":      "internal_transfer",
		"source_account_id": "not-a-uuid",
		"amount":            100.0,
		"currency":          "USD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPayments_Empty(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	h := handlers.NewPaymentHandler(nil, nil, paymentRepo)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp []PaymentResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestRefundPayment_InvalidUUID(t *testing.T) {
	h := handlers.NewPaymentHandler(nil, nil, nil)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/bad-uuid/refund", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCancelPayment_InvalidUUID(t *testing.T) {
	h := handlers.NewPaymentHandler(nil, nil, nil)
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/bad-uuid/cancel", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTransfer_InvalidSourceUUID(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}
	createUC := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	h := handlers.NewPaymentHandler(createUC, nil, paymentRepo)
	router := newTestRouter(h)

	body, _ := json.Marshal(map[string]interface{}{
		"source_account_id":      "not-a-uuid",
		"destination_account_id": uuid.New().String(),
		"amount":                 50.0,
		"currency":               "USD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
