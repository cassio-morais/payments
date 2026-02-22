package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all application metrics
type Metrics struct {
	// Payment metrics
	PaymentsTotal          *prometheus.CounterVec
	PaymentDuration        *prometheus.HistogramVec
	ActivePayments         prometheus.Gauge
	PaymentRetries         *prometheus.CounterVec
	PaymentErrors          *prometheus.CounterVec

	// Account metrics
	AccountBalance         *prometheus.GaugeVec
	AccountTransactions    *prometheus.CounterVec

	// HTTP metrics
	HTTPRequestsTotal      *prometheus.CounterVec
	HTTPRequestDuration    *prometheus.HistogramVec

	// Circuit breaker metrics
	CircuitBreakerState    *prometheus.GaugeVec
	CircuitBreakerRequests *prometheus.CounterVec

	// Worker metrics
	WorkerMessagesProcessed  *prometheus.CounterVec
	WorkerProcessingDuration *prometheus.HistogramVec
}

// NewMetrics creates and registers all metrics against the given registry.
// If reg is nil, prometheus.DefaultRegisterer is used.
func NewMetrics(namespace string, reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	factory := prometheus.WrapRegistererWith(nil, reg)

	m := &Metrics{
		PaymentsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "payments_total",
				Help:      "Total number of payments by type and status",
			},
			[]string{"type", "status"},
		),
		PaymentDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "payment_duration_seconds",
				Help:      "Payment processing duration in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"type", "status"},
		),
		ActivePayments: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_payments",
				Help:      "Number of currently active payments",
			},
		),
		PaymentRetries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "payment_retries_total",
				Help:      "Total number of payment retries",
			},
			[]string{"type"},
		),
		PaymentErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "payment_errors_total",
				Help:      "Total number of payment errors",
			},
			[]string{"type", "error_type"},
		),
		AccountBalance: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "account_balance",
				Help:      "Current account balance",
			},
			[]string{"account_id", "currency"},
		),
		AccountTransactions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "account_transactions_total",
				Help:      "Total number of account transactions",
			},
			[]string{"type"},
		),
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		CircuitBreakerState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "circuit_breaker_state",
				Help:      "Circuit breaker state (0=closed, 1=half-open, 2=open)",
			},
			[]string{"name"},
		),
		CircuitBreakerRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "circuit_breaker_requests_total",
				Help:      "Total number of circuit breaker requests",
			},
			[]string{"name", "result"},
		),
		WorkerMessagesProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "worker_messages_processed_total",
				Help:      "Total number of worker messages processed",
			},
			[]string{"stream", "status"},
		),
		WorkerProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "worker_processing_duration_seconds",
				Help:      "Worker message processing duration in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"stream"},
		),
	}

	// Register all collectors
	factory.MustRegister(
		m.PaymentsTotal,
		m.PaymentDuration,
		m.ActivePayments,
		m.PaymentRetries,
		m.PaymentErrors,
		m.AccountBalance,
		m.AccountTransactions,
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.CircuitBreakerState,
		m.CircuitBreakerRequests,
		m.WorkerMessagesProcessed,
		m.WorkerProcessingDuration,
	)

	return m
}
