package account

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
)

// CreateAccountRequest holds the input for creating an account.
type CreateAccountRequest struct {
	UserID         string
	InitialBalance int64 // in cents
	Currency       string
}

// CreateAccountUseCase orchestrates account creation.
type CreateAccountUseCase struct {
	accountRepo account.Repository
}

// NewCreateAccountUseCase creates a new CreateAccountUseCase.
func NewCreateAccountUseCase(accountRepo account.Repository) *CreateAccountUseCase {
	return &CreateAccountUseCase{accountRepo: accountRepo}
}

// Execute creates a new account.
func (uc *CreateAccountUseCase) Execute(ctx context.Context, req CreateAccountRequest) (*account.Account, error) {
	acct, err := account.NewAccount(req.UserID, req.InitialBalance, req.Currency)
	if err != nil {
		return nil, err
	}
	if err := uc.accountRepo.Create(ctx, acct); err != nil {
		return nil, err
	}
	return acct, nil
}
