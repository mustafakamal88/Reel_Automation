package config

import (
	"errors"
	"os"
)

// Config holds all runtime configuration loaded from environment variables.
// Never hard-code secrets here — load them from environment only.
type Config struct {
	// Server
	Port    string
	AppEnv  string
	AppBase string
	APIBase string

	// Database
	DatabaseURL string

	// Session / encryption
	SessionSecret      string
	TokenEncryptionKey string

	// Platform OAuth credentials — loaded from .env, never committed
	YouTubeClientID     string
	YouTubeClientSecret string

	TikTokClientKey    string
	TikTokClientSecret string

	MetaAppID     string
	MetaAppSecret string

	XClientID     string
	XClientSecret string
}

// Load reads configuration from environment variables.
// Returns an error if required values are missing.
func Load() (*Config, error) {
	cfg := &Config{
		Port:    getEnv("PORT", "8080"),
		AppEnv:  getEnv("APP_ENV", "development"),
		AppBase: getEnv("APP_BASE_URL", "http://localhost:5173"),
		APIBase: getEnv("API_BASE_URL", "http://localhost:8080"),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		SessionSecret:      os.Getenv("SESSION_SECRET"),
		TokenEncryptionKey: os.Getenv("TOKEN_ENCRYPTION_KEY"),

		YouTubeClientID:     os.Getenv("YOUTUBE_CLIENT_ID"),
		YouTubeClientSecret: os.Getenv("YOUTUBE_CLIENT_SECRET"),

		TikTokClientKey:    os.Getenv("TIKTOK_CLIENT_KEY"),
		TikTokClientSecret: os.Getenv("TIKTOK_CLIENT_SECRET"),

		MetaAppID:     os.Getenv("META_APP_ID"),
		MetaAppSecret: os.Getenv("META_APP_SECRET"),

		XClientID:     os.Getenv("X_CLIENT_ID"),
		XClientSecret: os.Getenv("X_CLIENT_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return nil, errors.New("SESSION_SECRET is required")
	}
	if cfg.TokenEncryptionKey == "" {
		return nil, errors.New("TOKEN_ENCRYPTION_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
