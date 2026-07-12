package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
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

	// ExportDir is the local folder ZIP exports are written to. Never
	// committed to git — see backend/.gitignore.
	ExportDir string

	// Real media rendering. Provider credentials are loaded server-side only;
	// never expose these values to the frontend.
	RenderProvider     string
	MediaOutputDir     string
	OpenAIAPIKey       string
	OpenAITextModel    string
	OpenAITTSModel     string
	OpenAIImageModel   string
	FFmpegPath         string
	FFprobePath        string
	LocalAIWorkerURL   string
	LocalAIWorkerToken string

	// Trend discovery. Provider credentials and provider selection stay
	// server-side only; the browser receives status metadata and candidates.
	TrendDiscoveryProvider string
	TrendDiscoveryBaseURL  string
	TrendDiscoveryTimeout  string
	DefaultTrendCountry    string
	YouTubeAPIKey          string
	TikTokResearchClientID string
	TikTokResearchSecret   string
	TikTokResearchToken    string

	GoogleAdsDeveloperToken  string
	GoogleAdsCustomerID      string
	GoogleAdsLoginCustomerID string
	GoogleAdsClientID        string
	GoogleAdsClientSecret    string
	GoogleAdsRefreshToken    string

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

// LoadDotEnv loads the repository-root .env for local development. Existing
// process environment values win, so Railway variables continue to take
// precedence in production.
func LoadDotEnv() error {
	candidates := []string{
		filepath.Join(".", ".env"),
		filepath.Join("..", ".env"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return godotenv.Load(candidate)
		}
	}
	return nil
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
		ExportDir:   getEnv("EXPORT_DIR", defaultArtifactDir("exports")),

		RenderProvider:     getEnv("RENDER_PROVIDER", "ffmpeg"),
		MediaOutputDir:     getEnv("MEDIA_OUTPUT_DIR", defaultArtifactDir("generated-media")),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		OpenAITextModel:    getEnv("OPENAI_TEXT_MODEL", "gpt-4o-mini"),
		OpenAITTSModel:     getEnv("OPENAI_TTS_MODEL", "gpt-4o-mini-tts"),
		OpenAIImageModel:   getEnv("OPENAI_IMAGE_MODEL", "gpt-image-1"),
		FFmpegPath:         getEnv("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:        getEnv("FFPROBE_PATH", "ffprobe"),
		LocalAIWorkerURL:   os.Getenv("LOCAL_AI_WORKER_URL"),
		LocalAIWorkerToken: os.Getenv("LOCAL_AI_WORKER_TOKEN"),

		TrendDiscoveryProvider: getEnv("TREND_DISCOVERY_PROVIDER", ""),
		TrendDiscoveryBaseURL:  getEnv("TREND_DISCOVERY_BASE_URL", "https://trends.google.com/trending/rss"),
		TrendDiscoveryTimeout:  getEnv("TREND_DISCOVERY_TIMEOUT", "10s"),
		DefaultTrendCountry:    getEnv("DEFAULT_TREND_COUNTRY", "GB"),
		YouTubeAPIKey:          os.Getenv("YOUTUBE_API_KEY"),
		TikTokResearchClientID: os.Getenv("TIKTOK_RESEARCH_CLIENT_ID"),
		TikTokResearchSecret:   os.Getenv("TIKTOK_RESEARCH_CLIENT_SECRET"),
		TikTokResearchToken:    os.Getenv("TIKTOK_RESEARCH_TOKEN"),

		GoogleAdsDeveloperToken:  os.Getenv("GOOGLE_ADS_DEVELOPER_TOKEN"),
		GoogleAdsCustomerID:      os.Getenv("GOOGLE_ADS_CUSTOMER_ID"),
		GoogleAdsLoginCustomerID: os.Getenv("GOOGLE_ADS_LOGIN_CUSTOMER_ID"),
		GoogleAdsClientID:        os.Getenv("GOOGLE_ADS_CLIENT_ID"),
		GoogleAdsClientSecret:    os.Getenv("GOOGLE_ADS_CLIENT_SECRET"),
		GoogleAdsRefreshToken:    os.Getenv("GOOGLE_ADS_REFRESH_TOKEN"),

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

func defaultArtifactDir(name string) string {
	return filepath.Join(os.TempDir(), "trendcortex", name)
}
