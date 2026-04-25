package xendit

import (
	"encoding/json"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/xendit/xendit-go"
	"github.com/xendit/xendit-go/invoice"
)

// MapToInvoiceRequest converts a unified ChargeRequest to Xendit Invoice request.
func MapToInvoiceRequest(req *domain.ChargeRequest) *invoice.CreateParams {
	return &invoice.CreateParams{
		ExternalID:  req.ExternalID,
		Amount:      float64(req.Amount),
		Description: req.Description,
		PayerEmail:  req.CustomerEmail,
		Customer: xendit.InvoiceCustomer{
			GivenNames:   req.CustomerName,
			Email:        req.CustomerEmail,
			MobileNumber: req.CustomerPhone,
		},
		CustomerNotificationPreference: xendit.InvoiceCustomerNotificationPreference{
			InvoiceCreated:  []string{"email"},
			InvoiceReminder: []string{"email"},
			InvoicePaid:     []string{"email"},
		},
		SuccessRedirectURL: req.ReturnURL,
		FailureRedirectURL: req.ReturnURL,
		Currency:           "IDR",
		InvoiceDuration:    86400, // 24 hours in seconds
	}
}

// MapToChargeResponse converts a Xendit Invoice to unified ChargeResponse.
func MapToChargeResponse(inv *xendit.Invoice) *domain.ChargeResponse {
	rawMap := map[string]any{
		"invoice_id":  inv.ID,
		"invoice_url": inv.InvoiceURL,
		"status":      inv.Status,
	}

	return &domain.ChargeResponse{
		ExternalID:  inv.ID,
		Status:      domain.StatusPending,
		PaymentURL:  inv.InvoiceURL,
		ExpiredAt:   inv.ExpiryDate,
		RawResponse: rawMap,
	}
}

// MapStatusResponse converts a Xendit Invoice status to unified ChargeResponse.
func MapStatusResponse(inv *xendit.Invoice) *domain.ChargeResponse {
	rawMap := map[string]any{
		"invoice_id": inv.ID,
		"status":     inv.Status,
	}

	return &domain.ChargeResponse{
		ExternalID:  inv.ExternalID,
		Status:      mapXenditStatus(inv.Status),
		RawResponse: rawMap,
	}
}

// ParseWebhookPayload parses the raw Xendit webhook body into a unified payload.
func ParseWebhookPayload(payload []byte) (*domain.WebhookPayload, error) {
	var notif map[string]any
	if err := json.Unmarshal(payload, &notif); err != nil {
		return nil, fmt.Errorf("failed to parse xendit webhook: %w", err)
	}

	externalID, _ := notif["external_id"].(string)
	status, _ := notif["status"].(string)
	amount, _ := notif["amount"].(float64)
	currency, _ := notif["currency"].(string)

	if currency == "" {
		currency = "IDR"
	}

	return &domain.WebhookPayload{
		ExternalID: externalID,
		EventType:  fmt.Sprintf("xendit.invoice.%s", status),
		Status:     mapXenditStatus(status),
		Amount:     int64(amount),
		Currency:   currency,
		RawData:    notif,
	}, nil
}

// mapXenditStatus maps Xendit invoice status to domain status.
func mapXenditStatus(status string) domain.TransactionStatus {
	switch status {
	case "PAID", "SETTLED":
		return domain.StatusSuccess
	case "EXPIRED":
		return domain.StatusExpired
	case "PENDING":
		return domain.StatusPending
	default:
		return domain.StatusFailed
	}
}
