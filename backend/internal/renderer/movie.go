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
	"strings"
)

const MovieRendererVersion = "movie-studio-v1"

type MovieRenderInput struct {
	WorkspaceID      string
	MovieEditID      string
	Title            string
	Width            int
	Height           int
	FrameRate        int
	QualityPreset    string
	VoiceoverPath    string
	MusicPath        string
	NarrationVolume  float64
	MusicVolume      float64
	MusicLoop        bool
	CaptionsEnabled  bool
	BrandingText     string
	BrandingPosition string
	Scenes           []MovieSceneInput
}

type MovieSceneInput struct {
	ID             string
	Title          string
	ScriptText     string
	VisualPath     string
	VisualMimeType string
	Duration       float64
	TrimIn         *float64
	TrimOut        *float64
	FitMode        string
	MotionPreset   string
	CaptionText    string
	TransitionType string
}

type MovieRenderResult struct {
	Status               string
	Notes                string
	VideoPath            string
	VideoFormat          string
	VideoWidth           int
	VideoHeight          int
	VideoDurationSeconds *float64
	VideoCodec           string
	AudioCodec           string
	RendererVersion      string
}

func RenderMovie(ctx context.Context, cfg Config, input MovieRenderInput) MovieRenderResult {
	if status, notes := localRendererPrerequisiteStatus(cfg); status != "" {
		return MovieRenderResult{Status: status, Notes: notes}
	}
	if err := validateMovieInput(input); err != nil {
		return MovieRenderResult{Status: StatusFailed, Notes: err.Error()}
	}
	dir, err := SafeOutputDir(cfg.OutputDir, input.WorkspaceID, input.MovieEditID)
	if err != nil {
		return MovieRenderResult{Status: StatusFailed, Notes: err.Error()}
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("create movie output dir: %v", err)}
	}
	if err := writeMovieRenderPlan(dir, input); err != nil {
		return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("store render plan: %v", err)}
	}

	scenePaths := make([]string, 0, len(input.Scenes))
	for i, scene := range input.Scenes {
		overlayPath := filepath.Join(dir, fmt.Sprintf("scene-%02d-overlay.png", i+1))
		if err := renderMovieOverlayPNG(overlayPath, input, scene, i); err != nil {
			return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("prepare scene overlay: %v", err)}
		}
		scenePath := filepath.Join(dir, fmt.Sprintf("scene-%02d.mp4", i+1))
		if err := renderMovieScene(ctx, cfg.FFmpegPath, input, scene, overlayPath, scenePath); err != nil {
			return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("render scene %d: %v", i+1, err)}
		}
		if !fileExists(scenePath) {
			return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("scene %d did not render", i+1)}
		}
		scenePaths = append(scenePaths, scenePath)
	}

	silentPath := filepath.Join(dir, "video-silent.mp4")
	if err := concatMovieScenes(ctx, cfg.FFmpegPath, scenePaths, silentPath); err != nil {
		return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("assemble scenes: %v", err)}
	}
	videoPath := filepath.Join(dir, "movie.mp4")
	if err := muxMovieAudio(ctx, cfg.FFmpegPath, input, silentPath, videoPath); err != nil {
		return MovieRenderResult{Status: StatusFailed, Notes: fmt.Sprintf("mix audio: %v", err)}
	}
	if !fileExists(videoPath) {
		return MovieRenderResult{Status: StatusFailed, Notes: "movie renderer did not produce an MP4"}
	}
	duration := probeDuration(ctx, cfg.FFprobePath, videoPath)
	return MovieRenderResult{
		Status:               StatusCompleted,
		Notes:                "Rendered Movie Studio MP4 from persisted scenes and Asset Library media.",
		VideoPath:            videoPath,
		VideoFormat:          "mp4",
		VideoWidth:           normalizedMovieWidth(input.Width),
		VideoHeight:          normalizedMovieHeight(input.Height),
		VideoDurationSeconds: duration,
		VideoCodec:           "h264",
		AudioCodec:           "aac",
		RendererVersion:      MovieRendererVersion,
	}
}

func validateMovieInput(input MovieRenderInput) error {
	if len(input.Scenes) == 0 {
		return errors.New("movie edit needs at least one scene")
	}
	if len(input.Scenes) > 40 {
		return errors.New("movie edit has too many scenes")
	}
	for i, scene := range input.Scenes {
		if strings.TrimSpace(scene.VisualPath) == "" {
			return fmt.Errorf("scene %d needs a visual before rendering", i+1)
		}
		if !fileExists(scene.VisualPath) {
			return fmt.Errorf("scene %d visual is no longer available", i+1)
		}
		if scene.Duration <= 0 || scene.Duration > 120 {
			return fmt.Errorf("scene %d has an invalid duration", i+1)
		}
		if scene.TrimIn != nil && *scene.TrimIn < 0 {
			return fmt.Errorf("scene %d has an invalid trim start", i+1)
		}
		if scene.TrimIn != nil && scene.TrimOut != nil && *scene.TrimOut <= *scene.TrimIn {
			return fmt.Errorf("scene %d trim end must be after trim start", i+1)
		}
	}
	return nil
}

func writeMovieRenderPlan(dir string, input MovieRenderInput) error {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "movie-render-plan.json"), payload, 0640)
}

func renderMovieScene(ctx context.Context, ffmpegPath string, input MovieRenderInput, scene MovieSceneInput, overlayPath, outputPath string) error {
	width, height := normalizedMovieWidth(input.Width), normalizedMovieHeight(input.Height)
	fps := normalizedMovieFrameRate(input.FrameRate)
	duration := scene.Duration
	args := []string{"-y"}
	isImage := strings.HasPrefix(strings.ToLower(scene.VisualMimeType), "image/")
	if isImage {
		args = append(args, "-loop", "1", "-framerate", fmt.Sprint(fps), "-t", formatSeconds(duration), "-i", scene.VisualPath)
	} else {
		if scene.TrimIn != nil {
			args = append(args, "-ss", formatSeconds(*scene.TrimIn))
		}
		args = append(args, "-t", formatSeconds(duration), "-i", scene.VisualPath)
	}
	args = append(args, "-loop", "1", "-i", overlayPath)
	filter := movieSceneFilter(scene, width, height, duration)
	args = append(args,
		"-filter_complex", filter,
		"-map", "[vout]",
		"-an",
		"-r", fmt.Sprint(fps),
		"-c:v", "libx264",
		"-preset", moviePreset(input.QualityPreset),
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		outputPath,
	)
	return runCommand(ctx, ffmpegPath, args...)
}

func movieSceneFilter(scene MovieSceneInput, width, height int, duration float64) string {
	mode := strings.TrimSpace(scene.FitMode)
	if mode == "" {
		mode = "fill_crop"
	}
	if mode == "fit_background" || mode == "original" {
		return fmt.Sprintf("[0:v]split=2[srcbg][srcfg];[srcbg]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=28:8,eq=brightness=-0.16:saturation=0.82,setsar=1[bg];[srcfg]scale=%d:%d:force_original_aspect_ratio=decrease,setsar=1[fg];[bg][fg]overlay=x=(W-w)/2:y=(H-h)/2[tmp];[tmp][1:v]overlay=x=0:y=0:shortest=1,trim=duration=%s,setpts=PTS-STARTPTS,format=yuv420p[vout]", width, height, width, height, width, height, formatSeconds(duration))
	}
	if strings.TrimSpace(scene.MotionPreset) == "slow_zoom_in" {
		return fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,zoompan=z='min(zoom+0.00045,1.055)':d=%d:s=%dx%d:fps=30,setsar=1[base];[base][1:v]overlay=x=0:y=0:shortest=1,trim=duration=%s,setpts=PTS-STARTPTS,format=yuv420p[vout]", width+96, height+170, width, height, int(duration*30), width, height, formatSeconds(duration))
	}
	return fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1[base];[base][1:v]overlay=x=0:y=0:shortest=1,trim=duration=%s,setpts=PTS-STARTPTS,format=yuv420p[vout]", width, height, width, height, formatSeconds(duration))
}

func concatMovieScenes(ctx context.Context, ffmpegPath string, scenePaths []string, outputPath string) error {
	dir := filepath.Dir(outputPath)
	listPath := filepath.Join(dir, "movie-scenes.txt")
	var b strings.Builder
	for _, p := range scenePaths {
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(p, "'", "'\\''"))
		b.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0640); err != nil {
		return err
	}
	return runCommand(ctx, ffmpegPath, "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", outputPath)
}

func muxMovieAudio(ctx context.Context, ffmpegPath string, input MovieRenderInput, silentPath, outputPath string) error {
	total := 0.0
	for _, scene := range input.Scenes {
		total += scene.Duration
	}
	args := []string{"-y", "-i", silentPath}
	audioInputs := 0
	if strings.TrimSpace(input.VoiceoverPath) != "" && fileExists(input.VoiceoverPath) {
		args = append(args, "-i", input.VoiceoverPath)
		audioInputs++
	}
	if strings.TrimSpace(input.MusicPath) != "" && fileExists(input.MusicPath) {
		if input.MusicLoop {
			args = append(args, "-stream_loop", "-1")
		}
		args = append(args, "-i", input.MusicPath)
		audioInputs++
	}
	if audioInputs == 0 {
		args = append(args, "-f", "lavfi", "-t", formatSeconds(total), "-i", "anullsrc=channel_layout=stereo:sample_rate=44100", "-map", "0:v", "-map", "1:a")
	} else if audioInputs == 1 {
		args = append(args, "-map", "0:v", "-map", "1:a")
	} else {
		narrVol := input.NarrationVolume
		if narrVol <= 0 {
			narrVol = 1
		}
		musicVol := input.MusicVolume
		if musicVol <= 0 {
			musicVol = 0.18
		}
		args = append(args, "-filter_complex", fmt.Sprintf("[1:a]volume=%s[a1];[2:a]volume=%s[a2];[a1][a2]amix=inputs=2:duration=first:dropout_transition=2[aout]", formatSeconds(narrVol), formatSeconds(musicVol)), "-map", "0:v", "-map", "[aout]")
	}
	args = append(args,
		"-t", formatSeconds(total),
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "160k",
		"-movflags", "+faststart",
		"-shortest",
		outputPath,
	)
	return runCommand(ctx, ffmpegPath, args...)
}

func renderMovieOverlayPNG(path string, input MovieRenderInput, scene MovieSceneInput, index int) error {
	width, height := normalizedMovieWidth(input.Width), normalizedMovieHeight(input.Height)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if input.CaptionsEnabled {
		caption := firstNonEmpty(scene.CaptionText, scene.ScriptText, scene.Title)
		if strings.TrimSpace(caption) != "" {
			boxY := height - 430
			fillRect(img, image.Rect(76, boxY, width-76, boxY+190), color.RGBA{5, 8, 15, 188})
			fillRect(img, image.Rect(76, boxY, 90, boxY+190), color.RGBA{45, 212, 191, 240})
			drawMultilineBitmapText(img, 112, boxY+46, wrapOverlayText(caption, 34, 3), 5, 13, color.RGBA{248, 250, 252, 255})
		}
	}
	if strings.TrimSpace(input.BrandingText) != "" {
		y := 110
		if input.BrandingPosition == "bottom_right" {
			y = height - 156
		}
		drawMultilineBitmapText(img, width-360, y, wrapOverlayText(input.BrandingText, 22, 1), 4, 8, color.RGBA{248, 250, 252, 205})
	}
	drawBadge(img, 78, 96, fmt.Sprintf("%02d", index+1), color.RGBA{15, 23, 42, 190}, color.RGBA{226, 232, 240, 230})
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func normalizedMovieWidth(v int) int {
	if v <= 0 {
		return VideoWidth
	}
	return v
}

func normalizedMovieHeight(v int) int {
	if v <= 0 {
		return VideoHeight
	}
	return v
}

func normalizedMovieFrameRate(v int) int {
	if v <= 0 {
		return 30
	}
	return v
}

func moviePreset(quality string) string {
	switch strings.TrimSpace(quality) {
	case "high":
		return "medium"
	case "draft":
		return "ultrafast"
	default:
		return "veryfast"
	}
}
