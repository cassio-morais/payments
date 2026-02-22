package payment

import (
	"context"
	"fmt"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/infrastructure/providers"
	"github.com/google/uuid"
)

// RefundPaymentUseCase handles payment refunds.
type RefundPaymentUseCase struct {
	paymentRepo     payment.Repository
	accountRepo     account.Repository
	txManager       TransactionManager
	providerFactory *providers.Factory
}

// NewRefundPaymentUseCase creates a new RefundPaymentUseCase.
func NewRefundPaymentUseCase(
	paymentRepo payment.Repository,
	accountRepo account.Repository,
	txManager TransactionManager,
	providerFactory *providers.Factory,
) *RefundPaymentUseCase {
	return &RefundPaymentUseCase{
		paymentRepo:     paymentRepo,
		accountRepo:     accountRepo,
		txManager:       txManager,
		providerFactory: providerFactory,
	}
}

// Execute refunds a completed payment.
func (uc *RefundPaymentUseCase) Execute(ctx context.Context, paymentID uuid.UUID) (*payment.Payment, error) {
	p, err := uc.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	if p.Status != payment.StatusCompleted {
		return nil, domainErrors.NewDomainError(
			"invalid_refund",
			fmt.Sprintf("cannot refund payment in status %s", p.Status),
			domainErrors.ErrInvalidStateTransition,
		)
	}

	// For external payments, call provider refund.
	if p.PaymentType == payment.ExternalPayment && p.Provider != nil {
		provider, breaker, err := uc.providerFactory.Get(*p.Provider)
		if err != nil {
			return nil, err
		}

		txID := ""
		if p.ProviderTransactionID != nil {
			txID = *p.ProviderTransactionID
		}

		_, cbErr := breaker.Execute(func() (*providers.ProviderResult, error) {
			return provider.RefundPayment(ctx, providers.RefundRequest{
				PaymentID:     p.ID.String(),
				TransactionID: txID,
				AmountCents:   p.Amount.ValueCents,
				Currency:      p.Amount.Currency,
			})
		})
		if cbErr != nil {
			return nil, fmt.Errorf("provider refund: %w", cbErr)
		}
	}

	ops := &accountOps{accountRepo: uc.accountRepo}

	// Credit the source account back.
	if p.SourceAccountID != nil {
		if err := uc.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := ops.creditAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "refund")
			return err
		}); err != nil {
			return nil, err
		}
	}

	// For internal transfers, also debit the destination account.
	if p.PaymentType == payment.InternalTransfer && p.DestinationAccountID != nil {
		if err := uc.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := ops.debitAccount(txCtx, *p.DestinationAccountID, p.ID, p.Amount.ValueCents, "refund reversal")
			return err
		}); err != nil {
			return nil, err
		}
	}

	if err := p.MarkRefunded(); err != nil {
		return nil, err
	}
	if err := uc.paymentRepo.Update(ctx, p); err != nil {
		return nil, err
	}

	uc.paymentRepo.AddEvent(ctx, &payment.PaymentEvent{
		ID: uuid.New(), PaymentID: p.ID, EventType: "payment.refunded",
		EventData: map[string]interface{}{"amount_cents": p.Amount.ValueCents},
	})

	return p, nil
}
