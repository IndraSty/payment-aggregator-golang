package http

import (
	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/delivery/http/handler"
	"github.com/IndraSty/payment-aggregator-golang/internal/delivery/http/middleware"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	redisrepo "github.com/IndraSty/payment-aggregator-golang/internal/repository/redis"
	"github.com/IndraSty/payment-aggregator-golang/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
)

// RouterDeps holds all dependencies needed to set up routes.
type RouterDeps struct {
	Config          *config.Config
	DB              *pgxpool.Pool
	RDB             *redis.Client
	AuthUC          domain.AuthUsecase
	ChargeUC        domain.ChargeUsecase
	WebhookUC       domain.WebhookUsecase
	ReconcileUC     domain.ReconcileUsecase
	UserRepo        domain.UserRepository
	IdempotencyRepo domain.IdempotencyRepository
	RateLimitRepo   *redisrepo.RateLimitRepository
}

// NewRouter creates and configures the Echo instance with all routes and middleware.
func NewRouter(deps *RouterDeps) *echo.Echo {
	e := echo.New()

	// Hide Echo banner and default error handler
	e.HideBanner = true
	e.HidePort = true

	// Custom error handler — ensures all errors use our APIResponse format
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
			return
		}
		_ = c.JSON(500, map[string]string{"error": "internal server error"})
	}

	// -------------------------------------------------------------------------
	// Global middleware — applied to ALL routes
	// -------------------------------------------------------------------------
	e.Use(middleware.RecoverWithLog())
	e.Use(middleware.RequestLogger())
	e.Use(echomiddleware.RequestID())
	e.Use(middleware.SecurityHeaders())
	e.Use(middleware.HTTPSRedirect(deps.Config.IsProduction()))
	e.Use(middleware.CORS())

	// Global IP rate limit — brute force protection
	e.Use(middleware.RateLimitByIP(deps.RateLimitRepo, deps.Config.RateLimit.PerIP))

	// -------------------------------------------------------------------------
	// Handlers
	// -------------------------------------------------------------------------
	authHandler := handler.NewAuthHandler(deps.AuthUC)
	chargeHandler := handler.NewChargeHandler(deps.ChargeUC)
	webhookHandler := handler.NewWebhookHandler(deps.WebhookUC)
	reconcileHandler := handler.NewReconciliationHandler(deps.ReconcileUC)
	healthHandler := handler.NewHealthHandler(deps.DB, deps.RDB)

	// -------------------------------------------------------------------------
	// Public routes — no auth required
	// -------------------------------------------------------------------------
	e.GET("/health", healthHandler.Health)
	e.GET("/metrics", echo.WrapHandler(metrics.Handler()))

	// Auth routes — IP rate limited only
	auth := e.Group("/api/v1/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.RefreshToken)

	// -------------------------------------------------------------------------
	// Webhook routes — no JWT auth, but each provider verifies its own signature
	// -------------------------------------------------------------------------
	webhooks := e.Group("/webhooks")
	webhooks.POST("/midtrans", webhookHandler.MidtransWebhook)
	webhooks.POST("/xendit", webhookHandler.XenditWebhook)
	webhooks.POST("/stripe", webhookHandler.StripeWebhook)

	// -------------------------------------------------------------------------
	// Protected routes — JWT auth + API key rate limit
	// -------------------------------------------------------------------------
	jwtMiddleware := middleware.JWTAuth(deps.Config.JWT.Secret)
	apiKeyRateLimit := middleware.RateLimitByAPIKey(deps.RateLimitRepo, deps.Config.RateLimit.PerAPIKey)

	api := e.Group("/api/v1", jwtMiddleware, apiKeyRateLimit)

	// Charges
	charges := api.Group("/charges")
	charges.POST("", chargeHandler.CreateCharge,
		middleware.IdempotencyCheck(deps.IdempotencyRepo), // only on POST
	)
	charges.GET("", chargeHandler.ListCharges)
	charges.GET("/:id", chargeHandler.GetCharge)
	charges.POST("/:id/expire", chargeHandler.ExpireCharge)

	// Reconciliation
	reconcile := api.Group("/reconciliation")
	reconcile.POST("/run", reconcileHandler.RunReconciliation)
	reconcile.GET("/reports", reconcileHandler.ListReports)
	reconcile.GET("/reports/:id", reconcileHandler.GetReport)

	return e
}
