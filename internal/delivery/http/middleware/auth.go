package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const (
	ContextKeyUserID = "user_id"
	ContextKeyEmail  = "email"
)

// JWTAuth validates the Authorization: Bearer <token> header.
// Injects user_id and email into the Echo context for downstream handlers.
func JWTAuth(jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(401, map[string]string{
					"error": domain.ErrUnauthorized.Error(),
				})
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(401, map[string]string{
					"error": "invalid authorization header format",
				})
			}

			tokenString := parts[1]

			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, domain.ErrInvalidToken
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				return echo.NewHTTPError(401, map[string]string{
					"error": domain.ErrInvalidToken.Error(),
				})
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return echo.NewHTTPError(401, map[string]string{
					"error": domain.ErrInvalidToken.Error(),
				})
			}

			// Verify it's an access token, not a refresh token
			tokenType, _ := claims["type"].(string)
			if tokenType != "access" {
				return echo.NewHTTPError(401, map[string]string{
					"error": "invalid token type",
				})
			}

			// Inject claims into context for handlers
			c.Set(ContextKeyUserID, claims["sub"])
			c.Set(ContextKeyEmail, claims["email"])

			return next(c)
		}
	}
}

// APIKeyAuth validates the X-API-Key header.
// Hashes the provided key with SHA-256 and looks it up in the database.
func APIKeyAuth(userRepo domain.UserRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			apiKey := c.Request().Header.Get("X-API-Key")
			if apiKey == "" {
				return echo.NewHTTPError(401, map[string]string{
					"error": domain.ErrInvalidAPIKey.Error(),
				})
			}

			// Hash the received key before looking up — never compare plaintext
			hash := sha256.Sum256([]byte(apiKey))
			hashedKey := hex.EncodeToString(hash[:])

			user, err := userRepo.GetByAPIKey(c.Request().Context(), hashedKey)
			if err != nil {
				return echo.NewHTTPError(401, map[string]string{
					"error": domain.ErrInvalidAPIKey.Error(),
				})
			}

			// Inject user info into context
			c.Set(ContextKeyUserID, user.ID)
			c.Set(ContextKeyEmail, user.Email)

			return next(c)
		}
	}
}

// GetUserID extracts the authenticated user ID from Echo context.
// Panics if JWTAuth or APIKeyAuth middleware was not applied — by design.
func GetUserID(c echo.Context) string {
	userID, _ := c.Get(ContextKeyUserID).(string)
	return userID
}
