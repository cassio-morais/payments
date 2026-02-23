package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/cassiomorais/payments/internal/infrastructure/config"
	"github.com/cassiomorais/payments/internal/infrastructure/observability"
	"github.com/cassiomorais/payments/internal/repository/postgres"
	infraRedis "github.com/cassiomorais/payments/internal/infrastructure/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type App struct {
	Config  *config.Config
	Logger  zerolog.Logger
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Metrics *observability.Metrics
}

func New(ctx context.Context, serviceName string, metricsNamespace string) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logger := observability.InitLogger(cfg.Observability.LogLevel, os.Stdout)
	logger.Info().Str("service", serviceName).Msg("Starting")

	if cfg.Observability.EnableTracing {
		tp, err := observability.InitTracer(serviceName, cfg.Observability.JaegerEndpoint)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize tracer, continuing without tracing")
		} else {
			go func() {
				<-ctx.Done()
				observability.Shutdown(context.Background(), tp)
			}()
			logger.Info().Msg("Tracing enabled")
		}
	}

	metrics := observability.NewMetrics(metricsNamespace, nil)
	logger.Info().Msg("Metrics initialized")

	pool, err := postgres.NewPool(ctx, &cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	logger.Info().Msg("Connected to PostgreSQL")

	redisClient, err := infraRedis.NewClient(ctx, &cfg.Redis)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to redis: %w", err)
	}
	logger.Info().Msg("Connected to Redis")

	return &App{
		Config:  cfg,
		Logger:  logger,
		Pool:    pool,
		Redis:   redisClient,
		Metrics: metrics,
	}, nil
}

func (a *App) Close() {
	a.Redis.Close()
	a.Pool.Close()
}
