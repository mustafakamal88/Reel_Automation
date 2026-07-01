package http

import (
	"os"
	"path/filepath"
	"testing"

	"trendcortex/api/internal/models"
)

func TestDecideExportStatus(t *testing.T) {
	tests := []struct {
		name             string
		missingVideo     []int
		missingThumbnail []int
		want             string
	}{
		{"nothing missing", nil, nil, ""},
		{"video only missing", []int{1, 3}, nil, models.ExportJobStatusVideoMissing},
		{"thumbnail only missing", nil, []int{2}, models.ExportJobStatusThumbnailMissing},
		{"both missing", []int{1}, []int{2}, models.ExportJobStatusMediaMissing},
		{"same reel missing both", []int{1}, []int{1}, models.ExportJobStatusMediaMissing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideExportStatus(tc.missingVideo, tc.missingThumbnail)
			if got != tc.want {
				t.Errorf("decideExportStatus(%v, %v) = %q, want %q", tc.missingVideo, tc.missingThumbnail, got, tc.want)
			}
		})
	}
}

func TestBuildMissingArtifactMessage(t *testing.T) {
	got := buildMissingArtifactMessage([]int{1, 3}, []int{2})
	want := "video.mp4 missing for reel(s): 1, 3; thumbnail.png missing for reel(s): 2"
	if got != want {
		t.Errorf("buildMissingArtifactMessage = %q, want %q", got, want)
	}

	got = buildMissingArtifactMessage(nil, []int{5})
	want = "thumbnail.png missing for reel(s): 5"
	if got != want {
		t.Errorf("buildMissingArtifactMessage = %q, want %q", got, want)
	}
}

func TestFileExists(t *testing.T) {
	if fileExists("") {
		t.Error("fileExists(\"\") must be false — an empty path is never a real artifact")
	}
	if fileExists("/path/does/not/exist/video.mp4") {
		t.Error("fileExists must be false for a nonexistent path")
	}

	tmp := t.TempDir()
	realFile := filepath.Join(tmp, "video.mp4")
	if err := os.WriteFile(realFile, []byte("data"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if !fileExists(realFile) {
		t.Error("fileExists must be true for a real file on disk")
	}

	dirPath := filepath.Join(tmp, "a-directory")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if fileExists(dirPath) {
		t.Error("fileExists must be false for a directory")
	}
}

func TestCheckReelArtifacts_HonestAboutMissingArtifacts(t *testing.T) {
	tmp := t.TempDir()
	realVideo := filepath.Join(tmp, "video.mp4")
	if err := os.WriteFile(realVideo, []byte("data"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	missingPath := filepath.Join(tmp, "does-not-exist.mp4")
	reels := []models.ReelPlan{
		{ID: "r1", Rank: 1, VideoArtifactPath: &realVideo, ThumbnailArtifactPath: nil},
		{ID: "r2", Rank: 2, VideoArtifactPath: &missingPath, ThumbnailArtifactPath: &missingPath},
	}

	checks := checkReelArtifacts(reels)
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}
	if !checks[0].hasVideo {
		t.Error("reel 1 has a real video file on disk — hasVideo should be true")
	}
	if checks[0].hasThumbnail {
		t.Error("reel 1 has no thumbnail_artifact_path — hasThumbnail must be false, never assumed true")
	}
	if checks[1].hasVideo || checks[1].hasThumbnail {
		t.Error("reel 2's artifact paths point to nonexistent files — both must report false")
	}
}

func TestReelExportStatusFor(t *testing.T) {
	tests := []struct {
		name                       string
		hasVideo, hasThumbnail     bool
		wantStatus                 string
		wantErrPresent             bool
	}{
		{"both present", true, true, models.ReelExportStatusReady, false},
		{"both missing", false, false, models.ReelExportStatusArtifactMissing, true},
		{"video missing only", false, true, models.ReelExportStatusVideoMissing, true},
		{"thumbnail missing only", true, false, models.ReelExportStatusThumbnailMissing, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := reelArtifactCheck{hasVideo: tc.hasVideo, hasThumbnail: tc.hasThumbnail}
			status, errMsg := reelExportStatusFor(c)
			if status != tc.wantStatus {
				t.Errorf("status = %q, want %q", status, tc.wantStatus)
			}
			if (errMsg != nil) != tc.wantErrPresent {
				t.Errorf("errMsg present = %v, want %v", errMsg != nil, tc.wantErrPresent)
			}
		})
	}
}
