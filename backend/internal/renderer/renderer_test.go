package renderer

import (
	"context"
	"os"
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
