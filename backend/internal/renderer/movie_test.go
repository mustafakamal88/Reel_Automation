package renderer

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderMovieRequiresSceneVisuals(t *testing.T) {
	result := RenderMovie(context.Background(), Config{OutputDir: t.TempDir(), FFmpegPath: "ffmpeg"}, MovieRenderInput{
		WorkspaceID: "workspace",
		MovieEditID: "edit",
		Scenes: []MovieSceneInput{{
			Title:    "Opening",
			Duration: 2,
		}},
	})
	if result.Status != StatusFailed {
		t.Fatalf("expected failed render, got %s", result.Status)
	}
	if result.Notes != "scene 1 needs a visual before rendering" {
		t.Fatalf("unexpected notes: %s", result.Notes)
	}
}

func TestRenderMovieProducesValidatedMP4(t *testing.T) {
	if _, err := os.Stat("/usr/bin/true"); err != nil {
		t.Skip("standard shell tools unavailable")
	}
	if _, err := os.Stat("/opt/homebrew/bin/ffmpeg"); err != nil {
		if _, err := os.Stat("/usr/local/bin/ffmpeg"); err != nil {
			if _, err := os.Stat("/usr/bin/ffmpeg"); err != nil {
				t.Skip("ffmpeg unavailable")
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual.png")
	writeTestMoviePNG(t, visual)

	result := RenderMovie(ctx, Config{OutputDir: filepath.Join(dir, "out"), FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}, MovieRenderInput{
		WorkspaceID:      "workspace",
		MovieEditID:      "edit",
		Title:            "Movie render",
		Width:            1080,
		Height:           1920,
		FrameRate:        30,
		QualityPreset:    "draft",
		CaptionsEnabled:  true,
		BrandingText:     "Creator",
		BrandingPosition: "top_right",
		Scenes: []MovieSceneInput{{
			ID:             "scene-1",
			Title:          "Opening",
			ScriptText:     "This is a real Movie Studio render test.",
			VisualPath:     visual,
			VisualMimeType: "image/png",
			Duration:       1.2,
			FitMode:        "fill_crop",
			MotionPreset:   "none",
			CaptionText:    "Real render test",
		}},
	})
	if result.Status != StatusCompleted {
		t.Fatalf("render failed: %s", result.Notes)
	}
	if _, err := os.Stat(result.VideoPath); err != nil {
		t.Fatalf("video missing: %v", err)
	}
	if result.VideoWidth != 1080 || result.VideoHeight != 1920 {
		t.Fatalf("unexpected dimensions: %dx%d", result.VideoWidth, result.VideoHeight)
	}
	if result.VideoDurationSeconds == nil || *result.VideoDurationSeconds <= 0 {
		t.Fatalf("expected probed duration, got %#v", result.VideoDurationSeconds)
	}
	if keepDir := os.Getenv("MOVIE_RENDER_KEEP_DIR"); keepDir != "" {
		if err := os.MkdirAll(keepDir, 0750); err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(keepDir, "movie-smoke.mp4")
		copyFileForTest(t, result.VideoPath, dst)
		t.Logf("kept movie render at %s", dst)
	}
}

func writeTestMoviePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 540, 960))
	for y := 0; y < 960; y++ {
		for x := 0; x < 540; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 120, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func copyFileForTest(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}
