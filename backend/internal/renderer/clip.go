package renderer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ClipSourceUserUpload                           = "user_upload"
	ClipSourceOwnChannelSource                     = "own_channel_source"
	ClipSourceCreativeCommons                      = "creative_commons"
	ClipSourcePublicDomain                         = "public_domain"
	ClipSourceLicensedSource                       = "licensed_source"
	ClipSourceExternalURLPendingRightsConfirmation = "external_url_pending_rights_confirmation"

	ClipRendererVersion = "clip-studio-v1"
)

type ClipRightsMetadata struct {
	SourceURL            string `json:"source_url,omitempty"`
	SourceTitle          string `json:"source_title,omitempty"`
	SourceCreator        string `json:"source_creator,omitempty"`
	SourceLicense        string `json:"source_license,omitempty"`
	AttributionText      string `json:"attribution_text,omitempty"`
	UserConfirmedRights  bool   `json:"user_confirmed_rights"`
	CopyrightOverlayText string `json:"copyright_overlay_text,omitempty"`
	PlatformSource       string `json:"platform_source,omitempty"`
}

type ClipBrandingSettings struct {
	TopBannerText     string `json:"top_banner_text,omitempty"`
	BottomBannerText  string `json:"bottom_banner_text,omitempty"`
	LogoPath          string `json:"logo_path,omitempty"`
	WatermarkText     string `json:"watermark_text,omitempty"`
	CTAText           string `json:"cta_text,omitempty"`
	FontStylePreset   string `json:"font_style_preset,omitempty"`
	TopBannerColor    string `json:"top_banner_color,omitempty"`
	BottomBannerColor string `json:"bottom_banner_color,omitempty"`
}

type ClipManualRange struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

type ClipAIHighlightMetadata struct {
	TranscriptionStatus  string `json:"transcription_status"`
	SuggestedClipsStatus string `json:"suggested_clips_status"`
	HookScoreStatus      string `json:"hook_score_status"`
}

type ClipInput struct {
	WorkspaceID     string                  `json:"workspace_id,omitempty"`
	ClipID          string                  `json:"clip_id,omitempty"`
	SourceModel     string                  `json:"source_model"`
	SourceVideoPath string                  `json:"source_video_path,omitempty"`
	Rights          ClipRightsMetadata      `json:"rights"`
	Branding        ClipBrandingSettings    `json:"branding"`
	ManualRange     ClipManualRange         `json:"manual_range"`
	Captions        string                  `json:"captions,omitempty"`
	IncludeCaptions bool                    `json:"include_captions"`
	AIHighlights    ClipAIHighlightMetadata `json:"ai_highlights"`
}

func DefaultClipAIHighlightMetadata() ClipAIHighlightMetadata {
	return ClipAIHighlightMetadata{
		TranscriptionStatus:  "not_run",
		SuggestedClipsStatus: "not_run",
		HookScoreStatus:      "not_run",
	}
}

func RenderManualClip(ctx context.Context, cfg Config, input ClipInput) Result {
	if err := validateClipInput(input); err != nil {
		return Result{Status: StatusFailed, Notes: err.Error()}
	}
	if status, notes := localRendererPrerequisiteStatus(cfg); status != "" {
		return Result{Status: status, Notes: notes}
	}

	dir, err := SafeOutputDir(cfg.OutputDir, firstNonEmpty(input.WorkspaceID, "workspace"), firstNonEmpty(input.ClipID, "clip-studio"))
	if err != nil {
		return Result{Status: StatusFailed, Notes: err.Error()}
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("create clip output dir: %v", err)}
	}
	if err := writeClipMetadata(dir, input); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("store clip metadata: %v", err)}
	}

	videoPath := filepath.Join(dir, "video.mp4")
	duration := input.ManualRange.EndSeconds - input.ManualRange.StartSeconds
	overlayPath := filepath.Join(dir, "clip-overlay.png")
	if err := renderClipOverlayPNG(overlayPath, input); err != nil {
		return Result{Status: StatusThumbnailMissing, Notes: fmt.Sprintf("clip overlay render failed: %v", err)}
	}
	if err := renderManualClipVideo(ctx, cfg.FFmpegPath, input, duration, overlayPath, videoPath); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("ffmpeg clip render failed: %v", err)}
	}
	if !fileExists(videoPath) {
		return Result{Status: StatusFailed, Notes: "clip renderer did not produce video.mp4"}
	}

	thumbnailPath := filepath.Join(dir, "thumbnail.png")
	if err := renderClipThumbnail(ctx, cfg.FFmpegPath, videoPath, thumbnailPath); err != nil {
		return Result{Status: StatusThumbnailMissing, Notes: fmt.Sprintf("thumbnail render failed: %v", err)}
	}
	if !fileExists(thumbnailPath) {
		return Result{Status: StatusThumbnailMissing, Notes: "clip renderer did not produce thumbnail.png"}
	}

	tw, th, err := pngDimensions(thumbnailPath)
	if err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("read thumbnail dimensions: %v", err)}
	}
	probed := probeDuration(ctx, cfg.FFprobePath, videoPath)
	return Result{
		Status:               StatusCompleted,
		Notes:                "Rendered Clip Studio MP4 from confirmed source media with banners, overlays, and attribution metadata.",
		VideoPath:            videoPath,
		VideoFormat:          "mp4",
		VideoWidth:           VideoWidth,
		VideoHeight:          VideoHeight,
		VideoDurationSeconds: probed,
		VideoCodec:           "h264",
		AudioCodec:           "aac",
		ThumbnailPath:        thumbnailPath,
		ThumbnailFormat:      "png",
		ThumbnailWidth:       tw,
		ThumbnailHeight:      th,
		RendererVersion:      ClipRendererVersion,
	}
}

func validateClipInput(input ClipInput) error {
	switch strings.TrimSpace(input.SourceModel) {
	case ClipSourceUserUpload, ClipSourceOwnChannelSource, ClipSourceCreativeCommons, ClipSourcePublicDomain, ClipSourceLicensedSource:
	case ClipSourceExternalURLPendingRightsConfirmation:
		if !input.Rights.UserConfirmedRights {
			return errors.New("external URL source requires user rights confirmation before rendering")
		}
	default:
		return fmt.Errorf("unsupported clip source model %q", input.SourceModel)
	}
	if strings.TrimSpace(input.SourceVideoPath) == "" {
		return errors.New("source_video_path is required; Clip Studio does not auto-rip external URLs")
	}
	if !fileExists(input.SourceVideoPath) {
		return fmt.Errorf("source video does not exist: %s", input.SourceVideoPath)
	}
	if input.ManualRange.StartSeconds < 0 {
		return errors.New("start time must be zero or greater")
	}
	if input.ManualRange.EndSeconds <= input.ManualRange.StartSeconds {
		return errors.New("end time must be after start time")
	}
	if input.ManualRange.EndSeconds-input.ManualRange.StartSeconds > 180 {
		return errors.New("manual clip range is too long for short-form export")
	}
	if !input.Rights.UserConfirmedRights && input.SourceModel != ClipSourcePublicDomain {
		return errors.New("user_confirmed_rights is required before rendering this source")
	}
	return nil
}

func writeClipMetadata(dir string, input ClipInput) error {
	if input.AIHighlights.TranscriptionStatus == "" {
		input.AIHighlights = DefaultClipAIHighlightMetadata()
	}
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "clip-metadata.json"), payload, 0640)
}

func renderManualClipVideo(ctx context.Context, ffmpegPath string, input ClipInput, duration float64, overlayPath, videoPath string) error {
	filter := "[0:v]scale=1080:1080:force_original_aspect_ratio=decrease,pad=1080:1080:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1[clip];" +
		fmt.Sprintf("color=c=0x05070b:s=1080x1920:d=%s[base];", formatSeconds(duration)) +
		"[base][clip]overlay=x=0:y=390[tmp];[tmp][1:v]overlay=x=0:y=0,format=yuv420p[vout]"
	args := []string{
		"-y",
		"-ss", formatSeconds(input.ManualRange.StartSeconds),
		"-t", formatSeconds(duration),
		"-i", input.SourceVideoPath,
		"-loop", "1",
		"-i", overlayPath,
		"-filter_complex", filter,
		"-map", "[vout]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-shortest",
		videoPath,
	}
	return runCommand(ctx, ffmpegPath, args...)
}

func renderClipThumbnail(ctx context.Context, ffmpegPath, videoPath, thumbnailPath string) error {
	return runCommand(ctx, ffmpegPath, "-y", "-i", videoPath, "-frames:v", "1", thumbnailPath)
}

func renderClipOverlayPNG(path string, input ClipInput) error {
	img := image.NewRGBA(image.Rect(0, 0, VideoWidth, VideoHeight))
	topColor := parseHexColor(firstNonEmpty(input.Branding.TopBannerColor, "#111827"), color.RGBA{17, 24, 39, 255})
	bottomColor := parseHexColor(firstNonEmpty(input.Branding.BottomBannerColor, "#0f766e"), color.RGBA{15, 118, 110, 255})
	fillRect(img, image.Rect(0, 0, VideoWidth, 300), topColor)
	fillRect(img, image.Rect(0, 1500, VideoWidth, VideoHeight), bottomColor)
	fillRect(img, image.Rect(0, 300, VideoWidth, 390), color.RGBA{5, 7, 11, 255})
	fillRect(img, image.Rect(0, 1470, VideoWidth, 1500), color.RGBA{5, 7, 11, 255})

	topText := firstNonEmpty(input.Branding.TopBannerText, input.Rights.SourceTitle, "Clip Studio")
	bottomText := firstNonEmpty(input.Branding.BottomBannerText, input.Branding.CTAText, "Follow for more")
	drawMultilineBitmapText(img, 72, 84, wrapOverlayText(topText, 18, 2), 10, 18, color.RGBA{248, 250, 252, 255})
	drawMultilineBitmapText(img, 72, 1542, wrapOverlayText(bottomText, 20, 2), 9, 16, color.RGBA{248, 250, 252, 255})
	if input.Branding.CTAText != "" && input.Branding.CTAText != bottomText {
		drawMultilineBitmapText(img, 72, 1658, wrapOverlayText(input.Branding.CTAText, 30, 1), 5, 10, color.RGBA{204, 251, 241, 255})
	}
	if input.IncludeCaptions && strings.TrimSpace(input.Captions) != "" {
		fillRect(img, image.Rect(70, 1292, 1010, 1436), color.RGBA{0, 0, 0, 185})
		drawMultilineBitmapText(img, 96, 1322, wrapOverlayText(input.Captions, 34, 2), 5, 12, color.RGBA{248, 250, 252, 255})
	}
	if input.Branding.WatermarkText != "" {
		drawMultilineBitmapText(img, 760, 1424, wrapOverlayText(input.Branding.WatermarkText, 18, 1), 4, 8, color.RGBA{226, 232, 240, 210})
	}
	copyright := firstNonEmpty(input.Rights.CopyrightOverlayText, input.Rights.AttributionText)
	if copyright != "" {
		drawMultilineBitmapText(img, 72, 1450, wrapOverlayText(copyright, 42, 1), 3, 6, color.RGBA{226, 232, 240, 205})
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func parseHexColor(s string, fallback color.RGBA) color.RGBA {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return fallback
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return fallback
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

func formatSeconds(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}
