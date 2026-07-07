package storage

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildDailyReelsPackageZipLayout(t *testing.T) {
	tmp := t.TempDir()
	reels := []DailyPackageReelContent{{
		Rank:           1,
		Script:         "script",
		Caption:        "caption",
		Description:    "description",
		Hashtags:       "#one\n#two",
		ThumbnailBrief: "thumbnail brief",
		Metadata: DailyPackageReelMetadata{
			Rank:         1,
			CandidateID:  "candidate-1",
			Source:       "google_trends_rss",
			RenderStatus: "provider_not_connected",
		},
	}}
	manifest := DailyPackageManifest{
		Date:        "2026-07-07",
		GeneratedAt: "2026-07-07T00:00:00Z",
		Status:      "ready_with_render_failures",
		Message:     "ready without rendered video",
		ReelCount:   1,
	}

	zipPath, included, err := BuildDailyReelsPackageZip(filepath.Join(tmp, "exports"), reels, manifest)
	if err != nil {
		t.Fatalf("BuildDailyReelsPackageZip: %v", err)
	}
	if filepath.Base(zipPath) != DailyReelsPackageFilename {
		t.Fatalf("zip filename = %q, want %q", filepath.Base(zipPath), DailyReelsPackageFilename)
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
	for _, name := range []string{
		"reel-01/script.txt",
		"reel-01/caption.txt",
		"reel-01/description.txt",
		"reel-01/hashtags.txt",
		"reel-01/thumbnail_brief.txt",
		"reel-01/platform_posts.json",
		"reel-01/trend_evidence.json",
		"reel-01/metadata.json",
		"manifest.json",
	} {
		if !names[name] {
			t.Fatalf("expected %q in ZIP; entries=%v", name, names)
		}
	}
	if names["reel-01/video.mp4"] {
		t.Fatal("video.mp4 must not be included unless a real rendered file exists")
	}
	if !containsString(included, "manifest.json") {
		t.Fatalf("included files missing manifest.json: %v", included)
	}
}

func TestDailyPackageRebuildAfterSuccessfulReel01RenderIncludesMedia(t *testing.T) {
	tmp := t.TempDir()
	exportDir := filepath.Join(tmp, "exports")
	mediaDir := filepath.Join(tmp, "media")
	if err := os.MkdirAll(mediaDir, 0750); err != nil {
		t.Fatalf("mkdir media dir: %v", err)
	}
	videoPath := filepath.Join(mediaDir, "video.mp4")
	thumbnailPath := filepath.Join(mediaDir, "thumbnail.png")
	if err := os.WriteFile(videoPath, []byte("real rendered mp4 bytes"), 0640); err != nil {
		t.Fatalf("write video: %v", err)
	}
	if err := os.WriteFile(thumbnailPath, []byte("real thumbnail png bytes"), 0640); err != nil {
		t.Fatalf("write thumbnail: %v", err)
	}

	reels := make([]DailyPackageReelContent, 0, 6)
	manifestReels := make([]DailyPackageManifestReel, 0, 6)
	for rank := 1; rank <= 6; rank++ {
		reels = append(reels, DailyPackageReelContent{
			Rank:           rank,
			Script:         "script",
			Caption:        "caption",
			Description:    "description",
			Hashtags:       "#one",
			ThumbnailBrief: "thumbnail brief",
			Metadata: DailyPackageReelMetadata{
				Rank:         rank,
				CandidateID:  "candidate",
				Source:       "google_trends_rss",
				RenderStatus: "not_attempted",
				HasVideo:     false,
			},
		})
		manifestReels = append(manifestReels, DailyPackageManifestReel{
			Rank:         rank,
			CandidateID:  "candidate",
			Title:        "title",
			Source:       "google_trends_rss",
			RenderStatus: "not_attempted",
			HasVideo:     false,
		})
	}
	manifest := DailyPackageManifest{
		Date:        "2026-07-07",
		GeneratedAt: "2026-07-07T00:00:00Z",
		Status:      "ready_with_render_failures",
		Message:     "ready without rendered video",
		ReelCount:   6,
		Reels:       manifestReels,
	}

	zipPath, _, err := BuildDailyReelsPackageZip(exportDir, reels, manifest)
	if err != nil {
		t.Fatalf("initial BuildDailyReelsPackageZip: %v", err)
	}
	reels, manifest, err = ReadDailyReelsPackageZip(zipPath)
	if err != nil {
		t.Fatalf("ReadDailyReelsPackageZip: %v", err)
	}

	reels[0].VideoSrcPath = videoPath
	reels[0].ThumbnailSrcPath = thumbnailPath
	reels[0].Metadata.RenderStatus = "rendered"
	reels[0].Metadata.RenderNotes = "rendered successfully"
	reels[0].Metadata.HasVideo = true
	reels[0].Metadata.HasThumbnail = true
	reels[0].Metadata.VideoFile = "video.mp4"
	reels[0].Metadata.ThumbnailFile = "thumbnail.png"
	manifest.Reels[0].RenderStatus = "rendered"
	manifest.Reels[0].RenderNotes = "rendered successfully"
	manifest.Reels[0].HasVideo = true
	manifest.Reels[0].VideoFile = "video.mp4"
	manifest.Reels[0].ThumbnailFile = "thumbnail.png"

	zipPath, included, err := BuildDailyReelsPackageZip(exportDir, reels, manifest)
	if err != nil {
		t.Fatalf("rebuilt BuildDailyReelsPackageZip: %v", err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open rebuilt zip: %v", err)
	}
	defer zr.Close()

	names := map[string]bool{}
	var rebuiltManifest DailyPackageManifest
	for _, f := range zr.File {
		names[f.Name] = true
		if f.Name == "manifest.json" {
			if err := readZipJSON(f, &rebuiltManifest); err != nil {
				t.Fatalf("read manifest: %v", err)
			}
		}
	}
	for _, name := range []string{
		"reel-01/video.mp4",
		"reel-01/thumbnail.png",
		"reel-01/metadata.json",
		"manifest.json",
	} {
		if !names[name] {
			t.Fatalf("expected %q in rebuilt ZIP; entries=%v", name, names)
		}
		if !containsString(included, name) {
			t.Fatalf("included files missing %q: %v", name, included)
		}
		if !containsString(rebuiltManifest.IncludedFiles, name) {
			t.Fatalf("manifest included files missing %q: %v", name, rebuiltManifest.IncludedFiles)
		}
	}
	for rank := 2; rank <= 6; rank++ {
		if names[fmt.Sprintf("reel-%02d/video.mp4", rank)] {
			t.Fatalf("reel-%02d must not include video.mp4 before rendering", rank)
		}
		if got := rebuiltManifest.Reels[rank-1].RenderStatus; got != "not_attempted" {
			t.Fatalf("reel-%02d render_status = %q, want not_attempted", rank, got)
		}
	}
}
