package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// local-ai-worker-stub is a development/test-only fake worker. It creates
// obvious FFmpeg test-pattern clips so the production backend integration can
// be exercised without pretending real AI video generation is available.
type job struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Path   string `json:"-"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9099"
	}
	dir := filepath.Join(os.TempDir(), "trendcortex-local-ai-worker-stub")
	_ = os.MkdirAll(dir, 0750)
	var mu sync.Mutex
	jobs := map[string]job{}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "development/test-only fake worker"})
	})
	http.HandleFunc("/generate-scene", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := fmt.Sprintf("stub-%d", len(jobs)+1)
		path := filepath.Join(dir, id+".mp4")
		err := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc2=size=1080x1920:rate=30:duration=1", "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=44100", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", path).Run()
		status := "completed"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
		}
		mu.Lock()
		jobs[id] = job{ID: id, Status: status, Error: errMsg, Path: path}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(job{ID: id, Status: status, Error: errMsg})
	})
	http.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		id, output := parseJobPath(r.URL.Path)
		mu.Lock()
		j, ok := jobs[id]
		mu.Unlock()
		if !ok {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		if output {
			http.ServeFile(w, r, j.Path)
			return
		}
		_ = json.NewEncoder(w).Encode(j)
	})
	_ = http.ListenAndServe(":"+port, nil)
}

func parseJobPath(path string) (id string, output bool) {
	path = filepath.Clean(path)
	base := filepath.Base(path)
	if base == "output" {
		return filepath.Base(filepath.Dir(path)), true
	}
	return base, false
}
