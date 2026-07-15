package http

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"trendcortex/api/internal/blobstore"
	"trendcortex/api/internal/voice"
)

type voiceGenerationRequest struct {
	ProjectID          string  `json:"project_id"`
	SceneID            string  `json:"scene_id"`
	SourceType         string  `json:"source_type"`
	SourceScriptID     string  `json:"source_script_id"`
	Text               string  `json:"text"`
	Mode               string  `json:"mode"`
	VoiceID            string  `json:"voice_id"`
	Speed              float64 `json:"speed"`
	DeliveryPreset     string  `json:"delivery_preset"`
	CustomInstructions string  `json:"custom_instructions"`
	OutputFormat       string  `json:"output_format"`
	IdempotencyKey     string  `json:"idempotency_key"`
}

type publicVoiceGeneration struct {
	ID                       string                  `json:"id"`
	ProjectID                string                  `json:"project_id,omitempty"`
	SceneID                  string                  `json:"scene_id,omitempty"`
	AssetID                  string                  `json:"asset_id,omitempty"`
	Mode                     string                  `json:"mode"`
	SourceType               string                  `json:"source_type"`
	VoiceID                  string                  `json:"voice_id"`
	VoiceDisplayName         string                  `json:"voice_display_name"`
	Speed                    float64                 `json:"speed"`
	DeliveryPreset           string                  `json:"delivery_preset"`
	OutputFormat             string                  `json:"output_format"`
	Status                   string                  `json:"status"`
	FailureCategory          string                  `json:"failure_category,omitempty"`
	FailureMessage           string                  `json:"failure_message,omitempty"`
	InputCharacters          int                     `json:"input_characters"`
	EstimatedDurationSeconds float64                 `json:"estimated_duration_seconds,omitempty"`
	GeneratedDurationSeconds float64                 `json:"generated_duration_seconds,omitempty"`
	FileSizeBytes            int64                   `json:"file_size_bytes,omitempty"`
	SceneCount               int                     `json:"scene_count"`
	UsageKind                string                  `json:"usage_kind"`
	Asset                    *publicMediaAsset       `json:"asset,omitempty"`
	Children                 []publicVoiceGeneration `json:"children,omitempty"`
	CreatedAt                time.Time               `json:"created_at"`
	UpdatedAt                time.Time               `json:"updated_at"`
	CompletedAt              *time.Time              `json:"completed_at,omitempty"`
}

func (s *Server) handleVoiceStudioRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/voice-studio/"), "/")
	parts := strings.Split(rest, "/")
	switch {
	case r.Method == http.MethodGet && rest == "voices":
		jsonOK(w, map[string]any{"voices": voice.Catalogue(), "formats": []string{"mp3", "wav", "aac", "opus", "flac"}})
	case r.Method == http.MethodPost && rest == "previews":
		s.handleCreateVoicePreview(w, r)
	case r.Method == http.MethodPost && rest == "generations":
		s.handleCreateVoiceGeneration(w, r)
	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "generations" && looksLikeUUID(parts[1]):
		s.handleGetVoiceGeneration(w, r, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "generations" && looksLikeUUID(parts[1]) && parts[2] == "retry":
		jsonErrorCode(w, "not_supported", "Retry is available for failed scene items from the scene list.", http.StatusNotImplemented)
	case r.Method == http.MethodPost && rest == "upload":
		s.handleUploadVoiceover(w, r)
	default:
		jsonErrorCode(w, "not_found", "voice studio route not found", http.StatusNotFound)
	}
}

func (s *Server) voiceProvider() voice.Provider {
	return voice.OpenAIProvider{APIKey: s.cfg.OpenAIAPIKey, Model: s.cfg.OpenAITTSModel, Client: http.DefaultClient}
}

func (s *Server) handleCreateVoicePreview(w http.ResponseWriter, r *http.Request) {
	var req voiceGenerationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Mode = "full"
	req.OutputFormat = normalizeVoiceFormat(req.OutputFormat)
	req.Text = previewText(req.Text)
	gen, err := s.generateVoiceAsset(r, req, "preview")
	if err != nil {
		s.writeVoiceError(w, err)
		return
	}
	jsonOK(w, map[string]any{"generation": gen})
}

func (s *Server) handleCreateVoiceGeneration(w http.ResponseWriter, r *http.Request) {
	var req voiceGenerationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Mode == "" {
		req.Mode = "full"
	}
	req.OutputFormat = normalizeVoiceFormat(req.OutputFormat)
	if req.Mode == "scene" {
		gens, err := s.generateSceneVoiceAssets(r, req)
		if err != nil {
			s.writeVoiceError(w, err)
			return
		}
		jsonOK(w, map[string]any{"generation": aggregateSceneGeneration(gens), "scene_generations": gens})
		return
	}
	gen, err := s.generateVoiceAsset(r, req, "production")
	if err != nil {
		s.writeVoiceError(w, err)
		return
	}
	jsonOK(w, map[string]any{"generation": gen})
}

func (s *Server) generateSceneVoiceAssets(r *http.Request, req voiceGenerationRequest) ([]publicVoiceGeneration, error) {
	if !looksLikeUUID(req.ProjectID) {
		return nil, errors.New("select a project before scene-by-scene generation")
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		return nil, err
	}
	if _, err := s.getContentProject(r, workspaceID, req.ProjectID); err != nil {
		return nil, err
	}
	scenes, err := s.listContentProjectScenes(r.Context(), req.ProjectID)
	if err != nil {
		return nil, err
	}
	out := []publicVoiceGeneration{}
	for _, scene := range scenes {
		if strings.TrimSpace(scene.SpokenText) == "" {
			continue
		}
		sceneReq := req
		sceneReq.SceneID = scene.ID
		sceneReq.SourceType = "scene"
		sceneReq.Text = scene.SpokenText
		sceneReq.Mode = "scene"
		sceneReq.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey) + "-" + scene.ID
		gen, err := s.generateVoiceAsset(r, sceneReq, "production")
		if err != nil {
			out = append(out, publicVoiceGeneration{ProjectID: req.ProjectID, SceneID: scene.ID, Mode: "scene", Status: "failed", FailureMessage: "This scene could not be generated."})
			continue
		}
		out = append(out, gen)
	}
	if len(out) == 0 {
		return nil, errors.New("project scenes do not have narration text")
	}
	return out, nil
}

func (s *Server) generateVoiceAsset(r *http.Request, req voiceGenerationRequest, usageKind string) (publicVoiceGeneration, error) {
	req.Text = voice.NormalizeText(req.Text)
	req.VoiceID = strings.TrimSpace(req.VoiceID)
	if req.VoiceID == "" {
		req.VoiceID = "marin"
	}
	if req.Speed == 0 {
		req.Speed = 1
	}
	if req.DeliveryPreset == "" {
		req.DeliveryPreset = "natural"
	}
	v, ok := voice.VoiceByID(req.VoiceID)
	if !ok {
		return publicVoiceGeneration{}, voice.ErrInvalidVoice
	}
	instructions := voice.SanitizeInstructions(strings.TrimSpace(deliveryInstruction(req.DeliveryPreset) + "\n" + req.CustomInstructions))
	providerReq := voice.Request{Text: req.Text, VoiceID: req.VoiceID, Speed: req.Speed, Instructions: instructions, Format: req.OutputFormat}
	if err := voice.ValidateRequest(providerReq); err != nil {
		return publicVoiceGeneration{}, err
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	if req.ProjectID != "" {
		if _, err := s.getContentProject(r, workspaceID, req.ProjectID); err != nil {
			return publicVoiceGeneration{}, err
		}
	}
	if req.SceneID != "" {
		if _, err := s.getContentProjectScene(r.Context(), req.ProjectID, req.SceneID); err != nil {
			return publicVoiceGeneration{}, err
		}
	}
	result, err := s.voiceProvider().Generate(r.Context(), providerReq)
	if err != nil {
		_, _ = s.insertVoiceGeneration(r.Context(), workspaceID, req, v.DisplayName, usageKind, "", "failed", voiceErrorCategory(err), "Voice generation failed.", 0, 0, instructions)
		return publicVoiceGeneration{}, err
	}
	store, err := s.ensureMediaStore()
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	assetName := voiceAssetName(req, usageKind)
	key, err := blobstore.JoinKey("voice-studio", workspaceID, time.Now().UTC().Format("2006/01/02"), randomishKey(assetName, req.OutputFormat))
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	info, err := store.Put(r.Context(), key, bytes.NewReader(result.Audio), blobstore.PutOptions{ContentType: result.MimeType, OriginalFilename: assetName})
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	size := info.Size
	est := voice.EstimateDurationSeconds(req.Text, req.Speed)
	assetID, err := s.upsertMediaAsset(r.Context(), mediaAssetUpsert{
		WorkspaceID: workspaceID, ProjectID: req.ProjectID, SceneID: nullableSceneID(req.SceneID), AssetType: "voiceover",
		SourceWorkflow: "voice_studio", DisplayName: assetName, OriginalName: assetName, MimeType: result.MimeType,
		SizeBytes: &size, DurationSeconds: &est, StorageProvider: info.Provider, StorageKey: info.Key,
		StorageETag: info.ETag, StorageSHA256: info.SHA256, Status: mediaAssetStatusReady,
	})
	if err != nil {
		_ = store.Delete(r.Context(), key)
		return publicVoiceGeneration{}, err
	}
	genID, err := s.insertVoiceGeneration(r.Context(), workspaceID, req, v.DisplayName, usageKind, assetID, "completed", "", "", size, est, instructions)
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	if usageKind == "production" && req.ProjectID != "" && req.Mode == "full" {
		_, _ = s.db.ExecContext(r.Context(), `UPDATE content_projects SET active_voiceover_asset_id = $3, current_stage = CASE WHEN current_stage IN ('idea','brief','script','scenes') THEN 'voice' ELSE current_stage END, updated_at = NOW() WHERE workspace_id = $1 AND id = $2`, workspaceID, req.ProjectID, assetID)
	}
	return s.getPublicVoiceGeneration(r.Context(), workspaceID, genID)
}

func (s *Server) handleGetVoiceGeneration(w http.ResponseWriter, r *http.Request, id string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	gen, err := s.getPublicVoiceGeneration(r.Context(), workspaceID, id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "voice generation not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "voice generation lookup failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"generation": gen})
}

func (s *Server) handleUploadVoiceover(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(80 << 20); err != nil {
		jsonErrorCode(w, "validation_error", "Upload is too large or invalid.", http.StatusBadRequest)
		return
	}
	projectID := strings.TrimSpace(r.FormValue("project_id"))
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonErrorCode(w, "validation_error", "Choose an audio file to upload.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	gen, err := s.saveUploadedVoiceover(r, projectID, file, header)
	if err != nil {
		s.writeVoiceError(w, err)
		return
	}
	jsonOK(w, map[string]any{"generation": gen})
}

func (s *Server) saveUploadedVoiceover(r *http.Request, projectID string, file multipart.File, header *multipart.FileHeader) (publicVoiceGeneration, error) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	if projectID != "" {
		if _, err := s.getContentProject(r, workspaceID, projectID); err != nil {
			return publicVoiceGeneration{}, err
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, 80<<20))
	if err != nil || len(data) == 0 {
		return publicVoiceGeneration{}, errors.New("uploaded audio could not be read")
	}
	mimeType := http.DetectContentType(data[:minInt(len(data), 512)])
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !supportedUploadAudio(mimeType, ext) {
		return publicVoiceGeneration{}, errors.New("upload a supported audio file")
	}
	if mimeType == "application/octet-stream" {
		mimeType = voice.MimeType(strings.TrimPrefix(ext, "."))
	}
	name := safeOutputDisplayName(header.Filename, "Uploaded narration")
	format := uploadFormat(ext, mimeType)
	key, err := blobstore.JoinKey("voice-studio", workspaceID, "uploads", randomishKey(name, format))
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	store, err := s.ensureMediaStore()
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	info, err := store.Put(r.Context(), key, bytes.NewReader(data), blobstore.PutOptions{ContentType: mimeType, OriginalFilename: name})
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	size := info.Size
	assetID, err := s.upsertMediaAsset(r.Context(), mediaAssetUpsert{WorkspaceID: workspaceID, ProjectID: projectID, AssetType: "voiceover", SourceWorkflow: "voice_studio", DisplayName: name, OriginalName: name, MimeType: mimeType, SizeBytes: &size, StorageProvider: info.Provider, StorageKey: info.Key, StorageETag: info.ETag, StorageSHA256: info.SHA256, Status: mediaAssetStatusReady})
	if err != nil {
		_ = store.Delete(r.Context(), key)
		return publicVoiceGeneration{}, err
	}
	req := voiceGenerationRequest{ProjectID: projectID, SourceType: "upload", Mode: "full", VoiceID: "uploaded", Speed: 1, DeliveryPreset: "uploaded", OutputFormat: format, Text: name}
	genID, err := s.insertVoiceGeneration(r.Context(), workspaceID, req, "Uploaded narration", "upload", assetID, "completed", "", "", size, 0, "")
	if err != nil {
		return publicVoiceGeneration{}, err
	}
	if projectID != "" {
		_, _ = s.db.ExecContext(r.Context(), `UPDATE content_projects SET active_voiceover_asset_id = $3, current_stage = CASE WHEN current_stage IN ('idea','brief','script','scenes') THEN 'voice' ELSE current_stage END, updated_at = NOW() WHERE workspace_id = $1 AND id = $2`, workspaceID, projectID, assetID)
	}
	return s.getPublicVoiceGeneration(r.Context(), workspaceID, genID)
}

func (s *Server) insertVoiceGeneration(ctx context.Context, workspaceID string, req voiceGenerationRequest, voiceName, usageKind, assetID, status, failureCategory, failureMessage string, fileSize int64, duration float64, instructions string) (string, error) {
	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "blank"
	}
	mode := req.Mode
	if mode == "" {
		mode = "full"
	}
	inputChars := len([]rune(voice.NormalizeText(req.Text)))
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO voice_generations (
			workspace_id, project_id, scene_id, asset_id, mode, source_type, source_script_id,
			voice_id, voice_display_name, speed, delivery_preset, custom_instructions, output_format,
			provider_name, provider_model, source_text_hash, input_characters, estimated_duration_seconds,
			generated_duration_seconds, file_size_bytes, scene_count, usage_kind, status, failure_category,
			failure_message, idempotency_key, completed_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'openai',$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,CASE WHEN $22 = 'completed' THEN NOW() ELSE NULL END,NOW())
		ON CONFLICT (workspace_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''
		DO UPDATE SET updated_at = voice_generations.updated_at
		RETURNING id`,
		workspaceID, nullableString(req.ProjectID), nullableString(req.SceneID), nullableString(assetID), mode, sourceType, nullableString(req.SourceScriptID),
		req.VoiceID, voiceName, req.Speed, req.DeliveryPreset, nullableString(instructions), req.OutputFormat, firstNonEmpty(s.cfg.OpenAITTSModel, voice.DefaultModel),
		voice.HashText(req.Text), inputChars, voice.EstimateDurationSeconds(req.Text, req.Speed), nullableFloat(duration), nullableInt64(fileSize), sceneCount(mode),
		usageKind, status, nullableString(failureCategory), nullableString(failureMessage), nullableString(req.IdempotencyKey)).Scan(&id)
	return id, err
}

func (s *Server) getPublicVoiceGeneration(ctx context.Context, workspaceID, id string) (publicVoiceGeneration, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(project_id::text,''), COALESCE(scene_id::text,''), COALESCE(asset_id::text,''), mode, source_type,
			voice_id, voice_display_name, speed, delivery_preset, output_format, status, COALESCE(failure_category,''), COALESCE(failure_message,''),
			input_characters, COALESCE(estimated_duration_seconds,0), COALESCE(generated_duration_seconds,0), COALESCE(file_size_bytes,0),
			scene_count, usage_kind, created_at, updated_at, completed_at
		FROM voice_generations WHERE workspace_id = $1 AND id = $2`, workspaceID, id)
	var g publicVoiceGeneration
	var completed sql.NullTime
	if err := row.Scan(&g.ID, &g.ProjectID, &g.SceneID, &g.AssetID, &g.Mode, &g.SourceType, &g.VoiceID, &g.VoiceDisplayName, &g.Speed, &g.DeliveryPreset, &g.OutputFormat, &g.Status, &g.FailureCategory, &g.FailureMessage, &g.InputCharacters, &g.EstimatedDurationSeconds, &g.GeneratedDurationSeconds, &g.FileSizeBytes, &g.SceneCount, &g.UsageKind, &g.CreatedAt, &g.UpdatedAt, &completed); err != nil {
		return g, err
	}
	if completed.Valid {
		g.CompletedAt = &completed.Time
	}
	if g.AssetID != "" {
		if asset, err := s.getMediaAsset(ctx, workspaceID, g.AssetID, true); err == nil {
			pub := asset.public()
			g.Asset = &pub
		}
	}
	return g, nil
}

func (s *Server) handleSetActiveVoiceover(w http.ResponseWriter, r *http.Request, projectID string) {
	var req struct {
		AssetID string `json:"asset_id"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.getMediaAsset(r.Context(), workspaceID, req.AssetID, false)
	if err != nil || !asset.ProjectID.Valid || asset.ProjectID.String != projectID || asset.AssetType != "voiceover" {
		jsonErrorCode(w, "validation_error", "Choose a project voiceover asset.", http.StatusBadRequest)
		return
	}
	_, err = s.db.ExecContext(r.Context(), `UPDATE content_projects SET active_voiceover_asset_id = $3, updated_at = NOW() WHERE workspace_id = $1 AND id = $2`, workspaceID, projectID, req.AssetID)
	if err != nil {
		jsonError(w, "active voiceover update failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"active_voiceover_asset_id": req.AssetID})
}

func (s *Server) writeVoiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, voice.ErrMissingConfig):
		jsonErrorCode(w, "provider_unavailable", "Voice generation is not configured yet.", http.StatusServiceUnavailable)
	case errors.Is(err, voice.ErrInvalidVoice):
		jsonErrorCode(w, "validation_error", "Choose an available voice.", http.StatusBadRequest)
	case errors.Is(err, voice.ErrRateLimited):
		jsonErrorCode(w, "provider_busy", "Voice generation is temporarily busy. Try again shortly.", http.StatusTooManyRequests)
	case errors.Is(err, voice.ErrUnauthorized):
		jsonErrorCode(w, "provider_unavailable", "Voice generation is unavailable.", http.StatusServiceUnavailable)
	default:
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
	}
}

func deliveryInstruction(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "energetic":
		return "Use an upbeat creator narration style with crisp pacing."
	case "calm":
		return "Use a calm, measured delivery with natural pauses."
	case "documentary":
		return "Use a grounded documentary narration style."
	case "conversational":
		return "Sound conversational and natural, like speaking directly to one viewer."
	case "promotional":
		return "Use a polished promotional delivery without sounding exaggerated."
	case "dramatic":
		return "Use a more dramatic narration style while keeping speech clear."
	default:
		return "Use a natural creator narration style with clear pacing."
	}
}

func normalizeVoiceFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav", "aac", "opus", "flac", "pcm":
		return strings.ToLower(strings.TrimSpace(format))
	default:
		return "mp3"
	}
}

func previewText(text string) string {
	text = voice.NormalizeText(text)
	if text == "" {
		return "Here is a short preview of this narration voice for your next video."
	}
	rs := []rune(text)
	if len(rs) > 220 {
		return string(rs[:220])
	}
	return text
}

func voiceAssetName(req voiceGenerationRequest, usageKind string) string {
	base := "Voiceover"
	if usageKind == "preview" {
		base = "Voice preview"
	}
	if req.SceneID != "" {
		base += " scene"
	}
	return safeOutputDisplayName(base+"."+normalizeVoiceFormat(req.OutputFormat), "Voiceover."+normalizeVoiceFormat(req.OutputFormat))
}

func randomishKey(name, format string) string {
	base := blobstore.SafeSegment(strings.TrimSuffix(name, filepath.Ext(name)), "voiceover")
	return fmt.Sprintf("%d-%s.%s", time.Now().UTC().UnixNano(), base, normalizeVoiceFormat(format))
}

func nullableSceneID(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func nullableFloat(value float64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func sceneCount(mode string) int {
	if mode == "scene" {
		return 1
	}
	return 0
}

func aggregateSceneGeneration(gens []publicVoiceGeneration) publicVoiceGeneration {
	status := "completed"
	for _, gen := range gens {
		if gen.Status != "completed" {
			status = "partially_completed"
		}
	}
	return publicVoiceGeneration{ID: "scene-set", Mode: "scene", SourceType: "all_scenes", Status: status, SceneCount: len(gens), Children: gens, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func supportedUploadAudio(mimeType, ext string) bool {
	if strings.HasPrefix(mimeType, "audio/") {
		return true
	}
	return map[string]bool{".mp3": true, ".wav": true, ".m4a": true, ".aac": true, ".flac": true, ".ogg": true, ".opus": true}[ext]
}

func uploadFormat(ext, mimeType string) string {
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	if ext == "m4a" {
		return "aac"
	}
	if ext != "" {
		return normalizeVoiceFormat(ext)
	}
	if strings.Contains(mimeType, "wav") {
		return "wav"
	}
	return "mp3"
}

func voiceErrorCategory(err error) string {
	switch {
	case errors.Is(err, voice.ErrMissingConfig):
		return "provider_unavailable"
	case errors.Is(err, voice.ErrRateLimited):
		return "provider_rate_limit"
	case errors.Is(err, voice.ErrUnauthorized):
		return "provider_auth"
	default:
		return "provider_error"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
