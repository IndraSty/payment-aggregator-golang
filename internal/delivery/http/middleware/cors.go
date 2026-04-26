package middleware

import (
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// CORS configures Cross-Origin Resource Sharing for the API.
// Restricts which origins, methods, and headers are allowed.
func CORS() echo.MiddlewareFunc {
	return echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		// In production, replace with specific allowed origins
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			"X-API-Key",
			"X-Idempotency-Key",
			"X-Request-ID",
		},
		ExposeHeaders: []string{
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-Idempotency-Replay",
		},
		MaxAge: 86400, // 24 hours preflight cache
	})
}
