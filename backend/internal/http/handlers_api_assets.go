package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"trendcortex/api/internal/blobstore"
)

const (
	mediaAssetStatusReady       = "ready"
	mediaAssetStatusProcessing  = "processing"
	mediaAssetStatusFailed      = "failed"
	mediaAssetStatusUnavailable = "unavailable"
	mediaAssetStatusArchived    = "archived"
)

type mediaAssetRecord struct {
	ID              string
	WorkspaceID     string
	ProjectID       sql.NullString
	ProjectTitle    sql.NullString
	SceneID         sql.NullString
	SceneLabel      sql.NullString
	AssetType       string
	SourceWorkflow  string
	DisplayName     string
	OriginalName    sql.NullString
	MimeType        sql.NullString
	SizeBytes       sql.NullInt64
	DurationSeconds sql.NullFloat64
	Width           sql.NullInt64
	Height          sql.NullInt64
	StorageProvider string
	StorageKey      string
	StorageETag     sql.NullString
	StorageSHA256   sql.NullString
	Status          string
	FailureCategory sql.NullString
	FailureMessage  sql.NullString
	ArchivedAt      sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastVerifiedAt  sql.NullTime
}

type publicMediaAsset struct {
	ID              string             `json:"id"`
	DisplayName     string             `json:"display_name"`
	OriginalName    string             `json:"original_filename,omitempty"`
	AssetType       string             `json:"asset_type"`
	Status          string             `json:"status"`
	SourceWorkflow  string             `json:"source_workflow"`
	MimeType        string             `json:"mime_type,omitempty"`
	SizeBytes       *int64             `json:"size_bytes,omitempty"`
	DurationSeconds *float64           `json:"duration_seconds,omitempty"`
	Width           *int               `json:"width,omitempty"`
	Height          *int               `json:"height,omitempty"`
	Project         *assetProjectRef   `json:"project,omitempty"`
	Scene           *assetSceneRef     `json:"scene,omitempty"`
	ChecksumSHA256  string             `json:"checksum_sha256,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	LastVerifiedAt  *time.Time         `json:"last_verified_at,omitempty"`
	ArchivedAt      *time.Time         `json:"archived_at,omitempty"`
	PreviewURL      string             `json:"preview_url,omitempty"`
	DownloadURL     string             `json:"download_url,omitempty"`
	Capabilities    assetCapabilities  `json:"capabilities"`
	Failure         *assetFailureState `json:"failure,omitempty"`
	StorageLabel    string             `json:"storage_label"`
}

type assetProjectRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type assetSceneRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type assetCapabilities struct {
	CanPreview   bool `json:"can_preview"`
	CanDownload  bool `json:"can_download"`
	CanReconcile bool `json:"can_reconcile"`
	CanArchive   bool `json:"can_archive"`
	CanRestore   bool `json:"can_restore"`
}

type assetFailureState struct {
	Category string `json:"category,omitempty"`
	Message  string `json:"message"`
}

type mediaAssetsListResponse struct {
	Assets     []publicMediaAsset      `json:"assets"`
	Pagination assetPagination         `json:"pagination"`
	Summary    map[string]int          `json:"summary"`
	Facets     map[string][]assetFacet `json:"facets"`
}

type assetPagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

type assetFacet struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type mediaAssetUpsert struct {
	WorkspaceID     string
	ProjectID       string
	SceneID         *string
	AssetType       string
	SourceWorkflow  string
	DisplayName     string
	OriginalName    string
	MimeType        string
	SizeBytes       *int64
	DurationSeconds *float64
	Width           *int
	Height          *int
	StorageProvider string
	StorageKey      string
	StorageETag     string
	StorageSHA256   string
	Status          string
	FailureCategory string
	FailureMessage  string
}

func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	resp, err := s.listMediaAssets(r.Context(), workspaceID, r.URL.Query())
	if err != nil {
		jsonError(w, "asset library unavailable", http.StatusInternalServerError)
		return
	}
	jsonOK(w, resp)
}

func (s *Server) handleAssetRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/assets/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" || !looksLikeUUID(parts[0]) {
		jsonErrorCode(w, "not_found", "asset not found", http.StatusNotFound)
		return
	}
	assetID := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleGetAsset(w, r, assetID)
		case http.MethodDelete:
			s.handleArchiveAsset(w, r, assetID)
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) != 2 {
		jsonErrorCode(w, "not_found", "asset route not found", http.StatusNotFound)
		return
	}
	switch parts[1] {
	case "preview":
		if r.Method == http.MethodGet {
			s.handlePreviewAsset(w, r, assetID)
			return
		}
	case "download":
		if r.Method == http.MethodGet {
			s.handleDownloadAsset(w, r, assetID)
			return
		}
	case "reconcile":
		if r.Method == http.MethodPost {
			s.handleReconcileAsset(w, r, assetID)
			return
		}
	case "restore":
		if r.Method == http.MethodPost {
			s.handleRestoreAsset(w, r, assetID)
			return
		}
	}
	jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.getMediaAsset(r.Context(), workspaceID, assetID, true)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "asset not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "asset lookup failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]publicMediaAsset{"asset": asset.public()})
}

func (s *Server) handlePreviewAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	s.serveAssetObject(w, r, assetID, true)
}

func (s *Server) handleDownloadAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	s.serveAssetObject(w, r, assetID, false)
}

func (s *Server) handleReconcileAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.reconcileMediaAsset(r.Context(), workspaceID, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "asset not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "storage verification failed", http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]publicMediaAsset{"asset": asset.public()})
}

func (s *Server) handleArchiveAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.archiveMediaAsset(r.Context(), workspaceID, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "asset not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "asset archive failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]publicMediaAsset{"asset": asset.public()})
}

func (s *Server) handleRestoreAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.restoreMediaAsset(r.Context(), workspaceID, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "asset not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "asset restore failed", http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]publicMediaAsset{"asset": asset.public()})
}

func (s *Server) listMediaAssets(ctx context.Context, workspaceID string, q map[string][]string) (mediaAssetsListResponse, error) {
	limit := boundedInt(assetFirstQuery(q, "limit"), 24, 1, 60)
	offset := boundedInt(assetFirstQuery(q, "offset"), 0, 0, 10000)
	search := limitText(strings.TrimSpace(assetFirstQuery(q, "search")), 120)
	assetType := assetFirstQuery(q, "asset_type")
	status := assetFirstQuery(q, "status")
	projectID := assetFirstQuery(q, "project_id")
	workflow := assetFirstQuery(q, "source_workflow")
	includeArchived := assetFirstQuery(q, "include_archived") == "true"
	sortExpr := "a.created_at DESC, a.id DESC"
	switch assetFirstQuery(q, "sort") {
	case "oldest":
		sortExpr = "a.created_at ASC, a.id ASC"
	case "name":
		sortExpr = "lower(a.display_name) ASC, a.created_at DESC"
	case "size_desc":
		sortExpr = "a.size_bytes DESC NULLS LAST, a.created_at DESC"
	case "size_asc":
		sortExpr = "a.size_bytes ASC NULLS LAST, a.created_at DESC"
	}

	where, args := []string{"a.workspace_id = $1"}, []any{workspaceID}
	if !includeArchived {
		where = append(where, "a.archived_at IS NULL")
	}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where = append(where, fmt.Sprintf("(lower(a.display_name) LIKE $%d OR lower(COALESCE(a.original_filename, '')) LIKE $%d)", len(args), len(args)))
	}
	if validAssetType(assetType) {
		args = append(args, assetType)
		where = append(where, fmt.Sprintf("a.asset_type = $%d", len(args)))
	}
	if validAssetStatus(status) {
		args = append(args, status)
		where = append(where, fmt.Sprintf("a.status = $%d", len(args)))
	}
	if looksLikeUUID(projectID) {
		args = append(args, projectID)
		where = append(where, fmt.Sprintf("a.project_id = $%d", len(args)))
	}
	if validAssetWorkflow(workflow) {
		args = append(args, workflow)
		where = append(where, fmt.Sprintf("a.source_workflow = $%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT COUNT(*) FROM media_assets a WHERE " + whereSQL
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return mediaAssetsListResponse{}, err
	}

	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+mediaAssetSelectColumns+`
		FROM media_assets a
		LEFT JOIN content_projects p ON p.id = a.project_id
		LEFT JOIN content_project_scenes sc ON sc.id = a.scene_id
		WHERE `+whereSQL+`
		ORDER BY `+sortExpr+`
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if err != nil {
		return mediaAssetsListResponse{}, err
	}
	defer rows.Close()
	assets := []publicMediaAsset{}
	for rows.Next() {
		rec, err := scanMediaAsset(rows)
		if err != nil {
			return mediaAssetsListResponse{}, err
		}
		assets = append(assets, rec.public())
	}
	if err := rows.Err(); err != nil {
		return mediaAssetsListResponse{}, err
	}
	summary, facets, err := s.mediaAssetSummary(ctx, workspaceID, includeArchived)
	if err != nil {
		return mediaAssetsListResponse{}, err
	}
	return mediaAssetsListResponse{
		Assets: assets,
		Pagination: assetPagination{
			Limit:   limit,
			Offset:  offset,
			Total:   total,
			HasMore: offset+len(assets) < total,
		},
		Summary: summary,
		Facets:  facets,
	}, nil
}

func (s *Server) mediaAssetSummary(ctx context.Context, workspaceID string, includeArchived bool) (map[string]int, map[string][]assetFacet, error) {
	whereArchived := "AND archived_at IS NULL"
	if includeArchived {
		whereArchived = ""
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT asset_type, status, COUNT(*)
		FROM media_assets
		WHERE workspace_id = $1 `+whereArchived+`
		GROUP BY asset_type, status`, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	summary := map[string]int{"total": 0, "videos": 0, "audio": 0, "thumbnails": 0, "packages": 0, "needs_attention": 0}
	typeCounts := map[string]int{}
	statusCounts := map[string]int{}
	for rows.Next() {
		var assetType, status string
		var count int
		if err := rows.Scan(&assetType, &status, &count); err != nil {
			return nil, nil, err
		}
		summary["total"] += count
		if strings.Contains(assetType, "video") {
			summary["videos"] += count
		}
		if assetType == "audio" || assetType == "voiceover" {
			summary["audio"] += count
		}
		if assetType == "thumbnail" {
			summary["thumbnails"] += count
		}
		if assetType == "package" {
			summary["packages"] += count
		}
		if status == mediaAssetStatusFailed || status == mediaAssetStatusUnavailable {
			summary["needs_attention"] += count
		}
		typeCounts[assetType] += count
		statusCounts[status] += count
	}
	facets := map[string][]assetFacet{
		"asset_type":      facetList(typeCounts),
		"status":          facetList(statusCounts),
		"source_workflow": {},
		"project":         {},
	}
	workflowRows, err := s.db.QueryContext(ctx, `SELECT source_workflow, COUNT(*) FROM media_assets WHERE workspace_id = $1 `+whereArchived+` GROUP BY source_workflow`, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	defer workflowRows.Close()
	workflowCounts := map[string]int{}
	for workflowRows.Next() {
		var value string
		var count int
		if err := workflowRows.Scan(&value, &count); err != nil {
			return nil, nil, err
		}
		workflowCounts[value] = count
	}
	facets["source_workflow"] = facetList(workflowCounts)
	projectRows, err := s.db.QueryContext(ctx, `
		SELECT a.project_id::text, COALESCE(p.title, 'Untitled project'), COUNT(*)
		FROM media_assets a
		JOIN content_projects p ON p.id = a.project_id
		WHERE a.workspace_id = $1 `+strings.ReplaceAll(whereArchived, "archived_at", "a.archived_at")+`
		GROUP BY a.project_id, p.title
		ORDER BY p.title`, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	defer projectRows.Close()
	projects := []assetFacet{}
	for projectRows.Next() {
		var value, label string
		var count int
		if err := projectRows.Scan(&value, &label, &count); err != nil {
			return nil, nil, err
		}
		projects = append(projects, assetFacet{Value: value, Label: label, Count: count})
	}
	facets["project"] = projects
	return summary, facets, rows.Err()
}

func (s *Server) getMediaAsset(ctx context.Context, workspaceID, assetID string, includeArchived bool) (mediaAssetRecord, error) {
	whereArchived := "AND a.archived_at IS NULL"
	if includeArchived {
		whereArchived = ""
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+mediaAssetSelectColumns+`
		FROM media_assets a
		LEFT JOIN content_projects p ON p.id = a.project_id
		LEFT JOIN content_project_scenes sc ON sc.id = a.scene_id
		WHERE a.workspace_id = $1 AND a.id = $2 `+whereArchived, workspaceID, assetID)
	return scanMediaAsset(row)
}

func (s *Server) serveAssetObject(w http.ResponseWriter, r *http.Request, assetID string, preview bool) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.getMediaAsset(r.Context(), workspaceID, assetID, true)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "asset not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "asset lookup failed", http.StatusInternalServerError)
		return
	}
	if asset.Status != mediaAssetStatusReady {
		jsonErrorCode(w, "unavailable", "asset is not ready", http.StatusGone)
		return
	}
	contentType := firstNonEmpty(asset.MimeType.String, "application/octet-stream")
	if preview && !supportsAssetPreview(asset.AssetType, contentType) {
		jsonErrorCode(w, "preview_unsupported", "preview is not available for this asset", http.StatusUnsupportedMediaType)
		return
	}
	store, err := s.ensureMediaStore()
	if err != nil || store.Provider() != asset.StorageProvider {
		_ = s.markMediaAssetUnavailable(r.Context(), workspaceID, asset.ID, "storage_unavailable", "Asset storage is unavailable.")
		jsonErrorCode(w, "unavailable", "asset storage is unavailable", http.StatusGone)
		return
	}
	filename := safeDownloadFilename(asset.DisplayName, contentType)
	dispositionType := "attachment"
	if preview {
		dispositionType = "inline"
	}
	contentDisposition := dispositionType + `; filename="` + filename + `"`
	if asset.StorageProvider == blobstore.ProviderS3 {
		if _, err := store.Stat(r.Context(), asset.StorageKey); err != nil {
			if errors.Is(err, blobstore.ErrNotFound) {
				_ = s.markMediaAssetUnavailable(r.Context(), workspaceID, asset.ID, "object_missing", "The stored object could not be found.")
				jsonErrorCode(w, "unavailable", "asset is unavailable", http.StatusGone)
				return
			}
			jsonError(w, "asset storage lookup failed", http.StatusBadGateway)
			return
		}
		u, err := store.PresignGet(r.Context(), asset.StorageKey, blobstore.PresignOptions{
			TTL:                s.cfg.MediaStorageSignedURLTTL,
			ContentDisposition: contentDisposition,
		})
		if err != nil {
			jsonError(w, "asset download could not be prepared", http.StatusBadGateway)
			return
		}
		http.Redirect(w, r, u, http.StatusFound)
		return
	}
	body, info, err := store.Open(r.Context(), asset.StorageKey)
	if err != nil {
		if errors.Is(err, blobstore.ErrNotFound) {
			_ = s.markMediaAssetUnavailable(r.Context(), workspaceID, asset.ID, "object_missing", "The stored object could not be found.")
			jsonErrorCode(w, "unavailable", "asset is unavailable", http.StatusGone)
			return
		}
		jsonError(w, "asset download failed", http.StatusBadGateway)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", firstNonEmpty(contentType, info.ContentType, "application/octet-stream"))
	w.Header().Set("Content-Disposition", contentDisposition)
	if seeker, ok := body.(io.ReadSeeker); ok {
		mod := info.LastModified
		if mod.IsZero() {
			mod = asset.UpdatedAt
		}
		http.ServeContent(w, r, filename, mod, seeker)
		return
	}
	_, _ = io.Copy(w, body)
}

func (s *Server) reconcileMediaAsset(ctx context.Context, workspaceID, assetID string) (mediaAssetRecord, error) {
	asset, err := s.getMediaAsset(ctx, workspaceID, assetID, true)
	if err != nil {
		return mediaAssetRecord{}, err
	}
	store, err := s.ensureMediaStore()
	if err != nil || store.Provider() != asset.StorageProvider {
		_ = s.markMediaAssetUnavailable(ctx, workspaceID, assetID, "storage_unavailable", "Asset storage is unavailable.")
		return s.getMediaAsset(ctx, workspaceID, assetID, true)
	}
	info, err := store.Stat(ctx, asset.StorageKey)
	if errors.Is(err, blobstore.ErrNotFound) {
		_ = s.markMediaAssetUnavailable(ctx, workspaceID, assetID, "object_missing", "The stored object could not be found.")
		return s.getMediaAsset(ctx, workspaceID, assetID, true)
	}
	if err != nil {
		return mediaAssetRecord{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE media_assets SET
			status = CASE WHEN archived_at IS NOT NULL THEN 'archived' ELSE 'ready' END,
			size_bytes = COALESCE(NULLIF($3, -1), size_bytes),
			mime_type = COALESCE(NULLIF($4, ''), mime_type),
			storage_etag = COALESCE(NULLIF($5, ''), storage_etag),
			last_verified_at = NOW(),
			failure_category = NULL,
			failure_message = NULL,
			updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2`, workspaceID, assetID, nullableInt64(info.Size), info.ContentType, info.ETag)
	if err != nil {
		return mediaAssetRecord{}, err
	}
	return s.getMediaAsset(ctx, workspaceID, assetID, true)
}

func (s *Server) archiveMediaAsset(ctx context.Context, workspaceID, assetID string) (mediaAssetRecord, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE media_assets
		SET status = 'archived', archived_at = COALESCE(archived_at, NOW()), updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2`, workspaceID, assetID)
	if err != nil {
		return mediaAssetRecord{}, err
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return mediaAssetRecord{}, sql.ErrNoRows
	}
	return s.getMediaAsset(ctx, workspaceID, assetID, true)
}

func (s *Server) restoreMediaAsset(ctx context.Context, workspaceID, assetID string) (mediaAssetRecord, error) {
	asset, err := s.getMediaAsset(ctx, workspaceID, assetID, true)
	if err != nil {
		return mediaAssetRecord{}, err
	}
	status := mediaAssetStatusReady
	store, storeErr := s.ensureMediaStore()
	if storeErr != nil || store.Provider() != asset.StorageProvider {
		status = mediaAssetStatusUnavailable
	} else if _, err := store.Stat(ctx, asset.StorageKey); errors.Is(err, blobstore.ErrNotFound) {
		status = mediaAssetStatusUnavailable
	} else if err != nil {
		return mediaAssetRecord{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE media_assets
		SET status = $3, archived_at = NULL, last_verified_at = NOW(),
			failure_category = CASE WHEN $3 = 'unavailable' THEN 'object_missing' ELSE NULL END,
			failure_message = CASE WHEN $3 = 'unavailable' THEN 'The stored object could not be found.' ELSE NULL END,
			updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2`, workspaceID, assetID, status)
	if err != nil {
		return mediaAssetRecord{}, err
	}
	return s.getMediaAsset(ctx, workspaceID, assetID, true)
}

func (s *Server) upsertMediaAsset(ctx context.Context, m mediaAssetUpsert) (string, error) {
	if s.db == nil || strings.TrimSpace(m.StorageKey) == "" || strings.TrimSpace(m.StorageProvider) == "" {
		return "", nil
	}
	if err := blobstore.ValidateKey(m.StorageKey); err != nil {
		return "", err
	}
	m.AssetType = normalizeAssetType(m.AssetType, m.MimeType)
	m.SourceWorkflow = normalizeAssetWorkflow(m.SourceWorkflow)
	m.Status = normalizeAssetStatus(m.Status)
	m.DisplayName = safeOutputDisplayName(m.DisplayName, readableAssetType(m.AssetType))
	var id string
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO media_assets (
			workspace_id, project_id, scene_id, asset_type, source_workflow, display_name, original_filename, mime_type,
			size_bytes, duration_seconds, width, height, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
			status, failure_category, failure_message, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,NOW())
		ON CONFLICT (workspace_id, storage_provider, storage_key) DO UPDATE SET
			project_id = COALESCE(media_assets.project_id, EXCLUDED.project_id),
			scene_id = COALESCE(media_assets.scene_id, EXCLUDED.scene_id),
			asset_type = EXCLUDED.asset_type,
			source_workflow = EXCLUDED.source_workflow,
			display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), media_assets.display_name),
			original_filename = COALESCE(media_assets.original_filename, EXCLUDED.original_filename),
			mime_type = COALESCE(EXCLUDED.mime_type, media_assets.mime_type),
			size_bytes = COALESCE(EXCLUDED.size_bytes, media_assets.size_bytes),
			duration_seconds = COALESCE(EXCLUDED.duration_seconds, media_assets.duration_seconds),
			width = COALESCE(EXCLUDED.width, media_assets.width),
			height = COALESCE(EXCLUDED.height, media_assets.height),
			storage_etag = COALESCE(EXCLUDED.storage_etag, media_assets.storage_etag),
			storage_checksum_sha256 = COALESCE(EXCLUDED.storage_checksum_sha256, media_assets.storage_checksum_sha256),
			status = CASE WHEN media_assets.archived_at IS NOT NULL THEN 'archived' ELSE EXCLUDED.status END,
			failure_category = EXCLUDED.failure_category,
			failure_message = EXCLUDED.failure_message,
			updated_at = NOW()
		RETURNING id`,
		m.WorkspaceID, nullableString(m.ProjectID), nullableStringPtr(m.SceneID), m.AssetType, m.SourceWorkflow, m.DisplayName,
		nullableString(m.OriginalName), nullableString(m.MimeType), m.SizeBytes, m.DurationSeconds, intPtrToAny(m.Width), intPtrToAny(m.Height),
		m.StorageProvider, m.StorageKey, nullableString(m.StorageETag), nullableString(m.StorageSHA256), m.Status,
		nullableString(m.FailureCategory), nullableString(m.FailureMessage)).Scan(&id)
	return id, err
}

func (s *Server) markMediaAssetUnavailable(ctx context.Context, workspaceID, assetID, category, message string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE media_assets
		SET status = 'unavailable', failure_category = $3, failure_message = $4,
			last_verified_at = NOW(), updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2`, workspaceID, assetID, limitText(category, 80), safeFailureMessage(message))
	return err
}

const mediaAssetSelectColumns = `
	a.id, a.workspace_id, a.project_id, COALESCE(p.title, ''), a.scene_id, COALESCE(sc.title, ''),
	a.asset_type, a.source_workflow, a.display_name, a.original_filename, a.mime_type, a.size_bytes,
	a.duration_seconds, a.width, a.height, a.storage_provider, a.storage_key, a.storage_etag,
	a.storage_checksum_sha256, a.status, a.failure_category, a.failure_message, a.archived_at,
	a.created_at, a.updated_at, a.last_verified_at`

type mediaAssetScanner interface {
	Scan(dest ...any) error
}

func scanMediaAsset(row mediaAssetScanner) (mediaAssetRecord, error) {
	var rec mediaAssetRecord
	err := row.Scan(
		&rec.ID, &rec.WorkspaceID, &rec.ProjectID, &rec.ProjectTitle, &rec.SceneID, &rec.SceneLabel,
		&rec.AssetType, &rec.SourceWorkflow, &rec.DisplayName, &rec.OriginalName, &rec.MimeType,
		&rec.SizeBytes, &rec.DurationSeconds, &rec.Width, &rec.Height, &rec.StorageProvider, &rec.StorageKey,
		&rec.StorageETag, &rec.StorageSHA256, &rec.Status, &rec.FailureCategory, &rec.FailureMessage,
		&rec.ArchivedAt, &rec.CreatedAt, &rec.UpdatedAt, &rec.LastVerifiedAt,
	)
	return rec, err
}

func (a mediaAssetRecord) public() publicMediaAsset {
	out := publicMediaAsset{
		ID:             a.ID,
		DisplayName:    a.DisplayName,
		OriginalName:   safeOriginalFilename(a.OriginalName.String),
		AssetType:      a.AssetType,
		Status:         a.Status,
		SourceWorkflow: a.SourceWorkflow,
		MimeType:       a.MimeType.String,
		ChecksumSHA256: a.StorageSHA256.String,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
		DownloadURL:    "/api/assets/" + a.ID + "/download",
		StorageLabel:   "Durable storage",
	}
	if a.SizeBytes.Valid {
		out.SizeBytes = &a.SizeBytes.Int64
	}
	if a.DurationSeconds.Valid {
		out.DurationSeconds = &a.DurationSeconds.Float64
	}
	if a.Width.Valid {
		v := int(a.Width.Int64)
		out.Width = &v
	}
	if a.Height.Valid {
		v := int(a.Height.Int64)
		out.Height = &v
	}
	if a.ProjectID.Valid {
		out.Project = &assetProjectRef{ID: a.ProjectID.String, Title: a.ProjectTitle.String}
	}
	if a.SceneID.Valid {
		out.Scene = &assetSceneRef{ID: a.SceneID.String, Label: a.SceneLabel.String}
	}
	if a.LastVerifiedAt.Valid {
		out.LastVerifiedAt = &a.LastVerifiedAt.Time
	}
	if a.ArchivedAt.Valid {
		out.ArchivedAt = &a.ArchivedAt.Time
	}
	canPreview := a.Status == mediaAssetStatusReady && supportsAssetPreview(a.AssetType, a.MimeType.String)
	if canPreview {
		out.PreviewURL = "/api/assets/" + a.ID + "/preview"
	}
	out.Capabilities = assetCapabilities{
		CanPreview:   canPreview,
		CanDownload:  a.Status == mediaAssetStatusReady,
		CanReconcile: true,
		CanArchive:   !a.ArchivedAt.Valid,
		CanRestore:   a.ArchivedAt.Valid,
	}
	if a.Status == mediaAssetStatusFailed || a.Status == mediaAssetStatusUnavailable {
		out.Failure = &assetFailureState{
			Category: a.FailureCategory.String,
			Message:  firstNonEmpty(a.FailureMessage.String, "This asset needs attention before it can be used."),
		}
	}
	return out
}

func supportsAssetPreview(assetType, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if assetType == "thumbnail" || strings.HasPrefix(mimeType, "image/") {
		return true
	}
	if strings.HasPrefix(mimeType, "video/") {
		return mimeType == "video/mp4" || mimeType == "video/quicktime" || mimeType == "video/webm"
	}
	if strings.HasPrefix(mimeType, "audio/") || assetType == "audio" || assetType == "voiceover" {
		return true
	}
	return false
}

func normalizeAssetType(assetType, mimeType string) string {
	switch assetType {
	case "source_video", "generated_video", "rendered_video", "ai_scene_video", "thumbnail", "package", "metadata", "audio", "voiceover":
		return assetType
	}
	if strings.HasPrefix(mimeType, "image/") {
		return "thumbnail"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "generated_video"
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return "audio"
	}
	if mimeType == "application/zip" {
		return "package"
	}
	return "metadata"
}

func normalizeAssetWorkflow(workflow string) string {
	switch workflow {
	case "clip_studio", "clip_generator", "project_output", "ai_scene", "voice_studio", "movie_studio":
		return workflow
	default:
		return "clip_studio"
	}
}

func normalizeAssetStatus(status string) string {
	if validAssetStatus(status) {
		return status
	}
	return mediaAssetStatusReady
}

func validAssetType(value string) bool {
	return map[string]bool{"source_video": true, "generated_video": true, "rendered_video": true, "ai_scene_video": true, "thumbnail": true, "package": true, "metadata": true, "audio": true, "voiceover": true}[value]
}

func validAssetStatus(value string) bool {
	return map[string]bool{"ready": true, "processing": true, "failed": true, "unavailable": true, "archived": true}[value]
}

func validAssetWorkflow(value string) bool {
	return map[string]bool{"clip_studio": true, "clip_generator": true, "project_output": true, "ai_scene": true, "voice_studio": true, "movie_studio": true}[value]
}

func readableAssetType(assetType string) string {
	switch assetType {
	case "source_video":
		return "Source video"
	case "generated_video":
		return "Generated video"
	case "rendered_video":
		return "Rendered video"
	case "ai_scene_video":
		return "AI scene video"
	case "thumbnail":
		return "Thumbnail"
	case "package":
		return "Package"
	case "audio":
		return "Audio"
	case "voiceover":
		return "Voiceover"
	default:
		return "Media asset"
	}
}

func safeOriginalFilename(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	if value == "." || value == "/" {
		return ""
	}
	return limitText(value, 255)
}

func assetFirstQuery(q map[string][]string, key string) string {
	if values := q[key]; len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	return ""
}

func boundedInt(value string, fallback, min, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func facetList(counts map[string]int) []assetFacet {
	out := make([]assetFacet, 0, len(counts))
	for value, count := range counts {
		out = append(out, assetFacet{Value: value, Label: readableFacetLabel(value), Count: count})
	}
	return out
}

func readableFacetLabel(value string) string {
	value = strings.ReplaceAll(value, "_", " ")
	if value == "" {
		return "Unknown"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func nullableStringPtr(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

func intPtrToAny(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
