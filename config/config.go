package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App        AppConfig
	JWT        JWTConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	RateLimit  RateLimitConfig
	Midtrans   MidtransConfig
	Xendit     XenditConfig
	Stripe     StripeConfig
	CB         CircuitBreakerConfig
	Reconcile  ReconcileConfig
	Prometheus PrometheusConfig
}

type AppConfig struct {
	Name    string
	Env     string
	Port    string
	BaseURL string
}

type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL             string
	IdempotencyTTL  time.Duration
	RateLimitWindow time.Duration
}

type RateLimitConfig struct {
	PerIP     int
	PerAPIKey int
}

type MidtransConfig struct {
	ServerKey    string
	ClientKey    string
	IsProduction bool
}

type XenditConfig struct {
	SecretKey     string
	CallbackToken string
	IsProduction  bool
}

type StripeConfig struct {
	SecretKey     string
	WebhookSecret string
	IsProduction  bool
}

type CircuitBreakerConfig struct {
	MaxRequests  uint32
	Interval     time.Duration
	Timeout      time.Duration
	FailureRatio float64
}

type ReconcileConfig struct {
	CronSchedule  string
	LookbackHours int
}

type PrometheusConfig struct {
	Enabled  bool
	PushURL  string
	Username string
	APIKey   string
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Read .env file if exists, ignore error if not found (Railway uses real env vars)
	_ = viper.ReadInConfig()

	setDefaults()

	cfg := &Config{
		App: AppConfig{
			Name:    viper.GetString("APP_NAME"),
			Env:     viper.GetString("APP_ENV"),
			Port:    viper.GetString("APP_PORT"),
			BaseURL: viper.GetString("APP_BASE_URL"),
		},
		JWT: JWTConfig{
			Secret:        viper.GetString("JWT_SECRET"),
			AccessExpiry:  viper.GetDuration("JWT_ACCESS_EXPIRY"),
			RefreshExpiry: viper.GetDuration("JWT_REFRESH_EXPIRY"),
		},
		Database: DatabaseConfig{
			URL:             viper.GetString("DATABASE_URL"),
			MaxOpenConns:    viper.GetInt("DATABASE_MAX_OPEN_CONNS"),
			MaxIdleConns:    viper.GetInt("DATABASE_MAX_IDLE_CONNS"),
			ConnMaxLifetime: viper.GetDuration("DATABASE_CONN_MAX_LIFETIME"),
		},
		Redis: RedisConfig{
			URL:             viper.GetString("REDIS_URL"),
			IdempotencyTTL:  viper.GetDuration("REDIS_IDEMPOTENCY_TTL"),
			RateLimitWindow: viper.GetDuration("REDIS_RATE_LIMIT_WINDOW"),
		},
		RateLimit: RateLimitConfig{
			PerIP:     viper.GetInt("RATE_LIMIT_PER_IP"),
			PerAPIKey: viper.GetInt("RATE_LIMIT_PER_API_KEY"),
		},
		Midtrans: MidtransConfig{
			ServerKey:    viper.GetString("MIDTRANS_SERVER_KEY"),
			ClientKey:    viper.GetString("MIDTRANS_CLIENT_KEY"),
			IsProduction: viper.GetBool("MIDTRANS_IS_PRODUCTION"),
		},
		Xendit: XenditConfig{
			SecretKey:     viper.GetString("XENDIT_SECRET_KEY"),
			CallbackToken: viper.GetString("XENDIT_CALLBACK_TOKEN"),
			IsProduction:  viper.GetBool("XENDIT_IS_PRODUCTION"),
		},
		Stripe: StripeConfig{
			SecretKey:     viper.GetString("STRIPE_SECRET_KEY"),
			WebhookSecret: viper.GetString("STRIPE_WEBHOOK_SECRET"),
			IsProduction:  viper.GetBool("STRIPE_IS_PRODUCTION"),
		},
		CB: CircuitBreakerConfig{
			MaxRequests:  uint32(viper.GetInt("CB_MAX_REQUESTS")),
			Interval:     viper.GetDuration("CB_INTERVAL"),
			Timeout:      viper.GetDuration("CB_TIMEOUT"),
			FailureRatio: viper.GetFloat64("CB_FAILURE_RATIO"),
		},
		Reconcile: ReconcileConfig{
			CronSchedule:  viper.GetString("RECONCILE_CRON_SCHEDULE"),
			LookbackHours: viper.GetInt("RECONCILE_LOOKBACK_HOURS"),
		},
		Prometheus: PrometheusConfig{
			Enabled:  viper.GetBool("PROMETHEUS_ENABLED"),
			PushURL:  viper.GetString("GRAFANA_PUSH_URL"),
			Username: viper.GetString("GRAFANA_USERNAME"),
			APIKey:   viper.GetString("GRAFANA_API_KEY"),
		},
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func setDefaults() {
	viper.SetDefault("APP_NAME", "payment-aggregator")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", "8080")
	viper.SetDefault("JWT_ACCESS_EXPIRY", "15m")
	viper.SetDefault("JWT_REFRESH_EXPIRY", "168h")
	viper.SetDefault("DATABASE_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DATABASE_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DATABASE_CONN_MAX_LIFETIME", "5m")
	viper.SetDefault("REDIS_IDEMPOTENCY_TTL", "24h")
	viper.SetDefault("REDIS_RATE_LIMIT_WINDOW", "1m")
	viper.SetDefault("RATE_LIMIT_PER_IP", 60)
	viper.SetDefault("RATE_LIMIT_PER_API_KEY", 1000)
	viper.SetDefault("CB_MAX_REQUESTS", 5)
	viper.SetDefault("CB_INTERVAL", "60s")
	viper.SetDefault("CB_TIMEOUT", "30s")
	viper.SetDefault("CB_FAILURE_RATIO", 0.6)
	viper.SetDefault("RECONCILE_CRON_SCHEDULE", "0 2 * * *")
	viper.SetDefault("RECONCILE_LOOKBACK_HOURS", 24)
	viper.SetDefault("PROMETHEUS_ENABLED", true)
}

func validate(cfg *Config) error {
	if cfg.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.Midtrans.ServerKey == "" {
		return fmt.Errorf("MIDTRANS_SERVER_KEY is required")
	}
	if cfg.Xendit.SecretKey == "" {
		return fmt.Errorf("XENDIT_SECRET_KEY is required")
	}
	if cfg.Stripe.SecretKey == "" {
		return fmt.Errorf("STRIPE_SECRET_KEY is required")
	}
	return nil
}

// IsDevelopment returns true when running in development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true when running in production mode
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
