package account

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/google/uuid"
)

// GetBalanceUseCase orchestrates retrieving an account's balance.
type GetBalanceUseCase struct {
	accountRepo account.Repository
}

// NewGetBalanceUseCase creates a new GetBalanceUseCase.
func NewGetBalanceUseCase(accountRepo account.Repository) *GetBalanceUseCase {
	return &GetBalanceUseCase{accountRepo: accountRepo}
}

// Execute returns the balance (in cents) and currency for an account.
func (uc *GetBalanceUseCase) Execute(ctx context.Context, id uuid.UUID) (int64, string, error) {
	acct, err := uc.accountRepo.GetByID(ctx, id)
	if err != nil {
		return 0, "", err
	}
	return acct.Balance, acct.Currency, nil
}
