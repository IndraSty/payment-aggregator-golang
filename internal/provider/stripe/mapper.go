package stripe

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/stripe/stripe-go/v76"
)

// MapToPaymentIntentParams converts a unified ChargeRequest to Stripe PaymentIntent params.
func MapToPaymentIntentParams(req *domain.ChargeRequest) *stripe.PaymentIntentParams {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(req.Amount),
		Currency: stripe.String(req.Currency),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Description: stripe.String(req.Description),
		Metadata: map[string]string{
			"external_id":    req.ExternalID,
			"customer_email": req.CustomerEmail,
			"customer_name":  req.CustomerName,
		},
	}

	// Add return URL for redirect-based payment methods
	if req.ReturnURL != "" {
		params.ReturnURL = stripe.String(req.ReturnURL)
	}

	return params
}

// MapToChargeResponse converts a Stripe PaymentIntent to unified ChargeResponse.
func MapToChargeResponse(pi *stripe.PaymentIntent) *domain.ChargeResponse {
	rawMap := map[string]any{
		"payment_intent_id": pi.ID,
		"client_secret":     pi.ClientSecret,
		"status":            string(pi.Status),
	}

	// PaymentIntent expiry is not set by Stripe — we set our own 24h default
	expiredAt := time.Now().Add(24 * time.Hour)

	return &domain.ChargeResponse{
		ExternalID:  pi.ID,
		Status:      mapStripeStatus(pi.Status),
		PaymentURL:  "", // Stripe uses client_secret for frontend integration
		ExpiredAt:   &expiredAt,
		RawResponse: rawMap,
	}
}

// MapStatusResponse converts a Stripe PaymentIntent status to unified ChargeResponse.
func MapStatusResponse(pi *stripe.PaymentIntent) *domain.ChargeResponse {
	rawMap := map[string]any{
		"payment_intent_id": pi.ID,
		"status":            string(pi.Status),
	}

	return &domain.ChargeResponse{
		ExternalID:  pi.ID,
		Status:      mapStripeStatus(pi.Status),
		RawResponse: rawMap,
	}
}

// MapWebhookPayload converts a Stripe webhook event to unified WebhookPayload.
func MapWebhookPayload(event *stripe.Event) (*domain.WebhookPayload, error) {
	switch event.Type {
	case "payment_intent.succeeded",
		"payment_intent.payment_failed",
		"payment_intent.canceled",
		"payment_intent.created":

		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return nil, fmt.Errorf("failed to parse payment intent from webhook: %w", err)
		}

		return &domain.WebhookPayload{
			ExternalID: pi.ID,
			EventType:  string(event.Type),
			Status:     mapStripeStatus(pi.Status),
			Amount:     pi.Amount,
			Currency:   string(pi.Currency),
			RawData: map[string]any{
				"event_id":          event.ID,
				"event_type":        string(event.Type),
				"payment_intent_id": pi.ID,
				"status":            string(pi.Status),
			},
		}, nil

	default:
		// Unsupported event type — return minimal payload, usecase will ignore
		return &domain.WebhookPayload{
			ExternalID: "",
			EventType:  string(event.Type),
			Status:     domain.StatusPending,
			RawData: map[string]any{
				"event_id":   event.ID,
				"event_type": string(event.Type),
			},
		}, nil
	}
}

// mapStripeStatus maps Stripe PaymentIntent status to domain status.
func mapStripeStatus(status stripe.PaymentIntentStatus) domain.TransactionStatus {
	switch status {
	case stripe.PaymentIntentStatusSucceeded:
		return domain.StatusSuccess
	case stripe.PaymentIntentStatusCanceled:
		return domain.StatusFailed
	case stripe.PaymentIntentStatusRequiresPaymentMethod,
		stripe.PaymentIntentStatusRequiresConfirmation,
		stripe.PaymentIntentStatusRequiresAction,
		stripe.PaymentIntentStatusProcessing:
		return domain.StatusPending
	default:
		return domain.StatusPending
	}
}
