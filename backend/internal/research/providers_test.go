package research

import (
	"context"
	"strings"
	"testing"
)

func TestYouTubeProviderNotConfiguredIsHonest(t *testing.T) {
	provider := NewYouTubeProvider("", nil)

	status := provider.Status()
	if status.Status != StatusNotConfigured {
		t.Fatalf("Status = %q, want %q", status.Status, StatusNotConfigured)
	}
	if !strings.Contains(status.Message, "YOUTUBE_API_KEY") {
		t.Fatalf("Message = %q, want YOUTUBE_API_KEY guidance", status.Message)
	}

	result, err := provider.AnalyzeVideo(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != ErrNotConfigured {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	if result.Status != StatusNotConfigured {
		t.Fatalf("result.Status = %q, want %q", result.Status, StatusNotConfigured)
	}
	if len(result.Limitations) == 0 {
		t.Fatalf("result.Limitations is empty")
	}
}

func TestExtractYouTubeVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":       "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?si=abc":               "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ?start=1": "dQw4w9WgXcQ",
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
	}
	for input, want := range cases {
		got, err := ExtractYouTubeVideoID(input)
		if err != nil {
			t.Fatalf("ExtractYouTubeVideoID(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ExtractYouTubeVideoID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTikTokResearchStatusDoesNotReturnFakeActiveData(t *testing.T) {
	status := TikTokResearchStatus("", "", "")
	if status.Status != StatusNotConfigured {
		t.Fatalf("Status = %q, want %q", status.Status, StatusNotConfigured)
	}

	configured := TikTokResearchStatus("client", "secret", "")
	if configured.Status != StatusUnavailable {
		t.Fatalf("configured Status = %q, want %q", configured.Status, StatusUnavailable)
	}
	if strings.Contains(strings.ToLower(configured.Message), "active") {
		t.Fatalf("configured Message = %q should not imply active fake trend data", configured.Message)
	}
}
