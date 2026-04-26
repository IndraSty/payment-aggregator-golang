package middleware

import (
	"fmt"
	"net/http"

	"github.com/IndraSty/payment-aggregator-golang/internal/repository/redis"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RateLimitByIP limits requests per IP address.
// Used to prevent brute force attacks on auth endpoints.
func RateLimitByIP(repo *redis.RateLimitRepository, limit int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			key := fmt.Sprintf("ip:%s", ip)

			allowed, count, err := repo.Allow(c.Request().Context(), key, limit)
			if err != nil {
				log.Warn().Err(err).Str("ip", ip).Msg("Rate limit check failed, allowing request")
				return next(c)
			}

			// Add rate limit info to response headers
			c.Response().Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(limit)-count)))

			if !allowed {
				log.Warn().
					Str("ip", ip).
					Int64("count", count).
					Msg("Rate limit exceeded by IP")
				return echo.NewHTTPError(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded, please slow down",
				})
			}

			return next(c)
		}
	}
}

// RateLimitByAPIKey limits requests per authenticated API key.
// Applied after auth middleware so user_id is available in context.
func RateLimitByAPIKey(repo *redis.RateLimitRepository, limit int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get(ContextKeyUserID).(string)
			if userID == "" {
				// No user in context — skip API key rate limiting
				return next(c)
			}

			key := fmt.Sprintf("apikey:%s", userID)

			allowed, count, err := repo.Allow(c.Request().Context(), key, limit)
			if err != nil {
				log.Warn().Err(err).Str("user_id", userID).Msg("API key rate limit check failed, allowing request")
				return next(c)
			}

			c.Response().Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(limit)-count)))

			if !allowed {
				log.Warn().
					Str("user_id", userID).
					Int64("count", count).
					Msg("Rate limit exceeded by API key")
				return echo.NewHTTPError(http.StatusTooManyRequests, map[string]string{
					"error": "API rate limit exceeded",
				})
			}

			return next(c)
		}
	}
}

// max returns the larger of two int64 values.
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
