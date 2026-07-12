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

func TestLoadDotEnvFromRepositoryRootWithoutOverridingProcessEnv(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=9999\nAPP_ENV=from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORT", "7777")
	os.Unsetenv("APP_ENV")
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := LoadDotEnv(); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := os.Getenv("PORT"); got != "7777" {
		t.Fatalf("PORT = %q, want process env to win", got)
	}
	if got := os.Getenv("APP_ENV"); got != "from_file" {
		t.Fatalf("APP_ENV = %q, want value from .env", got)
	}
}

func TestLoadDotEnvFromBackendDirectory(t *testing.T) {
	root := t.TempDir()
	backend := filepath.Join(root, "backend")
	if err := os.Mkdir(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("API_BASE_URL=http://example.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	os.Unsetenv("API_BASE_URL")
	if err := os.Chdir(backend); err != nil {
		t.Fatal(err)
	}
	if err := LoadDotEnv(); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := os.Getenv("API_BASE_URL"); got != "http://example.test" {
		t.Fatalf("API_BASE_URL = %q", got)
	}
}
