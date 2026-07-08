package renderer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrerequisiteStatus(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "provider key missing",
			cfg:  Config{Provider: "ffmpeg", FFmpegPath: "ffmpeg"},
			want: StatusProviderNotConnected,
		},
		{
			name: "unsupported provider",
			cfg:  Config{Provider: "remotion", OpenAIAPIKey: "present", FFmpegPath: "ffmpeg"},
			want: StatusProviderNotConnected,
		},
		{
			name: "renderer missing",
			cfg:  Config{Provider: "ffmpeg", OpenAIAPIKey: "present", FFmpegPath: filepath.Join(t.TempDir(), "missing-ffmpeg")},
			want: StatusRendererNotAvailable,
		},
		{
			name: "probe missing",
			cfg:  Config{Provider: "ffmpeg", OpenAIAPIKey: "present", FFmpegPath: "ffmpeg", FFprobePath: filepath.Join(t.TempDir(), "missing-ffprobe")},
			want: StatusRendererNotAvailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := prerequisiteStatus(tc.cfg)
			if got != tc.want {
				t.Fatalf("prerequisiteStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSafeOutputDirConfinesToBase(t *testing.T) {
	base := t.TempDir()
	got, err := SafeOutputDir(base, "../../workspace", "../reel")
	if err != nil {
		t.Fatalf("SafeOutputDir returned error: %v", err)
	}

	rel, err := filepath.Rel(base, got)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	if rel == ".." || len(rel) >= 3 && rel[:3] == "../" {
		t.Fatalf("output path escaped base: %s", got)
	}
	if filepath.Base(got) != "reel" {
		t.Fatalf("unexpected sanitized reel segment in %q", got)
	}
}

func TestRenderReel_NoFakeArtifactsWhenProviderMissing(t *testing.T) {
	base := t.TempDir()
	res := RenderReel(context.Background(), Config{
		Provider:  "ffmpeg",
		OutputDir: base,
	}, ReelInput{
		WorkspaceID:    "workspace-1",
		ReelPlanID:     "reel-1",
		Title:          "Title",
		Script:         "Script",
		ThumbnailBrief: "Brief",
	})

	if res.Status != StatusProviderNotConnected {
		t.Fatalf("status = %q, want %q", res.Status, StatusProviderNotConnected)
	}
	assertNoFakeMedia(t, base)
}

func TestRenderReel_NoFakeThumbnailWhenProviderMissing(t *testing.T) {
	base := t.TempDir()
	res := RenderReel(context.Background(), Config{
		Provider:  "ffmpeg",
		OutputDir: base,
	}, ReelInput{WorkspaceID: "w", ReelPlanID: "r", Script: "script"})
	if res.Status != StatusProviderNotConnected {
		t.Fatalf("status = %q, want %q", res.Status, StatusProviderNotConnected)
	}
	assertNoFakeMedia(t, base)
}

func TestRenderSimpleTextReelProducesArtifacts(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	base := t.TempDir()
	res := RenderSimpleTextReel(context.Background(), Config{
		OutputDir:   base,
		FFmpegPath:  "ffmpeg",
		FFprobePath: "ffprobe",
	}, ReelInput{
		WorkspaceID:    "workspace-1",
		ReelPlanID:     "daily-package-reel-01",
		Title:          "A Real Renderer Test",
		Script:         "This is a controlled single reel render with readable overlay text.",
		Description:    "No stock footage is used.",
		ThumbnailBrief: "Simple generated visual background",
	})

	if res.Status != StatusCompleted {
		t.Fatalf("status = %q notes = %q", res.Status, res.Notes)
	}
	if !fileExists(res.VideoPath) {
		t.Fatalf("video artifact missing: %s", res.VideoPath)
	}
	if !fileExists(res.ThumbnailPath) {
		t.Fatalf("thumbnail artifact missing: %s", res.ThumbnailPath)
	}
	if res.VideoWidth != VideoWidth || res.VideoHeight != VideoHeight {
		t.Fatalf("resolution = %dx%d, want %dx%d", res.VideoWidth, res.VideoHeight, VideoWidth, VideoHeight)
	}
	if res.RendererVersion != SimpleRendererVersion {
		t.Fatalf("renderer version = %q, want %q", res.RendererVersion, SimpleRendererVersion)
	}
	if res.RendererVersion != "quality_v1" {
		t.Fatalf("renderer version = %q, want quality_v1", res.RendererVersion)
	}
	if res.VideoDurationSeconds == nil {
		t.Fatal("duration missing")
	}
	if *res.VideoDurationSeconds < 25 || *res.VideoDurationSeconds > 45 {
		t.Fatalf("duration = %.2fs, want 25-45s", *res.VideoDurationSeconds)
	}
}

func assertNoFakeMedia(t *testing.T, base string) {
	t.Helper()
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Base(path) {
		case "video.mp4", "thumbnail.png":
			t.Fatalf("unexpected fake media artifact created: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk media dir: %v", err)
	}
}
