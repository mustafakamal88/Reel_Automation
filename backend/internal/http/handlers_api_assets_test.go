package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"trendcortex/api/internal/blobstore"
	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
)

func TestMediaAssetsMigrationBackfillsAndDeduplicates(t *testing.T) {
	s := testAssetServer(t)
	workspaceID := createAssetWorkspace(t, s)
	projectID := createAssetProject(t, s, workspaceID)
	key := "workspaces/" + workspaceID + "/clip-studio/outputs/shared/package.zip"

	_, err := s.db.ExecContext(t.Context(), `
		INSERT INTO clip_studio_sources (id, workspace_id, source_kind, original_filename, storage_provider, storage_key, content_type, size_bytes, source_model, rights_metadata, status, direct_video, supported_type)
		VALUES ('src-backfill-' || $1::text, $1::uuid, 'upload', 'source.mp4', 'local', $2, 'video/mp4', 123, 'user_upload', '{}', 'ready', true, true)
		ON CONFLICT (id) DO NOTHING`, workspaceID, key)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	_, err = s.db.ExecContext(t.Context(), `
		INSERT INTO clip_studio_exports (workspace_id, filename, export_kind, generation_id, storage_provider, storage_key, mime_type, size_bytes)
		VALUES ($1, $2, 'generated_zip', 'gen-backfill', 'local', $3, 'application/zip', 123)
		ON CONFLICT (workspace_id, filename) DO UPDATE SET storage_key = EXCLUDED.storage_key`, workspaceID, "shared.zip", key)
	if err != nil {
		t.Fatalf("insert export: %v", err)
	}
	_, err = s.db.ExecContext(t.Context(), `
		INSERT INTO content_project_outputs (project_id, output_scope, output_type, source_workflow, render_job_id, status, display_name, mime_type, file_size_bytes, storage_provider, storage_key)
		VALUES ($1, 'project', 'generated_clip', 'clip_generator', 'render-backfill', 'completed', 'shared.zip', 'application/zip', 123, 'local', $2)
		ON CONFLICT (project_id, render_job_id) WHERE render_job_id IS NOT NULL AND render_job_id <> '' DO UPDATE SET storage_key = EXCLUDED.storage_key`, projectID, key)
	if err != nil {
		t.Fatalf("insert output: %v", err)
	}
	if err := s.db.Migrate(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if err := s.db.Migrate(); err != nil {
		t.Fatalf("second repeat migration: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM media_assets WHERE workspace_id = $1 AND storage_provider = 'local' AND storage_key = $2`, workspaceID, key).Scan(&count); err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if count != 1 {
		t.Fatalf("asset count = %d, want 1", count)
	}
	var linkedSources, linkedExports, linkedOutputs int
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM clip_studio_sources WHERE workspace_id = $1 AND asset_id IS NOT NULL`, workspaceID).Scan(&linkedSources); err != nil {
		t.Fatalf("count linked sources: %v", err)
	}
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM clip_studio_exports WHERE workspace_id = $1 AND asset_id IS NOT NULL`, workspaceID).Scan(&linkedExports); err != nil {
		t.Fatalf("count linked exports: %v", err)
	}
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM content_project_outputs o JOIN content_projects p ON p.id = o.project_id WHERE p.workspace_id = $1 AND o.asset_id IS NOT NULL`, workspaceID).Scan(&linkedOutputs); err != nil {
		t.Fatalf("count linked outputs: %v", err)
	}
	if linkedSources == 0 || linkedExports == 0 || linkedOutputs == 0 {
		t.Fatalf("expected all domain rows linked, sources=%d exports=%d outputs=%d", linkedSources, linkedExports, linkedOutputs)
	}
}

func TestAssetListDetailArchiveRestoreAndNoInternalMetadata(t *testing.T) {
	s := testAssetServer(t)
	workspaceID := defaultAssetWorkspace(t, s)
	name := "asset-list-" + blobstore.SafeSegment(t.Name(), "test") + ".mp4"
	key := putLocalAssetObject(t, s, workspaceID, name, []byte("video"))
	assetID := insertTestMediaAsset(t, s, workspaceID, key, "source_video", "ready", name)

	req := httptest.NewRequest(http.MethodGet, "/api/assets?search="+name+"&asset_type=source_video&status=ready", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "storage_key") || strings.Contains(body, "storage_provider") || strings.Contains(body, key) {
		t.Fatalf("list response exposed internal storage metadata: %s", body)
	}
	var listed mediaAssetsListResponse
	if err := json.Unmarshal([]byte(body), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Assets) != 1 || listed.Assets[0].ID != assetID || listed.Pagination.Total != 1 {
		t.Fatalf("unexpected list: %+v", listed)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID, nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "storage_key") || strings.Contains(rec.Body.String(), key) {
		t.Fatalf("detail response exposed internal storage metadata: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/assets/"+assetID, nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := s.mediaStore.Stat(t.Context(), key); err != nil {
		t.Fatalf("archive removed object: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/assets/"+assetID+"/restore", nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestAssetPreviewDownloadReconcileAndMissingLocalObject(t *testing.T) {
	s := testAssetServer(t)
	workspaceID := defaultAssetWorkspace(t, s)
	name := "asset-preview-" + blobstore.SafeSegment(t.Name(), "test") + ".mp4"
	key := putLocalAssetObject(t, s, workspaceID, name, []byte("video-bytes"))
	assetID := insertTestMediaAsset(t, s, workspaceID, key, "source_video", "ready", name)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID+"/preview", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "video-bytes" {
		t.Fatalf("preview status = %d body = %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Accept-Ranges") == "" {
		t.Fatalf("preview did not enable byte range support")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID+"/download", nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "video-bytes" {
		t.Fatalf("download status = %d body = %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/assets/"+assetID+"/reconcile", nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reconcile status = %d body = %s", rec.Code, rec.Body.String())
	}

	if err := s.mediaStore.Delete(t.Context(), key); err != nil {
		t.Fatalf("remove object: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID+"/download", nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("missing download status = %d body = %s", rec.Code, rec.Body.String())
	}
	var status string
	if err := s.db.QueryRowContext(t.Context(), `SELECT status FROM media_assets WHERE id = $1`, assetID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", status)
	}
}

func TestAssetS3RedirectAndZipPreviewRejected(t *testing.T) {
	s := testAssetServer(t)
	workspaceID := defaultAssetWorkspace(t, s)
	s3Key := "workspaces/" + workspaceID + "/" + blobstore.SafeSegment(t.Name(), "pkg") + ".zip"
	fake := &fakeAssetStore{provider: blobstore.ProviderS3, objects: map[string][]byte{s3Key: []byte("zip")}}
	s.mediaStore = fake
	assetID := insertTestMediaAssetWithProvider(t, s, workspaceID, s3Key, "package", "ready", "pkg.zip", "application/zip", blobstore.ProviderS3)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID+"/preview", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("zip preview status = %d body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/assets/"+assetID+"/download", nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "signed.example.test") {
		t.Fatalf("s3 download did not redirect, status=%d location=%s body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if fake.deletes != 0 {
		t.Fatalf("download/archive path called delete")
	}
}

func TestNewClipStudioSourceUploadCreatesMediaAsset(t *testing.T) {
	s := testAssetServer(t)
	workspaceID, err := s.defaultWorkspaceID(t.Context())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	var before int
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM media_assets WHERE workspace_id = $1 AND asset_type = 'source_video'`, workspaceID).Scan(&before); err != nil {
		t.Fatalf("count assets before upload: %v", err)
	}
	var body bytes.Buffer
	mw := multipartWriter(t, &body, "video", "asset-source.mp4", []byte("mp4"))
	req := httptest.NewRequest(http.MethodPost, "/api/clip-studio/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d body = %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM media_assets WHERE workspace_id = $1 AND asset_type = 'source_video'`, workspaceID).Scan(&count); err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if count != before+1 {
		t.Fatalf("source media asset count = %d, want %d", count, before+1)
	}
}

func testAssetServer(t *testing.T) *Server {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	tmp := t.TempDir()
	store, err := blobstore.NewLocal(filepath.Join(tmp, "objects"), "")
	if err != nil {
		t.Fatalf("local store: %v", err)
	}
	return NewServerWithMediaStore(&config.Config{
		MediaOutputDir:           filepath.Join(tmp, "media"),
		ExportDir:                filepath.Join(tmp, "exports"),
		MediaStorageLocalDir:     filepath.Join(tmp, "objects"),
		MediaStorageSignedURLTTL: time.Minute,
	}, db, nil, nil, store)
}

func createAssetWorkspace(t *testing.T, s *Server) string {
	t.Helper()
	var id string
	err := s.db.QueryRowContext(t.Context(), `INSERT INTO workspaces (name, owner_id) VALUES ($1, gen_random_uuid()) RETURNING id`, "Asset tests "+t.Name()).Scan(&id)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = s.db.ExecContext(t.Context(), `DELETE FROM workspaces WHERE id = $1`, id) })
	return id
}

func defaultAssetWorkspace(t *testing.T, s *Server) string {
	t.Helper()
	id, err := s.defaultWorkspaceID(t.Context())
	if err != nil {
		t.Fatalf("default workspace: %v", err)
	}
	return id
}

func createAssetProject(t *testing.T, s *Server, workspaceID string) string {
	t.Helper()
	var id string
	err := s.db.QueryRowContext(t.Context(), `
		INSERT INTO content_projects (workspace_id, title, topic, source_type, status, current_stage, target_platforms, content_format, target_duration_seconds, language)
		VALUES ($1, $2, 'Asset tests', 'manual', 'draft', 'video', ARRAY['youtube'], 'short_video', 30, 'en-US')
		RETURNING id`, workspaceID, "Asset project "+t.Name()).Scan(&id)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return id
}

func putLocalAssetObject(t *testing.T, s *Server, workspaceID, filename string, data []byte) string {
	t.Helper()
	key := "workspaces/" + workspaceID + "/assets/" + filename
	info, err := s.mediaStore.Put(t.Context(), key, bytes.NewReader(data), blobstore.PutOptions{ContentType: "video/mp4", OriginalFilename: filename})
	if err != nil {
		t.Fatalf("put local object: %v", err)
	}
	return info.Key
}

func insertTestMediaAsset(t *testing.T, s *Server, workspaceID, key, assetType, status, name string) string {
	t.Helper()
	return insertTestMediaAssetWithProvider(t, s, workspaceID, key, assetType, status, name, "video/mp4", blobstore.ProviderLocal)
}

func insertTestMediaAssetWithProvider(t *testing.T, s *Server, workspaceID, key, assetType, status, name, mimeType, provider string) string {
	t.Helper()
	var id string
	err := s.db.QueryRowContext(t.Context(), `
		INSERT INTO media_assets (workspace_id, asset_type, source_workflow, display_name, original_filename, mime_type, size_bytes, storage_provider, storage_key, status)
		VALUES ($1,$2,'clip_studio',$3,$3,$4,10,$5,$6,$7)
		ON CONFLICT (workspace_id, storage_provider, storage_key) DO UPDATE SET
			asset_type = EXCLUDED.asset_type,
			display_name = EXCLUDED.display_name,
			original_filename = EXCLUDED.original_filename,
			mime_type = EXCLUDED.mime_type,
			status = EXCLUDED.status,
			archived_at = NULL,
			updated_at = NOW()
		RETURNING id`, workspaceID, assetType, name, mimeType, provider, key, status).Scan(&id)
	if err != nil {
		t.Fatalf("insert media asset: %v", err)
	}
	t.Cleanup(func() { _, _ = s.db.ExecContext(t.Context(), `DELETE FROM media_assets WHERE id = $1`, id) })
	return id
}

func multipartWriter(t *testing.T, body *bytes.Buffer, field, filename string, data []byte) *multipart.Writer {
	t.Helper()
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return mw
}

type fakeAssetStore struct {
	provider string
	objects  map[string][]byte
	deletes  int
}

func (f *fakeAssetStore) Put(context.Context, string, io.Reader, blobstore.PutOptions) (blobstore.ObjectInfo, error) {
	return blobstore.ObjectInfo{}, errors.New("not implemented")
}

func (f *fakeAssetStore) PutFile(context.Context, string, string, blobstore.PutOptions) (blobstore.ObjectInfo, error) {
	return blobstore.ObjectInfo{}, errors.New("not implemented")
}

func (f *fakeAssetStore) Open(context.Context, string) (io.ReadCloser, blobstore.ObjectInfo, error) {
	return nil, blobstore.ObjectInfo{}, blobstore.ErrUnsupported
}

func (f *fakeAssetStore) Materialize(context.Context, string, string, string) (string, func(), blobstore.ObjectInfo, error) {
	return "", nil, blobstore.ObjectInfo{}, blobstore.ErrUnsupported
}

func (f *fakeAssetStore) Stat(_ context.Context, key string) (blobstore.ObjectInfo, error) {
	data, ok := f.objects[key]
	if !ok {
		return blobstore.ObjectInfo{}, blobstore.ErrNotFound
	}
	return blobstore.ObjectInfo{Key: key, Provider: f.provider, Size: int64(len(data)), ContentType: "application/zip"}, nil
}

func (f *fakeAssetStore) Delete(context.Context, string) error {
	f.deletes++
	return nil
}

func (f *fakeAssetStore) PresignGet(_ context.Context, key string, _ blobstore.PresignOptions) (string, error) {
	if _, ok := f.objects[key]; !ok {
		return "", blobstore.ErrNotFound
	}
	return "https://signed.example.test/" + key, nil
}

func (f *fakeAssetStore) Provider() string { return f.provider }
