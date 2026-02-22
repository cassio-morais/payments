package account

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/google/uuid"
)

// GetTransactionsUseCase orchestrates retrieving account transactions.
type GetTransactionsUseCase struct {
	accountRepo account.Repository
}

// NewGetTransactionsUseCase creates a new GetTransactionsUseCase.
func NewGetTransactionsUseCase(accountRepo account.Repository) *GetTransactionsUseCase {
	return &GetTransactionsUseCase{accountRepo: accountRepo}
}

// Execute returns transactions for an account.
func (uc *GetTransactionsUseCase) Execute(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
	return uc.accountRepo.GetTransactions(ctx, accountID, limit, offset)
}
