package scoring

import "testing"

func TestScoreIsDeterministic(t *testing.T) {
	in := Input{
		Topic:            "5 morning habits that boost focus",
		Description:      "A quick how-to breakdown for productivity-minded viewers.",
		PlatformHint:     "tiktok",
		Velocity:         72,
		SourceConfidence: 0.8,
	}

	first := Score(in)
	second := Score(in)

	if first.TotalScore != second.TotalScore {
		t.Fatalf("expected deterministic score, got %v then %v", first.TotalScore, second.TotalScore)
	}
	if first.TotalScore <= 0 || first.TotalScore > 100 {
		t.Fatalf("expected score in (0, 100], got %v", first.TotalScore)
	}
}

func TestScoreRewardsHigherVelocityAndConfidence(t *testing.T) {
	low := Score(Input{Topic: "generic topic idea", PlatformHint: "x", Velocity: 10, SourceConfidence: 0.2})
	high := Score(Input{Topic: "generic topic idea", PlatformHint: "x", Velocity: 90, SourceConfidence: 0.9})

	if high.TotalScore <= low.TotalScore {
		t.Fatalf("expected higher velocity/confidence to score higher: low=%v high=%v", low.TotalScore, high.TotalScore)
	}
}

func TestScorePlatformFitFavorsShortFormPlatforms(t *testing.T) {
	tiktok := Score(Input{Topic: "topic", PlatformHint: "tiktok", Velocity: 50, SourceConfidence: 0.5})
	unknown := Score(Input{Topic: "topic", PlatformHint: "myspace", Velocity: 50, SourceConfidence: 0.5})

	if tiktok.PlatformFitScore <= unknown.PlatformFitScore {
		t.Fatalf("expected tiktok platform fit to beat an unrecognized platform: tiktok=%v unknown=%v",
			tiktok.PlatformFitScore, unknown.PlatformFitScore)
	}
}

func TestScoreFlagsUnsafeContent(t *testing.T) {
	safe := Score(Input{Topic: "fun recipe ideas for dinner", Velocity: 50, SourceConfidence: 0.5})
	unsafe := Score(Input{Topic: "gambling tips and tricks", Velocity: 50, SourceConfidence: 0.5})

	if unsafe.Flagged != true {
		t.Fatalf("expected gambling topic to be flagged")
	}
	if safe.Flagged {
		t.Fatalf("expected safe topic to not be flagged")
	}
	if unsafe.SafetyScore >= safe.SafetyScore {
		t.Fatalf("expected flagged topic to have a lower safety score: safe=%v unsafe=%v", safe.SafetyScore, unsafe.SafetyScore)
	}
	if unsafe.TotalScore >= safe.TotalScore {
		t.Fatalf("expected flagged topic to have a lower total score: safe=%v unsafe=%v", safe.TotalScore, unsafe.TotalScore)
	}
}

func TestScoreWithinBounds(t *testing.T) {
	cases := []Input{
		{Topic: "", Velocity: 0, SourceConfidence: 0},
		{Topic: "extremely long topic phrase with many specific descriptive words here", Velocity: 100, SourceConfidence: 1},
		{Topic: "violence and gore and hate and nsfw", Velocity: 100, SourceConfidence: 1},
	}
	for _, in := range cases {
		r := Score(in)
		for name, v := range map[string]float64{
			"total": r.TotalScore, "velocity": r.VelocityScore, "confidence": r.SourceConfidenceScore,
			"platform": r.PlatformFitScore, "safety": r.SafetyScore, "watchTime": r.WatchTimeScore,
			"competition": r.CompetitionScore,
		} {
			if v < 0 || v > 100 {
				t.Fatalf("%s score out of bounds: %v (input: %+v)", name, v, in)
			}
		}
	}
}
