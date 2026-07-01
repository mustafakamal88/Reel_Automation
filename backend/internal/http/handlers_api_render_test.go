package http

import (
	"testing"

	"trendcortex/api/internal/models"
	"trendcortex/api/internal/renderer"
)

func TestExportStatusForRenderResult(t *testing.T) {
	tests := []struct {
		name       string
		result     renderer.Result
		wantStatus string
		wantErr    bool
	}{
		{
			name:       "completed is ready",
			result:     renderer.Result{Status: renderer.StatusCompleted},
			wantStatus: models.ReelExportStatusReady,
			wantErr:    false,
		},
		{
			name:       "provider missing keeps artifacts missing",
			result:     renderer.Result{Status: renderer.StatusProviderNotConnected, Notes: "OPENAI_API_KEY is not configured"},
			wantStatus: models.ReelExportStatusArtifactMissing,
			wantErr:    true,
		},
		{
			name:       "renderer missing keeps artifacts missing",
			result:     renderer.Result{Status: renderer.StatusRendererNotAvailable, Notes: "FFmpeg is not available"},
			wantStatus: models.ReelExportStatusArtifactMissing,
			wantErr:    true,
		},
		{
			name:       "audio missing keeps artifacts missing",
			result:     renderer.Result{Status: renderer.StatusAudioArtifactMissing, Notes: "voiceover missing"},
			wantStatus: models.ReelExportStatusArtifactMissing,
			wantErr:    true,
		},
		{
			name:       "thumbnail missing keeps artifacts missing",
			result:     renderer.Result{Status: renderer.StatusThumbnailMissing, Notes: "thumbnail missing"},
			wantStatus: models.ReelExportStatusArtifactMissing,
			wantErr:    true,
		},
		{
			name:       "failed keeps artifacts missing",
			result:     renderer.Result{Status: renderer.StatusFailed, Notes: "ffmpeg failed"},
			wantStatus: models.ReelExportStatusArtifactMissing,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotErr := exportStatusForRenderResult(tc.result)
			if gotStatus != tc.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, tc.wantStatus)
			}
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("err present = %v, want %v", gotErr != nil, tc.wantErr)
			}
		})
	}
}
