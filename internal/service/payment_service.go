package service

import (
	"context"
	"fmt"
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/outbox"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/providers"
	"github.com/google/uuid"
)

type PaymentService struct {
	paymentRepo     payment.Repository
	accountRepo     account.Repository
	outboxRepo      outbox.Repository
	txManager       TransactionManager
	providerFactory *providers.Factory
}

func NewPaymentService(
	paymentRepo payment.Repository,
	accountRepo account.Repository,
	outboxRepo outbox.Repository,
	txManager TransactionManager,
	providerFactory *providers.Factory,
) *PaymentService {
	return &PaymentService{
		paymentRepo:     paymentRepo,
		accountRepo:     accountRepo,
		outboxRepo:      outboxRepo,
		txManager:       txManager,
		providerFactory: providerFactory,
	}
}

func (s *PaymentService) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	existing, err := s.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil && existing != nil {
		return &CreatePaymentResponse{
			Payment: existing,
			IsAsync: existing.PaymentType == payment.ExternalPayment,
		}, nil
	}

	if req.SourceAccountID != nil {
		src, err := s.accountRepo.GetByID(ctx, *req.SourceAccountID)
		if err != nil {
			return nil, err
		}
		if src.Status != account.StatusActive {
			return nil, domainErrors.ErrAccountInactive
		}
		if src.Currency != req.Currency {
			return nil, domainErrors.ErrInvalidCurrency
		}
	}

	if req.PaymentType == payment.InternalTransfer {
		if req.DestinationAccountID == nil {
			return nil, domainErrors.NewValidationError("destination_account_id", "required for internal transfers")
		}
		dst, err := s.accountRepo.GetByID(ctx, *req.DestinationAccountID)
		if err != nil {
			return nil, err
		}
		if dst.Status != account.StatusActive {
			return nil, domainErrors.ErrAccountInactive
		}
	}

	p, err := payment.NewPayment(
		req.IdempotencyKey,
		req.PaymentType,
		req.SourceAccountID,
		req.DestinationAccountID,
		payment.Amount{ValueCents: req.Amount, Currency: req.Currency},
	)
	if err != nil {
		return nil, err
	}
	if req.Provider != nil {
		p.SetProvider(*req.Provider)
	}

	switch req.PaymentType {
	case payment.InternalTransfer:
		return s.executeSync(ctx, p)
	case payment.ExternalPayment:
		return s.enqueueAsync(ctx, p)
	default:
		return nil, domainErrors.ErrInvalidPaymentType
	}
}

func (s *PaymentService) executeSync(ctx context.Context, p *payment.Payment) (*CreatePaymentResponse, error) {
	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		ids := sortUUIDs(*p.SourceAccountID, *p.DestinationAccountID)
		if _, err := s.accountRepo.Lock(txCtx, ids[0]); err != nil {
			return err
		}
		if _, err := s.accountRepo.Lock(txCtx, ids[1]); err != nil {
			return err
		}

		if err := p.MarkCompleted(nil); err != nil {
			return err
		}

		if err := s.paymentRepo.Create(txCtx, p); err != nil {
			return err
		}

		if _, err := s.debitAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "internal transfer debit"); err != nil {
			return err
		}
		if _, err := s.creditAccount(txCtx, *p.DestinationAccountID, p.ID, p.Amount.ValueCents, "internal transfer credit"); err != nil {
			return err
		}

		return s.paymentRepo.AddEvent(txCtx, &payment.PaymentEvent{
			ID: uuid.New(), PaymentID: p.ID, EventType: string(payment.EventPaymentCompleted),
			EventData: map[string]any{
				"type":         string(p.PaymentType),
				"amount_cents": p.Amount.ValueCents,
				"status":       string(p.Status),
			},
		})
	})
	if err != nil {
		return nil, err
	}

	return &CreatePaymentResponse{Payment: p, IsAsync: false}, nil
}

func (s *PaymentService) enqueueAsync(ctx context.Context, p *payment.Payment) (*CreatePaymentResponse, error) {
	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.paymentRepo.Create(txCtx, p); err != nil {
			return err
		}

		entry := outbox.NewEntry(
			"payment",
			p.ID,
			"payment.created",
			map[string]any{
				"payment_id":   p.ID.String(),
				"type":         string(p.PaymentType),
				"amount_cents": p.Amount.ValueCents,
				"currency":     p.Amount.Currency,
				"provider":     string(*p.Provider),
			},
		)
		if err := s.outboxRepo.Insert(txCtx, entry); err != nil {
			return err
		}

		return s.paymentRepo.AddEvent(txCtx, &payment.PaymentEvent{
			ID: uuid.New(), PaymentID: p.ID, EventType: string(payment.EventPaymentCreated),
			EventData: map[string]any{
				"type":         string(p.PaymentType),
				"amount_cents": p.Amount.ValueCents,
				"status":       string(p.Status),
			},
		})
	})
	if err != nil {
		return nil, err
	}

	return &CreatePaymentResponse{Payment: p, IsAsync: true}, nil
}

func (s *PaymentService) Transfer(ctx context.Context, req TransferRequest) (*CreatePaymentResponse, error) {
	return s.CreatePayment(ctx, CreatePaymentRequest{
		IdempotencyKey:       req.IdempotencyKey,
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &req.SourceAccountID,
		DestinationAccountID: &req.DestinationAccountID,
		Amount:               req.Amount,
		Currency:             req.Currency,
	})
}

func (s *PaymentService) ProcessPayment(ctx context.Context, paymentID uuid.UUID) error {
	p, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("load payment: %w", err)
	}

	if p.Status != payment.StatusPending && p.Status != payment.StatusFailed {
		return nil
	}

	if p.Status == payment.StatusFailed {
		if err := p.IncrementRetry(); err != nil {
			return err
		}
	}
	if err := p.MarkProcessing(); err != nil {
		return err
	}
	if err := s.paymentRepo.Update(ctx, p); err != nil {
		return err
	}

	if err := s.processExternalPayment(ctx, p); err != nil {
		return s.failPayment(ctx, p, err.Error())
	}

	return nil
}

func (s *PaymentService) processExternalPayment(ctx context.Context, p *payment.Payment) error {
	if p.Provider == nil {
		return fmt.Errorf("no provider specified")
	}

	provider, breaker, err := s.providerFactory.Get(*p.Provider)
	if err != nil {
		return err
	}

	if p.SourceAccountID != nil {
		if err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := s.debitAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "external payment reserve")
			return err
		}); err != nil {
			return fmt.Errorf("reserve funds: %w", err)
		}
	}

	result, err := breaker.Execute(func() (*providers.ProviderResult, error) {
		return provider.ProcessPayment(ctx, providers.ProcessRequest{
			PaymentID:   p.ID.String(),
			AmountCents: p.Amount.ValueCents,
			Currency:    p.Amount.Currency,
			Metadata:    p.Metadata,
		})
	})
	if err != nil {
		if p.SourceAccountID != nil {
			_ = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
				_, err := s.creditAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "external payment compensation")
				return err
			})
		}
		return fmt.Errorf("provider call: %w", err)
	}

	txID := result.TransactionID
	if err := p.MarkCompleted(&txID); err != nil {
		return err
	}
	if err := s.paymentRepo.Update(ctx, p); err != nil {
		return err
	}

	s.paymentRepo.AddEvent(ctx, &payment.PaymentEvent{
		ID: uuid.New(), PaymentID: p.ID, EventType: string(payment.EventPaymentCompleted),
		EventData: map[string]any{
			"provider_tx_id": txID,
			"amount_cents":   p.Amount.ValueCents,
		},
	})

	return nil
}

func (s *PaymentService) failPayment(ctx context.Context, p *payment.Payment, reason string) error {
	if err := p.MarkFailed(reason); err != nil {
		return err
	}
	if err := s.paymentRepo.Update(ctx, p); err != nil {
		return err
	}
	s.paymentRepo.AddEvent(ctx, &payment.PaymentEvent{
		ID: uuid.New(), PaymentID: p.ID, EventType: string(payment.EventPaymentFailed),
		EventData: map[string]any{"error": reason},
	})
	return domainErrors.NewDomainError("payment_failed", reason, nil)
}

func (s *PaymentService) RefundPayment(ctx context.Context, paymentID uuid.UUID) (*payment.Payment, error) {
	p, err := s.paymentRepo.GetByID(ctx, paymentID)
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

	if p.PaymentType == payment.ExternalPayment && p.Provider != nil {
		provider, breaker, err := s.providerFactory.Get(*p.Provider)
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

	if p.SourceAccountID != nil {
		if err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := s.creditAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "refund")
			return err
		}); err != nil {
			return nil, err
		}
	}

	if p.PaymentType == payment.InternalTransfer && p.DestinationAccountID != nil {
		if err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := s.debitAccount(txCtx, *p.DestinationAccountID, p.ID, p.Amount.ValueCents, "refund reversal")
			return err
		}); err != nil {
			return nil, err
		}
	}

	if err := p.MarkRefunded(); err != nil {
		return nil, err
	}
	if err := s.paymentRepo.Update(ctx, p); err != nil {
		return nil, err
	}

	s.paymentRepo.AddEvent(ctx, &payment.PaymentEvent{
		ID: uuid.New(), PaymentID: p.ID, EventType: string(payment.EventPaymentRefunded),
		EventData: map[string]any{"amount_cents": p.Amount.ValueCents},
	})

	return p, nil
}

func (s *PaymentService) debitAccount(ctx context.Context, accountID uuid.UUID, paymentID uuid.UUID, amount int64, description string) (balanceAfter int64, err error) {
	acct, err := s.accountRepo.Lock(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if err := acct.Debit(amount); err != nil {
		return 0, err
	}
	if err := s.accountRepo.Update(ctx, acct); err != nil {
		return 0, err
	}
	if err := s.accountRepo.AddTransaction(ctx, &account.Transaction{
		ID: uuid.New(), AccountID: acct.ID, PaymentID: &paymentID,
		TransactionType: account.TransactionDebit, Amount: amount,
		BalanceAfter: acct.Balance, Description: description, CreatedAt: time.Now(),
	}); err != nil {
		return 0, err
	}
	return acct.Balance, nil
}

func (s *PaymentService) creditAccount(ctx context.Context, accountID uuid.UUID, paymentID uuid.UUID, amount int64, description string) (balanceAfter int64, err error) {
	acct, err := s.accountRepo.Lock(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if err := acct.Credit(amount); err != nil {
		return 0, err
	}
	if err := s.accountRepo.Update(ctx, acct); err != nil {
		return 0, err
	}
	if err := s.accountRepo.AddTransaction(ctx, &account.Transaction{
		ID: uuid.New(), AccountID: acct.ID, PaymentID: &paymentID,
		TransactionType: account.TransactionCredit, Amount: amount,
		BalanceAfter: acct.Balance, Description: description, CreatedAt: time.Now(),
	}); err != nil {
		return 0, err
	}
	return acct.Balance, nil
}

func sortUUIDs(a, b uuid.UUID) [2]uuid.UUID {
	if a.String() < b.String() {
		return [2]uuid.UUID{a, b}
	}
	return [2]uuid.UUID{b, a}
}
