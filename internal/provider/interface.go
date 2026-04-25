package provider

import (
	"context"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/circuitbreaker"
	"github.com/rs/zerolog/log"
)

// Router implements domain.ProviderRouter.
// Selects the correct provider based on currency and circuit breaker state.
type Router struct {
	providers map[domain.PaymentProvider]domain.PaymentProviderClient
	breakers  map[domain.PaymentProvider]*circuitbreaker.Breaker
}

// NewRouter creates a provider router with all registered providers.
func NewRouter(
	cfg *config.Config,
	providers map[domain.PaymentProvider]domain.PaymentProviderClient,
	breakers map[domain.PaymentProvider]*circuitbreaker.Breaker,
) *Router {
	return &Router{
		providers: providers,
		breakers:  breakers,
	}
}

// Route selects the best available provider for the given currency.
// Skips providers whose circuit breaker is open (currently down).
// Falls back to next provider in the list if primary is unavailable.
func (r *Router) Route(ctx context.Context, currency string) (domain.PaymentProviderClient, error) {
	allowed, ok := domain.SupportedCurrencies[currency]
	if !ok {
		return nil, domain.ErrInvalidCurrency
	}

	for _, providerName := range allowed {
		breaker, hasCB := r.breakers[providerName]
		if hasCB && !breaker.IsAvailable() {
			log.Warn().
				Str("provider", string(providerName)).
				Str("currency", currency).
				Msg("Provider circuit breaker is open, skipping")
			continue
		}

		client, exists := r.providers[providerName]
		if !exists {
			continue
		}

		return client, nil
	}

	return nil, fmt.Errorf("%w: no available provider for currency %s", domain.ErrNoProviderAvailable, currency)
}

// GetProvider returns a specific provider by name.
func (r *Router) GetProvider(name domain.PaymentProvider) (domain.PaymentProviderClient, error) {
	client, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not registered", name)
	}
	return client, nil
}
