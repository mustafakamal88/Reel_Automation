package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const DailyReelsPackageFilename = "daily-reels-package.zip"

type DailyPackagePlatformPosts struct {
	Instagram string `json:"instagram"`
	TikTok    string `json:"tiktok"`
	YouTube   struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"youtube"`
	Facebook string `json:"facebook"`
	X        string `json:"x"`
}

type DailyPackageTrendEvidence struct {
	Candidate any `json:"candidate"`
}

type DailyPackageReelMetadata struct {
	Rank            int      `json:"rank"`
	CandidateID     string   `json:"candidate_id"`
	Source          string   `json:"source"`
	SourceURL       string   `json:"source_url,omitempty"`
	RenderStatus    string   `json:"render_status"`
	RenderNotes     string   `json:"render_notes,omitempty"`
	HasVideo        bool     `json:"has_video"`
	HasThumbnail    bool     `json:"has_thumbnail"`
	PlatformTargets []string `json:"platform_targets"`
	GeneratedAt     string   `json:"generated_at"`
	Provider        string   `json:"provider,omitempty"`
	ProviderModel   string   `json:"provider_model,omitempty"`
	VideoFormat     string   `json:"video_format,omitempty"`
	VideoWidth      int      `json:"video_width,omitempty"`
	VideoHeight     int      `json:"video_height,omitempty"`
	ThumbnailFormat string   `json:"thumbnail_format,omitempty"`
	ThumbnailWidth  int      `json:"thumbnail_width,omitempty"`
	ThumbnailHeight int      `json:"thumbnail_height,omitempty"`
}

type DailyPackageReelContent struct {
	Rank             int
	Script           string
	Caption          string
	Description      string
	Hashtags         string
	ThumbnailBrief   string
	PlatformPosts    DailyPackagePlatformPosts
	TrendEvidence    DailyPackageTrendEvidence
	Metadata         DailyPackageReelMetadata
	VideoSrcPath     string
	ThumbnailSrcPath string
}

type DailyPackageManifestReel struct {
	Rank         int    `json:"rank"`
	CandidateID  string `json:"candidate_id"`
	Title        string `json:"title"`
	Source       string `json:"source"`
	RenderStatus string `json:"render_status"`
	RenderNotes  string `json:"render_notes,omitempty"`
	HasVideo     bool   `json:"has_video"`
}

type DailyPackageManifest struct {
	Date          string                     `json:"date"`
	GeneratedAt   string                     `json:"generated_at"`
	Status        string                     `json:"status"`
	Message       string                     `json:"message"`
	ReelCount     int                        `json:"reel_count"`
	IncludedFiles []string                   `json:"included_files"`
	Reels         []DailyPackageManifestReel `json:"reels"`
}

func BuildDailyReelsPackageZip(exportDir string, reels []DailyPackageReelContent, manifest DailyPackageManifest) (zipPath string, includedFiles []string, err error) {
	if err = os.MkdirAll(exportDir, 0750); err != nil {
		return "", nil, fmt.Errorf("storage: mkdir daily package dir: %w", err)
	}
	zipPath = filepath.Join(exportDir, DailyReelsPackageFilename)

	f, err := os.Create(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("storage: create daily package zip: %w", err)
	}
	zw := zip.NewWriter(f)
	defer func() {
		if closeErr := zw.Close(); err == nil {
			err = closeErr
		}
		f.Close()
		if err != nil {
			os.Remove(zipPath)
			zipPath = ""
			includedFiles = nil
		}
	}()

	for _, reel := range reels {
		prefix := fmt.Sprintf("reel-%02d/", reel.Rank)
		textFiles := map[string]string{
			"script.txt":          reel.Script,
			"caption.txt":         reel.Caption,
			"description.txt":     reel.Description,
			"hashtags.txt":        reel.Hashtags,
			"thumbnail_brief.txt": reel.ThumbnailBrief,
		}
		for _, name := range []string{"script.txt", "caption.txt", "description.txt", "hashtags.txt", "thumbnail_brief.txt"} {
			entry := prefix + name
			if err = writeTextToZip(zw, entry, textFiles[name]); err != nil {
				return
			}
			includedFiles = append(includedFiles, entry)
		}
		if err = writeExportJSONToZip(zw, prefix+"platform_posts.json", reel.PlatformPosts); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"platform_posts.json")
		if err = writeExportJSONToZip(zw, prefix+"trend_evidence.json", reel.TrendEvidence); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"trend_evidence.json")
		if err = writeExportJSONToZip(zw, prefix+"metadata.json", reel.Metadata); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"metadata.json")
		if reel.VideoSrcPath != "" {
			if err = addDailyPackageFileToZip(zw, reel.VideoSrcPath, prefix+"video.mp4"); err != nil {
				err = fmt.Errorf("add video for reel %02d: %w", reel.Rank, err)
				return
			}
			includedFiles = append(includedFiles, prefix+"video.mp4")
		}
	}

	manifest.IncludedFiles = append(append([]string{}, includedFiles...), "manifest.json")
	if err = writeExportJSONToZip(zw, "manifest.json", manifest); err != nil {
		return
	}
	includedFiles = append(includedFiles, "manifest.json")
	return
}

func addDailyPackageFileToZip(zw *zip.Writer, srcPath, name string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, src)
	return err
}
