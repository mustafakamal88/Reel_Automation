package storage

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"trendcortex/api/internal/renderer"
)

type ClipStudioExportMetadata struct {
	ClipID          string                           `json:"clip_id"`
	SourceModel     string                           `json:"source_model"`
	Rights          renderer.ClipRightsMetadata      `json:"rights"`
	Branding        renderer.ClipBrandingSettings    `json:"branding"`
	ManualRange     renderer.ClipManualRange         `json:"manual_range"`
	AIHighlights    renderer.ClipAIHighlightMetadata `json:"ai_highlights"`
	RendererVersion string                           `json:"renderer_version"`
	VideoFile       string                           `json:"video_file,omitempty"`
	ThumbnailFile   string                           `json:"thumbnail_file,omitempty"`
}

type ClipStudioExportContent struct {
	ClipID           string
	VideoSrcPath     string
	ThumbnailSrcPath string
	Metadata         ClipStudioExportMetadata
}

func ClipStudioExportZipFilename(clipID string) string {
	if clipID == "" {
		clipID = "clip-studio"
	}
	return fmt.Sprintf("trendcortex-%s.zip", clipID)
}

func BuildClipStudioExportZip(exportDir string, content ClipStudioExportContent) (zipPath string, includedFiles []string, err error) {
	if err = os.MkdirAll(exportDir, 0750); err != nil {
		return "", nil, fmt.Errorf("storage: mkdir clip studio export dir: %w", err)
	}
	zipPath = filepath.Join(exportDir, ClipStudioExportZipFilename(content.ClipID))

	f, err := os.Create(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("storage: create clip studio zip: %w", err)
	}
	zw := zip.NewWriter(f)
	defer func() {
		if closeErr := zw.Close(); err == nil {
			err = closeErr
		}
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(zipPath)
			zipPath = ""
			includedFiles = nil
		}
	}()

	meta := content.Metadata
	if meta.AIHighlights.TranscriptionStatus == "" {
		meta.AIHighlights = renderer.DefaultClipAIHighlightMetadata()
	}
	if content.VideoSrcPath != "" {
		if err = addFileToZip(zw, content.VideoSrcPath, "video.mp4"); err != nil {
			err = fmt.Errorf("add clip video: %w", err)
			return
		}
		includedFiles = append(includedFiles, "video.mp4")
		meta.VideoFile = "video.mp4"
	}
	if content.ThumbnailSrcPath != "" {
		if err = addFileToZip(zw, content.ThumbnailSrcPath, "thumbnail.png"); err != nil {
			err = fmt.Errorf("add clip thumbnail: %w", err)
			return
		}
		includedFiles = append(includedFiles, "thumbnail.png")
		meta.ThumbnailFile = "thumbnail.png"
	}
	if err = writeExportJSONToZip(zw, "attribution.json", meta); err != nil {
		return
	}
	includedFiles = append(includedFiles, "attribution.json")
	if meta.Rights.AttributionText != "" {
		if err = writeTextToZip(zw, "attribution.txt", meta.Rights.AttributionText); err != nil {
			return
		}
		includedFiles = append(includedFiles, "attribution.txt")
	}
	return
}
