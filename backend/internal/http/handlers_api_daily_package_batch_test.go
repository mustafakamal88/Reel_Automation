package http

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/renderer"
	"trendcortex/api/internal/storage"
)

func TestDailyPackageBatchRenderSuccessIncludesRenderedMedia(t *testing.T) {
	tmp := t.TempDir()
	exportDir := filepath.Join(tmp, "exports", "workspace", "daily-package")
	zipPath := buildDailyPackageRenderFixture(t, exportDir, 6)

	srv := NewServer(&config.Config{
		ExportDir:      filepath.Join(tmp, "exports"),
		MediaOutputDir: filepath.Join(tmp, "media"),
	}, nil, nil, nil)
	srv.renderDailyPackageReel = fakeDailyPackageRenderer(t, nil)

	jobID := "batch-success"
	srv.setDailyPackageRenderJob(dailyPackageRenderJob{ID: jobID, Status: "rendering"})
	srv.runDailyPackageBatchRender("workspace", exportDir, zipPath, jobID)

	job := srv.dailyRenderJobs[jobID]
	if job.Status != "completed" {
		t.Fatalf("job status = %q, want completed; error=%s", job.Status, job.RenderError)
	}
	if job.CompletedCount != 6 || job.FailedCount != 0 {
		t.Fatalf("counts = %d completed, %d failed; want 6, 0", job.CompletedCount, job.FailedCount)
	}

	names, manifest := readDailyPackageZipManifest(t, zipPath)
	if manifest.Status != "ready" {
		t.Fatalf("manifest status = %q, want ready", manifest.Status)
	}
	for rank := 1; rank <= 6; rank++ {
		video := fmt.Sprintf("reel-%02d/video.mp4", rank)
		thumbnail := fmt.Sprintf("reel-%02d/thumbnail.png", rank)
		if !names[video] || !names[thumbnail] {
			t.Fatalf("ZIP missing rendered media for reel-%02d; names=%v", rank, names)
		}
		if got := manifest.Reels[rank-1].RenderStatus; got != "rendered" {
			t.Fatalf("reel-%02d render_status = %q, want rendered", rank, got)
		}
	}
}

func TestDailyPackageBatchRenderFailureDoesNotCreateFakeFilesAndContinues(t *testing.T) {
	tmp := t.TempDir()
	exportDir := filepath.Join(tmp, "exports", "workspace", "daily-package")
	zipPath := buildDailyPackageRenderFixture(t, exportDir, 6)

	srv := NewServer(&config.Config{
		ExportDir:      filepath.Join(tmp, "exports"),
		MediaOutputDir: filepath.Join(tmp, "media"),
	}, nil, nil, nil)
	srv.renderDailyPackageReel = fakeDailyPackageRenderer(t, map[int]renderer.Result{
		2: {Status: renderer.StatusFailed, Notes: "intentional render failure"},
	})

	jobID := "batch-mixed"
	srv.setDailyPackageRenderJob(dailyPackageRenderJob{ID: jobID, Status: "rendering"})
	srv.runDailyPackageBatchRender("workspace", exportDir, zipPath, jobID)

	job := srv.dailyRenderJobs[jobID]
	if job.Status != "completed" {
		t.Fatalf("job status = %q, want completed; error=%s", job.Status, job.RenderError)
	}
	if job.CompletedCount != 5 || job.FailedCount != 1 {
		t.Fatalf("counts = %d completed, %d failed; want 5, 1", job.CompletedCount, job.FailedCount)
	}

	names, manifest := readDailyPackageZipManifest(t, zipPath)
	if manifest.Status != "ready_with_render_failures" {
		t.Fatalf("manifest status = %q, want ready_with_render_failures", manifest.Status)
	}
	if names["reel-02/video.mp4"] || names["reel-02/thumbnail.png"] {
		t.Fatalf("failed reel must not include fake media; names=%v", names)
	}
	if got := manifest.Reels[1].RenderStatus; got != "failed" {
		t.Fatalf("reel-02 render_status = %q, want failed", got)
	}
	if manifest.Reels[1].RenderError != "intentional render failure" {
		t.Fatalf("reel-02 render_error = %q", manifest.Reels[1].RenderError)
	}
	for _, rank := range []int{1, 3, 4, 5, 6} {
		video := fmt.Sprintf("reel-%02d/video.mp4", rank)
		thumbnail := fmt.Sprintf("reel-%02d/thumbnail.png", rank)
		if !names[video] || !names[thumbnail] {
			t.Fatalf("batch did not continue to reel-%02d; names=%v", rank, names)
		}
		if got := manifest.Reels[rank-1].RenderStatus; got != "rendered" {
			t.Fatalf("reel-%02d render_status = %q, want rendered", rank, got)
		}
	}
}

func buildDailyPackageRenderFixture(t *testing.T, exportDir string, count int) string {
	t.Helper()
	reels := make([]storage.DailyPackageReelContent, 0, count)
	manifestReels := make([]storage.DailyPackageManifestReel, 0, count)
	for rank := 1; rank <= count; rank++ {
		reels = append(reels, storage.DailyPackageReelContent{
			Rank:           rank,
			Script:         fmt.Sprintf("script %02d", rank),
			Caption:        "caption",
			Description:    "description",
			Hashtags:       "#one",
			ThumbnailBrief: "thumbnail brief",
			Metadata: storage.DailyPackageReelMetadata{
				Rank:         rank,
				CandidateID:  fmt.Sprintf("candidate-%02d", rank),
				Source:       "google_trends_rss",
				RenderStatus: "not_attempted",
			},
		})
		manifestReels = append(manifestReels, storage.DailyPackageManifestReel{
			Rank:         rank,
			CandidateID:  fmt.Sprintf("candidate-%02d", rank),
			Title:        fmt.Sprintf("title %02d", rank),
			Source:       "google_trends_rss",
			RenderStatus: "not_attempted",
		})
	}
	zipPath, _, err := storage.BuildDailyReelsPackageZip(exportDir, reels, storage.DailyPackageManifest{
		Date:        "2026-07-08",
		GeneratedAt: "2026-07-08T00:00:00Z",
		Status:      "ready_with_render_failures",
		Message:     "ready without rendered video",
		ReelCount:   count,
		Reels:       manifestReels,
	})
	if err != nil {
		t.Fatalf("build fixture zip: %v", err)
	}
	return zipPath
}

func fakeDailyPackageRenderer(t *testing.T, failures map[int]renderer.Result) func(context.Context, renderer.Config, renderer.ReelInput) renderer.Result {
	t.Helper()
	return func(_ context.Context, cfg renderer.Config, input renderer.ReelInput) renderer.Result {
		if result, ok := failures[input.Rank]; ok {
			return result
		}
		dir := filepath.Join(cfg.OutputDir, input.WorkspaceID, input.ReelPlanID)
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatalf("mkdir fake render dir: %v", err)
		}
		videoPath := filepath.Join(dir, "video.mp4")
		thumbnailPath := filepath.Join(dir, "thumbnail.png")
		if err := os.WriteFile(videoPath, []byte(fmt.Sprintf("rendered video %02d", input.Rank)), 0640); err != nil {
			t.Fatalf("write fake video: %v", err)
		}
		if err := os.WriteFile(thumbnailPath, []byte(fmt.Sprintf("rendered thumbnail %02d", input.Rank)), 0640); err != nil {
			t.Fatalf("write fake thumbnail: %v", err)
		}
		duration := 12.0
		return renderer.Result{
			Status:               renderer.StatusCompleted,
			Notes:                "test renderer produced files",
			VideoPath:            videoPath,
			VideoFormat:          "mp4",
			VideoWidth:           renderer.VideoWidth,
			VideoHeight:          renderer.VideoHeight,
			VideoDurationSeconds: &duration,
			VideoCodec:           "h264",
			AudioCodec:           "aac",
			ThumbnailPath:        thumbnailPath,
			ThumbnailFormat:      "png",
			ThumbnailWidth:       renderer.VideoWidth,
			ThumbnailHeight:      renderer.VideoHeight,
			RendererVersion:      "test-renderer",
		}
	}
}

func readDailyPackageZipManifest(t *testing.T, zipPath string) (map[string]bool, storage.DailyPackageManifest) {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	names := map[string]bool{}
	var manifest storage.DailyPackageManifest
	for _, f := range zr.File {
		names[f.Name] = true
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open manifest: %v", err)
		}
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			rc.Close()
			t.Fatalf("decode manifest: %v", err)
		}
		rc.Close()
	}
	return names, manifest
}
