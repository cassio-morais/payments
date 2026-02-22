package payment_test

import (
	"context"
	"testing"

	paymentApp "github.com/cassiomorais/payments/internal/servicepayment"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/google/uuid"
)

// mockOutboxWriter implements paymentApp.OutboxWriter for tests.
type mockOutboxWriter struct {
	entries []*paymentApp.OutboxEntry
	err     error
}

func (m *mockOutboxWriter) Insert(_ context.Context, entry *paymentApp.OutboxEntry) error {
	if m.err != nil {
		return m.err
	}
	m.entries = append(m.entries, entry)
	return nil
}

func TestCreatePayment_InternalTransfer_Success(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	destAcct := testutil.NewTestAccount("user-2", 50_00, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	resp, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       uuid.New().String(),
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               25_00,
		Currency:             "USD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsAsync {
		t.Error("expected sync transfer, got async")
	}
	if resp.Payment.Status != payment.StatusCompleted {
		t.Errorf("expected status completed, got %s", resp.Payment.Status)
	}

	// Check balances
	src := accountRepo.GetAccountByID(sourceAcct.ID)
	dst := accountRepo.GetAccountByID(destAcct.ID)
	if src.Balance != 75_00 {
		t.Errorf("expected source balance 7500, got %d", src.Balance)
	}
	if dst.Balance != 75_00 {
		t.Errorf("expected dest balance 7500, got %d", dst.Balance)
	}
}

func TestCreatePayment_InternalTransfer_InsufficientFunds(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 10_00, "USD")
	destAcct := testutil.NewTestAccount("user-2", 50_00, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	_, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       uuid.New().String(),
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               25_00,
		Currency:             "USD",
	})
	if err == nil {
		t.Fatal("expected insufficient funds error, got nil")
	}
	if err != domainErrors.ErrInsufficientFunds {
		t.Errorf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestCreatePayment_Idempotency_ReturnsExisting(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	destAcct := testutil.NewTestAccount("user-2", 50_00, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)
	idempotencyKey := uuid.New().String()

	// First call
	resp1, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       idempotencyKey,
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10_00,
		Currency:             "USD",
	})
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}

	// Second call with same key
	resp2, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       idempotencyKey,
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10_00,
		Currency:             "USD",
	})
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if resp1.Payment.ID != resp2.Payment.ID {
		t.Error("expected same payment to be returned for duplicate idempotency key")
	}
}

func TestCreatePayment_ExternalPayment_Async(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)
	provider := payment.ProviderStripe

	resp, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:  uuid.New().String(),
		PaymentType:     payment.ExternalPayment,
		SourceAccountID: &sourceAcct.ID,
		Amount:          50_00,
		Currency:        "USD",
		Provider:        &provider,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsAsync {
		t.Error("expected async payment")
	}
	if resp.Payment.Status != payment.StatusPending {
		t.Errorf("expected status pending, got %s", resp.Payment.Status)
	}
	if len(outbox.entries) != 1 {
		t.Errorf("expected 1 outbox entry, got %d", len(outbox.entries))
	}
}

func TestCreatePayment_ValidationError_InvalidAmount(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	destAcct := testutil.NewTestAccount("user-2", 50_00, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	_, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       uuid.New().String(),
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               0, // invalid
		Currency:             "USD",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestCreatePayment_InactiveSourceAccount(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	sourceAcct.Status = "inactive"
	destAcct := testutil.NewTestAccount("user-2", 50_00, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	_, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       uuid.New().String(),
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10_00,
		Currency:             "USD",
	})
	if err != domainErrors.ErrAccountInactive {
		t.Errorf("expected ErrAccountInactive, got %v", err)
	}
}

func TestCreatePayment_CurrencyMismatch(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "EUR")
	destAcct := testutil.NewTestAccount("user-2", 50_00, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	_, err := uc.Execute(ctx, paymentApp.CreatePaymentRequest{
		IdempotencyKey:       uuid.New().String(),
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10_00,
		Currency:             "USD", // doesn't match source account (EUR)
	})
	if err != domainErrors.ErrInvalidCurrency {
		t.Errorf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestTransfer_Convenience(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	outbox := &mockOutboxWriter{}

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	destAcct := testutil.NewTestAccount("user-2", 0, "USD")
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	uc := paymentApp.NewCreatePaymentUseCase(paymentRepo, accountRepo, outbox, txManager)

	resp, err := uc.Transfer(ctx, paymentApp.TransferRequest{
		IdempotencyKey:       uuid.New().String(),
		SourceAccountID:      sourceAcct.ID,
		DestinationAccountID: destAcct.ID,
		Amount:               30_00,
		Currency:             "USD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Payment.PaymentType != payment.InternalTransfer {
		t.Errorf("expected internal_transfer, got %s", resp.Payment.PaymentType)
	}
	if resp.Payment.Status != payment.StatusCompleted {
		t.Errorf("expected status completed, got %s", resp.Payment.Status)
	}
}
