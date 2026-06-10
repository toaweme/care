package rating

import "testing"

func Test_Evaluate_Score(t *testing.T) {
	tests := []struct {
		name       string
		checks     []Check
		wantScore  int
		wantRating string
	}{
		{
			name:       "all pass is perfect",
			checks:     []Check{{"build", Pass}, {"tests", Pass}, {"lint", Pass}},
			wantScore:  100,
			wantRating: "A+",
		},
		{
			name:       "nothing weighted grades perfect",
			checks:     []Check{{"benchmarks", Fail}},
			wantScore:  100,
			wantRating: "A+",
		},
		{
			name:       "skipped checks are excluded",
			checks:     []Check{{"build", Pass}, {"tests", Skip}, {"lint", Skip}},
			wantScore:  100,
			wantRating: "A+",
		},
		{
			name:       "minor failure barely dents an otherwise clean repo",
			checks:     []Check{{"build", Pass}, {"vet", Pass}, {"tests", Pass}, {"lint", Pass}, {"dependencies", Pass}, {"docs", Pass}, {"version_control", Pass}, {"format", Fail}},
			wantScore:  94,
			wantRating: "A",
		},
		{
			name:       "warning scores half weight",
			checks:     []Check{{"build", Pass}, {"docs", Warn}},
			wantScore:  90,
			wantRating: "A-",
		},
		{
			name:       "broken build is heavy but not capped",
			checks:     []Check{{"build", Fail}, {"vet", Pass}, {"tests", Pass}, {"lint", Pass}, {"dependencies", Pass}, {"docs", Pass}, {"version_control", Pass}, {"format", Pass}},
			wantScore:  74,
			wantRating: "C",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.checks, Default())
			if got.Score != tt.wantScore {
				t.Fatalf("score = %d, want %d", got.Score, tt.wantScore)
			}
			if got.Rating != tt.wantRating {
				t.Fatalf("rating = %q, want %q", got.Rating, tt.wantRating)
			}
		})
	}
}

func Test_Evaluate_Caps(t *testing.T) {
	tests := []struct {
		name       string
		checks     []Check
		wantRating string
	}{
		{
			name:       "a committed secret caps at F regardless of the rest",
			checks:     []Check{{"build", Pass}, {"tests", Pass}, {"lint", Pass}, {"secrets", Fail}},
			wantRating: "F",
		},
		{
			name:       "a reachable vulnerability caps an otherwise high score at C",
			checks:     []Check{{"build", Pass}, {"vet", Pass}, {"tests", Pass}, {"lint", Pass}, {"dependencies", Pass}, {"docs", Pass}, {"version_control", Pass}, {"format", Pass}, {"vulnerabilities", Fail}},
			wantRating: "C",
		},
		{
			name:       "the tightest cap wins",
			checks:     []Check{{"build", Pass}, {"secrets", Fail}, {"vulnerabilities", Fail}},
			wantRating: "F",
		},
		{
			name:       "a passing secret check imposes no cap",
			checks:     []Check{{"build", Pass}, {"tests", Pass}, {"secrets", Pass}},
			wantRating: "A+",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.checks, Default())
			if got.Rating != tt.wantRating {
				t.Fatalf("rating = %q (score %d), want %q", got.Rating, got.Score, tt.wantRating)
			}
		})
	}
}

func Test_Evaluate_Verdict(t *testing.T) {
	tests := []struct {
		want    string
		checkFn func() []Check
	}{
		{want: "healthy", checkFn: func() []Check { return []Check{{"build", Pass}} }},
		{want: "needs-attention", checkFn: func() []Check {
			return []Check{{"build", Pass}, {"vet", Pass}, {"tests", Pass}, {"lint", Pass}, {"dependencies", Pass}, {"docs", Pass}, {"version_control", Pass}, {"format", Pass}, {"vulnerabilities", Fail}}
		}},
		{want: "failing", checkFn: func() []Check { return []Check{{"secrets", Fail}, {"build", Pass}} }},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := Evaluate(tt.checkFn(), Default())
			if got.Verdict != tt.want {
				t.Fatalf("verdict = %q (score %d), want %q", got.Verdict, got.Score, tt.want)
			}
		})
	}
}

func Test_FromConfig_OverlaysDefaults(t *testing.T) {
	cfg := FromConfig(map[string]int{"docs": 0}, nil)
	if cfg.Weights["build"] != 20 {
		t.Fatalf("build weight = %d, want default 20", cfg.Weights["build"])
	}
	if cfg.Weights["docs"] != 0 {
		t.Fatalf("docs weight = %d, want overridden 0", cfg.Weights["docs"])
	}
	// docs now informational: a docs failure no longer affects the score.
	got := Evaluate([]Check{{"build", Pass}, {"docs", Fail}}, cfg)
	if got.Score != 100 {
		t.Fatalf("score = %d, want 100 (docs excluded)", got.Score)
	}
}
