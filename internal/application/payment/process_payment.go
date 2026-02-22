package payment

import (
	"context"
	"fmt"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/infrastructure/providers"
	"github.com/cassiomorais/payments/pkg/saga"
	"github.com/google/uuid"
)

// ProcessPaymentUseCase handles asynchronous payment processing by workers.
type ProcessPaymentUseCase struct {
	paymentRepo     payment.Repository
	accountRepo     account.Repository
	txManager       TransactionManager
	providerFactory *providers.Factory
}

// NewProcessPaymentUseCase creates a new ProcessPaymentUseCase.
func NewProcessPaymentUseCase(
	paymentRepo payment.Repository,
	accountRepo account.Repository,
	txManager TransactionManager,
	providerFactory *providers.Factory,
) *ProcessPaymentUseCase {
	return &ProcessPaymentUseCase{
		paymentRepo:     paymentRepo,
		accountRepo:     accountRepo,
		txManager:       txManager,
		providerFactory: providerFactory,
	}
}

// Execute processes a single payment by ID. Returns whether the payment reached a terminal state.
func (uc *ProcessPaymentUseCase) Execute(ctx context.Context, paymentID uuid.UUID) error {
	p, err := uc.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("load payment: %w", err)
	}

	// Only process payments that are pending or failed (retry).
	if p.Status != payment.StatusPending && p.Status != payment.StatusFailed {
		return nil // already processed or terminal
	}

	// Transition to processing.
	if p.Status == payment.StatusFailed {
		if err := p.IncrementRetry(); err != nil {
			return err
		}
	}
	if err := p.MarkProcessing(); err != nil {
		return err
	}
	if err := uc.paymentRepo.Update(ctx, p); err != nil {
		return err
	}

	// Run the external payment saga.
	return uc.executeSaga(ctx, p)
}

func (uc *ProcessPaymentUseCase) executeSaga(ctx context.Context, p *payment.Payment) error {
	if p.Provider == nil {
		return uc.failPayment(ctx, p, "no provider specified")
	}

	provider, breaker, err := uc.providerFactory.Get(*p.Provider)
	if err != nil {
		return uc.failPayment(ctx, p, err.Error())
	}

	ops := &accountOps{accountRepo: uc.accountRepo}
	var providerResult *providers.ProviderResult
	var reservedFunds bool

	s := saga.New("external-payment").
		// Step 1: Reserve funds (debit source account).
		AddStep(saga.Step{
			Name: "reserve-funds",
			Execute: func(ctx context.Context) error {
				if p.SourceAccountID == nil {
					return nil
				}
				return uc.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
					if _, err := ops.debitAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "external payment reserve"); err != nil {
						return err
					}
					reservedFunds = true
					return nil
				})
			},
			Compensate: func(ctx context.Context) error {
				if !reservedFunds || p.SourceAccountID == nil {
					return nil
				}
				return uc.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
					_, err := ops.creditAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "external payment compensation")
					return err
				})
			},
		}).
		// Step 2: Call external provider with circuit breaker + retry.
		AddStep(saga.Step{
			Name: "call-provider",
			Execute: func(ctx context.Context) error {
				result, cbErr := breaker.Execute(func() (*providers.ProviderResult, error) {
					return provider.ProcessPayment(ctx, providers.ProcessRequest{
						PaymentID:   p.ID.String(),
						AmountCents: p.Amount.ValueCents,
						Currency:    p.Amount.Currency,
						Metadata:    p.Metadata,
					})
				})
				if cbErr != nil {
					return fmt.Errorf("provider call: %w", cbErr)
				}
				providerResult = result
				return nil
			},
			Compensate: func(ctx context.Context) error {
				// If the provider call succeeded, we might need to refund it.
				if providerResult != nil && providerResult.Status == "success" {
					_, _ = provider.RefundPayment(ctx, providers.RefundRequest{
						PaymentID:     p.ID.String(),
						TransactionID: providerResult.TransactionID,
						AmountCents:   p.Amount.ValueCents,
						Currency:      p.Amount.Currency,
					})
				}
				return nil
			},
		})

	failedStep, sagaErr := s.Execute(ctx)
	if sagaErr != nil {
		return uc.failPayment(ctx, p, fmt.Sprintf("saga failed at step %d: %v", failedStep, sagaErr))
	}

	// Success — mark payment completed.
	txID := providerResult.TransactionID
	if err := p.MarkCompleted(&txID); err != nil {
		return err
	}
	if err := uc.paymentRepo.Update(ctx, p); err != nil {
		return err
	}

	// Record event.
	uc.paymentRepo.AddEvent(ctx, &payment.PaymentEvent{
		ID: uuid.New(), PaymentID: p.ID, EventType: "payment.completed",
		EventData: map[string]interface{}{
			"provider_tx_id": txID,
			"amount_cents":   p.Amount.ValueCents,
		},
	})

	return nil
}

func (uc *ProcessPaymentUseCase) failPayment(ctx context.Context, p *payment.Payment, reason string) error {
	if err := p.MarkFailed(reason); err != nil {
		return err
	}
	if err := uc.paymentRepo.Update(ctx, p); err != nil {
		return err
	}
	uc.paymentRepo.AddEvent(ctx, &payment.PaymentEvent{
		ID: uuid.New(), PaymentID: p.ID, EventType: "payment.failed",
		EventData: map[string]interface{}{"error": reason},
	})
	return domainErrors.NewDomainError("payment_failed", reason, nil)
}
