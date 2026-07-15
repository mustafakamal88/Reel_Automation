package voice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultModel         = "gpt-4o-mini-tts"
	MaxInputRunes        = 12000
	MaxInstructionsRunes = 600
	MaxChunkRunes        = 3800
)

var (
	ErrMissingConfig = errors.New("voice provider is not configured")
	ErrInvalidVoice  = errors.New("voice is not available")
	ErrRateLimited   = errors.New("voice provider is temporarily busy")
	ErrUnauthorized  = errors.New("voice provider credentials are unavailable")
	ErrProvider      = errors.New("voice provider failed")
)

type Voice struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Character    string   `json:"character"`
	BestFor      []string `json:"best_for"`
	LanguageNote string   `json:"language_note"`
	Recommended  bool     `json:"recommended"`
}

type Request struct {
	Text         string
	VoiceID      string
	Speed        float64
	Instructions string
	Format       string
}

type Result struct {
	Audio       []byte
	MimeType    string
	Format      string
	ProviderRef string
	Model       string
}

type Provider interface {
	Voices() []Voice
	Generate(ctx context.Context, req Request) (Result, error)
}

func Catalogue() []Voice {
	return []Voice{
		{ID: "marin", DisplayName: "Marin", Character: "Clear, polished narration", BestFor: []string{"education", "explainers", "brand stories"}, LanguageNote: "Optimized for English", Recommended: true},
		{ID: "cedar", DisplayName: "Cedar", Character: "Grounded, steady delivery", BestFor: []string{"documentary", "finance", "analysis"}, LanguageNote: "Optimized for English", Recommended: true},
		{ID: "coral", DisplayName: "Coral", Character: "Bright and expressive", BestFor: []string{"shorts", "promos", "creator updates"}, LanguageNote: "Optimized for English"},
		{ID: "alloy", DisplayName: "Alloy", Character: "Balanced and neutral", BestFor: []string{"tutorials", "general narration"}, LanguageNote: "Optimized for English"},
		{ID: "ash", DisplayName: "Ash", Character: "Direct and composed", BestFor: []string{"news", "business", "commentary"}, LanguageNote: "Optimized for English"},
		{ID: "ballad", DisplayName: "Ballad", Character: "Warm and narrative", BestFor: []string{"storytelling", "essays"}, LanguageNote: "Optimized for English"},
		{ID: "echo", DisplayName: "Echo", Character: "Crisp and present", BestFor: []string{"product demos", "recaps"}, LanguageNote: "Optimized for English"},
		{ID: "fable", DisplayName: "Fable", Character: "Animated story voice", BestFor: []string{"character narration", "story formats"}, LanguageNote: "Optimized for English"},
		{ID: "nova", DisplayName: "Nova", Character: "Smooth and conversational", BestFor: []string{"lifestyle", "how-to", "social video"}, LanguageNote: "Optimized for English"},
		{ID: "onyx", DisplayName: "Onyx", Character: "Deep and deliberate", BestFor: []string{"documentary", "dramatic reads"}, LanguageNote: "Optimized for English"},
		{ID: "sage", DisplayName: "Sage", Character: "Calm and confident", BestFor: []string{"wellness", "education", "analysis"}, LanguageNote: "Optimized for English"},
		{ID: "shimmer", DisplayName: "Shimmer", Character: "Light and upbeat", BestFor: []string{"beauty", "travel", "social clips"}, LanguageNote: "Optimized for English"},
		{ID: "verse", DisplayName: "Verse", Character: "Expressive and rhythmic", BestFor: []string{"announcements", "creative narration"}, LanguageNote: "Optimized for English"},
	}
}

func VoiceByID(id string) (Voice, bool) {
	for _, v := range Catalogue() {
		if v.ID == id {
			return v, true
		}
	}
	return Voice{}, false
}

func NormalizeText(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	spaceRE := regexp.MustCompile(`[ \t]+`)
	blank := false
	for _, line := range lines {
		clean := strings.TrimSpace(spaceRE.ReplaceAllString(line, " "))
		if clean == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
				blank = true
			}
			continue
		}
		blank = false
		out = append(out, clean)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func SanitizeInstructions(value string) string {
	value = NormalizeText(value)
	value = strings.ReplaceAll(value, "<", "")
	value = strings.ReplaceAll(value, ">", "")
	if len([]rune(value)) > MaxInstructionsRunes {
		return string([]rune(value)[:MaxInstructionsRunes])
	}
	return value
}

func HashText(value string) string {
	sum := sha256.Sum256([]byte(NormalizeText(value)))
	return hex.EncodeToString(sum[:])
}

func EstimateDurationSeconds(text string, speed float64) float64 {
	words := len(strings.Fields(text))
	if speed <= 0 {
		speed = 1
	}
	return (float64(words) / 155.0) * 60.0 / speed
}

func ValidateRequest(req Request) error {
	req.Text = NormalizeText(req.Text)
	if req.Text == "" {
		return errors.New("voiceover text is required")
	}
	if len([]rune(req.Text)) > MaxInputRunes {
		return fmt.Errorf("voiceover text is too long; keep it under %d characters", MaxInputRunes)
	}
	if _, ok := VoiceByID(req.VoiceID); !ok {
		return ErrInvalidVoice
	}
	if req.Speed < 0.25 || req.Speed > 4 {
		return errors.New("speaking speed must be between 0.25 and 4.0")
	}
	switch req.Format {
	case "mp3", "opus", "aac", "flac", "wav", "pcm":
		return nil
	default:
		return errors.New("audio format is not supported")
	}
}

func Chunks(text string, maxRunes int) []string {
	text = NormalizeText(text)
	if maxRunes <= 0 {
		maxRunes = MaxChunkRunes
	}
	if len([]rune(text)) <= maxRunes {
		return []string{text}
	}
	paras := strings.Split(text, "\n\n")
	chunks := []string{}
	var cur strings.Builder
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	for _, para := range paras {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if len([]rune(para)) > maxRunes {
			for _, sentence := range splitSentences(para) {
				if cur.Len() > 0 && len([]rune(cur.String()+" "+sentence)) > maxRunes {
					flush()
				}
				if len([]rune(sentence)) > maxRunes {
					rs := []rune(sentence)
					for len(rs) > 0 {
						n := maxRunes
						if len(rs) < n {
							n = len(rs)
						}
						chunks = append(chunks, strings.TrimSpace(string(rs[:n])))
						rs = rs[n:]
					}
					continue
				}
				if cur.Len() > 0 {
					cur.WriteByte(' ')
				}
				cur.WriteString(sentence)
			}
			continue
		}
		add := para
		if cur.Len() > 0 {
			add = "\n\n" + add
		}
		if cur.Len() > 0 && len([]rune(cur.String()+add)) > maxRunes {
			flush()
			add = para
		}
		cur.WriteString(add)
	}
	flush()
	return chunks
}

func splitSentences(text string) []string {
	re := regexp.MustCompile(`([^.!?]+[.!?]+|[^.!?]+$)`)
	matches := re.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if clean := strings.TrimSpace(m); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

type OpenAIProvider struct {
	APIKey string
	Model  string
	Client *http.Client
}

func (p OpenAIProvider) Voices() []Voice { return Catalogue() }

func (p OpenAIProvider) Generate(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return Result{}, ErrMissingConfig
	}
	if p.Model == "" {
		p.Model = DefaultModel
	}
	if p.Client == nil {
		p.Client = http.DefaultClient
	}
	if err := ValidateRequest(req); err != nil {
		return Result{}, err
	}
	body := map[string]any{
		"model":           p.Model,
		"input":           NormalizeText(req.Text),
		"voice":           req.VoiceID,
		"response_format": req.Format,
		"speed":           req.Speed,
	}
	if strings.TrimSpace(req.Instructions) != "" {
		body["instructions"] = SanitizeInstructions(req.Instructions)
	}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := p.Client.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 50*1024*1024))
	if err != nil {
		return Result{}, fmt.Errorf("%w: invalid audio response", ErrProvider)
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return Result{}, ErrUnauthorized
	}
	if res.StatusCode == http.StatusTooManyRequests {
		return Result{}, ErrRateLimited
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Result{}, ErrProvider
	}
	if len(data) == 0 {
		return Result{}, fmt.Errorf("%w: empty audio response", ErrProvider)
	}
	return Result{Audio: data, MimeType: MimeType(req.Format), Format: req.Format, Model: p.Model, ProviderRef: fmt.Sprintf("speech-%d", time.Now().UnixNano())}, nil
}

func MimeType(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/ogg"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "pcm":
		return "audio/L16"
	default:
		return "application/octet-stream"
	}
}
