package postgres

import (
	"context"
	"fmt"

	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountRepository implements account.Repository using PostgreSQL.
type AccountRepository struct {
	pool *pgxpool.Pool
}

// NewAccountRepository creates a new AccountRepository.
func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

func (r *AccountRepository) db(ctx context.Context) DBTX {
	return ConnFromCtx(ctx, r.pool)
}

// scanAccount scans an account from any source implementing the scanner interface.
func (r *AccountRepository) scanAccount(s scanner) (*account.Account, error) {
	a := &account.Account{}
	var (
		status     string
		balanceStr string
	)
	err := s.Scan(&a.ID, &a.UserID, &balanceStr, &a.Currency, &a.Version, &status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domainErrors.ErrAccountNotFound
		}
		return nil, fmt.Errorf("scan account: %w", err)
	}

	cents, err := numericStringToCents(balanceStr)
	if err != nil {
		return nil, fmt.Errorf("parse balance: %w", err)
	}
	a.Balance = cents
	a.Status = account.AccountStatus(status)
	return a, nil
}

// Create inserts a new account.
func (r *AccountRepository) Create(ctx context.Context, a *account.Account) error {
	balanceStr := centsToNumericString(a.Balance)
	_, err := r.db(ctx).Exec(ctx,
		`INSERT INTO accounts (id, user_id, balance, currency, version, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.UserID, balanceStr, a.Currency, a.Version, string(a.Status), a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

// GetByID retrieves an account by its ID.
func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	return r.scanAccount(r.db(ctx).QueryRow(ctx,
		`SELECT id, user_id, balance, currency, version, status, created_at, updated_at
		 FROM accounts WHERE id = $1`, id))
}

// GetByUserID retrieves an account by user ID and currency.
func (r *AccountRepository) GetByUserID(ctx context.Context, userID string, currency string) (*account.Account, error) {
	return r.scanAccount(r.db(ctx).QueryRow(ctx,
		`SELECT id, user_id, balance, currency, version, status, created_at, updated_at
		 FROM accounts WHERE user_id = $1 AND currency = $2`, userID, currency))
}

// Update updates an account with optimistic locking.
func (r *AccountRepository) Update(ctx context.Context, a *account.Account) error {
	balanceStr := centsToNumericString(a.Balance)
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE accounts SET balance = $1, currency = $2, version = $3, status = $4, updated_at = $5
		 WHERE id = $6 AND version = $7`,
		balanceStr, a.Currency, a.Version, string(a.Status), a.UpdatedAt, a.ID, a.Version-1,
	)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainErrors.ErrOptimisticLockFailed
	}
	return nil
}

// AddTransaction inserts an account transaction record.
func (r *AccountRepository) AddTransaction(ctx context.Context, tx *account.Transaction) error {
	amountStr := centsToNumericString(tx.Amount)
	balanceAfterStr := centsToNumericString(tx.BalanceAfter)
	_, err := r.db(ctx).Exec(ctx,
		`INSERT INTO account_transactions (id, account_id, payment_id, transaction_type, amount, balance_after, description, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tx.ID, tx.AccountID, tx.PaymentID, string(tx.TransactionType), amountStr, balanceAfterStr, tx.Description, tx.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert account transaction: %w", err)
	}
	return nil
}

// GetTransactions retrieves transactions for an account.
func (r *AccountRepository) GetTransactions(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db(ctx).Query(ctx,
		`SELECT id, account_id, payment_id, transaction_type, amount, balance_after, description, created_at
		 FROM account_transactions WHERE account_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		accountID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txns []*account.Transaction
	for rows.Next() {
		tx := &account.Transaction{}
		var (
			txType          string
			amountStr       string
			balanceAfterStr string
		)
		if err := rows.Scan(&tx.ID, &tx.AccountID, &tx.PaymentID, &txType, &amountStr, &balanceAfterStr, &tx.Description, &tx.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		tx.TransactionType = account.TransactionType(txType)
		cents, err := numericStringToCents(amountStr)
		if err != nil {
			return nil, fmt.Errorf("parse transaction amount: %w", err)
		}
		tx.Amount = cents
		balCents, err := numericStringToCents(balanceAfterStr)
		if err != nil {
			return nil, fmt.Errorf("parse balance_after: %w", err)
		}
		tx.BalanceAfter = balCents
		txns = append(txns, tx)
	}
	return txns, rows.Err()
}

// Lock acquires a row-level lock on the account (SELECT FOR UPDATE).
func (r *AccountRepository) Lock(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	return r.scanAccount(r.db(ctx).QueryRow(ctx,
		`SELECT id, user_id, balance, currency, version, status, created_at, updated_at
		 FROM accounts WHERE id = $1 FOR UPDATE`, id))
}
