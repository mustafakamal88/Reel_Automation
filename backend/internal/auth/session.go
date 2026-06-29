package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const (
	sessionCookieName = "tc_session"
	sessionTTL        = 30 * 24 * time.Hour
)

// Session represents an authenticated server-side session.
// Tokens are stored server-side; the browser only holds the session ID cookie.
type Session struct {
	ID          string
	UserID      string
	WorkspaceID string
	ExpiresAt   time.Time
}

// SessionStore is an interface for session persistence.
// Implement with database or Redis in production.
type SessionStore interface {
	Create(userID, workspaceID string) (*Session, error)
	Get(sessionID string) (*Session, error)
	Delete(sessionID string) error
}

// SetCookie writes an HttpOnly, Secure, SameSite=Lax session cookie.
// Tokens are NEVER written to cookies — only the opaque session ID.
func SetCookie(w http.ResponseWriter, sessionID string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
}

// ClearCookie removes the session cookie.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// SessionIDFromRequest reads the session ID from the request cookie.
func SessionIDFromRequest(r *http.Request) (string, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", errors.New("auth: no session cookie")
	}
	if c.Value == "" {
		return "", errors.New("auth: empty session cookie")
	}
	return c.Value, nil
}

// GenerateID returns a cryptographically random 32-byte hex session ID.
func GenerateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
