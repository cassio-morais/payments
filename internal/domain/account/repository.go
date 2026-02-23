package account

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	// Create creates a new account
	Create(ctx context.Context, account *Account) error

	// GetByID retrieves an account by ID
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)

	// GetByUserID retrieves an account by user ID and currency
	GetByUserID(ctx context.Context, userID string, currency string) (*Account, error)

	// Update updates an existing account with optimistic locking
	Update(ctx context.Context, account *Account) error

	// AddTransaction records an account transaction
	AddTransaction(ctx context.Context, tx *Transaction) error

	// GetTransactions retrieves transactions for an account
	GetTransactions(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*Transaction, error)

	// Lock locks an account for update (SELECT FOR UPDATE)
	Lock(ctx context.Context, id uuid.UUID) (*Account, error)
}

type Transaction struct {
	ID              uuid.UUID
	AccountID       uuid.UUID
	PaymentID       *uuid.UUID
	TransactionType TransactionType
	Amount          int64 // in cents
	BalanceAfter    int64 // in cents
	Description     string
	CreatedAt       time.Time
}

type TransactionType string

const (
	TransactionDebit  TransactionType = "debit"
	TransactionCredit TransactionType = "credit"
)
