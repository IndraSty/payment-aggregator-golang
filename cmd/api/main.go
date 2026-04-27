// @title           Payment Aggregator API
// @version         1.0
// @description     Unified payment middleware supporting Midtrans, Xendit, and Stripe.
// @description     Routes payments automatically based on currency (IDR → Midtrans/Xendit, USD/EUR → Stripe).

// @contact.name    Indra Sty
// @contact.url     https://github.com/IndraSty
// @contact.email   your@email.com

// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     Format: "Bearer {token}"

// @securityDefinitions.apikey APIKeyAuth
// @in              header
// @name            X-API-Key

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/config"
	deliveryhttp "github.com/IndraSty/payment-aggregator-golang/internal/delivery/http"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/internal/provider"
	midtransprovider "github.com/IndraSty/payment-aggregator-golang/internal/provider/midtrans"
	stripeprovider "github.com/IndraSty/payment-aggregator-golang/internal/provider/stripe"
	xenditprovider "github.com/IndraSty/payment-aggregator-golang/internal/provider/xendit"
	"github.com/IndraSty/payment-aggregator-golang/internal/repository/postgres"
	redisrepo "github.com/IndraSty/payment-aggregator-golang/internal/repository/redis"
	"github.com/IndraSty/payment-aggregator-golang/internal/usecase"
	"github.com/IndraSty/payment-aggregator-golang/pkg/circuitbreaker"
	"github.com/IndraSty/payment-aggregator-golang/pkg/database"
	"github.com/IndraSty/payment-aggregator-golang/pkg/logger"
	"github.com/IndraSty/payment-aggregator-golang/pkg/scheduler"
	"github.com/rs/zerolog/log"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
		os.Exit(1)
	}

	logger.Init(cfg.App.Env)

	log.Info().Str("version", version).Str("env", cfg.App.Env).Msg("Payment Aggregator starting")

	// Migrations
	if err := database.RunMigrations(cfg.Database.URL); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
		os.Exit(1)
	}

	// PostgreSQL
	db, err := database.NewPostgresPool(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
		os.Exit(1)
	}
	defer db.Close()

	// Redis
	rdb, err := database.NewRedisClient(&cfg.Redis)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
		os.Exit(1)
	}
	defer rdb.Close()

	// Repositories
	userRepo := postgres.NewUserRepository(db)
	transactionRepo := postgres.NewTransactionRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)
	reconcileRepo := postgres.NewReconciliationRepository(db)
	idempotencyRepo := redisrepo.NewIdempotencyRepository(rdb, cfg.Redis.IdempotencyTTL)
	rateLimitRepo := redisrepo.NewRateLimitRepository(rdb, cfg.Redis.RateLimitWindow)

	// Circuit breakers
	breakers := map[domain.PaymentProvider]*circuitbreaker.Breaker{
		domain.ProviderMidtrans: circuitbreaker.NewBreaker("midtrans", &cfg.CB),
		domain.ProviderXendit:   circuitbreaker.NewBreaker("xendit", &cfg.CB),
		domain.ProviderStripe:   circuitbreaker.NewBreaker("stripe", &cfg.CB),
	}

	// Providers
	providers := map[domain.PaymentProvider]domain.PaymentProviderClient{
		domain.ProviderMidtrans: midtransprovider.NewClient(&cfg.Midtrans, breakers[domain.ProviderMidtrans]),
		domain.ProviderXendit:   xenditprovider.NewClient(&cfg.Xendit, breakers[domain.ProviderXendit]),
		domain.ProviderStripe:   stripeprovider.NewClient(&cfg.Stripe, breakers[domain.ProviderStripe]),
	}

	providerRouter := provider.NewRouter(cfg, providers, breakers)

	// Usecases
	authUC := usecase.NewAuthUsecase(userRepo, &cfg.JWT)
	chargeUC := usecase.NewChargeUsecase(transactionRepo, auditRepo, idempotencyRepo, providerRouter)
	webhookUC := usecase.NewWebhookUsecase(transactionRepo, auditRepo, webhookRepo, providerRouter)
	reconcileUC := usecase.NewReconcileUsecase(transactionRepo, auditRepo, reconcileRepo, providerRouter, cfg.Reconcile.LookbackHours)

	sched := scheduler.New(reconcileUC)
	if err := sched.Register(cfg.Reconcile.CronSchedule); err != nil {
		log.Fatal().Err(err).Msg("Failed to register cron jobs")
		os.Exit(1)
	}
	sched.Start()
	defer sched.Stop()

	// HTTP Router
	router := deliveryhttp.NewRouter(&deliveryhttp.RouterDeps{
		Config:          cfg,
		DB:              db,
		RDB:             rdb,
		AuthUC:          authUC,
		ChargeUC:        chargeUC,
		WebhookUC:       webhookUC,
		ReconcileUC:     reconcileUC,
		UserRepo:        userRepo,
		IdempotencyRepo: idempotencyRepo,
		RateLimitRepo:   rateLimitRepo,
	})

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in goroutine so we can listen for shutdown signals
	go func() {
		log.Info().Str("port", cfg.App.Port).Msg("HTTP server started")
		if err := router.StartServer(srv); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for interrupt signal (Ctrl+C or SIGTERM from Railway)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server gracefully...")

	// Give in-flight requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := router.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped")
}
