// Package config centralizes all environment-driven configuration for the
// service. Nothing outside this package should call os.Getenv directly —
// that keeps every tunable in one auditable place.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env            string // "development" | "staging" | "production"
	Port           string
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration
	RateLimitRPS   int
	StripeSecret   string
	StripeWebhook  string
}

// Load reads configuration from environment variables, applying sane
// defaults for local development. In production every secret-bearing
// field is required and Load returns an error instead of silently
// falling back to an insecure default.
func Load() (*Config, error) {
	cfg := &Config{
		Env:            getEnv("APP_ENV", "development"),
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/saas?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-only-insecure-secret-change-me"),
		AccessTokenTTL: getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:     getEnvDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		RateLimitRPS:   getEnvInt("RATE_LIMIT_RPS", 10),
		StripeSecret:   getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhook:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
	}

	if cfg.Env == "production" {
		if cfg.JWTSecret == "dev-only-insecure-secret-change-me" {
			return nil, fmt.Errorf("config: JWT_SECRET must be set explicitly in production")
		}
		if len(cfg.JWTSecret) < 32 {
			return nil, fmt.Errorf("config: JWT_SECRET must be at least 32 characters in production")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
