package http

import (
	"strings"
	"testing"
)

func TestContentProjectSceneValidation(t *testing.T) {
	valid := normalizeContentProjectSceneRequest(contentProjectSceneRequest{
		Position:               1,
		Title:                  "Hook",
		SpokenText:             "Stop losing production context.",
		OnScreenText:           "One connected workflow",
		VisualDirection:        "Creator at desk with project timeline.",
		BrollDirection:         "Quick cuts of research, script, and edit views.",
		CameraDirection:        "Medium close-up.",
		TransitionDirection:    "Cut on beat.",
		PlannedDurationSeconds: 6,
		ProductionNotes:        "Keep the opening direct.",
	})
	if err := validateContentProjectSceneRequest(valid); err != nil {
		t.Fatalf("valid scene rejected: %v", err)
	}

	cases := []struct {
		name string
		mut  func(contentProjectSceneRequest) contentProjectSceneRequest
		want string
	}{
		{name: "negative duration", mut: func(req contentProjectSceneRequest) contentProjectSceneRequest {
			req.PlannedDurationSeconds = -1
			return req
		}, want: "positive"},
		{name: "excessive duration", mut: func(req contentProjectSceneRequest) contentProjectSceneRequest {
			req.PlannedDurationSeconds = 601
			return req
		}, want: "600"},
		{name: "empty content", mut: func(req contentProjectSceneRequest) contentProjectSceneRequest {
			req.Title, req.SpokenText, req.OnScreenText, req.VisualDirection = "", "", "", ""
			req.BrollDirection, req.CameraDirection, req.TransitionDirection, req.ProductionNotes = "", "", "", ""
			return req
		}, want: "requires"},
		{name: "title too long", mut: func(req contentProjectSceneRequest) contentProjectSceneRequest {
			req.Title = strings.Repeat("x", 121)
			return req
		}, want: "maximum"},
		{name: "position out of range", mut: func(req contentProjectSceneRequest) contentProjectSceneRequest { req.Position = 41; return req }, want: "position"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContentProjectSceneRequest(tc.mut(valid))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestContentProjectSceneReorderValidation(t *testing.T) {
	valid := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	}
	if err := validateSceneIDList(valid); err != nil {
		t.Fatalf("valid reorder rejected: %v", err)
	}
	if err := validateSceneIDList([]string{valid[0], valid[0]}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate reorder not rejected: %v", err)
	}
	if err := validateSceneIDList([]string{"bad"}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("invalid id not rejected: %v", err)
	}
}

func TestGeneratedScenePlanNormalization(t *testing.T) {
	scenes, err := normalizeGeneratedScenePlan([]contentProjectSceneRequest{
		{Title: "Hook", SpokenText: "Open with the problem.", PlannedDurationSeconds: 90},
		{Title: "Payoff", SpokenText: "Show the connected workflow.", PlannedDurationSeconds: 90},
	}, 30)
	if err != nil {
		t.Fatalf("generated scene plan rejected: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("scene count = %d", len(scenes))
	}
	if scenes[0].Position != 1 || scenes[1].Position != 2 {
		t.Fatalf("positions not normalized: %+v", scenes)
	}
	total := scenes[0].PlannedDurationSeconds + scenes[1].PlannedDurationSeconds
	if total < 28 || total > 32 {
		t.Fatalf("duration not normalized toward target: total=%d scenes=%+v", total, scenes)
	}
}

func TestGeneratedScenePlanRejectsMalformedOutput(t *testing.T) {
	_, err := normalizeGeneratedScenePlan([]contentProjectSceneRequest{{Title: "", PlannedDurationSeconds: 5}}, 30)
	if err == nil {
		t.Fatal("malformed generated scene plan accepted")
	}
}
