package midtrans

import (
	"context"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/circuitbreaker"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/rs/zerolog/log"
)

type Client struct {
	snapClient snap.Client
	coreClient coreapi.Client
	serverKey  string
	breaker    *circuitbreaker.Breaker
}

// NewClient creates a new Midtrans provider client.
func NewClient(cfg *config.MidtransConfig, breaker *circuitbreaker.Breaker) *Client {
	env := midtrans.Sandbox
	if cfg.IsProduction {
		env = midtrans.Production
	}

	var snapClient snap.Client
	snapClient.New(cfg.ServerKey, env)

	var coreClient coreapi.Client
	coreClient.New(cfg.ServerKey, env)

	return &Client{
		snapClient: snapClient,
		coreClient: coreClient,
		serverKey:  cfg.ServerKey,
		breaker:    breaker,
	}
}

func (c *Client) Name() domain.PaymentProvider {
	return domain.ProviderMidtrans
}

func (c *Client) SupportedMethods() []string {
	return []string{
		"credit_card",
		"bank_transfer",
		"gopay",
		"shopeepay",
		"qris",
		"indomaret",
		"alfamart",
	}
}

// Charge creates a Snap payment transaction at Midtrans.
func (c *Client) Charge(ctx context.Context, req *domain.ChargeRequest) (*domain.ChargeResponse, error) {
	snapReq := MapToSnapRequest(req)

	result, err := c.breaker.Execute(func() (any, error) {
		resp, err := c.snapClient.CreateTransaction(snapReq)
		if err != nil {
			return nil, fmt.Errorf("midtrans snap create: %w", err)
		}
		return resp, nil
	})
	if err != nil {
		log.Error().Err(err).Str("external_id", req.ExternalID).Msg("Midtrans charge failed")
		return nil, fmt.Errorf("midtrans charge: %w", err)
	}

	snapResp := result.(*snap.Response)
	return MapToChargeResponse(snapResp, req.ExternalID), nil
}

// GetStatus fetches the current transaction status from Midtrans.
// Used during reconciliation.
func (c *Client) GetStatus(ctx context.Context, externalID string) (*domain.ChargeResponse, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		resp, err := c.coreClient.CheckTransaction(externalID)
		if err != nil {
			return nil, fmt.Errorf("midtrans get status: %w", err)
		}
		return resp, nil
	})
	if err != nil {
		return nil, fmt.Errorf("midtrans GetStatus: %w", err)
	}

	statusResp := result.(*coreapi.TransactionStatusResponse)
	return MapStatusResponse(statusResp), nil
}

// ParseWebhook validates the Midtrans webhook signature and parses the payload.
// Midtrans signature: SHA-512(order_id + status_code + gross_amount + server_key)
func (c *Client) ParseWebhook(ctx context.Context, payload []byte, headers map[string]string) (*domain.WebhookPayload, error) {
	notification, err := ParseAndVerifyWebhook(payload, c.serverKey)
	if err != nil {
		return nil, fmt.Errorf("midtrans ParseWebhook: %w", err)
	}
	return MapWebhookPayload(notification), nil
}
