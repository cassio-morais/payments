package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Payment       PaymentConfig       `mapstructure:"payment"`
	Worker        WorkerConfig        `mapstructure:"worker"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	InstanceID    string              `mapstructure:"instance_id"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	CORS            CORSConfig    `mapstructure:"cors"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	MaxConnections  int           `mapstructure:"max_connections"`
	MinConnections  int           `mapstructure:"min_connections"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	SSLMode         string        `mapstructure:"ssl_mode"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	DB                int           `mapstructure:"db"`
	Password          string        `mapstructure:"password"`
	ConnectRetries    int           `mapstructure:"connect_retries"`
	ConnectRetryDelay time.Duration `mapstructure:"connect_retry_delay"`
}

// PaymentConfig holds payment processing configuration
type PaymentConfig struct {
	MaxRetries              int           `mapstructure:"max_retries"`
	RetryDelay              time.Duration `mapstructure:"retry_delay"`
	LockTTL                 time.Duration `mapstructure:"lock_ttl"`
	ProcessingTimeout       time.Duration `mapstructure:"processing_timeout"`
	CircuitBreakerThreshold int           `mapstructure:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `mapstructure:"circuit_breaker_timeout"`
}

// WorkerConfig holds worker processing configuration
type WorkerConfig struct {
	BatchSize        int64         `mapstructure:"batch_size"`
	BlockDuration    time.Duration `mapstructure:"block_duration"`
	OutboxPollInterval time.Duration `mapstructure:"outbox_poll_interval"`
	ConsumerGroup    string        `mapstructure:"consumer_group"`
	IdempotencyTTL   time.Duration `mapstructure:"idempotency_ttl"`
}

// ObservabilityConfig holds logging and tracing configuration
type ObservabilityConfig struct {
	LogLevel       string `mapstructure:"log_level"`
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
	EnableMetrics  bool   `mapstructure:"enable_metrics"`
	EnableTracing  bool   `mapstructure:"enable_tracing"`
}

// Load reads configuration from environment variables and config files
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from environment variables
	v.SetEnvPrefix("PAYMENTS")
	v.AutomaticEnv()

	// Read from config file if exists
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/payments")

	// Config file is optional
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that required configuration fields have valid values.
func (c *Config) Validate() error {
	var errs []error

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port))
	}
	if c.Server.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.read_timeout must be positive"))
	}
	if c.Server.WriteTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.write_timeout must be positive"))
	}
	if c.Database.Host == "" {
		errs = append(errs, fmt.Errorf("database.host is required"))
	}
	if c.Database.Port <= 0 {
		errs = append(errs, fmt.Errorf("database.port must be positive"))
	}
	if c.Redis.Port <= 0 {
		errs = append(errs, fmt.Errorf("redis.port must be positive"))
	}
	if c.Payment.LockTTL <= 0 {
		errs = append(errs, fmt.Errorf("payment.lock_ttl must be positive"))
	}
	if c.Worker.BatchSize <= 0 {
		errs = append(errs, fmt.Errorf("worker.batch_size must be positive"))
	}

	return errors.Join(errs...)
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.cors.allowed_origins", []string{"*"})
	v.SetDefault("server.cors.allow_credentials", false)

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "payments")
	v.SetDefault("database.password", "payments")
	v.SetDefault("database.database", "payments")
	v.SetDefault("database.max_connections", 25)
	v.SetDefault("database.min_connections", 5)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("database.ssl_mode", "disable")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.connect_retries", 5)
	v.SetDefault("redis.connect_retry_delay", "1s")

	// Worker defaults
	v.SetDefault("worker.batch_size", 10)
	v.SetDefault("worker.block_duration", "1s")
	v.SetDefault("worker.outbox_poll_interval", "2s")
	v.SetDefault("worker.consumer_group", "payment-processors")
	v.SetDefault("worker.idempotency_ttl", "24h")

	// Payment defaults
	v.SetDefault("payment.max_retries", 3)
	v.SetDefault("payment.retry_delay", "1s")
	v.SetDefault("payment.lock_ttl", "30s")
	v.SetDefault("payment.processing_timeout", "60s")
	v.SetDefault("payment.circuit_breaker_threshold", 10)
	v.SetDefault("payment.circuit_breaker_timeout", "30s")

	// Observability defaults
	v.SetDefault("observability.log_level", "info")
	v.SetDefault("observability.jaeger_endpoint", "http://localhost:14268/api/traces")
	v.SetDefault("observability.enable_metrics", true)
	v.SetDefault("observability.enable_tracing", true)

	// Instance ID
	v.SetDefault("instance_id", "payments-1")
}

// DatabaseDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) DatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// RedisAddr returns the Redis address
func (c *RedisConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
