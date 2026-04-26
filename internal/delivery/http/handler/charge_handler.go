package handler

import (
	"net/http"
	"strconv"

	"github.com/IndraSty/payment-aggregator-golang/internal/delivery/http/middleware"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/labstack/echo/v4"
)

type ChargeHandler struct {
	chargeUC domain.ChargeUsecase
}

func NewChargeHandler(chargeUC domain.ChargeUsecase) *ChargeHandler {
	return &ChargeHandler{chargeUC: chargeUC}
}

// CreateChargeRequest is the request body for POST /api/v1/charges.
type CreateChargeRequest struct {
	Amount        int64          `json:"amount"`
	Currency      string         `json:"currency"`
	PaymentMethod string         `json:"payment_method"`
	CustomerName  string         `json:"customer_name"`
	CustomerEmail string         `json:"customer_email"`
	CustomerPhone string         `json:"customer_phone"`
	Description   string         `json:"description"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// CreateCharge godoc
// @Summary Create a new payment charge
// @Tags charges
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param X-Idempotency-Key header string true "Idempotency key (UUID recommended)"
// @Param body body CreateChargeRequest true "Charge details"
// @Success 201 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Failure 503 {object} APIResponse
// @Router /api/v1/charges [post]
func (h *ChargeHandler) CreateCharge(c echo.Context) error {
	var req CreateChargeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body",
		})
	}

	userID := middleware.GetUserID(c)
	idempotencyKey, _ := c.Get("idempotency_key").(string)

	tx, err := h.chargeUC.CreateCharge(c.Request().Context(), &domain.CreateChargeInput{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Amount:         req.Amount,
		Currency:       req.Currency,
		PaymentMethod:  req.PaymentMethod,
		CustomerName:   req.CustomerName,
		CustomerEmail:  req.CustomerEmail,
		CustomerPhone:  req.CustomerPhone,
		Description:    req.Description,
		Metadata:       req.Metadata,
	})
	if err != nil {
		return handleError(c, err)
	}

	return created(c, tx)
}

// ListCharges godoc
// @Summary List charges for authenticated user
// @Tags charges
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Param status query string false "Filter by status (pending|success|failed|expired)"
// @Param provider query string false "Filter by provider (midtrans|xendit|stripe)"
// @Param currency query string false "Filter by currency (IDR|USD|EUR)"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /api/v1/charges [get]
func (h *ChargeHandler) ListCharges(c echo.Context) error {
	userID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	filter := domain.TransactionFilter{
		Page:  page,
		Limit: limit,
	}

	// Optional filters
	if s := c.QueryParam("status"); s != "" {
		status := domain.TransactionStatus(s)
		filter.Status = &status
	}
	if p := c.QueryParam("provider"); p != "" {
		prov := domain.PaymentProvider(p)
		filter.Provider = &prov
	}
	if cur := c.QueryParam("currency"); cur != "" {
		filter.Currency = &cur
	}

	txs, total, err := h.chargeUC.ListCharges(c.Request().Context(), userID, filter)
	if err != nil {
		return handleError(c, err)
	}

	return paginated(c, txs, filter.Page, filter.Limit, total)
}

// GetCharge godoc
// @Summary Get a single charge by ID
// @Tags charges
// @Security BearerAuth
// @Produce json
// @Param id path string true "Transaction ID (UUID)"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /api/v1/charges/{id} [get]
func (h *ChargeHandler) GetCharge(c echo.Context) error {
	userID := middleware.GetUserID(c)
	txID := c.Param("id")

	tx, err := h.chargeUC.GetCharge(c.Request().Context(), userID, txID)
	if err != nil {
		return handleError(c, err)
	}

	return ok(c, tx)
}

// ExpireCharge godoc
// @Summary Manually expire a pending charge
// @Tags charges
// @Security BearerAuth
// @Produce json
// @Param id path string true "Transaction ID (UUID)"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Failure 422 {object} APIResponse
// @Router /api/v1/charges/{id}/expire [post]
func (h *ChargeHandler) ExpireCharge(c echo.Context) error {
	userID := middleware.GetUserID(c)
	txID := c.Param("id")

	tx, err := h.chargeUC.ExpireCharge(c.Request().Context(), userID, txID)
	if err != nil {
		return handleError(c, err)
	}

	return ok(c, tx)
}
