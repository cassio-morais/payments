package service

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/google/uuid"
)

// AccountService handles account-related business logic.
type AccountService struct {
	accountRepo account.Repository
}

// NewAccountService creates a new AccountService.
func NewAccountService(accountRepo account.Repository) *AccountService {
	return &AccountService{
		accountRepo: accountRepo,
	}
}

// CreateAccountRequest holds the service layer input for creating an account.
// This uses business domain types (int64 cents, UUIDs) rather than HTTP types.
// Controllers convert their HTTP DTOs to this type.
type CreateAccountRequest struct {
	UserID         string
	InitialBalance int64 // in cents
	Currency       string
}

// CreateAccount creates a new account.
func (s *AccountService) CreateAccount(ctx context.Context, req CreateAccountRequest) (*account.Account, error) {
	acct, err := account.NewAccount(req.UserID, req.InitialBalance, req.Currency)
	if err != nil {
		return nil, err
	}
	if err := s.accountRepo.Create(ctx, acct); err != nil {
		return nil, err
	}
	return acct, nil
}

// GetAccount retrieves an account by ID.
func (s *AccountService) GetAccount(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	return s.accountRepo.GetByID(ctx, id)
}

// GetBalance returns the balance (in cents) and currency for an account.
func (s *AccountService) GetBalance(ctx context.Context, id uuid.UUID) (int64, string, error) {
	acct, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return 0, "", err
	}
	return acct.Balance, acct.Currency, nil
}

// GetTransactions returns transactions for an account.
func (s *AccountService) GetTransactions(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
	return s.accountRepo.GetTransactions(ctx, accountID, limit, offset)
}
