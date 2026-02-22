package service

import "context"

// TransactionManager defines the interface for transaction management.
// Services use this to wrap multiple repository operations in a single transaction.
type TransactionManager interface {
	// WithTransaction executes the given function within a database transaction.
	// If fn returns an error, the transaction is rolled back.
	// Otherwise, it is committed.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
