package service

import (
	"context"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/middleware"
	"github.com/google/uuid"
)

type AuthzService struct {
	accountRepo account.Repository
}

func NewAuthzService(accountRepo account.Repository) *AuthzService {
	return &AuthzService{accountRepo: accountRepo}
}

func (s *AuthzService) VerifyAccountOwnership(ctx context.Context, accountID uuid.UUID) error {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return errors.ErrUnauthorized
	}

	acct, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return err
	}

	if acct.UserID != userID {
		return errors.ErrForbidden
	}

	return nil
}

func (s *AuthzService) VerifyPaymentAuthorization(ctx context.Context, sourceAccountID *uuid.UUID) error {
	if sourceAccountID == nil {
		return nil // External payments without source account allowed
	}
	return s.VerifyAccountOwnership(ctx, *sourceAccountID)
}
