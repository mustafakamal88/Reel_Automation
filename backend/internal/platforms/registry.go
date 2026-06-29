package platforms

import (
	"trendcortex/api/internal/config"
	"trendcortex/api/internal/oauth"
)

// BuildRegistry constructs the platform adapter registry from config.
// All adapters are registered even if credentials are missing — methods
// will return ErrCredentialsMissing at call time with a clear message.
func BuildRegistry(cfg *config.Config) oauth.Registry {
	return oauth.Registry{
		"youtube":   NewYouTubeAdapter(cfg.YouTubeClientID, cfg.YouTubeClientSecret, cfg.APIBase),
		"tiktok":    NewTikTokAdapter(cfg.TikTokClientKey, cfg.TikTokClientSecret, cfg.APIBase),
		"instagram": NewInstagramAdapter(cfg.MetaAppID, cfg.MetaAppSecret, cfg.APIBase),
		"facebook":  NewFacebookAdapter(cfg.MetaAppID, cfg.MetaAppSecret, cfg.APIBase),
		"threads":   NewThreadsAdapter(cfg.MetaAppID, cfg.MetaAppSecret, cfg.APIBase),
		"x":         NewXAdapter(cfg.XClientID, cfg.XClientSecret, cfg.APIBase),
	}
}
