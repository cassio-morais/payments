package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/outbox"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/providers"
	"github.com/cassiomorais/payments/internal/service"
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/google/uuid"
)

func TestPaymentController_CreatePayment(t *testing.T) {
	paymentRepo := &testutil.MockPaymentRepository{}
	accountRepo := testutil.NewMockAccountRepository()
	outboxRepo := &testutil.MockOutboxRepository{}
	txManager := testutil.NewMockTransactionManager()
	providerFactory := providers.NewFactory()

	paymentService := service.NewPaymentService(paymentRepo, accountRepo, outboxRepo, txManager, providerFactory)
	handler := NewPaymentController(paymentService, paymentRepo)

	// Create a test source account
	sourceAcct, _ := account.NewAccount("user1", 10000, "USD")
	accountRepo.AddAccount(sourceAcct)

	// Setup mocks
	txManager.WithTransactionFunc = func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}
	paymentRepo.CreateFunc = func(ctx context.Context, p *payment.Payment) error {
		return nil
	}
	paymentRepo.GetByIdempotencyKeyFunc = func(ctx context.Context, key string) (*payment.Payment, error) {
		return nil, nil // Not found, will create new
	}
	outboxRepo.InsertFunc = func(ctx context.Context, entry *outbox.Entry) error {
		return nil
	}
	paymentRepo.AddEventFunc = func(ctx context.Context, event *payment.PaymentEvent) error {
		return nil
	}

	sourceIDStr := sourceAcct.ID.String()
	reqBody := CreatePaymentRequest{
		PaymentType:     "external_payment",
		SourceAccountID: &sourceIDStr,
		Amount:          50.0,
		Currency:        "USD",
		Provider:        stringPtr("stripe"),
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	rec := httptest.NewRecorder()

	handler.CreatePayment(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}
}

func stringPtr(s string) *string {
	return &s
}
