package account

import (
	"time"

	"github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/google/uuid"
)

type AccountStatus string

const (
	StatusActive    AccountStatus = "active"
	StatusInactive  AccountStatus = "inactive"
	StatusSuspended AccountStatus = "suspended"
)

type Account struct {
	ID        uuid.UUID
	UserID    string
	Balance   int64 // in cents
	Currency  string
	Version   int // Optimistic locking
	Status    AccountStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewAccount(userID string, initialBalance int64, currency string) (*Account, error) {
	if userID == "" {
		return nil, errors.NewValidationError("user_id", "cannot be empty")
	}
	if initialBalance < 0 {
		return nil, errors.NewValidationError("initial_balance", "cannot be negative")
	}
	if currency == "" {
		return nil, errors.NewValidationError("currency", "cannot be empty")
	}

	now := time.Now()
	return &Account{
		ID:        uuid.New(),
		UserID:    userID,
		Balance:   initialBalance,
		Currency:  currency,
		Version:   0,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (a *Account) Debit(amount int64) error {
	if a.Status != StatusActive {
		return errors.ErrAccountInactive
	}
	if amount <= 0 {
		return errors.NewValidationError("amount", "must be greater than 0")
	}
	if a.Balance < amount {
		return errors.ErrInsufficientFunds
	}

	a.Balance -= amount
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Credit(amount int64) error {
	if a.Status != StatusActive {
		return errors.ErrAccountInactive
	}
	if amount <= 0 {
		return errors.NewValidationError("amount", "must be greater than 0")
	}

	a.Balance += amount
	a.Version++
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Suspend() error {
	if a.Status == StatusInactive {
		return errors.ErrAccountInactive
	}
	a.Status = StatusSuspended
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Activate() error {
	a.Status = StatusActive
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Deactivate() error {
	a.Status = StatusInactive
	a.UpdatedAt = time.Now()
	return nil
}
