package midtrans

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

// MapToSnapRequest converts a unified ChargeRequest to Midtrans Snap request.
func MapToSnapRequest(req *domain.ChargeRequest) *snap.Request {
	return &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.ExternalID,
			GrossAmt: req.Amount,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: req.CustomerName,
			Email: req.CustomerEmail,
			Phone: req.CustomerPhone,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    "item-1",
				Name:  req.Description,
				Price: req.Amount,
				Qty:   1,
			},
		},
		Callbacks: &snap.Callbacks{
			Finish: req.ReturnURL,
		},
	}
}

// MapToChargeResponse converts Midtrans Snap response to unified ChargeResponse.
func MapToChargeResponse(resp *snap.Response, externalID string) *domain.ChargeResponse {
	expiredAt := time.Now().Add(24 * time.Hour) // Midtrans default expiry

	rawMap := map[string]any{
		"token":        resp.Token,
		"redirect_url": resp.RedirectURL,
	}

	return &domain.ChargeResponse{
		ExternalID:  externalID,
		Status:      domain.StatusPending, // Snap always starts as pending
		PaymentURL:  resp.RedirectURL,
		ExpiredAt:   &expiredAt,
		RawResponse: rawMap,
	}
}

// MapStatusResponse converts Midtrans status check response to unified ChargeResponse.
func MapStatusResponse(resp *coreapi.TransactionStatusResponse) *domain.ChargeResponse {
	rawMap := map[string]any{
		"transaction_status": resp.TransactionStatus,
		"fraud_status":       resp.FraudStatus,
		"payment_type":       resp.PaymentType,
	}

	return &domain.ChargeResponse{
		ExternalID:  resp.OrderID,
		Status:      mapMidtransStatus(resp.TransactionStatus, resp.FraudStatus),
		RawResponse: rawMap,
	}
}

// MapWebhookPayload converts a parsed Midtrans notification to unified WebhookPayload.
func MapWebhookPayload(notif map[string]any) *domain.WebhookPayload {
	orderID, _ := notif["order_id"].(string)
	txStatus, _ := notif["transaction_status"].(string)
	fraudStatus, _ := notif["fraud_status"].(string)
	grossAmount, _ := notif["gross_amount"].(string)
	currency, _ := notif["currency"].(string)

	// Parse amount — Midtrans sends as string (e.g. "150000.00")
	var amount int64
	if f, err := strconv.ParseFloat(grossAmount, 64); err == nil {
		amount = int64(f)
	}

	if currency == "" {
		currency = "IDR"
	}

	return &domain.WebhookPayload{
		ExternalID: orderID,
		EventType:  fmt.Sprintf("midtrans.%s", txStatus),
		Status:     mapMidtransStatus(txStatus, fraudStatus),
		Amount:     amount,
		Currency:   currency,
		RawData:    notif,
	}
}

// ParseAndVerifyWebhook validates the Midtrans webhook signature.
// Formula: SHA-512(order_id + status_code + gross_amount + server_key)
func ParseAndVerifyWebhook(payload []byte, serverKey string) (map[string]any, error) {
	var notif map[string]any
	if err := json.Unmarshal(payload, &notif); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	orderID, _ := notif["order_id"].(string)
	statusCode, _ := notif["status_code"].(string)
	grossAmount, _ := notif["gross_amount"].(string)
	receivedSig, _ := notif["signature_key"].(string)

	// Compute expected signature
	raw := orderID + statusCode + grossAmount + serverKey
	hash := sha512.Sum512([]byte(raw))
	expectedSig := fmt.Sprintf("%x", hash)

	if receivedSig != expectedSig {
		return nil, domain.ErrInvalidWebhookSig
	}

	return notif, nil
}

// mapMidtransStatus maps Midtrans transaction_status + fraud_status to domain status.
func mapMidtransStatus(txStatus, fraudStatus string) domain.TransactionStatus {
	switch txStatus {
	case "capture":
		if fraudStatus == "accept" || fraudStatus == "" {
			return domain.StatusSuccess
		}
		return domain.StatusFailed
	case "settlement":
		return domain.StatusSuccess
	case "deny", "cancel", "failure":
		return domain.StatusFailed
	case "expire":
		return domain.StatusExpired
	case "pending":
		return domain.StatusPending
	default:
		return domain.StatusPending
	}
}
