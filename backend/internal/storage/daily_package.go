package storage

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	RenderError     string   `json:"render_error,omitempty"`
	HasVideo        bool     `json:"has_video"`
	HasThumbnail    bool     `json:"has_thumbnail"`
	VideoFile       string   `json:"video_file,omitempty"`
	ThumbnailFile   string   `json:"thumbnail_file,omitempty"`
	PlatformTargets []string `json:"platform_targets"`
	GeneratedAt     string   `json:"generated_at"`
	Provider        string   `json:"provider,omitempty"`
	ProviderModel   string   `json:"provider_model,omitempty"`
	VideoFormat     string   `json:"video_format,omitempty"`
	VideoWidth      int      `json:"video_width,omitempty"`
	VideoHeight     int      `json:"video_height,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	RendererVersion string   `json:"renderer_version,omitempty"`
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
	Rank            int      `json:"rank"`
	CandidateID     string   `json:"candidate_id"`
	Title           string   `json:"title"`
	Source          string   `json:"source"`
	RenderStatus    string   `json:"render_status"`
	RenderNotes     string   `json:"render_notes,omitempty"`
	RenderError     string   `json:"render_error,omitempty"`
	HasVideo        bool     `json:"has_video"`
	VideoFile       string   `json:"video_file,omitempty"`
	ThumbnailFile   string   `json:"thumbnail_file,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	RendererVersion string   `json:"renderer_version,omitempty"`
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

	f, err := os.CreateTemp(exportDir, "daily-reels-package-*.zip.tmp")
	if err != nil {
		return "", nil, fmt.Errorf("storage: create daily package zip temp file: %w", err)
	}
	tmpPath := f.Name()
	zw := zip.NewWriter(f)
	defer func() {
		if closeErr := zw.Close(); err == nil {
			err = closeErr
		}
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(tmpPath)
			zipPath = ""
			includedFiles = nil
			return
		}
		if renameErr := os.Rename(tmpPath, zipPath); renameErr != nil {
			os.Remove(tmpPath)
			err = fmt.Errorf("storage: replace daily package zip: %w", renameErr)
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
		if reel.ThumbnailSrcPath != "" {
			if err = addDailyPackageFileToZip(zw, reel.ThumbnailSrcPath, prefix+"thumbnail.png"); err != nil {
				err = fmt.Errorf("add thumbnail for reel %02d: %w", reel.Rank, err)
				return
			}
			includedFiles = append(includedFiles, prefix+"thumbnail.png")
		}
	}

	manifest.IncludedFiles = append(append([]string{}, includedFiles...), "manifest.json")
	if err = writeExportJSONToZip(zw, "manifest.json", manifest); err != nil {
		return
	}
	includedFiles = append(includedFiles, "manifest.json")
	return
}

func ReadDailyReelsPackageZip(zipPath string) ([]DailyPackageReelContent, DailyPackageManifest, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, DailyPackageManifest{}, err
	}
	defer zr.Close()

	reelsByRank := map[int]*DailyPackageReelContent{}
	var manifest DailyPackageManifest
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			if err := readZipJSON(f, &manifest); err != nil {
				return nil, DailyPackageManifest{}, fmt.Errorf("read manifest.json: %w", err)
			}
			continue
		}
		if !strings.HasPrefix(f.Name, "reel-") {
			continue
		}
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) != 2 {
			continue
		}
		rank, err := strconv.Atoi(strings.TrimPrefix(parts[0], "reel-"))
		if err != nil || rank <= 0 {
			continue
		}
		reel := reelsByRank[rank]
		if reel == nil {
			reel = &DailyPackageReelContent{Rank: rank}
			reelsByRank[rank] = reel
		}
		switch parts[1] {
		case "script.txt":
			reel.Script, err = readZipText(f)
		case "caption.txt":
			reel.Caption, err = readZipText(f)
		case "description.txt":
			reel.Description, err = readZipText(f)
		case "hashtags.txt":
			reel.Hashtags, err = readZipText(f)
		case "thumbnail_brief.txt":
			reel.ThumbnailBrief, err = readZipText(f)
		case "platform_posts.json":
			err = readZipJSON(f, &reel.PlatformPosts)
		case "trend_evidence.json":
			err = readZipJSON(f, &reel.TrendEvidence)
		case "metadata.json":
			err = readZipJSON(f, &reel.Metadata)
		}
		if err != nil {
			return nil, DailyPackageManifest{}, fmt.Errorf("read %s: %w", f.Name, err)
		}
	}

	reels := make([]DailyPackageReelContent, 0, len(reelsByRank))
	for rank := 1; rank <= len(reelsByRank); rank++ {
		if reel := reelsByRank[rank]; reel != nil {
			if reel.Metadata.Rank == 0 {
				reel.Metadata.Rank = rank
			}
			reels = append(reels, *reel)
		}
	}
	return reels, manifest, nil
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

func readZipText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return string(b), err
}

func readZipJSON(f *zip.File, dst any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return json.NewDecoder(rc).Decode(dst)
}
