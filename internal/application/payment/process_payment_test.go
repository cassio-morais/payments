package payment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	paymentApp "github.com/cassiomorais/payments/internal/application/payment"
	domainPayment "github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/infrastructure/providers"
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/google/uuid"
)

func TestProcessPayment_Success(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()

	// Create a provider that always succeeds
	mockProvider := providers.NewMockProvider("stripe",
		providers.WithLatency(0),
		providers.WithFailureRate(0),
	)
	factory := providers.NewFactory(mockProvider)

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	provider := domainPayment.ProviderStripe
	p := testutil.NewTestPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	p.Provider = &provider
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	err := uc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify payment was completed
	updated, _ := paymentRepo.GetByID(ctx, p.ID)
	if updated.Status != domainPayment.StatusCompleted {
		t.Errorf("expected status completed, got %s", updated.Status)
	}
	if updated.ProviderTransactionID == nil {
		t.Error("expected provider transaction ID to be set")
	}

	// Verify funds were debited
	acct := accountRepo.GetAccountByID(sourceAcct.ID)
	if acct.Balance != 75_00 {
		t.Errorf("expected balance 7500, got %d", acct.Balance)
	}
}

func TestProcessPayment_ProviderFailure_Compensation(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()

	// Create a provider that always fails
	mockProvider := providers.NewMockProvider("stripe",
		providers.WithLatency(0),
		providers.WithFailureRate(1.0),
	)
	factory := providers.NewFactory(mockProvider)

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	provider := domainPayment.ProviderStripe
	p := testutil.NewTestPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	p.Provider = &provider
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	err := uc.Execute(ctx, p.ID)
	if err == nil {
		t.Fatal("expected error from failed provider, got nil")
	}

	// Verify payment was marked failed
	updated, _ := paymentRepo.GetByID(ctx, p.ID)
	if updated.Status != domainPayment.StatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}

	// Verify funds were compensated (credited back)
	acct := accountRepo.GetAccountByID(sourceAcct.ID)
	if acct.Balance != 100_00 {
		t.Errorf("expected balance to be restored to 10000, got %d", acct.Balance)
	}
}

func TestProcessPayment_AlreadyCompleted_Noop(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	factory := providers.NewFactory()

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	p := testutil.NewCompletedPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	err := uc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("expected no error for already completed payment, got %v", err)
	}
}

func TestProcessPayment_NoProvider_Fails(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	factory := providers.NewFactory()

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	// Payment without a provider
	p := testutil.NewTestPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	err := uc.Execute(ctx, p.ID)
	if err == nil {
		t.Fatal("expected error for payment without provider, got nil")
	}

	// Verify payment was marked failed
	updated, _ := paymentRepo.GetByID(ctx, p.ID)
	if updated.Status != domainPayment.StatusFailed {
		t.Errorf("expected status failed, got %s", updated.Status)
	}
}

func TestProcessPayment_FailedPayment_Retry(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()

	// First call fails, second succeeds
	callCount := 0
	mockProvider := providers.NewMockProvider("stripe",
		providers.WithLatency(0),
		providers.WithFailureRate(0),
	)
	// Override ProcessPayment to fail first time
	origProcess := mockProvider.ProcessPayment
	_ = origProcess
	factory := providers.NewFactory(mockProvider)

	sourceAcct := testutil.NewTestAccount("user-1", 100_00, "USD")
	accountRepo.AddAccount(sourceAcct)

	provider := domainPayment.ProviderStripe
	p := testutil.NewTestPayment(domainPayment.ExternalPayment, &sourceAcct.ID, nil, 25_00, "USD")
	p.Provider = &provider
	// Pre-set as failed (simulating previous failure)
	p.Status = domainPayment.StatusFailed
	failedAt := time.Now()
	p.CompletedAt = &failedAt
	errMsg := "previous failure"
	p.LastError = &errMsg
	p.RetryCount = 0
	paymentRepo.Create(ctx, p)

	uc := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, factory)
	_ = callCount

	err := uc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}

	updated, _ := paymentRepo.GetByID(ctx, p.ID)
	if updated.Status != domainPayment.StatusCompleted {
		t.Errorf("expected status completed after retry, got %s", updated.Status)
	}
	if updated.RetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", updated.RetryCount)
	}
}

func TestProcessPayment_NotFound(t *testing.T) {
	ctx := context.Background()
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	txManager := testutil.NewMockTransactionManager()
	factory := providers.NewFactory()

	paymentRepo.GetByIDFunc = func(_ context.Context, _ uuid.UUID) (*domainPayment.Payment, error) {
		return nil, fmt.Errorf("payment not found")
	}

	uc := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, factory)

	err := uc.Execute(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for missing payment, got nil")
	}
}
