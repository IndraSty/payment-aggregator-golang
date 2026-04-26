package circuitbreaker

import (
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/sony/gobreaker"
)

// Breaker wraps sony/gobreaker with a simpler interface.
type Breaker struct {
	cb *gobreaker.CircuitBreaker
}

// NewBreaker creates a new circuit breaker for a provider.
func NewBreaker(name string, cfg *config.CircuitBreakerConfig) *Breaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: cfg.MaxRequests, // max requests in half-open state
		Interval:    cfg.Interval,    // time window for counting failures
		Timeout:     cfg.Timeout,     // time in open state before going half-open

		// Open the circuit when failure ratio exceeds threshold
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < 3 {
				return false // need at least 3 requests before tripping
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cfg.FailureRatio
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// Logged by the caller — we don't import logger here to avoid cycle
			_ = fmt.Sprintf("circuit breaker %s: %s → %s", name, from, to)
		},
	}

	return &Breaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

// Execute runs the given function through the circuit breaker.
// If the breaker is open, returns an error immediately without calling fn.
func (b *Breaker) Execute(fn func() (any, error)) (any, error) {
	return b.cb.Execute(fn)
}

// IsAvailable returns true if the circuit breaker is closed or half-open.
func (b *Breaker) IsAvailable() bool {
	return b.cb.State() != gobreaker.StateOpen
}

// State returns the current state as a string.
func (b *Breaker) State() string {
	return b.cb.State().String()
}
