package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// allowedSortColumns is a whitelist of columns valid for ORDER BY.
var allowedSortColumns = map[string]string{
	"created_at": "created_at",
	"amount":     "amount",
	"status":     "status",
	"updated_at": "updated_at",
}

// PaymentRepository implements payment.Repository using PostgreSQL.
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) db(ctx context.Context) DBTX {
	return ConnFromCtx(ctx, r.pool)
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// Create inserts a new payment.
func (r *PaymentRepository) Create(ctx context.Context, p *payment.Payment) error {
	metadata, err := json.Marshal(p.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	var providerStr *string
	if p.Provider != nil {
		s := string(*p.Provider)
		providerStr = &s
	}

	amountStr := centsToNumericString(p.Amount.ValueCents)

	_, err = r.db(ctx).Exec(ctx,
		`INSERT INTO payments
		 (id, idempotency_key, payment_type, source_account_id, destination_account_id,
		  amount, currency, status, provider, provider_transaction_id,
		  retry_count, max_retries, last_error, saga_id, saga_step, metadata, created_at, updated_at, completed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		p.ID, p.IdempotencyKey, string(p.PaymentType), p.SourceAccountID, p.DestinationAccountID,
		amountStr, p.Amount.Currency, string(p.Status), providerStr, p.ProviderTransactionID,
		p.RetryCount, p.MaxRetries, p.LastError, p.SagaID, p.SagaStep, metadata, p.CreatedAt, p.UpdatedAt, p.CompletedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domainErrors.ErrDuplicateIdempotencyKey
		}
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

// GetByID retrieves a payment by its ID.
func (r *PaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*payment.Payment, error) {
	return r.scanPayment(r.db(ctx).QueryRow(ctx,
		`SELECT id, idempotency_key, payment_type, source_account_id, destination_account_id,
		        amount, currency, status, provider, provider_transaction_id,
		        retry_count, max_retries, last_error, saga_id, saga_step, metadata, created_at, updated_at, completed_at
		 FROM payments WHERE id = $1`, id))
}

// GetByIdempotencyKey retrieves a payment by idempotency key.
func (r *PaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*payment.Payment, error) {
	return r.scanPayment(r.db(ctx).QueryRow(ctx,
		`SELECT id, idempotency_key, payment_type, source_account_id, destination_account_id,
		        amount, currency, status, provider, provider_transaction_id,
		        retry_count, max_retries, last_error, saga_id, saga_step, metadata, created_at, updated_at, completed_at
		 FROM payments WHERE idempotency_key = $1`, key))
}

// Update updates an existing payment.
func (r *PaymentRepository) Update(ctx context.Context, p *payment.Payment) error {
	metadata, err := json.Marshal(p.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	var providerStr *string
	if p.Provider != nil {
		s := string(*p.Provider)
		providerStr = &s
	}

	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE payments SET
		  status=$1, provider=$2, provider_transaction_id=$3,
		  retry_count=$4, last_error=$5, saga_id=$6, saga_step=$7,
		  metadata=$8, updated_at=$9, completed_at=$10
		 WHERE id=$11`,
		string(p.Status), providerStr, p.ProviderTransactionID,
		p.RetryCount, p.LastError, p.SagaID, p.SagaStep,
		metadata, p.UpdatedAt, p.CompletedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainErrors.ErrPaymentNotFound
	}
	return nil
}

// List lists payments with optional filters.
func (r *PaymentRepository) List(ctx context.Context, f payment.ListFilter) ([]*payment.Payment, error) {
	query := `SELECT id, idempotency_key, payment_type, source_account_id, destination_account_id,
		        amount, currency, status, provider, provider_transaction_id,
		        retry_count, max_retries, last_error, saga_id, saga_step, metadata, created_at, updated_at, completed_at
		 FROM payments WHERE 1=1`
	args := []any{}
	argIdx := 1

	if f.AccountID != nil {
		query += fmt.Sprintf(" AND (source_account_id = $%d OR destination_account_id = $%d)", argIdx, argIdx)
		args = append(args, *f.AccountID)
		argIdx++
	}
	if f.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(*f.Status))
		argIdx++
	}
	if f.Provider != nil {
		query += fmt.Sprintf(" AND provider = $%d", argIdx)
		args = append(args, string(*f.Provider))
		argIdx++
	}

	// Strict whitelist for sort column
	sortBy := "created_at"
	if col, ok := allowedSortColumns[f.SortBy]; ok {
		sortBy = col
	}
	sortOrder := "DESC"
	if strings.EqualFold(f.SortOrder, "asc") {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := r.db(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var payments []*payment.Payment
	for rows.Next() {
		p, err := r.scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// AddEvent inserts a payment event.
func (r *PaymentRepository) AddEvent(ctx context.Context, event *payment.PaymentEvent) error {
	data, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("marshal event data: %w", err)
	}
	_, err = r.db(ctx).Exec(ctx,
		`INSERT INTO payment_events (id, payment_id, event_type, event_data, created_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		event.ID, event.PaymentID, event.EventType, data,
	)
	if err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}
	return nil
}

// GetEvents retrieves events for a payment.
func (r *PaymentRepository) GetEvents(ctx context.Context, paymentID uuid.UUID) ([]*payment.PaymentEvent, error) {
	rows, err := r.db(ctx).Query(ctx,
		`SELECT id, payment_id, event_type, event_data, created_at
		 FROM payment_events WHERE payment_id = $1 ORDER BY created_at ASC`, paymentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list payment events: %w", err)
	}
	defer rows.Close()

	var events []*payment.PaymentEvent
	for rows.Next() {
		e := &payment.PaymentEvent{}
		var data []byte
		if err := rows.Scan(&e.ID, &e.PaymentID, &e.EventType, &data, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if err := json.Unmarshal(data, &e.EventData); err != nil {
			return nil, fmt.Errorf("unmarshal event data: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- scanning helpers ---

// scanPayment scans a payment from any source implementing the scanner interface.
func (r *PaymentRepository) scanPayment(s scanner) (*payment.Payment, error) {
	p := &payment.Payment{Metadata: make(map[string]any)}
	var (
		paymentType string
		amountStr   string
		status      string
		provider    *string
		metadata    []byte
	)
	err := s.Scan(
		&p.ID, &p.IdempotencyKey, &paymentType, &p.SourceAccountID, &p.DestinationAccountID,
		&amountStr, &p.Amount.Currency, &status, &provider, &p.ProviderTransactionID,
		&p.RetryCount, &p.MaxRetries, &p.LastError, &p.SagaID, &p.SagaStep, &metadata, &p.CreatedAt, &p.UpdatedAt, &p.CompletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domainErrors.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("scan payment: %w", err)
	}

	cents, err := numericStringToCents(amountStr)
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}
	p.Amount.ValueCents = cents

	p.PaymentType = payment.PaymentType(paymentType)
	p.Status = payment.PaymentStatus(status)
	if provider != nil {
		prov := payment.Provider(*provider)
		p.Provider = &prov
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &p.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal payment metadata: %w", err)
		}
	}
	return p, nil
}
