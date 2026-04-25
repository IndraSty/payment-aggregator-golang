package xendit

import (
	"context"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/circuitbreaker"
	"github.com/rs/zerolog/log"
	"github.com/xendit/xendit-go"
	"github.com/xendit/xendit-go/invoice"
)

type Client struct {
	secretKey     string
	callbackToken string
	breaker       *circuitbreaker.Breaker
}

// NewClient creates a new Xendit provider client.
func NewClient(cfg *config.XenditConfig, breaker *circuitbreaker.Breaker) *Client {
	// Xendit Go SDK uses global API key
	xendit.Opt.SecretKey = cfg.SecretKey

	return &Client{
		secretKey:     cfg.SecretKey,
		callbackToken: cfg.CallbackToken,
		breaker:       breaker,
	}
}

func (c *Client) Name() domain.PaymentProvider {
	return domain.ProviderXendit
}

func (c *Client) SupportedMethods() []string {
	return []string{
		"credit_card",
		"bank_transfer",
		"ewallet",
		"qr_code",
		"retail_outlet",
		"direct_debit",
	}
}

// Charge creates a Xendit Invoice (hosted payment page).
func (c *Client) Charge(ctx context.Context, req *domain.ChargeRequest) (*domain.ChargeResponse, error) {
	invoiceReq := MapToInvoiceRequest(req)

	result, err := c.breaker.Execute(func() (any, error) {
		resp, err := invoice.Create(invoiceReq)
		if err != nil {
			return nil, fmt.Errorf("xendit invoice create: %w", err)
		}
		return resp, nil
	})
	if err != nil {
		log.Error().Err(err).Str("external_id", req.ExternalID).Msg("Xendit charge failed")
		return nil, fmt.Errorf("xendit charge: %w", err)
	}

	invoiceResp := result.(*xendit.Invoice)
	return MapToChargeResponse(invoiceResp), nil
}

// GetStatus fetches the current invoice status from Xendit.
func (c *Client) GetStatus(ctx context.Context, externalID string) (*domain.ChargeResponse, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		resp, err := invoice.Get(&invoice.GetParams{
			ID: externalID,
		})
		if err != nil {
			return nil, fmt.Errorf("xendit get invoice: %w", err)
		}
		return resp, nil
	})
	if err != nil {
		return nil, fmt.Errorf("xendit GetStatus: %w", err)
	}

	inv := result.(*xendit.Invoice)
	return MapStatusResponse(inv), nil
}

// ParseWebhook validates the Xendit webhook callback token and parses the payload.
// Xendit uses a static callback token in the x-callback-token header.
func (c *Client) ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*domain.WebhookPayload, error) {
	callbackToken, ok := headers["x-callback-token"]
	if !ok || callbackToken != c.callbackToken {
		return nil, domain.ErrInvalidWebhookSig
	}

	webhookPayload, err := ParseWebhookPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("xendit ParseWebhook: %w", err)
	}

	return webhookPayload, nil
}
