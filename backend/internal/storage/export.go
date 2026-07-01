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
	ReelPlanID           string  `json:"reel_plan_id"`
	Rank                 int     `json:"rank"`
	Platform             string  `json:"platform"`
	TitleIdea            string  `json:"title_idea"`
	Status               string  `json:"status"`
	HasVideo             bool    `json:"has_video"`
	HasThumbnail         bool    `json:"has_thumbnail"`
	VideoFormat          string  `json:"video_format,omitempty"`
	VideoWidth           int     `json:"video_width,omitempty"`
	VideoHeight          int     `json:"video_height,omitempty"`
	VideoDurationSeconds float64 `json:"video_duration_seconds,omitempty"`
	VideoCodec           string  `json:"video_codec,omitempty"`
	AudioCodec           string  `json:"audio_codec,omitempty"`
	ThumbnailFormat      string  `json:"thumbnail_format,omitempty"`
	ThumbnailWidth       int     `json:"thumbnail_width,omitempty"`
	ThumbnailHeight      int     `json:"thumbnail_height,omitempty"`
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

// ExportZipFilename returns the canonical filename for a batch's ZIP
// export, e.g. "trendcortex-batch-2026-06-30.zip".
func ExportZipFilename(date string) string {
	return fmt.Sprintf("trendcortex-batch-%s.zip", date)
}

// BuildReelBatchZip writes a real ZIP file at exportDir/<ExportZipFilename>
// containing one reel-NN/ folder per entry in reels. Text artifacts
// (title.txt, description.txt, hashtags.txt, script.txt,
// thumbnail-brief.txt, metadata.json) are always written. video.mp4 /
// thumbnail.png are only included when the corresponding *SrcPath is
// non-empty — callers must only set it after confirming the file exists on
// disk. Returns the path to the created ZIP.
func BuildReelBatchZip(exportDir, date string, summary BatchExportSummary, reels []ReelExportContent) (zipPath string, err error) {
	if err = os.MkdirAll(exportDir, 0750); err != nil {
		return "", fmt.Errorf("storage: mkdir export dir: %w", err)
	}
	zipPath = filepath.Join(exportDir, ExportZipFilename(date))

	f, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("storage: create zip: %w", err)
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
		}
	}()

	for _, reel := range reels {
		prefix := fmt.Sprintf("reel-%02d/", reel.Rank)

		if err = writeTextToZip(zw, prefix+"title.txt", reel.Title); err != nil {
			return
		}
		if err = writeTextToZip(zw, prefix+"description.txt", reel.Description); err != nil {
			return
		}
		if err = writeTextToZip(zw, prefix+"hashtags.txt", reel.Hashtags); err != nil {
			return
		}
		if err = writeTextToZip(zw, prefix+"script.txt", reel.Script); err != nil {
			return
		}
		if err = writeTextToZip(zw, prefix+"thumbnail-brief.txt", reel.ThumbnailBrief); err != nil {
			return
		}
		if err = writeExportJSONToZip(zw, prefix+"metadata.json", reel.Metadata); err != nil {
			return
		}
		if reel.VideoSrcPath != "" {
			if err = addFileToZip(zw, reel.VideoSrcPath, prefix+"video.mp4"); err != nil {
				err = fmt.Errorf("add video for reel %02d: %w", reel.Rank, err)
				return
			}
		}
		if reel.ThumbnailSrcPath != "" {
			if err = addFileToZip(zw, reel.ThumbnailSrcPath, prefix+"thumbnail.png"); err != nil {
				err = fmt.Errorf("add thumbnail for reel %02d: %w", reel.Rank, err)
				return
			}
		}
	}

	err = writeExportJSONToZip(zw, "batch-summary.json", summary)
	return
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
