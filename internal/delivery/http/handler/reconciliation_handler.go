package handler

import (
	"strconv"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/labstack/echo/v4"
)

type ReconciliationHandler struct {
	reconcileUC domain.ReconcileUsecase
}

func NewReconciliationHandler(reconcileUC domain.ReconcileUsecase) *ReconciliationHandler {
	return &ReconciliationHandler{reconcileUC: reconcileUC}
}

// RunReconciliation godoc
// @Summary Manually trigger reconciliation for all providers
// @Tags reconciliation
// @Security BearerAuth
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /api/v1/reconciliation/run [post]
func (h *ReconciliationHandler) RunReconciliation(c echo.Context) error {
	reports, err := h.reconcileUC.RunReconciliation(c.Request().Context())
	if err != nil {
		return handleError(c, err)
	}

	return ok(c, map[string]any{
		"reports": reports,
		"count":   len(reports),
	})
}

// ListReports godoc
// @Summary List reconciliation reports
// @Tags reconciliation
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Param provider query string false "Filter by provider"
// @Param status query string false "Filter by status"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /api/v1/reconciliation/reports [get]
func (h *ReconciliationHandler) ListReports(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	filter := domain.ReconciliationFilter{
		Page:  page,
		Limit: limit,
	}

	if p := c.QueryParam("provider"); p != "" {
		prov := domain.PaymentProvider(p)
		filter.Provider = &prov
	}
	if s := c.QueryParam("status"); s != "" {
		status := domain.ReconciliationStatus(s)
		filter.Status = &status
	}

	reports, total, err := h.reconcileUC.ListReports(c.Request().Context(), filter)
	if err != nil {
		return handleError(c, err)
	}

	return paginated(c, reports, filter.Page, filter.Limit, total)
}

// GetReport godoc
// @Summary Get a reconciliation report by ID
// @Tags reconciliation
// @Security BearerAuth
// @Produce json
// @Param id path string true "Report ID (UUID)"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /api/v1/reconciliation/reports/{id} [get]
func (h *ReconciliationHandler) GetReport(c echo.Context) error {
	id := c.Param("id")

	report, err := h.reconcileUC.GetReport(c.Request().Context(), id)
	if err != nil {
		return handleError(c, err)
	}

	return ok(c, report)
}
