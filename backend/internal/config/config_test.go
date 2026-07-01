package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultArtifactDirsUseWritableTempRoot(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SESSION_SECRET", "session-secret")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "token-key")
	t.Setenv("EXPORT_DIR", "")
	t.Setenv("MEDIA_OUTPUT_DIR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	wantExportDir := filepath.Join(os.TempDir(), "trendcortex", "exports")
	if cfg.ExportDir != wantExportDir {
		t.Fatalf("ExportDir = %q, want %q", cfg.ExportDir, wantExportDir)
	}

	wantMediaOutputDir := filepath.Join(os.TempDir(), "trendcortex", "generated-media")
	if cfg.MediaOutputDir != wantMediaOutputDir {
		t.Fatalf("MediaOutputDir = %q, want %q", cfg.MediaOutputDir, wantMediaOutputDir)
	}
}

func TestLoad_ArtifactDirsCanBeOverridden(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("SESSION_SECRET", "session-secret")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "token-key")
	t.Setenv("EXPORT_DIR", "/mnt/exports")
	t.Setenv("MEDIA_OUTPUT_DIR", "/mnt/media")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ExportDir != "/mnt/exports" {
		t.Fatalf("ExportDir = %q, want /mnt/exports", cfg.ExportDir)
	}
	if cfg.MediaOutputDir != "/mnt/media" {
		t.Fatalf("MediaOutputDir = %q, want /mnt/media", cfg.MediaOutputDir)
	}
}
