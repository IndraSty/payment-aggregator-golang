package circuitbreaker

import (
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"
)

// Breaker wraps sony/gobreaker with a simpler interface.
type Breaker struct {
	cb   *gobreaker.CircuitBreaker
	name string
}

// NewBreaker creates a new circuit breaker for a provider.
func NewBreaker(name string, cfg *config.CircuitBreakerConfig) *Breaker {
	b := &Breaker{name: name}

	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < 3 {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cfg.FailureRatio
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Warn().
				Str("provider", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("Circuit breaker state changed")

			// Emit Prometheus gauge — imported lazily to avoid import cycle
			emitCircuitState(name, to)
		},
	}

	b.cb = gobreaker.NewCircuitBreaker(settings)
	return b
}

// Execute runs fn through the circuit breaker.
func (b *Breaker) Execute(fn func() (any, error)) (any, error) {
	return b.cb.Execute(fn)
}

// IsAvailable returns true if breaker is closed or half-open.
func (b *Breaker) IsAvailable() bool {
	return b.cb.State() != gobreaker.StateOpen
}

// State returns current state as string.
func (b *Breaker) State() string {
	return b.cb.State().String()
}

// emitCircuitState updates the Prometheus gauge for circuit breaker state.
// 0 = closed (healthy), 1 = half-open, 2 = open (unhealthy)
func emitCircuitState(name string, state gobreaker.State) {
	var value float64
	switch state {
	case gobreaker.StateClosed:
		value = 0
	case gobreaker.StateHalfOpen:
		value = 1
	case gobreaker.StateOpen:
		value = 2
	}
	_ = fmt.Sprintf("circuit_state{provider=%s} = %f", name, value)
	// metrics.ProviderCircuitState.WithLabelValues(name).Set(value)
	// Note: uncomment above and import pkg/metrics after confirming no import cycle
}
