package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	paymentApp "github.com/cassiomorais/payments/internal/application/payment"
	"github.com/cassiomorais/payments/internal/bootstrap"
	"github.com/cassiomorais/payments/internal/infrastructure/postgres"
	"github.com/cassiomorais/payments/internal/infrastructure/providers"
	infraRedis "github.com/cassiomorais/payments/internal/infrastructure/redis"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.New(ctx, "payments-worker", "payments_worker")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bootstrap: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	// --- Repositories ---
	paymentRepo := postgres.NewPaymentRepository(app.Pool)
	accountRepo := postgres.NewAccountRepository(app.Pool)
	outboxRepo := postgres.NewOutboxRepository(app.Pool)
	txManager := postgres.NewTxManager(app.Pool)
	providerFactory := providers.NewFactory()
	streamProducer := infraRedis.NewStreamProducer(app.Redis)

	// --- Use cases ---
	processPaymentUC := paymentApp.NewProcessPaymentUseCase(paymentRepo, accountRepo, txManager, providerFactory)

	// --- Payment stream consumer ---
	workerCfg := app.Config.Worker
	consumer := infraRedis.NewStreamConsumer(
		app.Redis,
		infraRedis.PaymentStream,
		workerCfg.ConsumerGroup,
		app.Config.InstanceID,
		workerCfg.BatchSize,
		workerCfg.BlockDuration,
	)
	if err := consumer.CreateGroup(ctx); err != nil {
		app.Logger.Error().Err(err).Msg("Failed to create consumer group (may already exist)")
	}

	app.Logger.Info().
		Str("stream", infraRedis.PaymentStream).
		Str("group", workerCfg.ConsumerGroup).
		Str("consumer", app.Config.InstanceID).
		Msg("Worker started, listening for messages...")

	// Signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	g, gCtx := errgroup.WithContext(ctx)

	// 1. Payment processor (reads from Redis Streams).
	g.Go(func() error {
		return runPaymentProcessor(gCtx, app.Logger, consumer, processPaymentUC, app)
	})

	// 2. Outbox processor (polls outbox table and publishes to Redis Streams).
	g.Go(func() error {
		return runOutboxProcessor(gCtx, app.Logger, txManager, outboxRepo, streamProducer, workerCfg.OutboxPollInterval)
	})

	// 3. Wait for shutdown signal.
	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		case <-quit:
			app.Logger.Info().Msg("Shutting down worker...")
			cancel()
			return nil
		}
	})

	if err := g.Wait(); err != nil && err != context.Canceled {
		app.Logger.Error().Err(err).Msg("Worker error")
	}
	app.Logger.Info().Msg("Worker exited")
}

func runPaymentProcessor(
	ctx context.Context,
	logger zerolog.Logger,
	consumer *infraRedis.StreamConsumer,
	processUC *paymentApp.ProcessPaymentUseCase,
	app *bootstrap.App,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		streams, err := consumer.Read(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read from stream")
			time.Sleep(1 * time.Second)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				paymentIDStr, _ := msg.Values["payment_id"].(string)
				paymentID, err := uuid.Parse(paymentIDStr)
				if err != nil {
					logger.Error().Str("raw", paymentIDStr).Msg("Invalid payment ID in stream message")
					consumer.Ack(ctx, msg.ID)
					continue
				}

				lock := infraRedis.NewDistributedLock(app.Redis, "payment:"+paymentID.String(), app.Config.Payment.LockTTL)
				acquired, err := lock.Acquire(ctx)
				if err != nil || !acquired {
					logger.Warn().Str("payment_id", paymentID.String()).Msg("Could not acquire lock, skipping")
					continue
				}

				logger.Info().Str("payment_id", paymentID.String()).Msg("Processing payment")

				if err := processUC.Execute(ctx, paymentID); err != nil {
					logger.Error().Err(err).Str("payment_id", paymentID.String()).Msg("Failed to process payment")
					app.Metrics.PaymentErrors.WithLabelValues("external_payment", "processing_error").Inc()
				} else {
					app.Metrics.WorkerMessagesProcessed.WithLabelValues(infraRedis.PaymentStream, "success").Inc()
				}

				lock.Release(ctx)
				consumer.Ack(ctx, msg.ID)
			}
		}
	}
}

func runOutboxProcessor(
	ctx context.Context,
	logger zerolog.Logger,
	txManager *postgres.TxManager,
	outboxRepo *postgres.OutboxRepository,
	streamProducer *infraRedis.StreamProducer,
	pollInterval time.Duration,
) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		err := txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			entries, err := outboxRepo.GetPending(txCtx, 10)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := streamProducer.PublishPaymentEvent(
					ctx, entry.AggregateID.String(), entry.EventType, entry.Payload,
				); err != nil {
					logger.Error().Err(err).Str("outbox_id", entry.ID.String()).Msg("Failed to publish outbox event")
					outboxRepo.MarkFailed(txCtx, entry.ID)
					continue
				}
				outboxRepo.MarkPublished(txCtx, entry.ID)
			}
			return nil
		})
		if err != nil {
			logger.Error().Err(err).Msg("Outbox processor error")
		}
	}
}
