package http

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "tc_session"
	csrfCookieName    = "tc_csrf"
	sessionTTL        = 30 * 24 * time.Hour
	pbkdf2Iterations  = 210000
)

type authContextKey struct{}

type authContext struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	WorkspaceID string `json:"workspace_id"`
	Role        string `json:"role"`
	CSRFToken   string `json:"csrf_token,omitempty"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleAuthRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/auth/"), "/")
	switch {
	case rest == "login" && r.Method == http.MethodPost:
		s.handleLogin(w, r)
	case rest == "logout" && r.Method == http.MethodPost:
		s.handleLogout(w, r)
	case rest == "me" && r.Method == http.MethodGet:
		auth, ok := s.authenticateRequest(w, r)
		if !ok {
			return
		}
		jsonOK(w, map[string]any{"user": auth})
	default:
		jsonErrorCode(w, "not_found", "auth route not found", http.StatusNotFound)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.originAllowed(r) {
		jsonErrorCode(w, "csrf_rejected", "request origin is not allowed", http.StatusForbidden)
		return
	}
	var req loginRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		jsonErrorCode(w, "invalid_credentials", "email or password is incorrect", http.StatusUnauthorized)
		return
	}
	user, err := s.lookupLoginUser(r.Context(), email)
	if errors.Is(err, sql.ErrNoRows) || !verifyPassword(req.Password, user.passwordHash) {
		jsonErrorCode(w, "invalid_credentials", "email or password is incorrect", http.StatusUnauthorized)
		return
	}
	if err != nil {
		jsonError(w, "login failed", http.StatusInternalServerError)
		return
	}
	sessionToken, err := randomToken(32)
	if err != nil {
		jsonError(w, "session could not be created", http.StatusInternalServerError)
		return
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		jsonError(w, "session could not be created", http.StatusInternalServerError)
		return
	}
	expires := time.Now().UTC().Add(sessionTTL)
	_, err = s.db.ExecContext(r.Context(), `
		INSERT INTO user_sessions (session_hash, user_id, workspace_id, csrf_hash, user_agent, ip_address, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tokenHash(sessionToken), user.userID, user.workspaceID, tokenHash(csrfToken), limitText(r.UserAgent(), 300), requestIP(r), expires)
	if err != nil {
		jsonError(w, "session could not be created", http.StatusInternalServerError)
		return
	}
	secure := s.secureCookies()
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: sessionToken, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: csrfToken, Path: "/", HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode, Expires: expires})
	jsonOK(w, map[string]any{"user": authContext{UserID: user.userID, Email: user.email, WorkspaceID: user.workspaceID, Role: user.role, CSRFToken: csrfToken}})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		_, _ = s.db.ExecContext(r.Context(), `UPDATE user_sessions SET revoked_at = NOW(), last_seen_at = NOW() WHERE session_hash = $1 AND revoked_at IS NULL`, tokenHash(c.Value))
	}
	secure := s.secureCookies()
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: "", Path: "/", HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	jsonOK(w, map[string]bool{"ok": true})
}

type loginUser struct {
	userID       string
	email        string
	passwordHash string
	workspaceID  string
	role         string
}

func (s *Server) lookupLoginUser(ctx context.Context, email string) (loginUser, error) {
	var u loginUser
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.email, u.password_hash, wm.workspace_id::text, wm.role
		FROM users u
		JOIN workspace_memberships wm ON wm.user_id = u.id
		WHERE lower(u.email) = $1
		ORDER BY CASE wm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'editor' THEN 2 ELSE 3 END, wm.created_at ASC
		LIMIT 1`, email).Scan(&u.userID, &u.email, &u.passwordHash, &u.workspaceID, &u.role)
	return u, err
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (authContext, bool) {
	if s.db == nil {
		jsonErrorCode(w, "unauthorized", "authentication required", http.StatusUnauthorized)
		return authContext{}, false
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		jsonErrorCode(w, "unauthorized", "authentication required", http.StatusUnauthorized)
		return authContext{}, false
	}
	var auth authContext
	var csrfHash string
	err = s.db.QueryRowContext(r.Context(), `
		SELECT u.id::text, u.email, us.workspace_id::text, wm.role, us.csrf_hash
		FROM user_sessions us
		JOIN users u ON u.id = us.user_id
		JOIN workspace_memberships wm ON wm.user_id = us.user_id AND wm.workspace_id = us.workspace_id
		WHERE us.session_hash = $1 AND us.revoked_at IS NULL AND us.expires_at > NOW()
		LIMIT 1`, tokenHash(c.Value)).Scan(&auth.UserID, &auth.Email, &auth.WorkspaceID, &auth.Role, &csrfHash)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "unauthorized", "authentication required", http.StatusUnauthorized)
		return authContext{}, false
	}
	if err != nil {
		jsonError(w, "session lookup failed", http.StatusInternalServerError)
		return authContext{}, false
	}
	auth.CSRFToken = csrfHash
	_, _ = s.db.ExecContext(r.Context(), `UPDATE user_sessions SET last_seen_at = NOW() WHERE session_hash = $1`, tokenHash(c.Value))
	return auth, true
}

func (s *Server) validCSRF(r *http.Request, expectedHash string) bool {
	if !s.originAllowed(r) {
		return false
	}
	token := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if token == "" {
		if c, err := r.Cookie(csrfCookieName); err == nil {
			token = strings.TrimSpace(c.Value)
		}
	}
	return token != "" && subtle.ConstantTimeCompare([]byte(tokenHash(token)), []byte(expectedHash)) == 1
}

func currentAuth(ctx context.Context) (authContext, bool) {
	auth, ok := ctx.Value(authContextKey{}).(authContext)
	return auth, ok
}

func (s *Server) secureCookies() bool {
	return s.cfg != nil && strings.EqualFold(s.cfg.AppEnv, "production")
}

func isStateChanging(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch || method == http.MethodPut || method == http.MethodDelete
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func requestIP(r *http.Request) string {
	if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xf != "" {
		return limitText(strings.TrimSpace(strings.Split(xf, ",")[0]), 80)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return limitText(r.RemoteAddr, 80)
}

func hashPassword(password string) (string, error) {
	salt, err := randomToken(16)
	if err != nil {
		return "", err
	}
	saltBytes, _ := hex.DecodeString(salt)
	hash := pbkdf2SHA256([]byte(password), saltBytes, pbkdf2Iterations, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", pbkdf2Iterations, salt, hex.EncodeToString(hash)), nil
}

func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iters, err := strconv.Atoi(parts[1])
	if err != nil || iters < 100000 {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iters, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	hLen := 32
	numBlocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, numBlocks*hLen)
	for block := 1; block <= numBlocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iter; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
