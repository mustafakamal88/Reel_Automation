package storage

import (
	"archive/zip"
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
