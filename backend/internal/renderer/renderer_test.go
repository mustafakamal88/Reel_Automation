package renderer

import (
	"context"
	"encoding/json"
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

func TestRenderManualClipStoresRightsMetadata(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	base := t.TempDir()
	source := buildClipSourceFixture(t, base)
	input := testClipInput(source)
	res := RenderManualClip(context.Background(), Config{OutputDir: base, FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}, input)
	if res.Status != StatusCompleted {
		t.Fatalf("status = %q notes = %q", res.Status, res.Notes)
	}

	metaPath := filepath.Join(base, input.WorkspaceID, input.ClipID, "clip-metadata.json")
	f, err := os.Open(metaPath)
	if err != nil {
		t.Fatalf("open clip metadata: %v", err)
	}
	defer f.Close()
	var got ClipInput
	if err := json.NewDecoder(f).Decode(&got); err != nil {
		t.Fatalf("decode clip metadata: %v", err)
	}
	if got.Rights.SourceURL != input.Rights.SourceURL || got.Rights.AttributionText != input.Rights.AttributionText {
		t.Fatalf("rights metadata not stored: %+v", got.Rights)
	}
	if !got.Rights.UserConfirmedRights {
		t.Fatal("user_confirmed_rights was not stored")
	}
}

func TestRenderManualClipRefusesUnconfirmedExternalSource(t *testing.T) {
	base := t.TempDir()
	input := testClipInput(filepath.Join(base, "not-needed.mp4"))
	input.SourceModel = ClipSourceExternalURLPendingRightsConfirmation
	input.Rights.UserConfirmedRights = false
	input.Rights.SourceURL = "https://example.com/watch?v=random"

	res := RenderManualClip(context.Background(), Config{OutputDir: base, FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}, input)
	if res.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if res.Notes != "external URL source requires user rights confirmation before rendering" {
		t.Fatalf("notes = %q", res.Notes)
	}
	assertNoFakeMedia(t, base)
}

func TestRenderManualClipProducesVideoAndThumbnail(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	base := t.TempDir()
	source := buildClipSourceFixture(t, base)
	res := RenderManualClip(context.Background(), Config{OutputDir: base, FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}, testClipInput(source))
	if res.Status != StatusCompleted {
		t.Fatalf("status = %q notes = %q", res.Status, res.Notes)
	}
	if !fileExists(res.VideoPath) {
		t.Fatalf("video.mp4 missing: %s", res.VideoPath)
	}
	if !fileExists(res.ThumbnailPath) {
		t.Fatalf("thumbnail.png missing: %s", res.ThumbnailPath)
	}
	if res.VideoWidth != VideoWidth || res.VideoHeight != VideoHeight {
		t.Fatalf("resolution = %dx%d, want %dx%d", res.VideoWidth, res.VideoHeight, VideoWidth, VideoHeight)
	}
	if res.RendererVersion != ClipRendererVersion {
		t.Fatalf("renderer version = %q, want %q", res.RendererVersion, ClipRendererVersion)
	}
}

func testClipInput(source string) ClipInput {
	return ClipInput{
		WorkspaceID:     "workspace-1",
		ClipID:          "clip-1",
		SourceModel:     ClipSourceCreativeCommons,
		SourceVideoPath: source,
		Rights: ClipRightsMetadata{
			SourceURL:            "https://example.com/source",
			SourceTitle:          "Source Clip",
			SourceCreator:        "Creator",
			SourceLicense:        "CC BY 4.0",
			AttributionText:      "Source Clip by Creator, CC BY 4.0",
			UserConfirmedRights:  true,
			CopyrightOverlayText: "Creator / CC BY 4.0",
			PlatformSource:       "manual",
		},
		Branding: ClipBrandingSettings{
			TopBannerText:     "TREND CLIP",
			BottomBannerText:  "FOLLOW FOR THE FULL STORY",
			WatermarkText:     "@trendcortex",
			CTAText:           "Save this",
			FontStylePreset:   "bold_editorial",
			TopBannerColor:    "#111827",
			BottomBannerColor: "#0f766e",
		},
		ManualRange:     ClipManualRange{StartSeconds: 0, EndSeconds: 1.5},
		Captions:        "A short licensed clip.",
		IncludeCaptions: true,
		AIHighlights:    DefaultClipAIHighlightMetadata(),
	}
}

func buildClipSourceFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "source.mp4")
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=size=640x360:rate=30:duration=2",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build clip fixture: %v: %s", err, string(out))
	}
	return path
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
