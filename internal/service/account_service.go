package service

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/google/uuid"
)

type AccountService struct {
	accountRepo account.Repository
}

func NewAccountService(accountRepo account.Repository) *AccountService {
	return &AccountService{
		accountRepo: accountRepo,
	}
}

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

func (s *AccountService) GetAccount(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	return s.accountRepo.GetByID(ctx, id)
}

func (s *AccountService) GetBalance(ctx context.Context, id uuid.UUID) (int64, string, error) {
	acct, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return 0, "", err
	}
	return acct.Balance, acct.Currency, nil
}

func (s *AccountService) GetTransactions(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
	return s.accountRepo.GetTransactions(ctx, accountID, limit, offset)
}
