package http

import (
	"archive/zip"
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/renderer"
)

func TestRunRenderExportTest_MissingProviderCreatesManifestZipWithoutCrash(t *testing.T) {
	tmp := t.TempDir()
	s := &Server{cfg: &config.Config{
		RenderProvider: "ffmpeg",
		MediaOutputDir: filepath.Join(tmp, "media"),
		ExportDir:      filepath.Join(tmp, "exports"),
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
	}}

	res, err := s.runRenderExportTest(context.Background(), renderExportTestRequest{
		Title:              "Provider missing test",
		Script:             "A test script",
		Caption:            "A test caption",
		TargetPlatforms:    []string{"youtube", "tiktok"},
		NumberOfReels:      1,
		AllowLocalFallback: false,
	})
	if err != nil {
		t.Fatalf("runRenderExportTest: %v", err)
	}
	if !res.Success {
		t.Fatal("expected ZIP export to succeed even when provider-backed render is unavailable")
	}
	if res.RenderStatus != renderer.StatusProviderNotConnected {
		t.Fatalf("render_status = %q, want %q", res.RenderStatus, renderer.StatusProviderNotConnected)
	}
	if res.ExportStatus != "completed" {
		t.Fatalf("export_status = %q, want completed", res.ExportStatus)
	}
	if res.FallbackReason == "" {
		t.Fatal("expected fallback_reason to explain missing provider configuration")
	}

	zr, err := zip.OpenReader(res.ZipPath)
	if err != nil {
		t.Fatalf("open generated ZIP: %v", err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, name := range []string{
		"reel-01/caption.txt",
		"reel-01/metadata.json",
		"reel-01/youtube-description.txt",
		"reel-01/tiktok-caption.txt",
		"batch-summary.json",
		"export-manifest.json",
	} {
		if !names[name] {
			t.Fatalf("expected ZIP entry %q; entries=%v", name, names)
		}
	}
	for _, name := range []string{"reel-01/video.mp4", "reel-01/thumbnail.png"} {
		if names[name] {
			t.Fatalf("did not expect media entry %q when provider and fallback rendering are unavailable", name)
		}
	}
}

func TestRoutes_PostRenderExportTestCreatesZip(t *testing.T) {
	tmp := t.TempDir()
	s := &Server{cfg: &config.Config{
		RenderProvider: "ffmpeg",
		MediaOutputDir: filepath.Join(tmp, "media"),
		ExportDir:      filepath.Join(tmp, "exports"),
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
	}}

	body := strings.NewReader(`{
		"title":"Router smoke test",
		"script":"A test script",
		"caption":"A test caption",
		"target_platforms":["instagram"],
		"number_of_reels":1,
		"allow_local_fallback":false
	}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/reels/export-test", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusCreated {
		t.Fatalf("POST /api/reels/export-test status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("expected successful export response, body = %s", rec.Body.String())
	}
}
