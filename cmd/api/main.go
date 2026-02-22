package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cassiomorais/payments/internal/bootstrap"
	"github.com/cassiomorais/payments/internal/controller"
	"github.com/cassiomorais/payments/internal/providers"
	"github.com/cassiomorais/payments/internal/repository/postgres"
	"github.com/cassiomorais/payments/internal/service"
)

func main() {
	ctx := context.Background()

	app, err := bootstrap.New(ctx, "payments-api", "payments")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bootstrap: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	// --- Repositories ---
	accountRepo := postgres.NewAccountRepository(app.Pool)
	paymentRepo := postgres.NewPaymentRepository(app.Pool)
	outboxRepo := postgres.NewOutboxRepository(app.Pool)
	idempotencyRepo := postgres.NewIdempotencyRepository(app.Pool)
	txManager := postgres.NewTxManager(app.Pool)

	// --- Services ---
	providerFactory := providers.NewFactory()
	accountService := service.NewAccountService(accountRepo)
	paymentService := service.NewPaymentService(paymentRepo, accountRepo, outboxRepo, txManager, providerFactory)

	// --- Build router ---
	router := controller.NewRouter(controller.RouterDeps{
		Pool:            app.Pool,
		RedisClient:     app.Redis,
		PaymentRepo:     paymentRepo,
		AccountService:  accountService,
		PaymentService:  paymentService,
		IdempotencyRepo: idempotencyRepo,
		Metrics:         app.Metrics,
		CORSConfig:      app.Config.Server.CORS,
	})

	// --- HTTP server ---
	addr := fmt.Sprintf(":%d", app.Config.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  app.Config.Server.ReadTimeout,
		WriteTimeout: app.Config.Server.WriteTimeout,
	}

	go func() {
		app.Logger.Info().Str("addr", addr).Msg("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.Logger.Info().Msg("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.Config.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		app.Logger.Error().Err(err).Msg("Server forced to shutdown")
	}
	app.Logger.Info().Msg("Server exited")
}
