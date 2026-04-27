package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/IndraSty/payment-aggregator-golang/internal/domain/mock"
	"github.com/IndraSty/payment-aggregator-golang/internal/usecase"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthUsecase(userRepo *mock.MockUserRepository) domain.AuthUsecase {
	cfg := &config.JWTConfig{
		Secret:        "test-secret-key-minimum-32-characters-long",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	return usecase.NewAuthUsecase(userRepo, cfg)
}

// TestRegister_Success tests successful user registration
func TestRegister_Success(t *testing.T) {
	userRepo := new(mock.MockUserRepository)
	uc := setupAuthUsecase(userRepo)

	ctx := context.Background()
	input := &domain.RegisterInput{
		Email:    "newuser@example.com",
		Password: "securepassword123",
	}

	userRepo.On("Create", ctx, testifymock.AnythingOfType("*domain.User")).Return(nil)

	output, err := uc.Register(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.NotEmpty(t, output.PlaintextAPIKey)
	assert.Equal(t, input.Email, output.User.Email)
	// API key must be stored as hash — not plaintext
	assert.NotEqual(t, output.PlaintextAPIKey, output.User.APIKey)

	userRepo.AssertExpectations(t)
}

// TestRegister_DuplicateEmail tests that duplicate email returns conflict error
func TestRegister_DuplicateEmail(t *testing.T) {
	userRepo := new(mock.MockUserRepository)
	uc := setupAuthUsecase(userRepo)

	ctx := context.Background()
	input := &domain.RegisterInput{
		Email:    "existing@example.com",
		Password: "securepassword123",
	}

	userRepo.On("Create", ctx, testifymock.AnythingOfType("*domain.User")).Return(domain.ErrUserAlreadyExists)

	output, err := uc.Register(ctx, input)

	assert.Nil(t, output)
	assert.Error(t, err)

	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, 409, appErr.Code)
}

// TestRegister_WeakPassword tests password validation
func TestRegister_WeakPassword(t *testing.T) {
	userRepo := new(mock.MockUserRepository)
	uc := setupAuthUsecase(userRepo)

	ctx := context.Background()
	input := &domain.RegisterInput{
		Email:    "user@example.com",
		Password: "short", // less than 8 chars
	}

	output, err := uc.Register(ctx, input)

	assert.Nil(t, output)
	assert.Error(t, err)
	// userRepo.Create must never be called
	userRepo.AssertNotCalled(t, "Create")
}

// TestLogin_Success tests successful login returns token pair
func TestLogin_Success(t *testing.T) {
	userRepo := new(mock.MockUserRepository)
	uc := setupAuthUsecase(userRepo)

	ctx := context.Background()

	// Hash password as it would be stored
	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), 12)

	existingUser := &domain.User{
		ID:           "some-uuid",
		Email:        "user@example.com",
		PasswordHash: string(hash),
	}

	input := &domain.LoginInput{
		Email:    "user@example.com",
		Password: "correctpassword",
	}

	userRepo.On("GetByEmail", ctx, input.Email).Return(existingUser, nil)

	tokens, err := uc.Login(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, tokens)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, int64(900), tokens.ExpiresIn) // 15 minutes = 900 seconds
}

// TestLogin_WrongPassword tests that wrong password returns 401
func TestLogin_WrongPassword(t *testing.T) {
	userRepo := new(mock.MockUserRepository)
	uc := setupAuthUsecase(userRepo)

	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), 12)
	existingUser := &domain.User{
		ID:           "some-uuid",
		Email:        "user@example.com",
		PasswordHash: string(hash),
	}

	input := &domain.LoginInput{
		Email:    "user@example.com",
		Password: "wrongpassword",
	}

	userRepo.On("GetByEmail", ctx, input.Email).Return(existingUser, nil)

	tokens, err := uc.Login(ctx, input)

	assert.Nil(t, tokens)
	assert.Error(t, err)

	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, 401, appErr.Code)
}

// TestLogin_UserNotFound tests that unknown email returns same error as wrong password
// This prevents user enumeration attacks
func TestLogin_UserNotFound(t *testing.T) {
	userRepo := new(mock.MockUserRepository)
	uc := setupAuthUsecase(userRepo)

	ctx := context.Background()
	input := &domain.LoginInput{
		Email:    "notexist@example.com",
		Password: "anypassword",
	}

	userRepo.On("GetByEmail", ctx, input.Email).Return(nil, domain.ErrUserNotFound)

	tokens, err := uc.Login(ctx, input)

	assert.Nil(t, tokens)
	assert.Error(t, err)

	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	// Must be 401, not 404 — prevents user enumeration
	assert.Equal(t, 401, appErr.Code)
}
