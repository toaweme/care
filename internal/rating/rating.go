// Package rating turns a run's per-check outcomes into a single repo health grade:
// a 0-100 score, a letter rating, a coarse verdict, and a per-check breakdown of
// what moved the score. It is deliberately decoupled from the rest of care (it knows
// nothing of Output/Report types) and from any feature catalog: each Check carries
// its own weight and cap, so the grading policy lives at the ecosystem where the
// checks are registered, not in a central feature->weight map here.
package rating

import (
	"math"
	"sort"
)

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

// String returns the lowercase wire label for an outcome.
func (o Outcome) String() string {
	switch o {
	case Pass:
		return "pass"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	default:
		return "skip"
	}
}

// Check is one weighted input to the score: the feature that ran, how it fared, its
// weight in the average, and any score ceiling it imposes when it fails. Weight and
// Cap travel on the check itself (stamped from its ecosystem registration), so the
// engine grades without consulting an external policy map.
type Check struct {
	Feature string
	Outcome Outcome
	// Weight is the feature's relative importance; 0 (or negative) makes it
	// informational and excludes it from the score.
	Weight int
	// Cap is the worst score this check is allowed to leave standing when it fails,
	// active only when HasCap is set, so a genuine emergency (a committed secret)
	// cannot hide behind a good average.
	Cap    int
	HasCap bool
}

// Result is the computed grade: the numeric score, its letter rating, the coarse
// verdict tier, and the per-check breakdown explaining how the score was reached.
type Result struct {
	Score     int            `json:"score"`
	Rating    string         `json:"rating"`
	Verdict   string         `json:"verdict"`
	Breakdown []Contribution `json:"breakdown,omitempty"`
}

// Contribution is one graded check's effect on the score: its outcome, the weight it
// carried, the points it scored (Pass=100, Warn=50, Fail=0), and Deduction, the
// points it cost the final average versus a clean pass. A failing capped check also
// reports the Cap it imposed and whether that cap was Binding (the ceiling that
// actually lowered the grade). Sorted by Deduction so the biggest culprit reads first.
type Contribution struct {
	Feature   string  `json:"feature"`
	Outcome   string  `json:"outcome"`
	Weight    int     `json:"weight"`
	Points    float64 `json:"points"`
	Deduction float64 `json:"deduction"`
	Cap       *int    `json:"cap,omitempty"`
	Binding   bool    `json:"binding,omitempty"`
}

// Evaluate computes the grade as a weighted average of per-check scores (Pass=100,
// Warn=50, Fail=0), excluding skipped and zero-weight checks, then lowers it to the
// tightest ceiling any failing capped check imposes. With nothing weighted to grade,
// the score is a perfect 100 (nothing to flag). The returned Breakdown explains the
// result check by check.
func Evaluate(checks []Check) Result {
	var sum, total float64
	for _, c := range checks {
		if c.Weight <= 0 || c.Outcome == Skip {
			continue
		}
		sum += float64(c.Weight) * outcomeScore(c.Outcome)
		total += float64(c.Weight)
	}

	raw := 100
	if total > 0 {
		raw = int(math.Round(sum / total))
	}

	// the binding cap is the tightest ceiling a failing capped check imposes, applied
	// only when it actually lowers the weighted average.
	score := raw
	for _, c := range checks {
		if c.Outcome == Fail && c.HasCap && c.Cap < score {
			score = c.Cap
		}
	}

	return Result{
		Score:     score,
		Rating:    ratingFor(score),
		Verdict:   verdictFor(score),
		Breakdown: breakdown(checks, total, raw, score),
	}
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

// breakdown builds the per-check explanation: one Contribution per graded check, its
// Deduction the share of the 100-point scale it cost the average. A failing capped
// check carries its Cap, and the tightest cap that actually lowered the grade is
// flagged Binding. Sorted by Deduction descending, then feature for stability.
func breakdown(checks []Check, total float64, raw, score int) []Contribution {
	if total <= 0 {
		return nil
	}
	out := make([]Contribution, 0, len(checks))
	for _, c := range checks {
		if c.Weight <= 0 || c.Outcome == Skip {
			continue
		}
		points := outcomeScore(c.Outcome)
		contribution := Contribution{
			Feature:   c.Feature,
			Outcome:   c.Outcome.String(),
			Weight:    c.Weight,
			Points:    points,
			Deduction: round2((100 - points) * float64(c.Weight) / total),
		}
		if c.Outcome == Fail && c.HasCap {
			capValue := c.Cap
			contribution.Cap = &capValue
			contribution.Binding = score < raw && c.Cap == score
		}
		out = append(out, contribution)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Deduction != out[j].Deduction {
			return out[i].Deduction > out[j].Deduction
		}
		return out[i].Feature < out[j].Feature
	})
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

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
