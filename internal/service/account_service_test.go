package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

func setupAccountService() (*AccountService, *testutil.MockAccountRepository) {
	accountRepo := testutil.NewMockAccountRepository()
	service := NewAccountService(accountRepo)
	return service, accountRepo
}

// --- CreateAccount Tests ---

func TestCreateAccount_Success(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	req := CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: 100000, // $1000.00
		Currency:       "USD",
	}

	acct, err := svc.CreateAccount(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, acct)
	assert.Equal(t, "user123", acct.UserID)
	assert.Equal(t, int64(100000), acct.Balance)
	assert.Equal(t, "USD", acct.Currency)
	assert.Equal(t, account.StatusActive, acct.Status)
	assert.Equal(t, 0, acct.Version)

	// Verify account was created in repository
	stored := accountRepo.GetAccountByID(acct.ID)
	assert.NotNil(t, stored)
	assert.Equal(t, acct.ID, stored.ID)
}

func TestCreateAccount_ZeroInitialBalance(t *testing.T) {
	svc, _ := setupAccountService()
	ctx := context.Background()

	req := CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: 0,
		Currency:       "USD",
	}

	acct, err := svc.CreateAccount(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(0), acct.Balance)
}

func TestCreateAccount_InvalidUserID_Empty(t *testing.T) {
	svc, _ := setupAccountService()
	ctx := context.Background()

	req := CreateAccountRequest{
		UserID:         "", // Invalid
		InitialBalance: 100000,
		Currency:       "USD",
	}

	_, err := svc.CreateAccount(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user_id")
}

func TestCreateAccount_InvalidBalance_Negative(t *testing.T) {
	svc, _ := setupAccountService()
	ctx := context.Background()

	req := CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: -1000, // Invalid
		Currency:       "USD",
	}

	_, err := svc.CreateAccount(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initial_balance")
}

func TestCreateAccount_InvalidCurrency_Empty(t *testing.T) {
	svc, _ := setupAccountService()
	ctx := context.Background()

	req := CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: 100000,
		Currency:       "", // Invalid
	}

	_, err := svc.CreateAccount(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency")
}

func TestCreateAccount_RepositoryError(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	// Mock repository to return error
	accountRepo.CreateFunc = func(ctx context.Context, acct *account.Account) error {
		return errors.New("database error")
	}

	req := CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: 100000,
		Currency:       "USD",
	}

	_, err := svc.CreateAccount(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

// --- GetAccount Tests ---

func TestGetAccount_Success(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	expectedAcct := createTestAccount(t, "user123", 100000, account.StatusActive)
	accountRepo.AddAccount(expectedAcct)

	acct, err := svc.GetAccount(ctx, expectedAcct.ID)
	require.NoError(t, err)
	assert.NotNil(t, acct)
	assert.Equal(t, expectedAcct.ID, acct.ID)
	assert.Equal(t, expectedAcct.UserID, acct.UserID)
	assert.Equal(t, expectedAcct.Balance, acct.Balance)
}

func TestGetAccount_NotFound(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	nonExistentID := uuid.New()
	accountRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
		if id == nonExistentID {
			return nil, domainErrors.ErrAccountNotFound
		}
		return nil, domainErrors.ErrAccountNotFound
	}

	acct, err := svc.GetAccount(ctx, nonExistentID)
	assert.ErrorIs(t, err, domainErrors.ErrAccountNotFound)
	assert.Nil(t, acct)
}

// --- GetBalance Tests ---

func TestGetBalance_Success(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	expectedAcct := createTestAccount(t, "user123", 250000, account.StatusActive)
	accountRepo.AddAccount(expectedAcct)

	balance, currency, err := svc.GetBalance(ctx, expectedAcct.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(250000), balance)
	assert.Equal(t, "USD", currency)
}

func TestGetBalance_AccountNotFound(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	nonExistentID := uuid.New()
	accountRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
		return nil, domainErrors.ErrAccountNotFound
	}

	balance, currency, err := svc.GetBalance(ctx, nonExistentID)
	assert.ErrorIs(t, err, domainErrors.ErrAccountNotFound)
	assert.Equal(t, int64(0), balance)
	assert.Equal(t, "", currency)
}

// --- GetTransactions Tests ---

func TestGetTransactions_Success(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	accountID := uuid.New()
	paymentID1 := uuid.New()
	paymentID2 := uuid.New()

	expectedTxns := []*account.Transaction{
		{
			ID:              uuid.New(),
			AccountID:       accountID,
			PaymentID:       &paymentID1,
			TransactionType: account.TransactionDebit,
			Amount:          10000,
			BalanceAfter:    90000,
			Description:     "payment debit",
		},
		{
			ID:              uuid.New(),
			AccountID:       accountID,
			PaymentID:       &paymentID2,
			TransactionType: account.TransactionCredit,
			Amount:          5000,
			BalanceAfter:    95000,
			Description:     "payment credit",
		},
	}

	accountRepo.GetTransactionsFunc = func(ctx context.Context, id uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
		if id == accountID {
			return expectedTxns, nil
		}
		return nil, nil
	}

	txns, err := svc.GetTransactions(ctx, accountID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, txns, 2)
	assert.Equal(t, expectedTxns[0].ID, txns[0].ID)
	assert.Equal(t, expectedTxns[1].ID, txns[1].ID)
}

func TestGetTransactions_EmptyList(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	accountID := uuid.New()
	accountRepo.GetTransactionsFunc = func(ctx context.Context, id uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
		return nil, nil
	}

	txns, err := svc.GetTransactions(ctx, accountID, 10, 0)
	require.NoError(t, err)
	assert.Nil(t, txns)
}

func TestGetTransactions_WithPagination(t *testing.T) {
	svc, accountRepo := setupAccountService()
	ctx := context.Background()

	accountID := uuid.New()
	paymentID := uuid.New()

	allTxns := []*account.Transaction{
		{ID: uuid.New(), AccountID: accountID, PaymentID: &paymentID, TransactionType: account.TransactionDebit, Amount: 1000, BalanceAfter: 99000},
		{ID: uuid.New(), AccountID: accountID, PaymentID: &paymentID, TransactionType: account.TransactionDebit, Amount: 2000, BalanceAfter: 97000},
		{ID: uuid.New(), AccountID: accountID, PaymentID: &paymentID, TransactionType: account.TransactionCredit, Amount: 3000, BalanceAfter: 100000},
	}

	accountRepo.GetTransactionsFunc = func(ctx context.Context, id uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
		if id == accountID {
			if offset >= len(allTxns) {
				return nil, nil
			}
			end := offset + limit
			if end > len(allTxns) {
				end = len(allTxns)
			}
			return allTxns[offset:end], nil
		}
		return nil, nil
	}

	// First page (limit 2, offset 0)
	page1, err := svc.GetTransactions(ctx, accountID, 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, allTxns[0].ID, page1[0].ID)
	assert.Equal(t, allTxns[1].ID, page1[1].ID)

	// Second page (limit 2, offset 2)
	page2, err := svc.GetTransactions(ctx, accountID, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 1)
	assert.Equal(t, allTxns[2].ID, page2[0].ID)

	// Beyond available data
	page3, err := svc.GetTransactions(ctx, accountID, 2, 10)
	require.NoError(t, err)
	assert.Nil(t, page3)
}
