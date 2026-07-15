package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"trendcortex/api/internal/audit"
	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
)

func TestProtectedAPIsRejectUnauthenticatedRequests(t *testing.T) {
	srv := NewServer(&config.Config{AppBase: "https://app.example.test", APIBase: "https://api.example.test", AppEnv: "production"}, nil, nil, nil)
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/movie-studio/edits", ""},
		{http.MethodPost, "/api/movie-studio/edits", `{}`},
		{http.MethodGet, "/api/movie-studio/edits/11111111-1111-1111-1111-111111111111", ""},
		{http.MethodPost, "/api/movie-studio/edits/11111111-1111-1111-1111-111111111111/render", `{}`},
		{http.MethodGet, "/api/movie-studio/renders/11111111-1111-1111-1111-111111111111", ""},
		{http.MethodGet, "/api/movie-studio/renders/11111111-1111-1111-1111-111111111111/download", ""},
		{http.MethodPost, "/api/movie-studio/uploads", ""},
		{http.MethodGet, "/api/assets", ""},
		{http.MethodGet, "/api/assets/11111111-1111-1111-1111-111111111111/download", ""},
		{http.MethodPost, "/api/voice-studio/previews", `{}`},
		{http.MethodPost, "/api/voice-studio/generations", `{}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.RemoteAddr = "203.0.113.20:1234"
		req.Host = "api.example.test"
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, body = %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestInvalidSessionRejected(t *testing.T) {
	srv := NewServer(&config.Config{AppBase: "https://app.example.test", APIBase: "https://api.example.test", AppEnv: "production"}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/movie-studio/edits", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Host = "api.example.test"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid"})
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	srv := NewServer(&config.Config{AppBase: "https://app.example.test", APIBase: "https://api.example.test", AppEnv: "production"}, db, nil, audit.New(db.DB))
	email := "auth-test-" + time.Now().Format("150405.000000000") + "@example.test"
	password := "correct horse battery staple"
	session, csrf := createAuthTestUserSession(t, srv, email, password)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.RemoteAddr = "203.0.113.20:1234"
	logoutReq.Host = "api.example.test"
	logoutReq.Header.Set("Origin", "https://app.example.test")
	logoutReq.Header.Set("X-CSRF-Token", csrf)
	logoutReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	logoutRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/movie-studio/edits", nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Host = "api.example.test"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUserCannotAccessAnotherWorkspaceMovieEdit(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Connect(databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	srv := NewServer(&config.Config{AppBase: "https://app.example.test", APIBase: "https://api.example.test", AppEnv: "production"}, db, nil, audit.New(db.DB))
	sessionA, _ := createAuthTestUserSession(t, srv, "owner-a-"+time.Now().Format("150405.000000000")+"@example.test", "password a")
	sessionB, _ := createAuthTestUserSession(t, srv, "owner-b-"+time.Now().Format("150405.000000000")+"@example.test", "password b")
	workspaceA := sessionWorkspace(t, srv, sessionA)
	var editID string
	if err := srv.db.QueryRowContext(t.Context(), `INSERT INTO movie_edits (workspace_id, name) VALUES ($1,'Private edit') RETURNING id::text`, workspaceA).Scan(&editID); err != nil {
		t.Fatalf("insert edit: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/movie-studio/edits/"+editID, nil)
	req.RemoteAddr = "203.0.113.20:1234"
	req.Host = "api.example.test"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionB})
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func createAuthTestUserSession(t *testing.T, srv *Server, email, password string) (string, string) {
	t.Helper()
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	var userID, workspaceID string
	err = srv.db.QueryRowContext(t.Context(), `INSERT INTO users (email, password_hash, workspace_id) VALUES ($1,$2,gen_random_uuid()) RETURNING id::text, workspace_id::text`, email, hash).Scan(&userID, &workspaceID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = srv.db.ExecContext(t.Context(), `INSERT INTO workspaces (id, name, owner_id) VALUES ($1,'Auth test workspace',$2) ON CONFLICT (id) DO NOTHING`, workspaceID, userID)
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	_, err = srv.db.ExecContext(t.Context(), `INSERT INTO workspace_memberships (workspace_id, user_id, role) VALUES ($1,$2,'owner') ON CONFLICT DO NOTHING`, workspaceID, userID)
	if err != nil {
		t.Fatalf("insert membership: %v", err)
	}
	session, err := randomToken(32)
	if err != nil {
		t.Fatalf("session token: %v", err)
	}
	csrf, err := randomToken(32)
	if err != nil {
		t.Fatalf("csrf token: %v", err)
	}
	_, err = srv.db.ExecContext(t.Context(), `INSERT INTO user_sessions (session_hash, user_id, workspace_id, csrf_hash, expires_at) VALUES ($1,$2,$3,$4,NOW() + INTERVAL '1 hour')`, tokenHash(session), userID, workspaceID, tokenHash(csrf))
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	t.Cleanup(func() {
		_, _ = srv.db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return session, csrf
}

func sessionWorkspace(t *testing.T, srv *Server, session string) string {
	t.Helper()
	var workspaceID string
	if err := srv.db.QueryRowContext(t.Context(), `SELECT workspace_id::text FROM user_sessions WHERE session_hash = $1`, tokenHash(session)).Scan(&workspaceID); err != nil {
		t.Fatalf("session workspace: %v", err)
	}
	return workspaceID
}
