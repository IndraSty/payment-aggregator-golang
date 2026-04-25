package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/pkg/validator"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type authUsecase struct {
	userRepo domain.UserRepository
	cfg      *config.JWTConfig
}

// NewAuthUsecase creates a new auth usecase.
func NewAuthUsecase(userRepo domain.UserRepository, cfg *config.JWTConfig) domain.AuthUsecase {
	return &authUsecase{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

func (u *authUsecase) Register(ctx context.Context, input *domain.RegisterInput) (*domain.RegisterOutput, error) {
	// Validate input
	if err := validator.ValidateRegister(input.Email, input.Password); err != nil {
		return nil, domain.NewAppError(400, err.Error(), nil)
	}

	// Hash password with bcrypt (cost 12 — good balance of security vs speed)
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, domain.ErrInternal(fmt.Errorf("bcrypt hash: %w", err))
	}

	// Generate random API key (32 bytes = 64 hex chars)
	rawKey, err := generateSecureToken(32)
	if err != nil {
		return nil, domain.ErrInternal(fmt.Errorf("generate api key: %w", err))
	}

	// Store only the SHA-256 hash of the API key — never store plaintext
	hashedKey := sha256Hex(rawKey)

	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		PasswordHash: string(hash),
		APIKey:       hashedKey,
		CreatedAt:    time.Now(),
	}

	if err := u.userRepo.Create(ctx, user); err != nil {
		if err == domain.ErrUserAlreadyExists {
			return nil, domain.ErrConflict("email already registered")
		}
		return nil, domain.ErrInternal(err)
	}

	log.Info().Str("user_id", user.ID).Msg("New user registered")

	return &domain.RegisterOutput{
		User:            user,
		PlaintextAPIKey: rawKey, // returned ONCE — never stored, never logged
	}, nil
}

func (u *authUsecase) Login(ctx context.Context, input *domain.LoginInput) (*domain.TokenPair, error) {
	user, err := u.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		if err == domain.ErrUserNotFound {
			// Use same error message as wrong password — prevents user enumeration
			return nil, domain.ErrUnauth("invalid email or password")
		}
		return nil, domain.ErrInternal(err)
	}

	// Compare password with stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, domain.ErrUnauth("invalid email or password")
	}

	tokens, err := u.generateTokenPair(user.ID, user.Email)
	if err != nil {
		return nil, domain.ErrInternal(err)
	}

	log.Info().Str("user_id", user.ID).Msg("User logged in")
	return tokens, nil
}

func (u *authUsecase) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	claims, err := u.parseToken(refreshToken)
	if err != nil {
		return nil, domain.ErrUnauth("invalid or expired refresh token")
	}

	// Verify it's actually a refresh token
	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return nil, domain.ErrUnauth("invalid token type")
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	tokens, err := u.generateTokenPair(userID, email)
	if err != nil {
		return nil, domain.ErrInternal(err)
	}

	return tokens, nil
}

// generateTokenPair creates a new access + refresh token pair.
func (u *authUsecase) generateTokenPair(userID, email string) (*domain.TokenPair, error) {
	now := time.Now()

	// Access token — short lived (15 minutes)
	accessClaims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"type":  "access",
		"iat":   now.Unix(),
		"exp":   now.Add(u.cfg.AccessExpiry).Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).
		SignedString([]byte(u.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token — long lived (7 days)
	refreshClaims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"type":  "refresh",
		"iat":   now.Unix(),
		"exp":   now.Add(u.cfg.RefreshExpiry).Unix(),
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).
		SignedString([]byte(u.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(u.cfg.AccessExpiry.Seconds()),
	}, nil
}

// parseToken validates and parses a JWT token string.
func (u *authUsecase) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(u.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// sha256Hex returns the SHA-256 hex digest of a string.
func sha256Hex(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
