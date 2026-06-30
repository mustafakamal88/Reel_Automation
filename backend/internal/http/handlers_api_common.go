package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// defaultWorkspaceID returns the single workspace this Phase 4A pipeline
// operates against, creating it on first use.
//
// TODO(auth): once login/session handling lands, derive workspace_id from
// the authenticated session instead of a single shared default workspace.
func (s *Server) defaultWorkspaceID(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces ORDER BY created_at ASC LIMIT 1`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	err = s.db.QueryRowContext(ctx, `
		INSERT INTO workspaces (name, owner_id)
		VALUES ('Default Workspace', gen_random_uuid())
		RETURNING id`).Scan(&id)
	return id, err
}

// decodeJSONBody decodes a JSON request body into v. A missing/empty body
// is not an error — callers that allow an empty body should ignore io.EOF.
func decodeJSONBody(r *http.Request, v any) error {
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(v)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
