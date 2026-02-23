package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyEntry struct {
	Key            string
	ResponseBody   string
	ResponseStatus int
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

type IdempotencyRepository struct {
	pool *pgxpool.Pool
}

func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{pool: pool}
}

func (r *IdempotencyRepository) db(ctx context.Context) DBTX {
	return ConnFromCtx(ctx, r.pool)
}

func (r *IdempotencyRepository) Get(ctx context.Context, key string) (*IdempotencyEntry, error) {
	e := &IdempotencyEntry{}
	err := r.db(ctx).QueryRow(ctx,
		`SELECT key, response_body, response_status, created_at, expires_at
		 FROM idempotency_keys WHERE key = $1 AND expires_at > NOW()`, key,
	).Scan(&e.Key, &e.ResponseBody, &e.ResponseStatus, &e.CreatedAt, &e.ExpiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // not found
		}
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}
	return e, nil
}

func (r *IdempotencyRepository) Set(ctx context.Context, entry *IdempotencyEntry) error {
	_, err := r.db(ctx).Exec(ctx,
		`INSERT INTO idempotency_keys (key, response_body, response_status, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (key) DO UPDATE SET response_body = EXCLUDED.response_body, response_status = EXCLUDED.response_status`,
		entry.Key, entry.ResponseBody, entry.ResponseStatus, entry.CreatedAt, entry.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("set idempotency key: %w", err)
	}
	return nil
}

func (r *IdempotencyRepository) Cleanup(ctx context.Context) (int64, error) {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("cleanup idempotency keys: %w", err)
	}
	return tag.RowsAffected(), nil
}
