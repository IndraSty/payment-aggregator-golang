package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository creates a new PostgreSQL-backed user repository.
func NewUserRepository(db *pgxpool.Pool) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, api_key, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.APIKey,
		user.CreatedAt,
	)
	if err != nil {
		// Check for unique constraint violation (duplicate email or api_key)
		if isUniqueViolation(err) {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("userRepository.Create: %w", err)
	}
	return nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, api_key, created_at
		FROM users
		WHERE email = $1
	`
	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.APIKey,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("userRepository.GetByEmail: %w", err)
	}
	return user, nil
}

func (r *userRepository) GetByAPIKey(ctx context.Context, hashedKey string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, api_key, created_at
		FROM users
		WHERE api_key = $1
	`
	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, hashedKey).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.APIKey,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("userRepository.GetByAPIKey: %w", err)
	}
	return user, nil
}
