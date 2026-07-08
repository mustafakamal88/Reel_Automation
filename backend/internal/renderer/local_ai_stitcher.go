package renderer

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func stitchLocalAIScenes(ctx context.Context, ffmpegPath string, scenePaths []string, overlayPath, subtitlePath, videoPath string) error {
	if len(scenePaths) == 0 {
		return fmt.Errorf("no generated scene clips to stitch")
	}
	listPath := filepath.Join(filepath.Dir(videoPath), "scene-concat.txt")
	var list strings.Builder
	for _, path := range scenePaths {
		if !fileExists(path) {
			return fmt.Errorf("scene clip missing: %s", path)
		}
		list.WriteString("file '")
		list.WriteString(strings.ReplaceAll(path, "'", "'\\''"))
		list.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(list.String()), 0640); err != nil {
		return err
	}
	filter := "[0:v]scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920,setsar=1[base];[base][1:v]overlay=x=0:y=0:shortest=1,format=yuv420p[vout]"
	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-loop", "1",
		"-i", overlayPath,
		"-i", subtitlePath,
		"-filter_complex", filter,
		"-map", "[vout]",
		"-map", "0:a?",
		"-map", "2:0",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-c:s", "mov_text",
		"-shortest",
		"-movflags", "+faststart",
		videoPath,
	}
	return runCommand(ctx, ffmpegPath, args...)
}

func renderLocalAISceneOverlayPNG(path string, branding ClipBrandingSettings) error {
	img := image.NewRGBA(image.Rect(0, 0, VideoWidth, VideoHeight))
	topColor := parseHexColor(firstNonEmpty(branding.TopBannerColor, "#101828"), color.RGBA{16, 24, 40, 230})
	topColor.A = 220
	bottomColor := parseHexColor(firstNonEmpty(branding.BottomBannerColor, "#0f766e"), color.RGBA{15, 118, 110, 230})
	bottomColor.A = 220
	fillRect(img, image.Rect(0, 0, VideoWidth, 178), topColor)
	fillRect(img, image.Rect(0, 1682, VideoWidth, VideoHeight), bottomColor)
	topText := firstNonEmpty(branding.TopBannerText, "TREND CORTEX")
	bottomText := firstNonEmpty(branding.BottomBannerText, branding.CTAText, "FOLLOW FOR MORE")
	drawMultilineBitmapText(img, 62, 54, wrapOverlayText(topText, 24, 1), 7, 14, color.RGBA{248, 250, 252, 255})
	drawMultilineBitmapText(img, 62, 1740, wrapOverlayText(bottomText, 24, 1), 7, 14, color.RGBA{248, 250, 252, 255})
	if branding.WatermarkText != "" {
		drawMultilineBitmapText(img, 720, 1606, wrapOverlayText(branding.WatermarkText, 22, 1), 4, 8, color.RGBA{248, 250, 252, 210})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func writeSceneSubtitles(path string, scenes []ScenePlan) error {
	var b strings.Builder
	start := 0.0
	for i, scene := range scenes {
		end := start + scene.DurationSeconds
		text := strings.TrimSpace(scene.NarrationText)
		if text == "" {
			text = scene.SceneID
		}
		b.WriteString(fmt.Sprintf("%d\n%s --> %s\n%s\n\n", i+1, srtTime(start), srtTime(end), sanitizeSubtitleText(text)))
		start = end
	}
	return os.WriteFile(path, []byte(b.String()), 0640)
}

func srtTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	msTotal := int(seconds*1000 + 0.5)
	h := msTotal / 3600000
	msTotal %= 3600000
	m := msTotal / 60000
	msTotal %= 60000
	s := msTotal / 1000
	ms := msTotal % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func sanitizeSubtitleText(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "\r", " "), "\n", " ")
}
