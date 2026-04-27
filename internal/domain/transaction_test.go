package domain_test

import (
	"testing"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestCanTransitionTo tests the transaction state machine
func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		current  domain.TransactionStatus
		next     domain.TransactionStatus
		expected bool
	}{
		{"pending to success", domain.StatusPending, domain.StatusSuccess, true},
		{"pending to failed", domain.StatusPending, domain.StatusFailed, true},
		{"pending to expired", domain.StatusPending, domain.StatusExpired, true},
		{"success to failed", domain.StatusSuccess, domain.StatusFailed, false},
		{"success to pending", domain.StatusSuccess, domain.StatusPending, false},
		{"failed to success", domain.StatusFailed, domain.StatusSuccess, false},
		{"expired to pending", domain.StatusExpired, domain.StatusPending, false},
		{"expired to success", domain.StatusExpired, domain.StatusSuccess, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &domain.Transaction{Status: tt.current}
			result := tx.CanTransitionTo(tt.next)
			assert.Equal(t, tt.expected, result,
				"transition from %s to %s should be %v",
				tt.current, tt.next, tt.expected,
			)
		})
	}
}

// TestIsTerminal tests that terminal states are correctly identified
func TestIsTerminal(t *testing.T) {
	assert.False(t, (&domain.Transaction{Status: domain.StatusPending}).IsTerminal())
	assert.True(t, (&domain.Transaction{Status: domain.StatusSuccess}).IsTerminal())
	assert.True(t, (&domain.Transaction{Status: domain.StatusFailed}).IsTerminal())
	assert.True(t, (&domain.Transaction{Status: domain.StatusExpired}).IsTerminal())
}

// TestSupportedCurrencies tests currency routing configuration
func TestSupportedCurrencies(t *testing.T) {
	// IDR must route to Midtrans and Xendit
	idrProviders, ok := domain.SupportedCurrencies["IDR"]
	assert.True(t, ok)
	assert.Contains(t, idrProviders, domain.ProviderMidtrans)
	assert.Contains(t, idrProviders, domain.ProviderXendit)
	assert.NotContains(t, idrProviders, domain.ProviderStripe)

	// USD must route to Stripe only
	usdProviders, ok := domain.SupportedCurrencies["USD"]
	assert.True(t, ok)
	assert.Contains(t, usdProviders, domain.ProviderStripe)
	assert.NotContains(t, usdProviders, domain.ProviderMidtrans)

	// JPY must not be supported
	_, ok = domain.SupportedCurrencies["JPY"]
	assert.False(t, ok)
}
