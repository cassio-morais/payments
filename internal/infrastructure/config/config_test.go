package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "test",
			Password: "test",
			Database: "test_db",
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: 6379,
		},
		Payment: PaymentConfig{
			LockTTL: 30 * time.Second,
		},
		Worker: WorkerConfig{
			BatchSize: 10,
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_InvalidServerPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port too low", 0},
		{"port negative", -1},
		{"port too high", 99999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Port:         tt.port,
					ReadTimeout:  15 * time.Second,
					WriteTimeout: 15 * time.Second,
				},
				Database: DatabaseConfig{Host: "localhost", Port: 5432},
				Redis:    RedisConfig{Port: 6379},
				Payment:  PaymentConfig{LockTTL: 30 * time.Second},
				Worker:   WorkerConfig{BatchSize: 10},
			}

			err := cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "server.port")
		})
	}
}

func TestConfig_Validate_InvalidReadTimeout(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  0, // Invalid
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{Host: "localhost", Port: 5432},
		Redis:    RedisConfig{Port: 6379},
		Payment:  PaymentConfig{LockTTL: 30 * time.Second},
		Worker:   WorkerConfig{BatchSize: 10},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read_timeout")
}

func TestConfig_Validate_InvalidWriteTimeout(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0, // Invalid
		},
		Database: DatabaseConfig{Host: "localhost", Port: 5432},
		Redis:    RedisConfig{Port: 6379},
		Payment:  PaymentConfig{LockTTL: 30 * time.Second},
		Worker:   WorkerConfig{BatchSize: 10},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write_timeout")
}

func TestConfig_Validate_MissingDatabaseHost(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			Host: "", // Invalid
			Port: 5432,
		},
		Redis:   RedisConfig{Port: 6379},
		Payment: PaymentConfig{LockTTL: 30 * time.Second},
		Worker:  WorkerConfig{BatchSize: 10},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database.host")
}

func TestConfig_Validate_InvalidDatabasePort(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			Host: "localhost",
			Port: 0, // Invalid
		},
		Redis:   RedisConfig{Port: 6379},
		Payment: PaymentConfig{LockTTL: 30 * time.Second},
		Worker:  WorkerConfig{BatchSize: 10},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database.port")
}

func TestConfig_Validate_InvalidRedisPort(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{Host: "localhost", Port: 5432},
		Redis: RedisConfig{
			Port: 0, // Invalid
		},
		Payment: PaymentConfig{LockTTL: 30 * time.Second},
		Worker:  WorkerConfig{BatchSize: 10},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis.port")
}

func TestConfig_Validate_InvalidPaymentLockTTL(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{Host: "localhost", Port: 5432},
		Redis:    RedisConfig{Port: 6379},
		Payment: PaymentConfig{
			LockTTL: 0, // Invalid
		},
		Worker: WorkerConfig{BatchSize: 10},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payment.lock_ttl")
}

func TestConfig_Validate_InvalidWorkerBatchSize(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{Host: "localhost", Port: 5432},
		Redis:    RedisConfig{Port: 6379},
		Payment:  PaymentConfig{LockTTL: 30 * time.Second},
		Worker: WorkerConfig{
			BatchSize: 0, // Invalid
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker.batch_size")
}

func TestConfig_Validate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         0, // Invalid
			ReadTimeout:  0, // Invalid
			WriteTimeout: 0, // Invalid
		},
		Database: DatabaseConfig{
			Host: "", // Invalid
			Port: 0,  // Invalid
		},
		Redis: RedisConfig{
			Port: 0, // Invalid
		},
		Payment: PaymentConfig{
			LockTTL: 0, // Invalid
		},
		Worker: WorkerConfig{
			BatchSize: 0, // Invalid
		},
	}

	err := cfg.Validate()
	require.Error(t, err)

	// Should contain multiple error messages
	errStr := err.Error()
	assert.Contains(t, errStr, "server.port")
	assert.Contains(t, errStr, "read_timeout")
	assert.Contains(t, errStr, "write_timeout")
	assert.Contains(t, errStr, "database.host")
	assert.Contains(t, errStr, "database.port")
	assert.Contains(t, errStr, "redis.port")
	assert.Contains(t, errStr, "payment.lock_ttl")
	assert.Contains(t, errStr, "worker.batch_size")
}

func TestServerConfig_ValidPorts(t *testing.T) {
	validPorts := []int{80, 443, 8080, 8443, 3000, 5000, 9000, 65535}

	for _, port := range validPorts {
		t.Run("port_valid", func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Port:         port,
					ReadTimeout:  15 * time.Second,
					WriteTimeout: 15 * time.Second,
				},
				Database: DatabaseConfig{Host: "localhost", Port: 5432},
				Redis:    RedisConfig{Port: 6379},
				Payment:  PaymentConfig{LockTTL: 30 * time.Second},
				Worker:   WorkerConfig{BatchSize: 10},
			}

			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestDatabaseConfig_Fields(t *testing.T) {
	cfg := DatabaseConfig{
		Host:            "db.example.com",
		Port:            5432,
		User:            "app_user",
		Password:        "secret",
		Database:        "payments_db",
		MaxConnections:  50,
		MinConnections:  10,
		ConnMaxLifetime: 1 * time.Hour,
		SSLMode:         "require",
	}

	assert.Equal(t, "db.example.com", cfg.Host)
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "app_user", cfg.User)
	assert.Equal(t, "secret", cfg.Password)
	assert.Equal(t, "payments_db", cfg.Database)
	assert.Equal(t, 50, cfg.MaxConnections)
	assert.Equal(t, 10, cfg.MinConnections)
	assert.Equal(t, 1*time.Hour, cfg.ConnMaxLifetime)
	assert.Equal(t, "require", cfg.SSLMode)
}

func TestRedisConfig_Fields(t *testing.T) {
	cfg := RedisConfig{
		Host:              "redis.example.com",
		Port:              6379,
		DB:                1,
		Password:          "redis_secret",
		ConnectRetries:    3,
		ConnectRetryDelay: 2 * time.Second,
	}

	assert.Equal(t, "redis.example.com", cfg.Host)
	assert.Equal(t, 6379, cfg.Port)
	assert.Equal(t, 1, cfg.DB)
	assert.Equal(t, "redis_secret", cfg.Password)
	assert.Equal(t, 3, cfg.ConnectRetries)
	assert.Equal(t, 2*time.Second, cfg.ConnectRetryDelay)
}

func TestPaymentConfig_Fields(t *testing.T) {
	cfg := PaymentConfig{
		MaxRetries:              5,
		RetryDelay:              3 * time.Second,
		LockTTL:                 60 * time.Second,
		ProcessingTimeout:       120 * time.Second,
		CircuitBreakerThreshold: 20,
		CircuitBreakerTimeout:   60 * time.Second,
	}

	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 3*time.Second, cfg.RetryDelay)
	assert.Equal(t, 60*time.Second, cfg.LockTTL)
	assert.Equal(t, 120*time.Second, cfg.ProcessingTimeout)
	assert.Equal(t, 20, cfg.CircuitBreakerThreshold)
	assert.Equal(t, 60*time.Second, cfg.CircuitBreakerTimeout)
}

func TestWorkerConfig_Fields(t *testing.T) {
	cfg := WorkerConfig{
		BatchSize:          20,
		BlockDuration:      5 * time.Second,
		OutboxPollInterval: 10 * time.Second,
		ConsumerGroup:      "my-workers",
		IdempotencyTTL:     48 * time.Hour,
	}

	assert.Equal(t, int64(20), cfg.BatchSize)
	assert.Equal(t, 5*time.Second, cfg.BlockDuration)
	assert.Equal(t, 10*time.Second, cfg.OutboxPollInterval)
	assert.Equal(t, "my-workers", cfg.ConsumerGroup)
	assert.Equal(t, 48*time.Hour, cfg.IdempotencyTTL)
}

func TestCORSConfig_Fields(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins:   []string{"https://example.com", "https://app.example.com"},
		AllowCredentials: true,
	}

	assert.Equal(t, []string{"https://example.com", "https://app.example.com"}, cfg.AllowedOrigins)
	assert.True(t, cfg.AllowCredentials)
}
