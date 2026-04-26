package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RequestLogger logs every incoming HTTP request with structured fields.
// Replaces Echo's default logger with zerolog output.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			// Build log event — never log Authorization header or request body
			event := log.Info()
			if err != nil {
				event = log.Error().Err(err)
			}

			event.
				Str("method", req.Method).
				Str("path", req.URL.Path).
				Int("status", res.Status).
				Dur("latency", time.Since(start)).
				Str("ip", c.RealIP()).
				Str("request_id", res.Header().Get(echo.HeaderXRequestID)).
				Int64("bytes_out", res.Size).
				Msg("HTTP request")

			return err
		}
	}
}

// RecoverWithLog recovers from panics and logs them as errors.
// Prevents the entire server from crashing on unexpected panics.
func RecoverWithLog() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Str("path", c.Request().URL.Path).
						Msg("Recovered from panic")

					_ = c.JSON(500, map[string]string{
						"error": "internal server error",
					})
				}
			}()
			return next(c)
		}
	}
}
