package account

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/google/uuid"
)

// GetAccountUseCase orchestrates retrieving an account by ID.
type GetAccountUseCase struct {
	accountRepo account.Repository
}

// NewGetAccountUseCase creates a new GetAccountUseCase.
func NewGetAccountUseCase(accountRepo account.Repository) *GetAccountUseCase {
	return &GetAccountUseCase{accountRepo: accountRepo}
}

// Execute retrieves an account by ID.
func (uc *GetAccountUseCase) Execute(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	return uc.accountRepo.GetByID(ctx, id)
}
