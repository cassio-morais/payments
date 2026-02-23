package controller

import (
	"time"

	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/infrastructure/config"
	"github.com/cassiomorais/payments/internal/infrastructure/observability"
	customMW "github.com/cassiomorais/payments/internal/middleware"
	"github.com/cassiomorais/payments/internal/repository/postgres"
	"github.com/cassiomorais/payments/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type RouterDeps struct {
	Pool            *pgxpool.Pool
	RedisClient     *redis.Client
	PaymentRepo     payment.Repository
	AccountService  *service.AccountService
	PaymentService  *service.PaymentService
	IdempotencyRepo *postgres.IdempotencyRepository
	Metrics         *observability.Metrics
	CORSConfig      config.CORSConfig
}

func NewRouter(deps RouterDeps) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(customMW.Tracing())
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.CORSConfig.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: deps.CORSConfig.AllowCredentials,
		MaxAge:           300,
	}))
	r.Use(customMW.Metrics(deps.Metrics))

	healthH := NewHealthController(deps.Pool, deps.RedisClient)
	accountH := NewAccountController(deps.AccountService)
	paymentH := NewPaymentController(deps.PaymentService, deps.PaymentRepo)

	r.Get("/health", healthH.Health)
	r.Get("/health/live", healthH.Liveness)
	r.Get("/health/ready", healthH.Readiness)

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		// Idempotency middleware for mutating endpoints.
		idempotencyMW := customMW.Idempotency(deps.IdempotencyRepo)

		// Accounts
		r.Post("/accounts", accountH.Create)
		r.Get("/accounts/{id}", accountH.Get)
		r.Get("/accounts/{id}/balance", accountH.GetBalance)
		r.Get("/accounts/{id}/transactions", accountH.GetTransactions)

		// Payments
		r.With(idempotencyMW).Post("/payments", paymentH.CreatePayment)
		r.Get("/payments/{id}", paymentH.GetPayment)
		r.Get("/payments", paymentH.ListPayments)
		r.Post("/payments/{id}/refund", paymentH.RefundPayment)
		r.Post("/payments/{id}/cancel", paymentH.CancelPayment)

		// Transfers
		r.With(idempotencyMW).Post("/transfers", paymentH.Transfer)
	})

	return r
}
