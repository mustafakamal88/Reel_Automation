package renderer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	StatusProviderNotConnected = "provider_not_connected"
	StatusRendererNotAvailable = "renderer_not_available"
	StatusAudioArtifactMissing = "audio_artifact_missing"
	StatusThumbnailMissing     = "thumbnail_artifact_missing"
	StatusFailed               = "failed"
	StatusCompleted            = "completed"

	VideoWidth  = 1080
	VideoHeight = 1920
)

type Config struct {
	Provider     string
	OutputDir    string
	OpenAIAPIKey string
	TTSModel     string
	ImageModel   string
	FFmpegPath   string
	FFprobePath  string
	HTTPClient   *http.Client
}

type ReelInput struct {
	WorkspaceID    string
	ReelPlanID     string
	Rank           int
	Title          string
	Script         string
	Description    string
	Hashtags       string
	ThumbnailBrief string
}

type Result struct {
	Status               string
	Notes                string
	VideoPath            string
	VideoFormat          string
	VideoWidth           int
	VideoHeight          int
	VideoDurationSeconds *float64
	VideoCodec           string
	AudioCodec           string
	ThumbnailPath        string
	ThumbnailFormat      string
	ThumbnailWidth       int
	ThumbnailHeight      int
}

type openAIClient struct {
	apiKey     string
	httpClient *http.Client
}

func RenderReel(ctx context.Context, cfg Config, input ReelInput) Result {
	if status, notes := prerequisiteStatus(cfg); status != "" {
		return Result{Status: status, Notes: notes}
	}

	dir, err := SafeOutputDir(cfg.OutputDir, input.WorkspaceID, input.ReelPlanID)
	if err != nil {
		return Result{Status: StatusFailed, Notes: err.Error()}
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("create media output dir: %v", err)}
	}

	client := openAIClient{apiKey: cfg.OpenAIAPIKey, httpClient: cfg.HTTPClient}
	script := strings.TrimSpace(input.Script)
	if script == "" {
		script = strings.TrimSpace(input.Title)
	}
	if script == "" {
		return Result{Status: StatusAudioArtifactMissing, Notes: "reel script is empty; cannot generate voiceover"}
	}

	audioPath := filepath.Join(dir, "voiceover.mp3")
	if err := client.generateSpeech(ctx, cfg.TTSModel, script, audioPath); err != nil {
		return Result{Status: StatusAudioArtifactMissing, Notes: fmt.Sprintf("voiceover generation failed: %v", err)}
	}
	if !fileExists(audioPath) {
		return Result{Status: StatusAudioArtifactMissing, Notes: "voiceover provider did not produce an audio artifact"}
	}

	imagePrompt := buildThumbnailPrompt(input)
	sourceThumbnailPath := filepath.Join(dir, "thumbnail-source.png")
	if err := client.generateImage(ctx, cfg.ImageModel, imagePrompt, sourceThumbnailPath); err != nil {
		return Result{Status: StatusThumbnailMissing, Notes: fmt.Sprintf("thumbnail generation failed: %v", err)}
	}
	if !fileExists(sourceThumbnailPath) {
		return Result{Status: StatusThumbnailMissing, Notes: "image provider did not produce a thumbnail artifact"}
	}

	thumbnailPath := filepath.Join(dir, "thumbnail.png")
	if err := normalizeThumbnail(ctx, cfg.FFmpegPath, sourceThumbnailPath, thumbnailPath); err != nil {
		return Result{Status: StatusThumbnailMissing, Notes: fmt.Sprintf("thumbnail normalization failed: %v", err)}
	}
	if !fileExists(thumbnailPath) {
		return Result{Status: StatusThumbnailMissing, Notes: "renderer did not produce thumbnail.png"}
	}

	videoPath := filepath.Join(dir, "video.mp4")
	if err := renderVideo(ctx, cfg.FFmpegPath, thumbnailPath, audioPath, videoPath); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("ffmpeg render failed: %v", err)}
	}
	if !fileExists(videoPath) {
		return Result{Status: StatusFailed, Notes: "renderer did not produce video.mp4"}
	}

	tw, th, err := pngDimensions(thumbnailPath)
	if err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("read thumbnail dimensions: %v", err)}
	}
	duration := probeDuration(ctx, cfg.FFprobePath, videoPath)

	return Result{
		Status:               StatusCompleted,
		Notes:                "Rendered real media artifacts with OpenAI TTS/image generation and FFmpeg.",
		VideoPath:            videoPath,
		VideoFormat:          "mp4",
		VideoWidth:           VideoWidth,
		VideoHeight:          VideoHeight,
		VideoDurationSeconds: duration,
		VideoCodec:           "h264",
		AudioCodec:           "aac",
		ThumbnailPath:        thumbnailPath,
		ThumbnailFormat:      "png",
		ThumbnailWidth:       tw,
		ThumbnailHeight:      th,
	}
}

// RenderLocalTestReel creates clearly labeled local test media with FFmpeg
// only. It is intended for end-to-end ZIP smoke tests when provider-backed
// rendering is unavailable; callers must report it as a fallback, not as
// provider success.
func RenderLocalTestReel(ctx context.Context, cfg Config, input ReelInput) Result {
	if status, notes := localRendererPrerequisiteStatus(cfg); status != "" {
		return Result{Status: status, Notes: notes}
	}

	dir, err := SafeOutputDir(cfg.OutputDir, input.WorkspaceID, input.ReelPlanID)
	if err != nil {
		return Result{Status: StatusFailed, Notes: err.Error()}
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("create media output dir: %v", err)}
	}

	thumbnailPath := filepath.Join(dir, "thumbnail.png")
	if err := renderLocalThumbnail(ctx, cfg.FFmpegPath, thumbnailPath); err != nil {
		return Result{Status: StatusThumbnailMissing, Notes: fmt.Sprintf("local thumbnail render failed: %v", err)}
	}
	if !fileExists(thumbnailPath) {
		return Result{Status: StatusThumbnailMissing, Notes: "local renderer did not produce thumbnail.png"}
	}

	videoPath := filepath.Join(dir, "video.mp4")
	if err := renderLocalTestVideo(ctx, cfg.FFmpegPath, videoPath); err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("local ffmpeg render failed: %v", err)}
	}
	if !fileExists(videoPath) {
		return Result{Status: StatusFailed, Notes: "local renderer did not produce video.mp4"}
	}

	tw, th, err := pngDimensions(thumbnailPath)
	if err != nil {
		return Result{Status: StatusFailed, Notes: fmt.Sprintf("read thumbnail dimensions: %v", err)}
	}
	duration := probeDuration(ctx, cfg.FFprobePath, videoPath)

	return Result{
		Status:               StatusCompleted,
		Notes:                "Rendered local FFmpeg test media because provider-backed rendering was unavailable.",
		VideoPath:            videoPath,
		VideoFormat:          "mp4",
		VideoWidth:           VideoWidth,
		VideoHeight:          VideoHeight,
		VideoDurationSeconds: duration,
		VideoCodec:           "h264",
		AudioCodec:           "aac",
		ThumbnailPath:        thumbnailPath,
		ThumbnailFormat:      "png",
		ThumbnailWidth:       tw,
		ThumbnailHeight:      th,
	}
}

func prerequisiteStatus(cfg Config) (status, notes string) {
	if strings.TrimSpace(cfg.Provider) != "ffmpeg" {
		return StatusProviderNotConnected, "RENDER_PROVIDER must be set to ffmpeg for the server-side renderer."
	}
	if strings.TrimSpace(cfg.OpenAIAPIKey) == "" {
		return StatusProviderNotConnected, "OPENAI_API_KEY is not configured; no TTS or image provider is connected."
	}
	ffmpegPath := strings.TrimSpace(cfg.FFmpegPath)
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		return StatusRendererNotAvailable, "FFmpeg is not available on PATH or FFMPEG_PATH."
	}
	ffprobePath := strings.TrimSpace(cfg.FFprobePath)
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	if _, err := exec.LookPath(ffprobePath); err != nil {
		return StatusRendererNotAvailable, "FFprobe is not available on PATH or FFPROBE_PATH."
	}
	return "", ""
}

func localRendererPrerequisiteStatus(cfg Config) (status, notes string) {
	ffmpegPath := strings.TrimSpace(cfg.FFmpegPath)
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		return StatusRendererNotAvailable, "FFmpeg is not available on PATH or FFMPEG_PATH."
	}
	ffprobePath := strings.TrimSpace(cfg.FFprobePath)
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	if _, err := exec.LookPath(ffprobePath); err != nil {
		return StatusRendererNotAvailable, "FFprobe is not available on PATH or FFPROBE_PATH."
	}
	return "", ""
}

func SafeOutputDir(baseDir, workspaceID, reelPlanID string) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "generated-media"
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	parts := []string{baseAbs, sanitizePathSegment(workspaceID), sanitizePathSegment(reelPlanID)}
	candidate := filepath.Join(parts...)
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("media output path escapes MEDIA_OUTPUT_DIR")
	}
	return candidateAbs, nil
}

func sanitizePathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "unknown"
	}
	return out
}

func buildThumbnailPrompt(input ReelInput) string {
	return fmt.Sprintf(
		"Create a vertical 9:16 social video thumbnail image for this reel. Title: %s. Description: %s. Visual brief: %s. Use a polished editorial style, no platform logos, no UI chrome, no readable small text.",
		input.Title, input.Description, input.ThumbnailBrief,
	)
}

func (c openAIClient) generateSpeech(ctx context.Context, model, input, outputPath string) error {
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	body := map[string]any{
		"model":           model,
		"voice":           "coral",
		"input":           input,
		"instructions":    "Speak clearly with an energetic, concise social-video narration style.",
		"response_format": "mp3",
	}
	resBody, status, err := c.postJSON(ctx, "https://api.openai.com/v1/audio/speech", body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("OpenAI speech API returned HTTP %d: %s", status, trimForLog(resBody))
	}
	return os.WriteFile(outputPath, resBody, 0640)
}

func (c openAIClient) generateImage(ctx context.Context, model, prompt, outputPath string) error {
	if model == "" {
		model = "gpt-image-1"
	}
	body := map[string]any{
		"model":  model,
		"prompt": prompt,
		"size":   "1024x1536",
	}
	resBody, status, err := c.postJSON(ctx, "https://api.openai.com/v1/images/generations", body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("OpenAI image API returned HTTP %d: %s", status, trimForLog(resBody))
	}

	var parsed struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return err
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return errors.New("image API response did not include b64_json")
	}
	img, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, img, 0640)
}

func (c openAIClient) postJSON(ctx context.Context, url string, body any) ([]byte, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return resBody, resp.StatusCode, nil
}

func normalizeThumbnail(ctx context.Context, ffmpegPath, src, dst string) error {
	args := []string{
		"-y",
		"-i", src,
		"-vf", "scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920,format=rgba",
		"-frames:v", "1",
		dst,
	}
	return runCommand(ctx, ffmpegPath, args...)
}

func renderVideo(ctx context.Context, ffmpegPath, thumbnailPath, audioPath, videoPath string) error {
	args := []string{
		"-y",
		"-loop", "1",
		"-i", thumbnailPath,
		"-i", audioPath,
		"-vf", "scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920,format=yuv420p",
		"-c:v", "libx264",
		"-preset", "medium",
		"-tune", "stillimage",
		"-c:a", "aac",
		"-b:a", "128k",
		"-shortest",
		"-movflags", "+faststart",
		videoPath,
	}
	return runCommand(ctx, ffmpegPath, args...)
}

func renderLocalThumbnail(ctx context.Context, ffmpegPath, thumbnailPath string) error {
	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", "color=c=0x15121f:s=1080x1920:d=1",
		"-frames:v", "1",
		thumbnailPath,
	}
	return runCommand(ctx, ffmpegPath, args...)
}

func renderLocalTestVideo(ctx context.Context, ffmpegPath, videoPath string) error {
	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", "color=c=0x15121f:s=1080x1920:r=30:d=2",
		"-f", "lavfi",
		"-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
		"-shortest",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		videoPath,
	}
	return runCommand(ctx, ffmpegPath, args...)
}

func runCommand(ctx context.Context, name string, args ...string) error {
	if name == "" {
		name = "ffmpeg"
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, trimForLog(stderr.Bytes()))
	}
	return nil
}

func pngDimensions(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func probeDuration(ctx context.Context, ffprobePath, videoPath string) *float64 {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	if _, err := exec.LookPath(ffprobePath); err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "json", videoPath)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil
	}
	var duration float64
	if _, err := fmt.Sscanf(parsed.Format.Duration, "%f", &duration); err != nil || duration <= 0 {
		return nil
	}
	return &duration
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func trimForLog(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 500 {
		return s[:500]
	}
	return s
}
