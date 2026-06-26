// Package rating turns a run's per-check outcomes into a single repo health grade:
// a 0-100 score, a letter rating, and a coarse verdict. It is deliberately
// decoupled from the rest of care (it knows nothing of Output/Report types), so any
// ecosystem can grade the same way by handing it a flat list of Checks.
package rating

import "math"

// Outcome is one check's result, mirrored here so the rating engine carries no
// dependency on the core care types.
type Outcome int

// The outcomes a single check can report, ordered best to worst.
const (
	Pass Outcome = iota
	Warn
	Fail
	Skip
)

// Check is one weighted input to the score: the feature that ran and how it fared.
// The feature string keys both its weight and any score cap.
type Check struct {
	Feature string
	Outcome Outcome
}

// Config is the operator-tunable grading policy: how much each feature counts and
// the worst-case score a failing feature is allowed to leave standing.
type Config struct {
	// Weights is the relative importance of each feature (a feature absent or set to
	// 0 is informational and excluded from the score, e.g. benchmarks).
	Weights map[string]int
	// Caps maps a feature to the maximum score allowed when that feature fails, so a
	// genuine emergency (a committed secret) cannot hide behind a good average. A
	// feature with no cap relies on its weight alone.
	Caps map[string]int
}

// Result is the computed grade: the numeric score, its letter rating, and the
// coarse verdict tier the letter falls into.
type Result struct {
	Score   int    `json:"score"`
	Rating  string `json:"rating"`
	Verdict string `json:"verdict"`
}

// DefaultWeights is the built-in feature importance: security and a broken build
// dominate, style and docs are minor, benchmarks are informational (weight 0).
func DefaultWeights() map[string]int {
	return map[string]int{
		"secrets":         20,
		"vulnerabilities": 20,
		"build":           20,
		"tests":           15,
		"lint":            20, // static analysis: golangci-lint, or the go vet + gofmt fallback
		"dependencies":    8,
		"docs":            5,
		"version_control": 5,
		"benchmarks":      0,
	}
}

// DefaultCaps is the built-in score ceiling for a failing critical feature: a
// committed secret caps at F (a live exposure, bad regardless of dev state); a
// reachable vulnerability caps at C (real, but often transitive and not instantly
// fixable). A broken build deliberately has no cap, so a transient non-compiling
// module in active development is only weighted, not graded as a failure.
func DefaultCaps() map[string]int {
	return map[string]int{
		"secrets":         40,
		"vulnerabilities": 72,
	}
}

// Default returns the built-in grading policy.
func Default() Config {
	return Config{Weights: DefaultWeights(), Caps: DefaultCaps()}
}

// FromConfig starts from the built-in policy and overlays any operator-provided
// weights and caps key-by-key, so a config that tweaks one weight keeps the
// defaults for the rest.
func FromConfig(weights, caps map[string]int) Config {
	cfg := Default()
	for k, v := range weights {
		cfg.Weights[k] = v
	}
	for k, v := range caps {
		cfg.Caps[k] = v
	}
	return cfg
}

// Evaluate computes the grade as a weighted average of per-check scores (Pass=100,
// Warn=50, Fail=0), excluding skipped and zero-weight checks, then applies any cap
// a failing critical feature triggers. With nothing weighted to grade, the score is
// a perfect 100 (nothing to flag).
func Evaluate(checks []Check, cfg Config) Result {
	weights := cfg.Weights
	if weights == nil {
		weights = DefaultWeights()
	}

	var sum, total float64
	for _, c := range checks {
		w := weights[c.Feature]
		if w <= 0 || c.Outcome == Skip {
			continue
		}
		sum += float64(w) * outcomeScore(c.Outcome)
		total += float64(w)
	}

	score := 100
	if total > 0 {
		score = int(math.Round(sum / total))
	}
	score = applyCaps(score, checks, cfg.Caps)

	return Result{Score: score, Rating: ratingFor(score), Verdict: verdictFor(score)}
}

func outcomeScore(o Outcome) float64 {
	switch o {
	case Pass:
		return 100
	case Warn:
		return 50
	default: // Fail
		return 0
	}
}

// applyCaps lowers the score to the tightest ceiling any failing capped feature
// imposes, so the worst critical failure wins.
func applyCaps(score int, checks []Check, caps map[string]int) int {
	if caps == nil {
		return score
	}
	for _, c := range checks {
		if c.Outcome != Fail {
			continue
		}
		if capValue, ok := caps[c.Feature]; ok && score > capValue {
			score = capValue
		}
	}
	return score
}

// ratingFor maps a score onto its letter grade.
func ratingFor(score int) string {
	switch {
	case score >= 97:
		return "A+"
	case score >= 93:
		return "A"
	case score >= 90:
		return "A-"
	case score >= 87:
		return "B+"
	case score >= 83:
		return "B"
	case score >= 80:
		return "B-"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// verdictFor maps a score onto its coarse tier, aligned to the letter bands: A/B
// healthy, C/D needs-attention, F failing.
func verdictFor(score int) string {
	switch {
	case score >= 80:
		return "healthy"
	case score >= 60:
		return "needs-attention"
	default:
		return "failing"
	}
}
