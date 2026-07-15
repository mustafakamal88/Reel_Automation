package voice

import (
	"strings"
	"testing"
)

func TestCatalogueIncludesVerifiedBuiltInVoices(t *testing.T) {
	voices := Catalogue()
	if len(voices) != 13 {
		t.Fatalf("expected 13 voices, got %d", len(voices))
	}
	for _, id := range []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer", "verse", "marin", "cedar"} {
		if _, ok := VoiceByID(id); !ok {
			t.Fatalf("missing voice %s", id)
		}
	}
}

func TestNormalizeTextPreservesParagraphs(t *testing.T) {
	in := "  First   line.\r\n\r\n\n  Second\tline.  "
	got := NormalizeText(in)
	want := "First line.\n\nSecond line."
	if got != want {
		t.Fatalf("NormalizeText() = %q, want %q", got, want)
	}
}

func TestChunksKeepAllTextInOrder(t *testing.T) {
	text := "One sentence. Two sentence.\n\nThree sentence. Four sentence."
	chunks := Chunks(text, 24)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %v", chunks)
	}
	joined := NormalizeText(strings.Join(chunks, " "))
	for _, phrase := range []string{"One sentence.", "Two sentence.", "Three sentence.", "Four sentence."} {
		if !strings.Contains(joined, phrase) {
			t.Fatalf("joined chunks lost %q: %q", phrase, joined)
		}
	}
}

func TestValidateRequestRejectsUnsupportedSettings(t *testing.T) {
	valid := Request{Text: "Hello world.", VoiceID: "marin", Speed: 1, Format: "mp3"}
	if err := ValidateRequest(valid); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}
	invalidVoice := valid
	invalidVoice.VoiceID = "missing"
	if err := ValidateRequest(invalidVoice); err != ErrInvalidVoice {
		t.Fatalf("expected ErrInvalidVoice, got %v", err)
	}
	badSpeed := valid
	badSpeed.Speed = 5
	if err := ValidateRequest(badSpeed); err == nil {
		t.Fatal("expected speed validation error")
	}
	badFormat := valid
	badFormat.Format = "zip"
	if err := ValidateRequest(badFormat); err == nil {
		t.Fatal("expected format validation error")
	}
}
