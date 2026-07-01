package storage

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportZipFilename(t *testing.T) {
	got := ExportZipFilename("2026-06-30")
	want := "trendcortex-batch-2026-06-30.zip"
	if got != want {
		t.Errorf("ExportZipFilename(%q) = %q, want %q", "2026-06-30", got, want)
	}
}

// TestBuildReelBatchZip_ContentPaths writes a ZIP for two reels — one with
// both real artifacts present on disk, one with neither — and asserts the
// resulting entry names exactly match the required layout, including that
// video.mp4/thumbnail.png are only present when a real source file existed.
func TestBuildReelBatchZip_ContentPaths(t *testing.T) {
	tmp := t.TempDir()

	// Simulate a real (already-rendered) artifact pair for reel 1 only.
	videoSrc := filepath.Join(tmp, "src-video.mp4")
	thumbSrc := filepath.Join(tmp, "src-thumb.png")
	if err := os.WriteFile(videoSrc, []byte("fake-mp4-bytes-for-test"), 0644); err != nil {
		t.Fatalf("write video fixture: %v", err)
	}
	if err := os.WriteFile(thumbSrc, []byte("fake-png-bytes-for-test"), 0644); err != nil {
		t.Fatalf("write thumb fixture: %v", err)
	}

	reels := []ReelExportContent{
		{
			Rank: 1, Title: "Reel One", Description: "desc", Hashtags: "#a", Script: "script", ThumbnailBrief: "brief",
			VideoSrcPath: videoSrc, ThumbnailSrcPath: thumbSrc,
			Metadata: ReelExportMetadata{Rank: 1, HasVideo: true, HasThumbnail: true},
		},
		{
			Rank: 2, Title: "Reel Two", Description: "desc2", Hashtags: "#b", Script: "script2", ThumbnailBrief: "brief2",
			Metadata: ReelExportMetadata{Rank: 2, HasVideo: false, HasThumbnail: false},
		},
	}
	summary := BatchExportSummary{BatchID: "batch-1", Date: "2026-06-30", ReelCount: 2}

	exportDir := filepath.Join(tmp, "exports")
	zipPath, err := BuildReelBatchZip(exportDir, "2026-06-30", summary, reels)
	if err != nil {
		t.Fatalf("BuildReelBatchZip: %v", err)
	}
	if filepath.Base(zipPath) != "trendcortex-batch-2026-06-30.zip" {
		t.Errorf("unexpected zip filename: %s", zipPath)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}

	wantPresent := []string{
		"reel-01/title.txt", "reel-01/description.txt", "reel-01/hashtags.txt",
		"reel-01/script.txt", "reel-01/thumbnail-brief.txt", "reel-01/metadata.json",
		"reel-01/video.mp4", "reel-01/thumbnail.png",
		"reel-02/title.txt", "reel-02/description.txt", "reel-02/hashtags.txt",
		"reel-02/script.txt", "reel-02/thumbnail-brief.txt", "reel-02/metadata.json",
		"batch-summary.json",
	}
	for _, name := range wantPresent {
		if !names[name] {
			t.Errorf("expected zip entry %q to be present, got entries: %v", name, names)
		}
	}

	wantAbsent := []string{"reel-02/video.mp4", "reel-02/thumbnail.png"}
	for _, name := range wantAbsent {
		if names[name] {
			t.Errorf("zip entry %q must NOT be present (no real artifact existed on disk)", name)
		}
	}
}

// TestBuildReelBatchZip_MetadataJSONStructure verifies metadata.json round-trips
// the per-reel artifact fields that the export endpoint is responsible for
// reporting honestly.
func TestBuildReelBatchZip_MetadataJSONStructure(t *testing.T) {
	tmp := t.TempDir()
	meta := ReelExportMetadata{
		ReelPlanID: "reel-plan-1", Rank: 1, Platform: "tiktok", TitleIdea: "Title", Status: "draft",
		HasVideo: true, HasThumbnail: false,
		VideoFormat: "mp4", VideoWidth: 1080, VideoHeight: 1920, VideoDurationSeconds: 12.5,
		VideoCodec: "h264", AudioCodec: "aac",
	}
	reels := []ReelExportContent{{Rank: 1, Title: "t", Metadata: meta}}
	summary := BatchExportSummary{BatchID: "b1", Date: "2026-06-30", ReelCount: 1}

	zipPath, err := BuildReelBatchZip(filepath.Join(tmp, "exports"), "2026-06-30", summary, reels)
	if err != nil {
		t.Fatalf("BuildReelBatchZip: %v", err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	var got ReelExportMetadata
	found := false
	for _, f := range zr.File {
		if f.Name != "reel-01/metadata.json" {
			continue
		}
		found = true
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open metadata.json: %v", err)
		}
		defer rc.Close()
		if err := json.NewDecoder(rc).Decode(&got); err != nil {
			t.Fatalf("decode metadata.json: %v", err)
		}
	}
	if !found {
		t.Fatal("reel-01/metadata.json not found in zip")
	}

	if got != meta {
		t.Errorf("metadata.json round-trip mismatch:\n got  %+v\n want %+v", got, meta)
	}
}

func TestBuildReelBatchZip_NoFakeMediaWhenAllArtifactsMissing(t *testing.T) {
	tmp := t.TempDir()
	reels := []ReelExportContent{
		{Rank: 1, Title: "t1", Metadata: ReelExportMetadata{Rank: 1}},
		{Rank: 2, Title: "t2", Metadata: ReelExportMetadata{Rank: 2}},
	}
	summary := BatchExportSummary{BatchID: "b1", Date: "2026-06-30", ReelCount: 2}

	zipPath, err := BuildReelBatchZip(filepath.Join(tmp, "exports"), "2026-06-30", summary, reels)
	if err != nil {
		t.Fatalf("BuildReelBatchZip: %v", err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.Base(f.Name) == "video.mp4" || filepath.Base(f.Name) == "thumbnail.png" {
			t.Errorf("no video.mp4/thumbnail.png should exist when no real artifact was provided, found %q", f.Name)
		}
	}
}
