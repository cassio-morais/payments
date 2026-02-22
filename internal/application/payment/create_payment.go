package payment

import (
	"context"
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)

// CreatePaymentRequest holds the input for creating a payment.
type CreatePaymentRequest struct {
	IdempotencyKey       string
	PaymentType          payment.PaymentType
	SourceAccountID      *uuid.UUID
	DestinationAccountID *uuid.UUID
	Amount               int64 // in cents
	Currency             string
	Provider             *payment.Provider
}

// CreatePaymentResponse holds the result of creating a payment.
type CreatePaymentResponse struct {
	Payment *payment.Payment
	IsAsync bool
}

// CreatePaymentUseCase orchestrates payment creation.
type CreatePaymentUseCase struct {
	paymentRepo payment.Repository
	accountRepo account.Repository
	outboxRepo  OutboxWriter
	txManager   TransactionManager
}

// NewCreatePaymentUseCase creates a new CreatePaymentUseCase.
func NewCreatePaymentUseCase(
	paymentRepo payment.Repository,
	accountRepo account.Repository,
	outboxRepo OutboxWriter,
	txManager TransactionManager,
) *CreatePaymentUseCase {
	return &CreatePaymentUseCase{
		paymentRepo: paymentRepo,
		accountRepo: accountRepo,
		outboxRepo:  outboxRepo,
		txManager:   txManager,
	}
}

// Execute creates a payment and routes it to the sync or async path.
func (uc *CreatePaymentUseCase) Execute(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	// 1. Check idempotency — if the payment already exists, return it.
	existing, err := uc.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil && existing != nil {
		return &CreatePaymentResponse{Payment: existing, IsAsync: existing.PaymentType == payment.ExternalPayment}, nil
	}

	// 2. Validate source account exists and is active.
	if req.SourceAccountID != nil {
		src, err := uc.accountRepo.GetByID(ctx, *req.SourceAccountID)
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

	// 3. Validate destination account for internal transfers.
	if req.PaymentType == payment.InternalTransfer {
		if req.DestinationAccountID == nil {
			return nil, domainErrors.NewValidationError("destination_account_id", "required for internal transfers")
		}
		dst, err := uc.accountRepo.GetByID(ctx, *req.DestinationAccountID)
		if err != nil {
			return nil, err
		}
		if dst.Status != account.StatusActive {
			return nil, domainErrors.ErrAccountInactive
		}
	}

	// 4. Create the payment entity.
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

	// 5. Route based on payment type.
	switch req.PaymentType {
	case payment.InternalTransfer:
		return uc.executeSync(ctx, p)
	case payment.ExternalPayment:
		return uc.enqueueAsync(ctx, p)
	default:
		return nil, domainErrors.ErrInvalidPaymentType
	}
}

// executeSync processes an internal transfer in a single transaction.
func (uc *CreatePaymentUseCase) executeSync(ctx context.Context, p *payment.Payment) (*CreatePaymentResponse, error) {
	ops := &accountOps{accountRepo: uc.accountRepo}

	err := uc.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Lock accounts in deterministic order to prevent deadlocks.
		ids := sortUUIDs(*p.SourceAccountID, *p.DestinationAccountID)
		// We need to lock in order, but then debit/credit the correct accounts.
		// Lock both first to prevent deadlocks.
		if _, err := uc.accountRepo.Lock(txCtx, ids[0]); err != nil {
			return err
		}
		if _, err := uc.accountRepo.Lock(txCtx, ids[1]); err != nil {
			return err
		}

		// Mark payment completed
		if err := p.MarkCompleted(nil); err != nil {
			return err
		}

		// Persist payment first (account_transactions has FK to payments)
		if err := uc.paymentRepo.Create(txCtx, p); err != nil {
			return err
		}

		// Debit source, credit destination
		if _, err := ops.debitAccount(txCtx, *p.SourceAccountID, p.ID, p.Amount.ValueCents, "internal transfer debit"); err != nil {
			return err
		}
		if _, err := ops.creditAccount(txCtx, *p.DestinationAccountID, p.ID, p.Amount.ValueCents, "internal transfer credit"); err != nil {
			return err
		}

		// Add event
		return uc.paymentRepo.AddEvent(txCtx, &payment.PaymentEvent{
			ID: uuid.New(), PaymentID: p.ID, EventType: "payment.completed",
			EventData: map[string]interface{}{
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

// enqueueAsync persists the payment as pending and writes to the outbox.
func (uc *CreatePaymentUseCase) enqueueAsync(ctx context.Context, p *payment.Payment) (*CreatePaymentResponse, error) {
	err := uc.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Persist payment as pending
		if err := uc.paymentRepo.Create(txCtx, p); err != nil {
			return err
		}

		// Write to outbox for reliable publishing
		if err := uc.outboxRepo.Insert(txCtx, &OutboxEntry{
			ID:            uuid.New(),
			AggregateType: "payment",
			AggregateID:   p.ID,
			EventType:     "payment.created",
			Payload: map[string]interface{}{
				"payment_id":   p.ID.String(),
				"type":         string(p.PaymentType),
				"amount_cents": p.Amount.ValueCents,
				"currency":     p.Amount.Currency,
				"provider":     string(*p.Provider),
			},
			Status:     "pending",
			RetryCount: 0,
			MaxRetries: 5,
			CreatedAt:  time.Now(),
		}); err != nil {
			return err
		}

		// Add event
		return uc.paymentRepo.AddEvent(txCtx, &payment.PaymentEvent{
			ID: uuid.New(), PaymentID: p.ID, EventType: "payment.created",
			EventData: map[string]interface{}{
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

// sortUUIDs returns two UUIDs in deterministic order (by string comparison).
func sortUUIDs(a, b uuid.UUID) [2]uuid.UUID {
	if a.String() < b.String() {
		return [2]uuid.UUID{a, b}
	}
	return [2]uuid.UUID{b, a}
}

// --- Transfer convenience wrapper ---

// TransferRequest is a simplified request for internal transfers.
type TransferRequest struct {
	IdempotencyKey       string
	SourceAccountID      uuid.UUID
	DestinationAccountID uuid.UUID
	Amount               int64 // in cents
	Currency             string
}

// Transfer creates an internal transfer between two accounts.
func (uc *CreatePaymentUseCase) Transfer(ctx context.Context, req TransferRequest) (*CreatePaymentResponse, error) {
	return uc.Execute(ctx, CreatePaymentRequest{
		IdempotencyKey:       req.IdempotencyKey,
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &req.SourceAccountID,
		DestinationAccountID: &req.DestinationAccountID,
		Amount:               req.Amount,
		Currency:             req.Currency,
	})
}
