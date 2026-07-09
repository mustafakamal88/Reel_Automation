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
	CaptionText     string                           `json:"caption_text,omitempty"`
	LayoutMode      string                           `json:"layout_mode,omitempty"`
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

type ClipStudioGeneratedClip struct {
	ClipID           string                   `json:"clip_id"`
	Folder           string                   `json:"folder"`
	VideoSrcPath     string                   `json:"-"`
	ThumbnailSrcPath string                   `json:"-"`
	Metadata         ClipStudioExportMetadata `json:"metadata"`
}

type ClipStudioGeneratedManifest struct {
	SourceID           string                    `json:"source_id,omitempty"`
	Prompt             string                    `json:"prompt,omitempty"`
	ClipLength         string                    `json:"clip_length"`
	ClipCount          int                       `json:"clip_count"`
	HighlightDetection string                    `json:"highlight_detection"`
	Clips              []ClipStudioGeneratedClip `json:"clips"`
}

type LocalAISceneExportManifest struct {
	ClipID              string                        `json:"clip_id"`
	Prompt              string                        `json:"prompt"`
	StylePreset         string                        `json:"style_preset"`
	TargetLengthSeconds int                           `json:"target_length_seconds"`
	Branding            renderer.ClipBrandingSettings `json:"branding"`
	Metadata            renderer.LocalAISceneMetadata `json:"metadata"`
	VideoFile           string                        `json:"video_file,omitempty"`
	ThumbnailFile       string                        `json:"thumbnail_file,omitempty"`
	SceneMetadataFile   string                        `json:"scene_metadata_file,omitempty"`
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

func BuildClipStudioGeneratedExportZip(exportDir, zipID string, manifest ClipStudioGeneratedManifest) (zipPath string, includedFiles []string, err error) {
	if err = os.MkdirAll(exportDir, 0750); err != nil {
		return "", nil, fmt.Errorf("storage: mkdir clip studio export dir: %w", err)
	}
	zipPath = filepath.Join(exportDir, ClipStudioExportZipFilename(zipID))

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

	if manifest.HighlightDetection == "" {
		manifest.HighlightDetection = "not_run"
	}
	for i := range manifest.Clips {
		clip := &manifest.Clips[i]
		if clip.Folder == "" {
			clip.Folder = fmt.Sprintf("clip-%02d", i+1)
		}
		if clip.Metadata.AIHighlights.TranscriptionStatus == "" {
			clip.Metadata.AIHighlights = renderer.DefaultClipAIHighlightMetadata()
		}
		if clip.VideoSrcPath != "" {
			name := filepath.ToSlash(filepath.Join(clip.Folder, "video.mp4"))
			if err = addFileToZip(zw, clip.VideoSrcPath, name); err != nil {
				err = fmt.Errorf("add generated clip video: %w", err)
				return
			}
			includedFiles = append(includedFiles, name)
			clip.Metadata.VideoFile = name
		}
		if clip.ThumbnailSrcPath != "" {
			name := filepath.ToSlash(filepath.Join(clip.Folder, "thumbnail.png"))
			if err = addFileToZip(zw, clip.ThumbnailSrcPath, name); err != nil {
				err = fmt.Errorf("add generated clip thumbnail: %w", err)
				return
			}
			includedFiles = append(includedFiles, name)
			clip.Metadata.ThumbnailFile = name
		}
		metaName := filepath.ToSlash(filepath.Join(clip.Folder, "attribution.json"))
		if err = writeExportJSONToZip(zw, metaName, clip.Metadata); err != nil {
			return
		}
		includedFiles = append(includedFiles, metaName)
	}
	if err = writeExportJSONToZip(zw, "manifest.json", manifest); err != nil {
		return
	}
	includedFiles = append(includedFiles, "manifest.json")
	return
}

func BuildLocalAISceneExportZip(exportDir, zipID, videoSrcPath, thumbnailSrcPath, sceneMetadataPath string, manifest LocalAISceneExportManifest) (zipPath string, includedFiles []string, err error) {
	if err = os.MkdirAll(exportDir, 0750); err != nil {
		return "", nil, fmt.Errorf("storage: mkdir local ai scene export dir: %w", err)
	}
	zipPath = filepath.Join(exportDir, ClipStudioExportZipFilename(zipID))

	f, err := os.Create(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("storage: create local ai scene zip: %w", err)
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

	if videoSrcPath != "" {
		if err = addFileToZip(zw, videoSrcPath, "video.mp4"); err != nil {
			err = fmt.Errorf("add local ai final video: %w", err)
			return
		}
		includedFiles = append(includedFiles, "video.mp4")
		manifest.VideoFile = "video.mp4"
	}
	if thumbnailSrcPath != "" {
		if err = addFileToZip(zw, thumbnailSrcPath, "thumbnail.png"); err != nil {
			err = fmt.Errorf("add local ai thumbnail: %w", err)
			return
		}
		includedFiles = append(includedFiles, "thumbnail.png")
		manifest.ThumbnailFile = "thumbnail.png"
	}
	if sceneMetadataPath != "" {
		if err = addFileToZip(zw, sceneMetadataPath, "scene-metadata.json"); err != nil {
			err = fmt.Errorf("add local ai scene metadata: %w", err)
			return
		}
		includedFiles = append(includedFiles, "scene-metadata.json")
		manifest.SceneMetadataFile = "scene-metadata.json"
	}
	if err = writeExportJSONToZip(zw, "manifest.json", manifest); err != nil {
		return
	}
	includedFiles = append(includedFiles, "manifest.json")
	return
}
