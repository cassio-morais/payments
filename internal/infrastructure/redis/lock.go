package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	// Lua script for safe lock release (only owner can release)
	releaseLockScript = redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	// Lua script for lock extension
	extendLockScript = redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
)

// DistributedLock represents a distributed lock using Redis
type DistributedLock struct {
	client  *redis.Client
	key     string
	value   string
	ttl     time.Duration
	acquired bool
}

// NewDistributedLock creates a new distributed lock
func NewDistributedLock(client *redis.Client, key string, ttl time.Duration) *DistributedLock {
	return &DistributedLock{
		client:   client,
		key:      fmt.Sprintf("lock:%s", key),
		value:    uuid.New().String(),
		ttl:      ttl,
		acquired: false,
	}
}

// Acquire attempts to acquire the lock
func (l *DistributedLock) Acquire(ctx context.Context) (bool, error) {
	// Use SET NX EX to atomically set the lock if it doesn't exist
	success, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	l.acquired = success
	return success, nil
}

// AcquireWithRetry attempts to acquire the lock with retries
func (l *DistributedLock) AcquireWithRetry(ctx context.Context, maxRetries int, retryDelay time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		acquired, err := l.Acquire(ctx)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
			continue
		}
	}

	return errors.New("failed to acquire lock after retries")
}

// Extend extends the lock TTL
func (l *DistributedLock) Extend(ctx context.Context, additionalTTL time.Duration) error {
	if !l.acquired {
		return errors.New("lock not acquired")
	}

	result, err := extendLockScript.Run(
		ctx,
		l.client,
		[]string{l.key},
		l.value,
		additionalTTL.Milliseconds(),
	).Result()
	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	val, ok := result.(int64)
	if !ok || val == 0 {
		return errors.New("lock not held or expired")
	}

	return nil
}

// Release releases the lock
func (l *DistributedLock) Release(ctx context.Context) error {
	if !l.acquired {
		return nil
	}

	result, err := releaseLockScript.Run(
		ctx,
		l.client,
		[]string{l.key},
		l.value,
	).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	val, ok := result.(int64)
	if !ok || val == 0 {
		return errors.New("lock not held or already released")
	}

	l.acquired = false
	return nil
}

// IsAcquired returns whether the lock is acquired
func (l *DistributedLock) IsAcquired() bool {
	return l.acquired
}
