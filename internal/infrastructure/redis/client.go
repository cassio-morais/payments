package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/cassiomorais/payments/internal/infrastructure/config"
	"github.com/redis/go-redis/v9"
)

// NewClient creates a new Redis client with configurable retry logic
func NewClient(ctx context.Context, cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	maxRetries := cfg.ConnectRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}
	retryDelay := cfg.ConnectRetryDelay
	if retryDelay <= 0 {
		retryDelay = 1 * time.Second
	}

	for i := 0; i < maxRetries; i++ {
		if err := client.Ping(ctx).Err(); err != nil {
			if i == maxRetries-1 {
				client.Close()
				return nil, fmt.Errorf("failed to connect to Redis after %d retries: %w", maxRetries, err)
			}
			time.Sleep(time.Duration(i+1) * retryDelay)
			continue
		}
		break
	}

	return client, nil
}
