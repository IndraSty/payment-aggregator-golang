package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateCreateCharge validates the input for creating a charge.
func ValidateCreateCharge(input *domain.CreateChargeInput) error {
	if input.Amount <= 0 {
		return domain.ErrInvalidAmount
	}

	currency := strings.ToUpper(input.Currency)
	if _, ok := domain.SupportedCurrencies[currency]; !ok {
		return fmt.Errorf("%w: %s", domain.ErrInvalidCurrency, input.Currency)
	}

	if input.IdempotencyKey == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if len(input.IdempotencyKey) > 255 {
		return fmt.Errorf("idempotency key must not exceed 255 characters")
	}

	if input.CustomerEmail != "" && !emailRegex.MatchString(input.CustomerEmail) {
		return fmt.Errorf("invalid customer email format")
	}

	if input.PaymentMethod == "" {
		return fmt.Errorf("payment method is required")
	}

	return nil
}

// ValidateRegister validates registration input.
func ValidateRegister(email, password string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 72 {
		// bcrypt silently truncates at 72 bytes — reject to avoid confusion
		return fmt.Errorf("password must not exceed 72 characters")
	}
	return nil
}

// ValidatePagination normalizes and validates pagination params.
func ValidatePagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
