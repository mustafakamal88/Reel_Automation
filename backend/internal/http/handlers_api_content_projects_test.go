package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
)

func TestContentProjectValidation(t *testing.T) {
	valid := normalizeContentProjectRequest(contentProjectRequest{
		Title:                 "Creator workflow",
		Topic:                 "Production systems",
		Status:                "draft",
		CurrentStage:          "script",
		TargetPlatforms:       []string{"youtube"},
		ContentFormat:         "short_video",
		TargetDurationSeconds: 30,
		Language:              "en-US",
	}, false)
	if err := validateContentProjectRequest(valid, false); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	cases := []struct {
		name string
		mut  func(contentProjectRequest) contentProjectRequest
		want string
	}{
		{name: "invalid status", mut: func(req contentProjectRequest) contentProjectRequest { req.Status = "raw"; return req }, want: "invalid project status"},
		{name: "invalid stage", mut: func(req contentProjectRequest) contentProjectRequest { req.CurrentStage = "raw"; return req }, want: "invalid project stage"},
		{name: "duration", mut: func(req contentProjectRequest) contentProjectRequest { req.TargetDurationSeconds = 1; return req }, want: "target_duration_seconds"},
		{name: "platform", mut: func(req contentProjectRequest) contentProjectRequest {
			req.TargetPlatforms = []string{"myspace"}
			return req
		}, want: "invalid target platform"},
		{name: "format", mut: func(req contentProjectRequest) contentProjectRequest { req.ContentFormat = "movie"; return req }, want: "invalid content format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContentProjectRequest(tc.mut(valid), false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLegacyContentProjectImportValidationAndKey(t *testing.T) {
	req := legacyContentProjectImportRequest{
		Title:           "Legacy idea",
		Topic:           "Creator systems",
		TargetPlatforms: []string{"yt", "tiktok"},
		Hook:            "Stop losing scripts",
		MainScript:      "A persistent project keeps every output connected.",
		Caption:         "Build the system.",
		Hashtags:        []string{"Creator", "#Workflow"},
		SavedAt:         "2026-07-14T10:00:00Z",
	}
	project, key, savedAt, err := normalizeLegacyContentProjectImport(req)
	if err != nil {
		t.Fatalf("legacy import rejected: %v", err)
	}
	if key == "" || savedAt == nil {
		t.Fatalf("missing key or savedAt: key=%q savedAt=%v", key, savedAt)
	}
	if project.Status != "draft" || project.CurrentStage != "script" {
		t.Fatalf("unexpected status/stage: %+v", project)
	}
	if project.TargetPlatforms[0] != "youtube" || project.Hashtags[0] != "creator" {
		t.Fatalf("legacy normalization failed: %+v", project)
	}
	_, key2, _, err := normalizeLegacyContentProjectImport(req)
	if err != nil {
		t.Fatalf("second legacy import rejected: %v", err)
	}
	if key != key2 {
		t.Fatalf("legacy import key not idempotent: %q != %q", key, key2)
	}
}

func TestContentProjectHandlersWithDatabase(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	srv := NewServer(&config.Config{}, db, nil, nil)

	created := postContentProject(t, srv, `{
		"title":"Test content project",
		"topic":"Persistent scripts",
		"target_platforms":["youtube","tiktok"],
		"content_format":"short_video",
		"target_duration_seconds":30,
		"language":"en-US"
	}`)
	if created.ID == "" || created.Status != "idea" {
		t.Fatalf("unexpected create response: %+v", created)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/content-projects", nil)
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/content-projects/"+created.ID, nil)
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", rec.Code, rec.Body.String())
	}

	updated := patchContentProject(t, srv, created.ID, `{
		"title":"Test content project",
		"topic":"Persistent scripts",
		"status":"draft",
		"current_stage":"script",
		"target_platforms":["youtube"],
		"content_format":"short_video",
		"target_duration_seconds":45,
		"language":"en-US",
		"hook":"Updated hook",
		"main_script":"Updated script"
	}`)
	if updated.Status != "draft" || updated.CurrentStage != "script" || updated.Hook != "Updated hook" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	legacyBody := `{
		"import_key":"test-legacy-idempotent-key",
		"title":"Legacy script",
		"topic":"Legacy topic",
		"target_platforms":["instagram"],
		"hook":"Legacy hook",
		"main_script":"Legacy script body",
		"caption":"Legacy caption",
		"hashtags":["legacy"],
		"platform_text":{"instagram":"Legacy platform text"},
		"saved_at":"2026-07-14T10:00:00Z"
	}`
	first := importLegacyProject(t, srv, legacyBody)
	second := importLegacyProject(t, srv, legacyBody)
	if first.ID != second.ID {
		t.Fatalf("legacy import duplicated project: %s != %s", first.ID, second.ID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/content-projects/"+created.ID, nil)
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/content-projects/not-a-real-id", nil)
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing project status = %d", rec.Code)
	}
}

func postContentProject(t *testing.T, srv *Server, body string) contentProject {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/content-projects", bytes.NewBufferString(body))
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got contentProject
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	return got
}

func patchContentProject(t *testing.T, srv *Server, id, body string) contentProject {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/content-projects/"+id, bytes.NewBufferString(body))
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got contentProject
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	return got
}

func importLegacyProject(t *testing.T, srv *Server, body string) contentProject {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/content-projects/import-legacy", bytes.NewBufferString(body))
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy import status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got contentProject
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode legacy import: %v", err)
	}
	return got
}
