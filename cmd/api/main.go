package main

import (
	"os"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/repository/postgres"
	redisrepo "github.com/IndraSty/payment-aggregator-golang/internal/repository/redis"
	"github.com/IndraSty/payment-aggregator-golang/pkg/database"
	"github.com/IndraSty/payment-aggregator-golang/pkg/logger"
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

	// Run migrations
	if err := database.RunMigrations(cfg.Database.URL); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
		os.Exit(1)
	}

	// Connect PostgreSQL
	db, err := database.NewPostgresPool(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
		os.Exit(1)
	}
	defer db.Close()

	// Connect Redis
	rdb, err := database.NewRedisClient(&cfg.Redis)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
		os.Exit(1)
	}
	defer rdb.Close()

	// Init repositories
	userRepo := postgres.NewUserRepository(db)
	transactionRepo := postgres.NewTransactionRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)
	reconcileRepo := postgres.NewReconciliationRepository(db)
	idempotencyRepo := redisrepo.NewIdempotencyRepository(rdb, cfg.Redis.IdempotencyTTL)
	rateLimitRepo := redisrepo.NewRateLimitRepository(rdb, cfg.Redis.RateLimitWindow)

	log.Info().Msg("All repositories initialized")

	// Suppress unused variable warnings until next stages
	_ = userRepo
	_ = transactionRepo
	_ = auditRepo
	_ = webhookRepo
	_ = reconcileRepo
	_ = idempotencyRepo
	_ = rateLimitRepo

}

// ensure time package is used
var _ = time.Second
