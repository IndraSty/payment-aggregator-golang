package handler

import (
	"net/http"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authUC domain.AuthUsecase
}

func NewAuthHandler(authUC domain.AuthUsecase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

// RegisterRequest is the request body for POST /api/v1/auth/register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest is the request body for POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest is the request body for POST /api/v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Register godoc
// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Registration details"
// @Success 201 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 409 {object} APIResponse
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body",
		})
	}

	output, err := h.authUC.Register(c.Request().Context(), &domain.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return handleError(c, err)
	}

	return created(c, map[string]any{
		"user": map[string]any{
			"id":         output.User.ID,
			"email":      output.User.Email,
			"created_at": output.User.CreatedAt,
		},
		// API key returned ONCE — client must store this securely
		"api_key": output.PlaintextAPIKey,
		"message": "Store your API key securely. It will not be shown again.",
	})
}

// Login godoc
// @Summary Login and obtain JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login credentials"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body",
		})
	}

	tokens, err := h.authUC.Login(c.Request().Context(), &domain.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return handleError(c, err)
	}

	return ok(c, tokens)
}

// RefreshToken godoc
// @Summary Refresh access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RefreshRequest true "Refresh token"
// @Success 200 {object} APIResponse
// @Failure 401 {object} APIResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "refresh_token is required",
		})
	}

	tokens, err := h.authUC.RefreshToken(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return handleError(c, err)
	}

	return ok(c, tokens)
}
