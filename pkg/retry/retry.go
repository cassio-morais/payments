package retry

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts uint
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultConfig returns default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// Do executes a function with exponential backoff retry
func Do(ctx context.Context, cfg Config, fn func() error) error {
	return retry.Do(
		fn,
		retry.Context(ctx),
		retry.Attempts(cfg.MaxAttempts),
		retry.Delay(cfg.InitialDelay),
		retry.MaxDelay(cfg.MaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			// Log retry attempts if needed
		}),
	)
}

// DoWithResult executes a function with exponential backoff retry and returns a result
func DoWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	err := Do(ctx, cfg, func() error {
		var err error
		result, err = fn()
		return err
	})
	return result, err
}
