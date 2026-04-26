package database

import (
	"context"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// NewPostgresPool creates a new PostgreSQL connection pool using pgx.
// Uses pgxpool for connection pooling — safe for concurrent use.
func NewPostgresPool(cfg *config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Apply pool settings from config
	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection is alive
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	stats := pool.Stat()
	log.Info().
		Int32("total_conns", stats.TotalConns()).
		Int32("max_conns", poolConfig.MaxConns).
		Msg("PostgreSQL connection pool established")

	return pool, nil
}

// RunMigrations runs all pending SQL migrations from the migrations directory.
func RunMigrations(databaseURL string) error {
	// Will be implemented using golang-migrate
	// Called from main.go before server starts
	log.Info().Msg("Running database migrations...")

	m, err := newMigrate(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to init migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		// "no change" is not an error — means migrations already applied
		if err.Error() == "no change" {
			log.Info().Msg("No pending migrations")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Info().Msg("Migrations applied successfully")
	return nil
}
