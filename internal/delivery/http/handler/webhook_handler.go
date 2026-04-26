package handler

import (
	"io"
	"net/http"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type WebhookHandler struct {
	webhookUC domain.WebhookUsecase
}

func NewWebhookHandler(webhookUC domain.WebhookUsecase) *WebhookHandler {
	return &WebhookHandler{webhookUC: webhookUC}
}

// collectHeaders extracts all request headers into a flat map for provider parsers.
func collectHeaders(c echo.Context) map[string]string {
	headers := make(map[string]string)
	for key, values := range c.Request().Header {
		if len(values) > 0 {
			headers[canonicalHeader(key)] = values[0]
		}
	}
	return headers
}

// canonicalHeader lowercases header names for consistent provider lookup.
func canonicalHeader(key string) string {
	result := make([]byte, len(key))
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

// MidtransWebhook godoc
// @Summary Midtrans payment notification webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /webhooks/midtrans [post]
func (h *WebhookHandler) MidtransWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read Midtrans webhook body")
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "failed to read request body",
		})
	}

	headers := collectHeaders(c)

	if err := h.webhookUC.ProcessMidtrans(c.Request().Context(), payload, headers); err != nil {
		log.Error().Err(err).Msg("Midtrans webhook processing failed")
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, APIResponse{Success: false, Error: appErr.Message})
		}
		// Always return 200 to Midtrans even on error — prevents endless retries
		// for non-signature errors. Only return non-200 for signature failures.
		return c.JSON(http.StatusOK, APIResponse{
			Success: false,
			Error:   "webhook processing failed",
		})
	}

	return ok(c, map[string]string{"status": "received"})
}

// XenditWebhook godoc
// @Summary Xendit payment notification webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /webhooks/xendit [post]
func (h *WebhookHandler) XenditWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read Xendit webhook body")
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "failed to read request body",
		})
	}

	headers := collectHeaders(c)

	if err := h.webhookUC.ProcessXendit(c.Request().Context(), payload, headers); err != nil {
		log.Error().Err(err).Msg("Xendit webhook processing failed")
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, APIResponse{Success: false, Error: appErr.Message})
		}
		return c.JSON(http.StatusOK, APIResponse{
			Success: false,
			Error:   "webhook processing failed",
		})
	}

	return ok(c, map[string]string{"status": "received"})
}

// StripeWebhook godoc
// @Summary Stripe payment notification webhook
// @Tags webhooks
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /webhooks/stripe [post]
func (h *WebhookHandler) StripeWebhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read Stripe webhook body")
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "failed to read request body",
		})
	}

	headers := collectHeaders(c)

	if err := h.webhookUC.ProcessStripe(c.Request().Context(), payload, headers); err != nil {
		log.Error().Err(err).Msg("Stripe webhook processing failed")
		if appErr, ok := err.(*domain.AppError); ok {
			return c.JSON(appErr.Code, APIResponse{Success: false, Error: appErr.Message})
		}
		return c.JSON(http.StatusOK, APIResponse{
			Success: false,
			Error:   "webhook processing failed",
		})
	}

	return ok(c, map[string]string{"status": "received"})
}
