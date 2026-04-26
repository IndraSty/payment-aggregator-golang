package handler

import (
	"net/http"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// APIResponse is the standard response envelope for all endpoints.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    any             `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}

// PaginationMeta holds pagination info for list endpoints.
type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

// ok returns a 200 success response.
func ok(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, APIResponse{Success: true, Data: data})
}

// created returns a 201 created response.
func created(c echo.Context, data any) error {
	return c.JSON(http.StatusCreated, APIResponse{Success: true, Data: data})
}

// paginated returns a 200 response with pagination metadata.
func paginated(c echo.Context, data any, page, limit int, total int64) error {
	totalPages := total / int64(limit)
	if total%int64(limit) != 0 {
		totalPages++
	}
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
		Meta: &PaginationMeta{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

// handleError maps domain errors to HTTP responses.
func handleError(c echo.Context, err error) error {
	if appErr, ok := err.(*domain.AppError); ok {
		return c.JSON(appErr.Code, APIResponse{
			Success: false,
			Error:   appErr.Message,
		})
	}

	// Map sentinel errors
	switch err {
	case domain.ErrTransactionNotFound, domain.ErrUserNotFound:
		return c.JSON(http.StatusNotFound, APIResponse{Success: false, Error: err.Error()})
	case domain.ErrTransactionDuplicate, domain.ErrUserAlreadyExists:
		return c.JSON(http.StatusConflict, APIResponse{Success: false, Error: err.Error()})
	case domain.ErrUnauthorized, domain.ErrInvalidToken, domain.ErrInvalidAPIKey:
		return c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Error: err.Error()})
	case domain.ErrInvalidCurrency, domain.ErrInvalidAmount, domain.ErrInvalidPaymentMethod:
		return c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
	case domain.ErrProviderUnavailable, domain.ErrNoProviderAvailable:
		return c.JSON(http.StatusServiceUnavailable, APIResponse{Success: false, Error: err.Error()})
	}

	// Unknown error — log internally, return generic message
	log.Error().Err(err).Str("path", c.Request().URL.Path).Msg("Unhandled error")
	return c.JSON(http.StatusInternalServerError, APIResponse{
		Success: false,
		Error:   "internal server error",
	})
}
