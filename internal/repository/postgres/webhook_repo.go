package postgres

import (
	"context"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type webhookRepository struct {
	db *pgxpool.Pool
}

// NewWebhookRepository creates a new PostgreSQL-backed webhook event repository.
func NewWebhookRepository(db *pgxpool.Pool) domain.WebhookRepository {
	return &webhookRepository{db: db}
}

func (r *webhookRepository) Create(ctx context.Context, event *domain.WebhookEvent) error {
	query := `
		INSERT INTO webhook_events (id, provider, event_type, raw_payload, processed, received_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query,
		event.ID,
		event.Provider,
		event.EventType,
		event.RawPayload,
		event.Processed,
		event.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("webhookRepository.Create: %w", err)
	}
	return nil
}

func (r *webhookRepository) ExistsByEventType(ctx context.Context, provider domain.PaymentProvider, eventType string) (bool, error) {
	// Used for replay attack prevention
	// Check if we already processed an event with this exact provider + event_type combination
	query := `
		SELECT EXISTS(
			SELECT 1 FROM webhook_events
			WHERE provider = $1
			  AND event_type = $2
			  AND processed = true
		)
	`
	var exists bool
	err := r.db.QueryRow(ctx, query, provider, eventType).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("webhookRepository.ExistsByEventType: %w", err)
	}
	return exists, nil
}

func (r *webhookRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE webhook_events
		SET processed = true
		WHERE id = $1
	`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("webhookRepository.MarkProcessed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("webhookRepository.MarkProcessed: event not found")
	}
	return nil
}
