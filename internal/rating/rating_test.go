package rating

import "testing"

// the Go ecosystem's default policy, mirrored here so the engine's math is exercised
// against realistic weights without the engine itself owning a feature catalog.
var (
	testWeights = map[string]int{
		"secrets": 20, "vulnerabilities": 20, "build": 20, "tests": 15,
		"lint": 20, "dependencies": 8, "docs": 5, "version_control": 5, "benchmarks": 0,
	}
	testCaps = map[string]int{"secrets": 40, "vulnerabilities": 72}
)

// ck builds a Check stamped with the Go ecosystem's default weight and cap for a
// feature, the way an ecosystem's WithRating registration would.
func ck(feature string, o Outcome) Check {
	c := Check{Feature: feature, Outcome: o, Weight: testWeights[feature]}
	if ceiling, ok := testCaps[feature]; ok {
		c.Cap, c.HasCap = ceiling, true
	}
	return c
}

func Test_Evaluate_Score(t *testing.T) {
	tests := []struct {
		name       string
		checks     []Check
		wantScore  int
		wantRating string
	}{
		{
			name:       "all pass is perfect",
			checks:     []Check{ck("build", Pass), ck("tests", Pass), ck("lint", Pass)},
			wantScore:  100,
			wantRating: "A+",
		},
		{
			name:       "nothing weighted grades perfect",
			checks:     []Check{ck("benchmarks", Fail)},
			wantScore:  100,
			wantRating: "A+",
		},
		{
			name:       "skipped checks are excluded",
			checks:     []Check{ck("build", Pass), ck("tests", Skip), ck("lint", Skip)},
			wantScore:  100,
			wantRating: "A+",
		},
		{
			name:       "minor failure barely dents an otherwise clean repo",
			checks:     []Check{ck("build", Pass), ck("tests", Pass), ck("lint", Pass), ck("dependencies", Pass), ck("version_control", Pass), ck("docs", Fail)},
			wantScore:  93,
			wantRating: "A",
		},
		{
			name:       "warning scores half weight",
			checks:     []Check{ck("build", Pass), ck("docs", Warn)},
			wantScore:  90,
			wantRating: "A-",
		},
		{
			name:       "broken build is heavy but not capped",
			checks:     []Check{ck("build", Fail), ck("tests", Pass), ck("lint", Pass), ck("dependencies", Pass), ck("docs", Pass), ck("version_control", Pass)},
			wantScore:  73,
			wantRating: "C",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.checks)
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
			checks:     []Check{ck("build", Pass), ck("tests", Pass), ck("lint", Pass), ck("secrets", Fail)},
			wantRating: "F",
		},
		{
			name:       "a reachable vulnerability caps an otherwise high score at C",
			checks:     []Check{ck("build", Pass), ck("tests", Pass), ck("lint", Pass), ck("dependencies", Pass), ck("docs", Pass), ck("version_control", Pass), ck("vulnerabilities", Fail)},
			wantRating: "C",
		},
		{
			name:       "the tightest cap wins",
			checks:     []Check{ck("build", Pass), ck("secrets", Fail), ck("vulnerabilities", Fail)},
			wantRating: "F",
		},
		{
			name:       "a passing secret check imposes no cap",
			checks:     []Check{ck("build", Pass), ck("tests", Pass), ck("secrets", Pass)},
			wantRating: "A+",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.checks)
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
		{want: "healthy", checkFn: func() []Check { return []Check{ck("build", Pass)} }},
		{want: "needs-attention", checkFn: func() []Check {
			return []Check{ck("build", Pass), ck("tests", Pass), ck("lint", Pass), ck("dependencies", Pass), ck("docs", Pass), ck("version_control", Pass), ck("vulnerabilities", Fail)}
		}},
		{want: "failing", checkFn: func() []Check { return []Check{ck("secrets", Fail), ck("build", Pass)} }},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := Evaluate(tt.checkFn())
			if got.Verdict != tt.want {
				t.Fatalf("verdict = %q (score %d), want %q", got.Verdict, got.Score, tt.want)
			}
		})
	}
}

// Test_Evaluate_Breakdown checks the per-check explanation: each graded check reports
// the points it cost the average, the biggest culprit sorts first, and a failing
// capped check flags the cap that actually lowered the grade as binding.
func Test_Evaluate_Breakdown(t *testing.T) {
	t.Run("deductions explain the weighted average and sort by impact", func(t *testing.T) {
		// lint (weight 20) fails, dependencies (weight 8) passes: total weight 28, raw
		// score 8*100/28 = 29, so lint costs 100-29 = 71 points of the final average.
		got := Evaluate([]Check{ck("lint", Fail), ck("dependencies", Pass)})
		if got.Score != 29 {
			t.Fatalf("score = %d, want 29", got.Score)
		}
		if len(got.Breakdown) != 2 {
			t.Fatalf("breakdown has %d entries, want 2", len(got.Breakdown))
		}
		first := got.Breakdown[0]
		if first.Feature != "lint" || first.Outcome != "fail" {
			t.Fatalf("biggest culprit = %s/%s, want lint/fail", first.Feature, first.Outcome)
		}
		if first.Deduction <= got.Breakdown[1].Deduction {
			t.Fatalf("breakdown not sorted by deduction: %v", got.Breakdown)
		}
		if first.Cap != nil {
			t.Fatalf("lint reported a cap %v, want none", first.Cap)
		}
	})

	t.Run("a failing capped check flags the binding ceiling", func(t *testing.T) {
		got := Evaluate([]Check{ck("build", Pass), ck("tests", Pass), ck("lint", Pass), ck("secrets", Fail)})
		if got.Score != 40 {
			t.Fatalf("score = %d, want 40 (secrets cap)", got.Score)
		}
		var secrets *Contribution
		for i := range got.Breakdown {
			if got.Breakdown[i].Feature == "secrets" {
				secrets = &got.Breakdown[i]
			}
		}
		if secrets == nil {
			t.Fatal("breakdown missing the secrets contribution")
		}
		if secrets.Cap == nil || *secrets.Cap != 40 || !secrets.Binding {
			t.Fatalf("secrets contribution = %+v, want cap 40 binding", secrets)
		}
	})

	t.Run("zero-weight feature excluded from the score", func(t *testing.T) {
		// docs weighted to 0 is informational: a docs failure no longer affects the grade.
		got := Evaluate([]Check{ck("build", Pass), {Feature: "docs", Outcome: Fail, Weight: 0}})
		if got.Score != 100 {
			t.Fatalf("score = %d, want 100 (docs excluded)", got.Score)
		}
		for _, c := range got.Breakdown {
			if c.Feature == "docs" {
				t.Fatalf("zero-weight docs leaked into the breakdown: %+v", c)
			}
		}
	})
}
