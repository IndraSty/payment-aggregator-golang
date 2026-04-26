package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the global zerolog logger based on environment.
// In development: pretty console output with colors.
// In production: structured JSON output for log aggregators.
func Init(env string) {
	zerolog.TimeFieldFormat = time.RFC3339

	if strings.ToLower(env) == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
		})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().
		Str("env", env).
		Msg("Logger initialized")
}

// WithRequestID returns a logger with request_id field attached.
func WithRequestID(requestID string) zerolog.Logger {
	return log.With().Str("request_id", requestID).Logger()
}

// WithTransaction returns a logger with transaction_id field attached.
func WithTransaction(txID string) zerolog.Logger {
	return log.With().Str("transaction_id", txID).Logger()
}

// WithProvider returns a logger with provider field attached.
func WithProvider(provider string) zerolog.Logger {
	return log.With().Str("provider", provider).Logger()
}

// MaskSensitive replaces sensitive string with masked version.
// Used to prevent logging card numbers, secrets, etc.
// Example: MaskSensitive("4111111111111111") → "411111******1111"
func MaskSensitive(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

// MaskAPIKey masks an API key for safe logging.
// Example: MaskAPIKey("sk_test_abc123xyz") → "sk_t****xyz"
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
