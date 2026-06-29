package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// Action constants for structured audit entries.
const (
	ActionOAuthStart      = "oauth.start"
	ActionOAuthCallback   = "oauth.callback"
	ActionOAuthDisconnect = "oauth.disconnect"
	ActionOAuthRefresh    = "oauth.refresh"
	ActionBatchCreate     = "batch.create"
	ActionBatchPublish    = "batch.publish"
	ActionBatchZip        = "batch.zip"
	ActionVideoApprove    = "video.approve"
	ActionVideoReject     = "video.reject"
	ActionUserLogin       = "user.login"
	ActionUserLogout      = "user.logout"
)

// Entry is the data written to the audit_log table.
type Entry struct {
	WorkspaceID string
	UserID      *string
	Action      string
	Resource    string
	ResourceID  *string
	IPAddress   *string
	UserAgent   *string
	Metadata    map[string]any
}

// Logger writes audit entries to the database.
type Logger struct {
	db *sql.DB
}

// New creates an audit Logger backed by the provided database connection.
func New(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Log writes an audit entry. Errors are logged but not returned to callers
// so that an audit failure never breaks the primary user action.
func (l *Logger) Log(ctx context.Context, e Entry) {
	metaJSON, _ := json.Marshal(e.Metadata)

	_, _ = l.db.ExecContext(ctx, `
		INSERT INTO audit_log
			(workspace_id, user_id, action, resource, resource_id, ip_address, user_agent, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.WorkspaceID,
		e.UserID,
		e.Action,
		e.Resource,
		e.ResourceID,
		e.IPAddress,
		e.UserAgent,
		metaJSON,
		time.Now().UTC(),
	)
}

// IPFromRequest extracts the real client IP, respecting X-Forwarded-For
// set by Railway's reverse proxy.
func IPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
