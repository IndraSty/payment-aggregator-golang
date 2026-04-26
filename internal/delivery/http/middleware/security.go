package middleware

import (
	"github.com/labstack/echo/v4"
)

// SecurityHeaders adds required security headers to every response.
// These headers are mandatory for payment systems.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Prevent MIME type sniffing
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			c.Response().Header().Set("X-Frame-Options", "DENY")

			// Force HTTPS for 1 year, include subdomains
			c.Response().Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			// Enable XSS filter in older browsers
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")

			// Restrict referrer information
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Disable browser features not needed by an API
			c.Response().Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			// Prevent caching of sensitive payment responses
			c.Response().Header().Set("Cache-Control", "no-store")
			c.Response().Header().Set("Pragma", "no-cache")

			return next(c)
		}
	}
}

// HTTPSRedirect rejects non-HTTPS requests in production.
// In development, allows HTTP for local testing.
func HTTPSRedirect(isProduction bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if isProduction && c.Request().Header.Get("X-Forwarded-Proto") == "http" {
				return c.Redirect(301, "https://"+c.Request().Host+c.Request().RequestURI)
			}
			return next(c)
		}
	}
}
