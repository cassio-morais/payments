package providers

import (
	"fmt"
	"time"

	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/sony/gobreaker/v2"
)

type Factory struct {
	providers       map[string]Provider
	circuitBreakers map[string]*gobreaker.CircuitBreaker[*ProviderResult]
}

func NewFactory(providersList ...Provider) *Factory {
	f := &Factory{
		providers:       make(map[string]Provider),
		circuitBreakers: make(map[string]*gobreaker.CircuitBreaker[*ProviderResult]),
	}

	if len(providersList) == 0 {
		f.Register(NewMockProvider("stripe",
			WithLatency(200*time.Millisecond),
			WithFailureRate(0.05),
		))
		f.Register(NewMockProvider("paypal",
			WithLatency(300*time.Millisecond),
			WithFailureRate(0.08),
		))
	} else {
		for _, p := range providersList {
			f.Register(p)
		}
	}

	return f
}

func (f *Factory) Register(p Provider) {
	f.providers[p.Name()] = p
	f.circuitBreakers[p.Name()] = gobreaker.NewCircuitBreaker[*ProviderResult](gobreaker.Settings{
		Name:        p.Name(),
		MaxRequests: 10,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.6
		},
	})
}

func (f *Factory) Get(name payment.Provider) (Provider, *gobreaker.CircuitBreaker[*ProviderResult], error) {
	p, ok := f.providers[string(name)]
	if !ok {
		return nil, nil, fmt.Errorf("unknown provider %q: %w", name, fmt.Errorf("provider not found"))
	}
	breaker := f.circuitBreakers[string(name)]
	return p, breaker, nil
}
