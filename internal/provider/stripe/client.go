package stripe

import (
	"context"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/circuitbreaker"
	"github.com/rs/zerolog/log"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/webhook"
)

type Client struct {
	webhookSecret string
	breaker       *circuitbreaker.Breaker
}

// NewClient creates a new Stripe provider client.
func NewClient(cfg *config.StripeConfig, breaker *circuitbreaker.Breaker) *Client {
	// Stripe Go SDK uses global API key
	stripe.Key = cfg.SecretKey

	return &Client{
		webhookSecret: cfg.WebhookSecret,
		breaker:       breaker,
	}
}

func (c *Client) Name() domain.PaymentProvider {
	return domain.ProviderStripe
}

func (c *Client) SupportedMethods() []string {
	return []string{
		"credit_card",
		"debit_card",
		"bank_transfer",
		"link",
	}
}

// Charge creates a Stripe PaymentIntent.
func (c *Client) Charge(ctx context.Context, req *domain.ChargeRequest) (*domain.ChargeResponse, error) {
	params := MapToPaymentIntentParams(req)

	result, err := c.breaker.Execute(func() (any, error) {
		pi, err := paymentintent.New(params)
		if err != nil {
			return nil, fmt.Errorf("stripe payment intent create: %w", err)
		}
		return pi, nil
	})
	if err != nil {
		log.Error().Err(err).Str("external_id", req.ExternalID).Msg("Stripe charge failed")
		return nil, fmt.Errorf("stripe charge: %w", err)
	}

	pi := result.(*stripe.PaymentIntent)
	return MapToChargeResponse(pi), nil
}

// GetStatus fetches the current PaymentIntent status from Stripe.
func (c *Client) GetStatus(ctx context.Context, externalID string) (*domain.ChargeResponse, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		pi, err := paymentintent.Get(externalID, nil)
		if err != nil {
			return nil, fmt.Errorf("stripe get payment intent: %w", err)
		}
		return pi, nil
	})
	if err != nil {
		return nil, fmt.Errorf("stripe GetStatus: %w", err)
	}

	pi := result.(*stripe.PaymentIntent)
	return MapStatusResponse(pi), nil
}

// ParseWebhook validates the Stripe webhook signature using the official SDK.
// Uses stripe.ConstructEvent() which handles timestamp validation automatically.
func (c *Client) ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*domain.WebhookPayload, error) {
	sigHeader, ok := headers["stripe-signature"]
	if !ok {
		return nil, domain.ErrInvalidWebhookSig
	}

	// stripe.ConstructEvent validates signature AND timestamp (prevents replay attacks)
	// Rejects events older than 300 seconds (5 minutes) by default
	event, err := webhook.ConstructEvent(payload, sigHeader, c.webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("stripe webhook verification failed: %w", domain.ErrInvalidWebhookSig)
	}

	webhookPayload, err := MapWebhookPayload(&event)
	if err != nil {
		return nil, fmt.Errorf("stripe ParseWebhook map: %w", err)
	}

	return webhookPayload, nil
}
