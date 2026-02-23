package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ctxKey is an unexported type for context keys in this package.
type ctxKey int

const txKey ctxKey = iota

type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// The transaction is committed if fn returns nil, rolled back otherwise.
func (m *TxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed (%v) after error: %w", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func ConnFromCtx(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return tx
	}
	return pool
}
