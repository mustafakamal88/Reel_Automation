package http

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/renderer"
)

func TestClipStudioUploadCreatesSourceID(t *testing.T) {
	s := testClipStudioServer(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("video", "source.mp4")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("mp4")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(stdhttp.MethodPost, "/api/clip-studio/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got clipStudioSourceResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.SourceID == "" || got.Status != "ready" || !got.CanRender {
		t.Fatalf("unexpected upload response: %+v", got)
	}
	if _, err := os.Stat(got.Metadata.FilePath); err != nil {
		t.Fatalf("uploaded source missing: %v", err)
	}
}

func TestClipStudioRenderFromSourceIDWorks(t *testing.T) {
	requireFFmpeg(t)
	s := testClipStudioServer(t)
	sourceID := writeClipStudioSourceFixture(t, s)

	body := strings.NewReader(`{
		"source_model":"user_upload",
		"source_id":"` + sourceID + `",
		"rights":{"user_confirmed_rights":true,"attribution_text":"Uploaded by test"},
		"branding":{"top_banner_text":"TOP","bottom_banner_text":"BOTTOM","watermark_text":"@test"},
		"manual_range":{"start_seconds":0,"end_seconds":1},
		"captions":"test",
		"include_captions":true,
		"ai_highlights":{"transcription_status":"not_run","suggested_clips_status":"not_run","hook_score_status":"not_run"}
	}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/clip-studio/render", body)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("render status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got clipStudioRenderResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Success || got.DownloadURL == "" {
		t.Fatalf("render did not succeed: %+v", got)
	}
}

func TestClipStudioUnconfirmedURLGenerateIsRefused(t *testing.T) {
	s := testClipStudioServer(t)
	body := strings.NewReader(`{
		"source_url":"https://example.com/source.mp4",
		"prompt":"make clips",
		"clip_length":"15s",
		"clip_count":1,
		"rights_confirmed":false,
		"rights":{"user_confirmed_rights":false},
		"branding":{}
	}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/clip-studio/generate", body)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusForbidden {
		t.Fatalf("generate status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}
}

func TestClipStudioDirectConfirmedVideoURLCanBeProcessed(t *testing.T) {
	requireFFmpeg(t)
	s := testClipStudioServer(t)
	sourcePath := buildClipStudioSourceFixture(t, t.TempDir())
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source fixture: %v", err)
	}
	oldClient := clipStudioHTTPClient
	clipStudioHTTPClient = &stdhttp.Client{Transport: roundTripFunc(func(req *stdhttp.Request) (*stdhttp.Response, error) {
		return &stdhttp.Response{
			StatusCode: stdhttp.StatusOK,
			Header:     stdhttp.Header{"Content-Type": []string{"video/mp4"}},
			Body:       io.NopCloser(bytes.NewReader(sourceBytes)),
		}, nil
	})}
	defer func() { clipStudioHTTPClient = oldClient }()

	body := strings.NewReader(`{
		"source_url":"https://example.com/source.mp4",
		"rights_confirmed":true,
		"rights":{"user_confirmed_rights":true,"attribution_text":"Direct source"}
	}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/clip-studio/source", body)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got clipStudioSourceResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "ready" || !got.DownloadReady {
		t.Fatalf("direct source was not processed: %+v", got)
	}
	if _, err := os.Stat(got.Metadata.FilePath); err != nil {
		t.Fatalf("downloaded source missing: %v", err)
	}
}

func TestClipStudioUnsupportedURLReturnsClearMessage(t *testing.T) {
	s := testClipStudioServer(t)
	body := strings.NewReader(`{
		"source_url":"https://www.youtube.com/watch?v=abc123",
		"rights_confirmed":true,
		"rights":{"user_confirmed_rights":true}
	}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/clip-studio/source", body)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got clipStudioSourceResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := "Upload the source file or connect an approved source before rendering."
	if got.Message != want || got.CanRender {
		t.Fatalf("unsupported URL response = %+v, want message %q and can_render=false", got, want)
	}
}

func TestClipStudioGenerateZipContainsClipsAndAttributionMetadata(t *testing.T) {
	requireFFmpeg(t)
	s := testClipStudioServer(t)
	sourceID := writeClipStudioSourceFixture(t, s)

	body := strings.NewReader(`{
		"source_id":"` + sourceID + `",
		"prompt":"make funny clips",
		"clip_length":"15s",
		"clip_count":1,
		"rights_confirmed":true,
		"rights":{"user_confirmed_rights":true},
		"advanced":{"source_model":"user_upload","attribution_text":"Generated attribution"},
		"branding":{"top_banner_text":"TOP","bottom_banner_text":"BOTTOM","watermark_text":"@test"}
	}`)
	req := httptest.NewRequest(stdhttp.MethodPost, "/api/clip-studio/generate", body)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("generate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got clipStudioGenerateResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Success || got.ZipFilename == "" {
		t.Fatalf("generate did not succeed: %+v", got)
	}

	zipPath := filepath.Join(s.cfg.ExportDir, "default-workspace", "clip-studio", got.ZipFilename)
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open generated zip: %v", err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, name := range []string{"clip-01/video.mp4", "clip-01/thumbnail.png", "clip-01/attribution.json", "manifest.json"} {
		if !names[name] {
			t.Fatalf("expected zip entry %q; entries=%v", name, names)
		}
	}
}

func testClipStudioServer(t *testing.T) *Server {
	t.Helper()
	tmp := t.TempDir()
	return &Server{cfg: &config.Config{
		MediaOutputDir: filepath.Join(tmp, "media"),
		ExportDir:      filepath.Join(tmp, "exports"),
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
	}}
}

func writeClipStudioSourceFixture(t *testing.T, s *Server) string {
	t.Helper()
	sourcePath := buildClipStudioSourceFixture(t, t.TempDir())
	meta := clipStudioSourceMetadata{
		SourceID:      "src-test",
		Kind:          "upload",
		FilePath:      sourcePath,
		SourceModel:   renderer.ClipSourceUserUpload,
		Rights:        renderer.ClipRightsMetadata{UserConfirmedRights: true, AttributionText: "fixture attribution"},
		Status:        "ready",
		CreatedAt:     time.Now().UTC(),
		DirectVideo:   true,
		SupportedType: true,
	}
	if err := s.writeClipStudioSource("default-workspace", meta); err != nil {
		t.Fatalf("write source metadata: %v", err)
	}
	return meta.SourceID
}

func buildClipStudioSourceFixture(t *testing.T, dir string) string {
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

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

type roundTripFunc func(*stdhttp.Request) (*stdhttp.Response, error)

func (f roundTripFunc) RoundTrip(req *stdhttp.Request) (*stdhttp.Response, error) {
	return f(req)
}
