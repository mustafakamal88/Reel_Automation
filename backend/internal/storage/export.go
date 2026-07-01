package storage

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReelExportMetadata is written as reel-NN/metadata.json — the real,
// database-backed state of a single reel's artifacts at export time.
type ReelExportMetadata struct {
	ReelPlanID           string   `json:"reel_plan_id"`
	Rank                 int      `json:"rank"`
	Platform             string   `json:"platform"`
	Platforms            []string `json:"platforms,omitempty"`
	TitleIdea            string   `json:"title_idea"`
	Status               string   `json:"status"`
	RenderStatus         string   `json:"render_status,omitempty"`
	Provider             string   `json:"provider,omitempty"`
	FallbackReason       string   `json:"fallback_reason,omitempty"`
	HasVideo             bool     `json:"has_video"`
	HasThumbnail         bool     `json:"has_thumbnail"`
	VideoFormat          string   `json:"video_format,omitempty"`
	VideoWidth           int      `json:"video_width,omitempty"`
	VideoHeight          int      `json:"video_height,omitempty"`
	VideoDurationSeconds float64  `json:"video_duration_seconds,omitempty"`
	VideoCodec           string   `json:"video_codec,omitempty"`
	AudioCodec           string   `json:"audio_codec,omitempty"`
	ThumbnailFormat      string   `json:"thumbnail_format,omitempty"`
	ThumbnailWidth       int      `json:"thumbnail_width,omitempty"`
	ThumbnailHeight      int      `json:"thumbnail_height,omitempty"`
}

// ReelExportContent is everything needed to write one reel-NN/ folder into
// the batch ZIP. VideoSrcPath / ThumbnailSrcPath are absolute/relative
// filesystem paths to real artifact files; leave empty to skip packaging
// that file (e.g. video.mp4 is never written unless VideoSrcPath is set
// to a file that was confirmed to exist on disk).
type ReelExportContent struct {
	Rank             int
	Title            string
	Description      string
	Hashtags         string
	Script           string
	ThumbnailBrief   string
	Caption          string
	PlatformCaptions map[string]string
	VideoSrcPath     string
	ThumbnailSrcPath string
	Metadata         ReelExportMetadata
}

// BatchExportReelEntry is one row of the batch-summary.json reel list.
type BatchExportReelEntry struct {
	Rank         int    `json:"rank"`
	Title        string `json:"title"`
	Platform     string `json:"platform"`
	HasVideo     bool   `json:"has_video"`
	HasThumbnail bool   `json:"has_thumbnail"`
}

// BatchExportSummary is written as batch-summary.json at the zip root.
type BatchExportSummary struct {
	BatchID     string                 `json:"batch_id"`
	Date        string                 `json:"date"`
	GeneratedAt string                 `json:"generated_at"`
	ReelCount   int                    `json:"reel_count"`
	Reels       []BatchExportReelEntry `json:"reels"`
}

// ExportManifest is written as export-manifest.json and gives callers a
// machine-readable index of ZIP contents and render/export status.
type ExportManifest struct {
	BatchID        string   `json:"batch_id"`
	Date           string   `json:"date"`
	GeneratedAt    string   `json:"generated_at"`
	ExportStatus   string   `json:"export_status"`
	RenderStatus   string   `json:"render_status,omitempty"`
	Provider       string   `json:"provider,omitempty"`
	FallbackReason string   `json:"fallback_reason,omitempty"`
	IncludedFiles  []string `json:"included_files"`
}

// ExportZipFilename returns the canonical filename for a batch's ZIP
// export, e.g. "trendcortex-batch-2026-06-30.zip".
func ExportZipFilename(date string) string {
	return fmt.Sprintf("trendcortex-batch-%s.zip", date)
}

// BuildReelBatchZip writes a real ZIP file at exportDir/<ExportZipFilename>
// containing one reel-NN/ folder per entry in reels. Text artifacts
// (title.txt, description.txt, hashtags.txt, script.txt, caption.txt,
// platform caption files, thumbnail-brief.txt, metadata.json) are always
// written. video.mp4 / thumbnail.png are only included when the corresponding
// *SrcPath is non-empty — callers must only set it after confirming the file
// exists on disk. Returns the path to the created ZIP.
func BuildReelBatchZip(exportDir, date string, summary BatchExportSummary, reels []ReelExportContent) (zipPath string, err error) {
	zipPath, _, err = BuildReelBatchZipWithManifest(exportDir, date, summary, reels, nil)
	return zipPath, err
}

// BuildReelBatchZipWithManifest is BuildReelBatchZip plus a root
// export-manifest.json. The returned includedFiles list exactly matches the
// ZIP entry names created during this call.
func BuildReelBatchZipWithManifest(exportDir, date string, summary BatchExportSummary, reels []ReelExportContent, manifest *ExportManifest) (zipPath string, includedFiles []string, err error) {
	if err = os.MkdirAll(exportDir, 0750); err != nil {
		return "", nil, fmt.Errorf("storage: mkdir export dir: %w", err)
	}
	zipPath = filepath.Join(exportDir, ExportZipFilename(date))

	f, err := os.Create(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("storage: create zip: %w", err)
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

		if err = writeTextToZip(zw, prefix+"title.txt", reel.Title); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"title.txt")
		if err = writeTextToZip(zw, prefix+"description.txt", reel.Description); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"description.txt")
		if err = writeTextToZip(zw, prefix+"hashtags.txt", reel.Hashtags); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"hashtags.txt")
		if err = writeTextToZip(zw, prefix+"script.txt", reel.Script); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"script.txt")
		if err = writeTextToZip(zw, prefix+"thumbnail-brief.txt", reel.ThumbnailBrief); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"thumbnail-brief.txt")
		caption := reel.Caption
		if caption == "" {
			caption = reel.Description
		}
		if err = writeTextToZip(zw, prefix+"caption.txt", caption); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"caption.txt")
		for _, platform := range []string{"youtube", "tiktok", "instagram", "facebook", "x"} {
			text := reel.PlatformCaptions[platform]
			if text == "" {
				text = caption
			}
			name := prefix + platformCaptionFilename(platform)
			if err = writeTextToZip(zw, name, text); err != nil {
				return
			}
			includedFiles = append(includedFiles, name)
		}
		if err = writeExportJSONToZip(zw, prefix+"metadata.json", reel.Metadata); err != nil {
			return
		}
		includedFiles = append(includedFiles, prefix+"metadata.json")
		if reel.VideoSrcPath != "" {
			if err = addFileToZip(zw, reel.VideoSrcPath, prefix+"video.mp4"); err != nil {
				err = fmt.Errorf("add video for reel %02d: %w", reel.Rank, err)
				return
			}
			includedFiles = append(includedFiles, prefix+"video.mp4")
		}
		if reel.ThumbnailSrcPath != "" {
			if err = addFileToZip(zw, reel.ThumbnailSrcPath, prefix+"thumbnail.png"); err != nil {
				err = fmt.Errorf("add thumbnail for reel %02d: %w", reel.Rank, err)
				return
			}
			includedFiles = append(includedFiles, prefix+"thumbnail.png")
		}
	}

	if err = writeExportJSONToZip(zw, "batch-summary.json", summary); err != nil {
		return
	}
	includedFiles = append(includedFiles, "batch-summary.json")
	if manifest != nil {
		m := *manifest
		m.IncludedFiles = append(append([]string{}, includedFiles...), "export-manifest.json")
		if err = writeExportJSONToZip(zw, "export-manifest.json", m); err != nil {
			return
		}
		includedFiles = append(includedFiles, "export-manifest.json")
	}
	return
}

func platformCaptionFilename(platform string) string {
	switch platform {
	case "youtube":
		return "youtube-description.txt"
	case "tiktok":
		return "tiktok-caption.txt"
	case "instagram":
		return "instagram-caption.txt"
	case "facebook":
		return "facebook-caption.txt"
	case "x":
		return "x-caption.txt"
	default:
		return platform + "-caption.txt"
	}
}

func writeTextToZip(zw *zip.Writer, name, content string) error {
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(fw, content)
	return err
}

func writeExportJSONToZip(zw *zip.Writer, name string, v any) error {
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(fw)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func addFileToZip(zw *zip.Writer, srcPath, name string) error {
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
