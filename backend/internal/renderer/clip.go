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

	ClipLayoutFitWithBars       = "fit_with_bars"
	ClipLayoutFillCrop          = "fill_crop"
	ClipLayoutBlurredBackground = "blurred_background"

	ClipCTASizeSmall  = "small"
	ClipCTASizeMedium = "medium"
	ClipCTASizeLarge  = "large"
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
	CTASize           string `json:"cta_size,omitempty"`
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
	CaptionText     string                  `json:"caption_text,omitempty"`
	IncludeCaptions bool                    `json:"include_captions"`
	LayoutMode      string                  `json:"layout_mode,omitempty"`
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
	input.LayoutMode = normalizeClipLayoutMode(input.LayoutMode)
	input.Branding.CTASize = normalizeClipCTASize(input.Branding.CTASize)
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "clip-metadata.json"), payload, 0640)
}

func renderManualClipVideo(ctx context.Context, ffmpegPath string, input ClipInput, duration float64, overlayPath, videoPath string) error {
	_, contentY, contentH := clipVideoContentRect(input)
	filter := renderClipLayoutFilter(input.LayoutMode, duration, contentY, contentH)
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

func renderClipLayoutFilter(layoutMode string, duration float64, contentY, contentH int) string {
	layoutMode = normalizeClipLayoutMode(layoutMode)
	switch layoutMode {
	case ClipLayoutFitWithBars:
		return fmt.Sprintf("[0:v]scale=1080:%d:force_original_aspect_ratio=decrease,pad=1080:%d:(ow-iw)/2:(oh-ih)/2:color=0x05070b,setsar=1[clip];", contentH, contentH) +
			fmt.Sprintf("color=c=0x05070b:s=1080x1920:d=%s[base];", formatSeconds(duration)) +
			fmt.Sprintf("[base][clip]overlay=x=0:y=%d[tmp];[tmp][1:v]overlay=x=0:y=0,format=yuv420p[vout]", contentY)
	case ClipLayoutFillCrop:
		return fmt.Sprintf("[0:v]scale=1080:%d:force_original_aspect_ratio=increase,crop=1080:%d,setsar=1[clip];", contentH, contentH) +
			fmt.Sprintf("color=c=0x05070b:s=1080x1920:d=%s[base];", formatSeconds(duration)) +
			fmt.Sprintf("[base][clip]overlay=x=0:y=%d[tmp];[tmp][1:v]overlay=x=0:y=0,format=yuv420p[vout]", contentY)
	default:
		return "[0:v]split=2[srcbg][srcfg];[srcbg]scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920,boxblur=32:10,eq=brightness=-0.10:saturation=0.86,setsar=1[bg];" +
			fmt.Sprintf("[srcfg]scale=1016:%d:force_original_aspect_ratio=decrease,setsar=1[fg];", contentH) +
			fmt.Sprintf("[bg][fg]overlay=x=(W-w)/2:y=%d[tmp];[tmp][1:v]overlay=x=0:y=0,format=yuv420p[vout]", contentY)
	}
}

func renderClipThumbnail(ctx context.Context, ffmpegPath, videoPath, thumbnailPath string) error {
	return runCommand(ctx, ffmpegPath, "-y", "-i", videoPath, "-frames:v", "1", thumbnailPath)
}

func renderClipOverlayPNG(path string, input ClipInput) error {
	img := image.NewRGBA(image.Rect(0, 0, VideoWidth, VideoHeight))
	input.Branding.CTASize = normalizeClipCTASize(input.Branding.CTASize)
	topColor := parseHexColor(firstNonEmpty(input.Branding.TopBannerColor, "#111827"), color.RGBA{17, 24, 39, 255})
	bottomColor := parseHexColor(firstNonEmpty(input.Branding.BottomBannerColor, "#0f766e"), color.RGBA{15, 118, 110, 255})
	topY, topH, bottomY, bottomH := clipBannerRects(input.Branding.CTASize)
	fillRect(img, image.Rect(0, topY, VideoWidth, topY+topH), topColor)
	fillRect(img, image.Rect(0, bottomY, VideoWidth, bottomY+bottomH), bottomColor)
	fillRect(img, image.Rect(0, topY+topH, VideoWidth, topY+topH+10), color.RGBA{248, 250, 252, 24})
	fillRect(img, image.Rect(0, bottomY-10, VideoWidth, bottomY), color.RGBA{248, 250, 252, 24})

	topText := firstNonEmpty(input.Branding.TopBannerText, input.Rights.SourceTitle, "Clip Studio")
	bottomText := firstNonEmpty(input.Branding.BottomBannerText, input.Branding.CTAText, "Follow for more")
	drawMultilineBitmapText(img, 68, topY+34, wrapOverlayText(topText, 26, 1), 7, 12, color.RGBA{248, 250, 252, 255})
	bottomScale := 6
	bottomLines := 1
	if input.Branding.CTASize == ClipCTASizeLarge {
		bottomScale = 9
		bottomLines = 2
	} else if input.Branding.CTASize == ClipCTASizeMedium {
		bottomScale = 7
		bottomLines = 2
	}
	drawMultilineBitmapText(img, 68, bottomY+36, wrapOverlayText(bottomText, 30, bottomLines), bottomScale, 14, color.RGBA{248, 250, 252, 255})
	if input.Branding.CTASize == ClipCTASizeLarge && input.Branding.CTAText != "" && input.Branding.CTAText != bottomText {
		drawMultilineBitmapText(img, 72, bottomY+188, wrapOverlayText(input.Branding.CTAText, 36, 1), 5, 10, color.RGBA{204, 251, 241, 255})
	}
	captionText := firstNonEmpty(input.CaptionText)
	if input.IncludeCaptions && captionText != "" {
		fillRect(img, image.Rect(76, bottomY-178, 1004, bottomY-44), color.RGBA{0, 0, 0, 165})
		drawMultilineBitmapText(img, 104, bottomY-146, wrapOverlayText(captionText, 38, 2), 5, 12, color.RGBA{248, 250, 252, 255})
	}
	if input.Branding.WatermarkText != "" {
		drawMultilineBitmapText(img, 744, bottomY-76, wrapOverlayText(input.Branding.WatermarkText, 20, 1), 4, 8, color.RGBA{248, 250, 252, 185})
	}
	copyright := firstNonEmpty(input.Rights.CopyrightOverlayText, input.Rights.AttributionText)
	if copyright != "" {
		drawMultilineBitmapText(img, 68, bottomY-34, wrapOverlayText(copyright, 46, 1), 3, 6, color.RGBA{226, 232, 240, 175})
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func normalizeClipLayoutMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case ClipLayoutFitWithBars:
		return ClipLayoutFitWithBars
	case ClipLayoutFillCrop:
		return ClipLayoutFillCrop
	default:
		return ClipLayoutBlurredBackground
	}
}

func normalizeClipCTASize(size string) string {
	switch strings.TrimSpace(size) {
	case ClipCTASizeMedium:
		return ClipCTASizeMedium
	case ClipCTASizeLarge:
		return ClipCTASizeLarge
	default:
		return ClipCTASizeSmall
	}
}

func clipBannerRects(ctaSize string) (topY, topH, bottomY, bottomH int) {
	topY = 78
	topH = 132
	switch normalizeClipCTASize(ctaSize) {
	case ClipCTASizeLarge:
		bottomH = 310
	case ClipCTASizeMedium:
		bottomH = 228
	default:
		bottomH = 172
	}
	bottomY = 1780 - bottomH
	return topY, topH, bottomY, bottomH
}

func clipVideoContentRect(input ClipInput) (topY, contentY, contentH int) {
	_, topH, bottomY, _ := clipBannerRects(input.Branding.CTASize)
	contentY = topH + 98
	if contentY < 230 {
		contentY = 230
	}
	contentH = bottomY - contentY - 28
	if contentH < 1000 {
		contentH = 1000
	}
	return 78, contentY, contentH
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
