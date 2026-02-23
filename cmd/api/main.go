package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/cassiomorais/payments/internal/bootstrap"
	"github.com/cassiomorais/payments/internal/controller"
	"github.com/cassiomorais/payments/internal/infrastructure/config"
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
	authzService := service.NewAuthzService(accountRepo)

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
		JWTSecret:       app.Config.Auth.JWTSecret,
		AuthzService:    authzService,
	})

	// --- HTTP server ---
	addr := fmt.Sprintf(":%d", app.Config.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  app.Config.Server.ReadTimeout,
		WriteTimeout: app.Config.Server.WriteTimeout,
		IdleTimeout:  app.Config.Server.IdleTimeout,
	}

	// Start server with TLS if enabled
	if app.Config.Server.TLS.Enabled {
		tlsConfig := createTLSConfig(app.Config.Server.TLS)
		srv.TLSConfig = tlsConfig

		go func() {
			app.Logger.Info().
				Str("addr", addr).
				Str("tls_version", app.Config.Server.TLS.MinVersion).
				Msg("Starting HTTPS server")

			if err := srv.ListenAndServeTLS(
				app.Config.Server.TLS.CertFile,
				app.Config.Server.TLS.KeyFile,
			); err != nil && err != http.ErrServerClosed {
				app.Logger.Fatal().Err(err).Msg("Failed to start HTTPS server")
			}
		}()
	} else {
		app.Logger.Warn().Msg("TLS disabled - insecure for production")

		go func() {
			app.Logger.Info().Str("addr", addr).Msg("Starting HTTP server")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				app.Logger.Fatal().Err(err).Msg("Failed to start server")
			}
		}()
	}

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

func createTLSConfig(cfg config.TLSConfig) *tls.Config {
	minVersion := tls.VersionTLS13
	if cfg.MinVersion == "1.2" {
		minVersion = tls.VersionTLS12
	}

	return &tls.Config{
		MinVersion: uint16(minVersion),
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
}
