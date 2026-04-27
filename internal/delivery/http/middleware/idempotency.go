package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

const IdempotencyKeyHeader = "X-Idempotency-Key"

// IdempotencyCheck enforces X-Idempotency-Key on POST /charges.
// If the key was already used, returns the cached response immediately
// without hitting the usecase or provider again.
// This is the HTTP-layer check — usecase also has its own check as defense in depth.
func IdempotencyCheck(idempotencyRepo domain.IdempotencyRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get(IdempotencyKeyHeader)
			if key == "" {
				return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
					"error": "X-Idempotency-Key header is required",
				})
			}

			if len(key) > 255 {
				return echo.NewHTTPError(http.StatusBadRequest, map[string]string{
					"error": "X-Idempotency-Key must not exceed 255 characters",
				})
			}

			// Check Redis for existing response
			cached, err := idempotencyRepo.Get(c.Request().Context(), key)
			if err != nil {
				log.Warn().Err(err).Str("idempotency_key", key).Msg("Idempotency cache read failed at middleware")
				// Proceed — don't block the request on cache failure
				return next(c)
			}

			if cached != nil && len(cached.Response) > 0 {
				// Return the exact cached response — same status code, same body
				log.Info().
					Str("idempotency_key", key).
					Int("cached_status", cached.StatusCode).
					Msg("Returning cached idempotent response from middleware")

				c.Response().Header().Set("X-Idempotency-Replay", "true")
				return c.JSONBlob(cached.StatusCode, cached.Response)
			}

			// Key not in cache — inject key into context for usecase to use
			c.Set("idempotency_key", key)

			// Wrap the response writer to capture the response body for caching
			crw := newCapturingResponseWriter(c.Response().Writer)
			c.Response().Writer = crw

			if err := next(c); err != nil {
				return err
			}

			// After handler runs, cache the response if it was successful (2xx)
			if crw.status >= 200 && crw.status < 300 {
				record := &domain.IdempotencyRecord{
					StatusCode: crw.status,
					Response:   crw.body.Bytes(),
				}
				if cacheErr := idempotencyRepo.Set(c.Request().Context(), key, record); cacheErr != nil {
					log.Warn().Err(cacheErr).Str("idempotency_key", key).Msg("Failed to cache idempotency response")
				}
			}

			return nil
		}
	}
}

// capturingResponseWriter wraps echo.Response to capture the body and status.
type capturingResponseWriter struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func newCapturingResponseWriter(w http.ResponseWriter) *capturingResponseWriter {
	return &capturingResponseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		status:         http.StatusOK,
	}
}

func (crw *capturingResponseWriter) WriteHeader(code int) {
	crw.status = code
	crw.ResponseWriter.WriteHeader(code)
}

func (crw *capturingResponseWriter) Write(b []byte) (int, error) {
	crw.body.Write(b)
	return crw.ResponseWriter.Write(b)
}

// isValidJSON checks if bytes are valid JSON — safety check before caching.
func isValidJSON(b []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(b, &js) == nil
}
