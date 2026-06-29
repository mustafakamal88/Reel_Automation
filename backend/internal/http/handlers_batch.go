package http

import (
	"net/http"
	"strings"
)

// handleBatchRoute dispatches POST /batches/{batchID}/zip and POST /batches/{batchID}/publish.
func (s *Server) handleBatchRoute(w http.ResponseWriter, r *http.Request) {
	// Path: /batches/{batchID}/{action}
	trimmed := strings.TrimPrefix(r.URL.Path, "/batches/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		jsonError(w, "invalid batch path — expected /batches/{batchID}/zip or /batches/{batchID}/publish", http.StatusBadRequest)
		return
	}
	batchID := parts[0]
	action := parts[1]

	switch action {
	case "zip":
		s.batchZip(w, r, batchID)
	case "publish":
		s.batchPublish(w, r, batchID)
	default:
		jsonError(w, "unknown batch action: "+action+" (expected zip or publish)", http.StatusBadRequest)
	}
}

// batchZip queues a ZIP creation job for the given batch.
//
// POST /batches/{batchID}/zip
//
// Response 202: { "job_id": "...", "status": "queued" }
func (s *Server) batchZip(w http.ResponseWriter, _ *http.Request, batchID string) {
	// Production:
	//   1. Verify batchID exists and belongs to the authenticated workspace.
	//   2. INSERT INTO download_zip_jobs (batch_id, status) VALUES ($1, 'queued') RETURNING id.
	//   3. Return job_id so the client can poll GET /jobs/{jobID}.
	_ = batchID
	jsonError(w, "batch zip not yet implemented — worker queue required", http.StatusNotImplemented)
}

// batchPublish queues one publish job per video per connected platform.
//
// POST /batches/{batchID}/publish
//
// Response 202: { "job_ids": ["..."], "queued": N }
func (s *Server) batchPublish(w http.ResponseWriter, _ *http.Request, batchID string) {
	// Production:
	//   1. Verify batchID exists and belongs to the authenticated workspace.
	//   2. Confirm human_approved = TRUE on all video_assets in the batch.
	//   3. For each video_asset × each connected platform:
	//        INSERT INTO publish_jobs (video_asset_id, platform, status) RETURNING id.
	//   4. Return list of job_ids for polling.
	_ = batchID
	jsonError(w, "batch publish not yet implemented — human approval gate + worker queue required", http.StatusNotImplemented)
}
