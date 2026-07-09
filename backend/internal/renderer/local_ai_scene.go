package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LocalAISceneRendererVersion     = "local_ai_scene_v1"
	StatusLocalAIWorkerNotConnected = "local_ai_worker_not_connected"
	localAIWorkerTimeoutSeconds     = 120
	localAIWorkerNextAction         = "Manual worker mode: generate this scene in Pinokio/Wan2GP, then save it as output.mp4 in the shown job folder."
)

type ScenePlan struct {
	SceneID         string  `json:"scene_id"`
	DurationSeconds float64 `json:"duration_seconds"`
	NarrationText   string  `json:"narration_text"`
	VisualPrompt    string  `json:"visual_prompt"`
	NegativePrompt  string  `json:"negative_prompt"`
	StylePreset     string  `json:"style_preset"`
	AspectRatio     string  `json:"aspect_ratio"`
	ModelHint       string  `json:"model_hint"`
}

type LocalAISceneInput struct {
	WorkspaceID         string
	ClipID              string
	Topic               string
	Prompt              string
	StylePreset         string
	TargetLengthSeconds int
	Branding            ClipBrandingSettings
}

type LocalAISceneMetadata struct {
	RendererVersion     string           `json:"renderer_version"`
	WorkerURLConfigured bool             `json:"worker_url_configured"`
	ModelHint           string           `json:"model_hint"`
	ScenePrompts        []ScenePlan      `json:"scene_prompts"`
	SceneJobIDs         []string         `json:"scene_job_ids"`
	SceneJobs           []WorkerSceneJob `json:"scene_jobs"`
	GenerationStatus    string           `json:"generation_status"`
	FallbackReason      string           `json:"fallback_reason,omitempty"`
}

type WorkerClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type WorkerHealth struct {
	Status                string `json:"status"`
	Message               string `json:"message,omitempty"`
	GeneratorMode         string `json:"generator_mode,omitempty"`
	AutoCommandConfigured bool   `json:"auto_command_configured,omitempty"`
	ModelHint             string `json:"model_hint,omitempty"`
	OutputDir             string `json:"output_dir,omitempty"`
}

type WorkerJob struct {
	ID                 string      `json:"id"`
	Status             string      `json:"status"`
	Error              string      `json:"error,omitempty"`
	VisualPrompt       string      `json:"visual_prompt,omitempty"`
	ExpectedOutputPath string      `json:"expected_output_path,omitempty"`
	ManualOutputPath   string      `json:"manual_output_path,omitempty"`
	WorkerMessage      string      `json:"worker_message,omitempty"`
	NextAction         string      `json:"next_action,omitempty"`
	TimeoutSeconds     int         `json:"timeout_seconds,omitempty"`
	Jobs               []WorkerJob `json:"jobs,omitempty"`
}

type WorkerSceneJob struct {
	SceneNumber      int    `json:"scene_number"`
	SceneID          string `json:"scene_id"`
	JobID            string `json:"job_id"`
	Status           string `json:"status"`
	VisualPrompt     string `json:"visual_prompt"`
	ManualOutputPath string `json:"manual_output_path"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	NextAction       string `json:"next_action"`
	WorkerMessage    string `json:"worker_message,omitempty"`
	Error            string `json:"error,omitempty"`
}

func WorkerConfigured(workerURL string) bool {
	return strings.TrimSpace(workerURL) != ""
}

func PlanLocalAIScenes(topic, script, prompt, stylePreset string, targetLengthSeconds int) []ScenePlan {
	text := strings.TrimSpace(firstNonEmpty(script, prompt, topic, "TrendCortex reel"))
	if targetLengthSeconds <= 0 {
		targetLengthSeconds = 60
	}
	sceneCount := targetLengthSeconds / 8
	if sceneCount < 4 {
		sceneCount = 4
	}
	if sceneCount > 12 {
		sceneCount = 12
	}
	duration := float64(targetLengthSeconds) / float64(sceneCount)
	chunks := splitSceneNarration(text, sceneCount)
	scenes := make([]ScenePlan, 0, sceneCount)
	basePrompt := strings.TrimSpace(firstNonEmpty(prompt, script, topic))
	for i := 0; i < sceneCount; i++ {
		narration := chunks[i]
		if strings.TrimSpace(narration) == "" {
			narration = fmt.Sprintf("%s scene %d", firstNonEmpty(topic, "Trend"), i+1)
		}
		visual := fmt.Sprintf("%s. Realistic vertical short-form video scene %d. %s. Natural lighting, believable camera motion, no readable text, no UI, no logos.", basePrompt, i+1, narration)
		scenes = append(scenes, ScenePlan{
			SceneID:         fmt.Sprintf("scene-%02d", i+1),
			DurationSeconds: duration,
			NarrationText:   narration,
			VisualPrompt:    visual,
			NegativePrompt:  "low quality, blurry, distorted faces, extra fingers, unreadable text, watermark, logo, UI screenshot, cartoon, synthetic-looking PNG",
			StylePreset:     firstNonEmpty(stylePreset, "realistic_editorial"),
			AspectRatio:     "9:16",
			ModelHint:       "auto",
		})
	}
	return scenes
}

func RenderLocalAISceneReel(ctx context.Context, cfg Config, input LocalAISceneInput) Result {
	workerConfigured := WorkerConfigured(cfg.LocalAIWorkerURL)
	scenes := PlanLocalAIScenes(input.Topic, "", input.Prompt, input.StylePreset, input.TargetLengthSeconds)
	meta := Result{
		RendererVersion:     LocalAISceneRendererVersion,
		WorkerURLConfigured: workerConfigured,
		ModelHint:           aggregateModelHint(scenes),
		ScenePrompts:        scenes,
		GenerationStatus:    "not_connected",
	}
	if !workerConfigured {
		meta.Status = StatusLocalAIWorkerNotConnected
		meta.Notes = "Local AI worker not connected. Connect local AI worker to generate AI scenes."
		meta.FallbackReason = "LOCAL_AI_WORKER_URL is missing"
		return meta
	}
	if status, notes := localRendererPrerequisiteStatus(cfg); status != "" {
		meta.Status = status
		meta.Notes = notes
		meta.GenerationStatus = "failed"
		return meta
	}
	dir, err := SafeOutputDir(cfg.OutputDir, input.WorkspaceID, input.ClipID)
	if err != nil {
		meta.Status = StatusFailed
		meta.Notes = err.Error()
		meta.GenerationStatus = "failed"
		return meta
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		meta.Status = StatusFailed
		meta.Notes = fmt.Sprintf("create AI scene output dir: %v", err)
		meta.GenerationStatus = "failed"
		return meta
	}

	client := WorkerClient{BaseURL: cfg.LocalAIWorkerURL, Token: cfg.LocalAIWorkerToken, HTTPClient: cfg.HTTPClient}
	workerTimeout := cfg.LocalAIWorkerTimeout
	if workerTimeout <= 0 {
		workerTimeout = time.Duration(localAIWorkerTimeoutSeconds) * time.Second
	}
	if _, err := client.Health(ctx); err != nil {
		meta.Status = StatusFailed
		meta.Notes = "Local AI worker health check failed: " + err.Error()
		meta.GenerationStatus = "failed"
		return meta
	}

	scenePaths := make([]string, 0, len(scenes))
	jobIDs := make([]string, 0, len(scenes))
	sceneJobs := make([]WorkerSceneJob, 0, len(scenes))
	for sceneIndex, scene := range scenes {
		job, err := client.GenerateScene(ctx, scene)
		if err != nil {
			meta.Status = StatusFailed
			meta.Notes = "Local AI worker scene submission failed: " + err.Error()
			meta.GenerationStatus = "failed"
			meta.SceneJobIDs = jobIDs
			meta.SceneJobs = sceneJobs
			return meta
		}
		jobIDs = append(jobIDs, job.ID)
		sceneJobs = append(sceneJobs, workerSceneJobFromWorker(sceneIndex+1, scene, job, "Scene job created"))
		meta.SceneJobIDs = jobIDs
		meta.SceneJobs = sceneJobs
		finalJob, err := client.WaitForJob(ctx, job.ID, workerTimeout)
		sceneJobs[len(sceneJobs)-1] = workerSceneJobFromWorker(sceneIndex+1, scene, finalJob, "")
		if err != nil {
			meta.Status = StatusFailed
			if finalJob.Status == "timed_out" {
				meta.Notes = "Timed out waiting for output.mp4. The worker job may still be pending. Add the generated file and retry/refresh."
				meta.GenerationStatus = "timed_out"
			} else if finalJob.Status == "generator_not_configured" {
				meta.Notes = firstNonEmpty(finalJob.WorkerMessage, "AI video generator is connected but automatic model generation is not configured yet.")
				meta.GenerationStatus = "generator_not_configured"
			} else {
				meta.Notes = "Local AI worker scene generation failed: " + err.Error()
				meta.GenerationStatus = "failed"
			}
			meta.SceneJobIDs = jobIDs
			meta.SceneJobs = sceneJobs
			return meta
		}
		if finalJob.Status != "completed" {
			meta.Status = StatusFailed
			meta.Notes = firstNonEmpty(finalJob.Error, "Local AI worker did not complete scene "+scene.SceneID)
			meta.GenerationStatus = finalJob.Status
			meta.SceneJobIDs = jobIDs
			meta.SceneJobs = sceneJobs
			return meta
		}
		outPath := filepath.Join(dir, scene.SceneID+".mp4")
		if err := client.DownloadOutput(ctx, job.ID, outPath); err != nil {
			meta.Status = StatusFailed
			meta.Notes = "Local AI worker output download failed: " + err.Error()
			meta.GenerationStatus = "failed"
			meta.SceneJobIDs = jobIDs
			meta.SceneJobs = sceneJobs
			return meta
		}
		scenePaths = append(scenePaths, outPath)
	}

	metadata := LocalAISceneMetadata{
		RendererVersion:     LocalAISceneRendererVersion,
		WorkerURLConfigured: true,
		ModelHint:           aggregateModelHint(scenes),
		ScenePrompts:        scenes,
		SceneJobIDs:         jobIDs,
		SceneJobs:           sceneJobs,
		GenerationStatus:    "completed",
	}
	if err := writeLocalAISceneMetadata(dir, metadata); err != nil {
		meta.Status = StatusFailed
		meta.Notes = "store scene metadata: " + err.Error()
		meta.GenerationStatus = "failed"
		meta.SceneJobIDs = jobIDs
		meta.SceneJobs = sceneJobs
		return meta
	}

	videoPath := filepath.Join(dir, "video.mp4")
	subtitlePath := filepath.Join(dir, "subtitles.srt")
	overlayPath := filepath.Join(dir, "scene-overlay.png")
	if err := writeSceneSubtitles(subtitlePath, scenes); err != nil {
		meta.Status = StatusFailed
		meta.Notes = "write subtitles: " + err.Error()
		meta.GenerationStatus = "failed"
		meta.SceneJobIDs = jobIDs
		meta.SceneJobs = sceneJobs
		return meta
	}
	if err := renderLocalAISceneOverlayPNG(overlayPath, input.Branding); err != nil {
		meta.Status = StatusFailed
		meta.Notes = "render branding overlay: " + err.Error()
		meta.GenerationStatus = "failed"
		meta.SceneJobIDs = jobIDs
		meta.SceneJobs = sceneJobs
		return meta
	}
	if err := stitchLocalAIScenes(ctx, cfg.FFmpegPath, scenePaths, overlayPath, subtitlePath, videoPath); err != nil {
		meta.Status = StatusFailed
		meta.Notes = "ffmpeg scene stitch failed: " + err.Error()
		meta.GenerationStatus = "failed"
		meta.SceneJobIDs = jobIDs
		meta.SceneJobs = sceneJobs
		return meta
	}
	thumbnailPath := filepath.Join(dir, "thumbnail.png")
	if err := renderClipThumbnail(ctx, cfg.FFmpegPath, videoPath, thumbnailPath); err != nil {
		meta.Status = StatusThumbnailMissing
		meta.Notes = "thumbnail render failed: " + err.Error()
		meta.GenerationStatus = "failed"
		meta.SceneJobIDs = jobIDs
		meta.SceneJobs = sceneJobs
		return meta
	}
	tw, th, err := pngDimensions(thumbnailPath)
	if err != nil {
		meta.Status = StatusFailed
		meta.Notes = "read thumbnail dimensions: " + err.Error()
		meta.GenerationStatus = "failed"
		meta.SceneJobIDs = jobIDs
		return meta
	}
	duration := probeDuration(ctx, cfg.FFprobePath, videoPath)
	meta.Status = StatusCompleted
	meta.Notes = "Rendered local AI scene reel from worker-generated clips, stitched with branding and subtitles."
	meta.VideoPath = videoPath
	meta.VideoFormat = "mp4"
	meta.VideoWidth = VideoWidth
	meta.VideoHeight = VideoHeight
	meta.VideoDurationSeconds = duration
	meta.VideoCodec = "h264"
	meta.AudioCodec = "aac"
	meta.ThumbnailPath = thumbnailPath
	meta.ThumbnailFormat = "png"
	meta.ThumbnailWidth = tw
	meta.ThumbnailHeight = th
	meta.SceneJobIDs = jobIDs
	meta.SceneJobs = sceneJobs
	meta.GenerationStatus = "completed"
	return meta
}

func (c WorkerClient) Health(ctx context.Context) (WorkerHealth, error) {
	var out WorkerHealth
	body, status, err := c.request(ctx, http.MethodPost, "/health", nil)
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("HTTP %d: %s", status, trimForLog(body))
	}
	if len(body) == 0 {
		return WorkerHealth{Status: "ok"}, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	if out.Status == "" {
		out.Status = "ok"
	}
	return out, nil
}

func (c WorkerClient) GenerateScene(ctx context.Context, scene ScenePlan) (WorkerJob, error) {
	var out WorkerJob
	body, status, err := c.request(ctx, http.MethodPost, "/generate-scene", scene)
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("HTTP %d: %s", status, trimForLog(body))
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	if out.ID == "" {
		return out, errors.New("worker response missing job id")
	}
	out.VisualPrompt = scene.VisualPrompt
	out.Status = normalizeWorkerStatus(out.Status)
	if out.ManualOutputPath == "" {
		out.ManualOutputPath = firstNonEmpty(out.ExpectedOutputPath, manualOutputPath(out.ID))
	}
	if out.ExpectedOutputPath == "" {
		out.ExpectedOutputPath = out.ManualOutputPath
	}
	if out.TimeoutSeconds == 0 {
		out.TimeoutSeconds = localAIWorkerTimeoutSeconds
	}
	if out.NextAction == "" {
		out.NextAction = localAIWorkerNextAction
	}
	return out, nil
}

func (c WorkerClient) Job(ctx context.Context, id string) (WorkerJob, error) {
	var out WorkerJob
	body, status, err := c.request(ctx, http.MethodGet, "/jobs/"+id, nil)
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("HTTP %d: %s", status, trimForLog(body))
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	if out.ID == "" {
		out.ID = id
	}
	out.Status = normalizeWorkerStatus(out.Status)
	if out.ManualOutputPath == "" {
		out.ManualOutputPath = firstNonEmpty(out.ExpectedOutputPath, manualOutputPath(out.ID))
	}
	if out.ExpectedOutputPath == "" {
		out.ExpectedOutputPath = out.ManualOutputPath
	}
	if out.TimeoutSeconds == 0 {
		out.TimeoutSeconds = localAIWorkerTimeoutSeconds
	}
	if out.NextAction == "" {
		out.NextAction = localAIWorkerNextAction
	}
	return out, nil
}

func (c WorkerClient) WaitForJob(ctx context.Context, id string, timeout time.Duration) (WorkerJob, error) {
	deadline := time.Now().Add(timeout)
	for {
		job, err := c.Job(ctx, id)
		if err != nil {
			return job, err
		}
		switch job.Status {
		case "completed", "failed", "error", "cancelled", "generator_not_configured":
			if job.Status == "failed" || job.Status == "error" || job.Status == "cancelled" || job.Status == "generator_not_configured" {
				return job, errors.New(firstNonEmpty(job.Error, "worker job "+job.Status))
			}
			return job, nil
		}
		if time.Now().After(deadline) {
			job.Status = "timed_out"
			job.WorkerMessage = "Timed out waiting for output.mp4. The worker job may still be pending."
			job.NextAction = "Add the generated file and retry/refresh."
			return job, errors.New("worker job timed out")
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (c WorkerClient) Jobs(ctx context.Context) ([]WorkerJob, error) {
	body, status, err := c.request(ctx, http.MethodGet, "/jobs", nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", status, trimForLog(body))
	}
	var out WorkerJob
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	for i := range out.Jobs {
		out.Jobs[i].Status = normalizeWorkerStatus(out.Jobs[i].Status)
		if out.Jobs[i].ManualOutputPath == "" {
			out.Jobs[i].ManualOutputPath = firstNonEmpty(out.Jobs[i].ExpectedOutputPath, manualOutputPath(out.Jobs[i].ID))
		}
		if out.Jobs[i].ExpectedOutputPath == "" {
			out.Jobs[i].ExpectedOutputPath = out.Jobs[i].ManualOutputPath
		}
		if out.Jobs[i].TimeoutSeconds == 0 {
			out.Jobs[i].TimeoutSeconds = localAIWorkerTimeoutSeconds
		}
		if out.Jobs[i].NextAction == "" {
			out.Jobs[i].NextAction = localAIWorkerNextAction
		}
	}
	return out.Jobs, nil
}

func workerSceneJobFromWorker(sceneNumber int, scene ScenePlan, job WorkerJob, fallbackMessage string) WorkerSceneJob {
	status := normalizeWorkerStatus(job.Status)
	message := firstNonEmpty(job.WorkerMessage, fallbackMessage)
	if status == "waiting_for_manual_output" && message == "" {
		message = "Waiting for output.mp4"
	}
	if status == "timed_out" && message == "" {
		message = "Timed out waiting for output.mp4. The worker job may still be pending."
	}
	return WorkerSceneJob{
		SceneNumber:      sceneNumber,
		SceneID:          scene.SceneID,
		JobID:            job.ID,
		Status:           status,
		VisualPrompt:     firstNonEmpty(job.VisualPrompt, scene.VisualPrompt),
		ManualOutputPath: firstNonEmpty(job.ManualOutputPath, job.ExpectedOutputPath, manualOutputPath(job.ID)),
		TimeoutSeconds:   firstNonZero(job.TimeoutSeconds, localAIWorkerTimeoutSeconds),
		NextAction:       firstNonEmpty(job.NextAction, localAIWorkerNextAction),
		WorkerMessage:    message,
		Error:            job.Error,
	}
}

func normalizeWorkerStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", "pending", "queued":
		return "waiting_for_manual_output"
	default:
		return status
	}
}

func manualOutputPath(jobID string) string {
	if strings.TrimSpace(jobID) == "" {
		return "outputs/<job-id>/output.mp4"
	}
	return "outputs/" + jobID + "/output.mp4"
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (c WorkerClient) DownloadOutput(ctx context.Context, id, outputPath string) error {
	body, status, err := c.request(ctx, http.MethodGet, "/jobs/"+id+"/output", nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("HTTP %d: %s", status, trimForLog(body))
	}
	return os.WriteFile(outputPath, body, 0640)
}

func (c WorkerClient) request(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, 0, errors.New("Local AI worker not connected")
	}
	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func splitSceneNarration(text string, sceneCount int) []string {
	words := strings.Fields(text)
	out := make([]string, sceneCount)
	if len(words) == 0 {
		return out
	}
	chunk := (len(words) + sceneCount - 1) / sceneCount
	for i := 0; i < sceneCount; i++ {
		start := i * chunk
		if start >= len(words) {
			startFallback := len(words) - chunk
			if startFallback < 0 {
				startFallback = 0
			}
			out[i] = strings.Join(words[startFallback:], " ")
			continue
		}
		end := start + chunk
		if end > len(words) {
			end = len(words)
		}
		out[i] = strings.Join(words[start:end], " ")
	}
	return out
}

func aggregateModelHint(scenes []ScenePlan) string {
	for _, scene := range scenes {
		if scene.ModelHint != "" && scene.ModelHint != "auto" {
			return scene.ModelHint
		}
	}
	return "auto"
}

func writeLocalAISceneMetadata(dir string, meta LocalAISceneMetadata) error {
	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "scene-metadata.json"), payload, 0640)
}
