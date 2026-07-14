package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
)

func TestContentProjectOutputsDatabaseLifecycle(t *testing.T) {
	s := testContentProjectOutputServer(t)
	project := createOutputTestProject(t, s, "scenes")
	filePath := filepath.Join(t.TempDir(), "output.zip")
	if err := os.WriteFile(filePath, []byte("zip"), 0640); err != nil {
		t.Fatalf("write output fixture: %v", err)
	}

	size := int64(3)
	output, err := s.upsertContentProjectOutput(t.Context(), project.workspaceID, project.id, nil, contentProjectOutputMutation{
		OutputScope:      projectOutputScopeProject,
		OutputType:       projectOutputTypeGenerated,
		SourceWorkflow:   projectOutputWorkflowClipGenerator,
		RenderJobID:      "render-lifecycle",
		Status:           projectOutputStatusCompleted,
		DisplayName:      "output.zip",
		MimeType:         "application/zip",
		FileSizeBytes:    &size,
		StorageReference: filePath,
		Retryable:        false,
	})
	if err != nil {
		t.Fatalf("upsert output: %v", err)
	}
	if output.Status != projectOutputStatusCompleted || output.DownloadURL == "" || !output.Available {
		t.Fatalf("unexpected completed output: %+v", output.contentProjectOutput)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/content-projects/"+project.id+"/outputs", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list outputs status = %d body = %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Outputs []contentProjectOutput `json:"outputs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed.Outputs) == 0 || listed.Outputs[0].DownloadURL == "" {
		t.Fatalf("listed output missing download URL: %+v", listed.Outputs)
	}

	req = httptest.NewRequest(http.MethodGet, listed.Outputs[0].DownloadURL, nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "zip" {
		t.Fatalf("download status = %d body = %q", rec.Code, rec.Body.String())
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatalf("remove output fixture: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/content-projects/"+project.id+"/outputs/"+output.ID, nil)
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get unavailable status = %d body = %s", rec.Code, rec.Body.String())
	}
	var unavailable contentProjectOutput
	if err := json.NewDecoder(rec.Body).Decode(&unavailable); err != nil {
		t.Fatalf("decode unavailable: %v", err)
	}
	if unavailable.Status != projectOutputStatusUnavailable || unavailable.DownloadURL != "" {
		t.Fatalf("missing file not represented honestly: %+v", unavailable)
	}

	var stage string
	if err := s.db.QueryRowContext(t.Context(), `SELECT current_stage FROM content_projects WHERE id = $1`, project.id).Scan(&stage); err != nil {
		t.Fatalf("read stage: %v", err)
	}
	if stage != "video" {
		t.Fatalf("stage = %q, want video", stage)
	}
}

func TestContentProjectOutputsValidationAndRetryRejection(t *testing.T) {
	s := testContentProjectOutputServer(t)
	projectA := createOutputTestProject(t, s, "script")
	projectB := createOutputTestProject(t, s, "script")
	sceneA := createOutputTestScene(t, s, projectA.id)

	_, err := s.upsertContentProjectOutput(t.Context(), projectA.workspaceID, projectB.id, &sceneA, contentProjectOutputMutation{
		OutputScope:    projectOutputScopeScene,
		OutputType:     projectOutputTypeGenerated,
		SourceWorkflow: projectOutputWorkflowClipGenerator,
		RenderJobID:    "cross-scene",
		Status:         projectOutputStatusProcessing,
		DisplayName:    "Bad scene",
	})
	if err == nil || !strings.Contains(err.Error(), "scene does not belong") {
		t.Fatalf("cross-project scene err = %v", err)
	}

	_, err = s.upsertContentProjectOutput(t.Context(), projectA.workspaceID, projectA.id, nil, contentProjectOutputMutation{
		OutputScope:    projectOutputScopeScene,
		OutputType:     projectOutputTypeGenerated,
		SourceWorkflow: projectOutputWorkflowClipGenerator,
		RenderJobID:    "missing-scene",
		Status:         projectOutputStatusProcessing,
		DisplayName:    "Missing scene",
	})
	if err == nil || !strings.Contains(err.Error(), "scene output requires") {
		t.Fatalf("missing scene err = %v", err)
	}

	failed, err := s.upsertContentProjectOutput(t.Context(), projectA.workspaceID, projectA.id, nil, contentProjectOutputMutation{
		OutputScope:    projectOutputScopeProject,
		OutputType:     projectOutputTypeGenerated,
		SourceWorkflow: projectOutputWorkflowClipGenerator,
		RenderJobID:    "non-retryable",
		Status:         projectOutputStatusFailed,
		DisplayName:    "Failed render",
		FailureMessage: "Renderer failed safely",
		Retryable:      false,
	})
	if err != nil {
		t.Fatalf("create failed output: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/content-projects/"+projectA.id+"/outputs/"+failed.ID+"/retry", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("retry non-retryable status = %d body = %s", rec.Code, rec.Body.String())
	}
	if failed.StorageReference != "" && strings.Contains(rec.Body.String(), failed.StorageReference) {
		t.Fatalf("retry response exposed storage reference: %s", rec.Body.String())
	}
}

type outputTestProject struct {
	id          string
	workspaceID string
}

func testContentProjectOutputServer(t *testing.T) *Server {
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
	return &Server{cfg: &config.Config{MediaOutputDir: filepath.Join(tmp, "media"), ExportDir: filepath.Join(tmp, "exports")}, db: db}
}

func createOutputTestProject(t *testing.T, s *Server, stage string) outputTestProject {
	t.Helper()
	workspaceID, err := s.defaultWorkspaceID(t.Context())
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	var id string
	err = s.db.QueryRowContext(t.Context(), `
		INSERT INTO content_projects (
			workspace_id, title, topic, source_type, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language
		)
		VALUES ($1, $2, 'Output tests', 'manual', 'draft', $3, ARRAY['youtube'], 'short_video', 30, 'en-US')
		RETURNING id`, workspaceID, "Output test "+stage+" "+t.Name(), stage).Scan(&id)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() { _, _ = s.db.ExecContext(t.Context(), `DELETE FROM content_projects WHERE id = $1`, id) })
	return outputTestProject{id: id, workspaceID: workspaceID}
}

func createOutputTestScene(t *testing.T, s *Server, projectID string) string {
	t.Helper()
	var id string
	err := s.db.QueryRowContext(t.Context(), `
		INSERT INTO content_project_scenes (project_id, position, title, spoken_text)
		VALUES ($1, 1, 'Scene one', 'Test scene')
		RETURNING id`, projectID).Scan(&id)
	if err != nil {
		t.Fatalf("create scene: %v", err)
	}
	return id
}
