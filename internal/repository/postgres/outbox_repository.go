package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cassiomorais/payments/internal/domain/outbox"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) db(ctx context.Context) DBTX {
	return ConnFromCtx(ctx, r.pool)
}

func (r *OutboxRepository) Insert(ctx context.Context, entry *outbox.Entry) error {
	payload, err := json.Marshal(entry.Payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	_, err = r.db(ctx).Exec(ctx,
		`INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, status, retry_count, max_retries, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.ID, entry.AggregateType, entry.AggregateID, entry.EventType, payload,
		string(entry.Status), entry.RetryCount, entry.MaxRetries, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert outbox entry: %w", err)
	}
	return nil
}

func (r *OutboxRepository) GetPending(ctx context.Context, limit int) ([]*outbox.Entry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db(ctx).Query(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, status, retry_count, max_retries, created_at, published_at
		 FROM outbox WHERE status = 'pending'
		 ORDER BY created_at ASC
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []*outbox.Entry
	for rows.Next() {
		e := &outbox.Entry{}
		var payload []byte
		var status string
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &payload, &status, &e.RetryCount, &e.MaxRetries, &e.CreatedAt, &e.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan outbox entry: %w", err)
		}
		e.Status = outbox.Status(status)
		if len(payload) > 0 {
			e.Payload = make(map[string]any)
			if err := json.Unmarshal(payload, &e.Payload); err != nil {
				return nil, fmt.Errorf("unmarshal outbox payload: %w", err)
			}
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE outbox SET status = 'published', published_at = $1 WHERE id = $2`, now, id,
	)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE outbox SET retry_count = retry_count + 1,
		        status = CASE WHEN retry_count + 1 >= max_retries THEN 'failed' ELSE 'pending' END
		 WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}
