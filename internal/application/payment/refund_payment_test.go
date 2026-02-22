package payment_test

import (
	"context"
	"errors"
	"testing"

	paymentApp "github.com/cassiomorais/payments/internal/application/payment"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	domainPayment "github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/infrastructure/providers"
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/google/uuid"
)

func TestRefundPayment_InternalTransfer_Success(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	factory := providers.NewFactory()

	sourceAcct := testutil.NewTestAccount("user-1", 75_00, "USD") // after debit
	destAcct := testutil.NewTestAccount("user-2", 75_00, "USD")   // after credit
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	p := testutil.NewCompletedPayment(domainPayment.InternalTransfer, &sourceAcct.ID, &destAcct.ID, 25_00, "USD")
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewRefundPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	refunded, err := uc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refunded.Status != domainPayment.StatusRefunded {
		t.Errorf("expected status refunded, got %s", refunded.Status)
	}

	// Verify balances reversed
	src := accountRepo.GetAccountByID(sourceAcct.ID)
	dst := accountRepo.GetAccountByID(destAcct.ID)
	if src.Balance != 100_00 {
		t.Errorf("expected source balance 10000, got %d", src.Balance)
	}
	if dst.Balance != 50_00 {
		t.Errorf("expected dest balance 5000, got %d", dst.Balance)
	}
}

func TestRefundPayment_ExternalPayment_Success(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()

	mockProvider := providers.NewMockProvider("stripe",
		providers.WithLatency(0),
		providers.WithFailureRate(0),
	)
	factory := providers.NewFactory(mockProvider)

	sourceAcct := testutil.NewTestAccount("user-1", 75_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	provider := domainPayment.ProviderStripe
	txID := "tx-123"
	p := testutil.NewCompletedPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	p.Provider = &provider
	p.ProviderTransactionID = &txID
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewRefundPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	refunded, err := uc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refunded.Status != domainPayment.StatusRefunded {
		t.Errorf("expected status refunded, got %s", refunded.Status)
	}

	// Verify source was credited back
	src := accountRepo.GetAccountByID(sourceAcct.ID)
	if src.Balance != 100_00 {
		t.Errorf("expected source balance 10000, got %d", src.Balance)
	}
}

func TestRefundPayment_InvalidState_NotCompleted(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	factory := providers.NewFactory()

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	// A pending payment cannot be refunded
	p := testutil.NewTestPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewRefundPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	_, err := uc.Execute(ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for refunding non-completed payment, got nil")
	}
	if !errors.Is(err, domainErrors.ErrInvalidStateTransition) {
		t.Errorf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestRefundPayment_NotFound(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	factory := providers.NewFactory()

	paymentRepo.GetByIDFunc = func(_ context.Context, _ uuid.UUID) (*domainPayment.Payment, error) {
		return nil, domainErrors.ErrPaymentNotFound
	}

	uc := paymentApp.NewRefundPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	_, err := uc.Execute(ctx, uuid.New())
	if !errors.Is(err, domainErrors.ErrPaymentNotFound) {
		t.Errorf("expected ErrPaymentNotFound, got %v", err)
	}
}
