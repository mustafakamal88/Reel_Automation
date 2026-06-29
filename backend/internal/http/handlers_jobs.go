package http

import (
	"net/http"
	"strings"
)

// handleJobStatus returns the current status of a background job.
//
// GET /jobs/{jobID}
//
// Response 200: { "job_id": "...", "type": "zip|publish", "status": "queued|running|done|failed", ... }
func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if jobID == "" {
		jsonError(w, "job ID required", http.StatusBadRequest)
		return
	}

	// Production:
	//   SELECT id, status, error_message, completed_at FROM publish_jobs WHERE id = $1
	//   UNION ALL
	//   SELECT id, status, error_message, completed_at FROM download_zip_jobs WHERE id = $1
	//   Verify the job belongs to the authenticated workspace before returning.
	_ = jobID
	jsonError(w, "job status not yet implemented — query publish_jobs or download_zip_jobs by ID", http.StatusNotImplemented)
}
