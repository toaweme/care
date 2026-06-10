package output

import (
	"github.com/toaweme/mend"
	"github.com/toaweme/mend/internal/rating"
)

// Health is the run's headline: a graded score + letter rating on top of the raw
// status tally, the run's wall-clock cost and its bottleneck, and the key metrics
// promoted out of individual check payloads so a consumer reads them off the top
// without decoding each check's data.
type Health struct {
	Score   int    `json:"score"`
	Rating  string `json:"rating"`
	Verdict string `json:"verdict"`

	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`

	DurationMs int64    `json:"duration_ms,omitempty"`
	Slowest    *Slowest `json:"slowest,omitempty"`

	Metrics Metrics `json:"metrics"`
}

// Slowest is the single longest-running check, the run's bottleneck. Checks run in
// parallel, so this is the wall-clock floor, not the sum of all check durations.
type Slowest struct {
	Feature    string `json:"feature"`
	DurationMs int64  `json:"duration_ms"`
}

// Metrics are the health numbers lifted from individual check payloads to the
// header, where a dashboard reads them directly instead of decoding each check.
type Metrics struct {
	Coverage *float64    `json:"coverage,omitempty"`
	Vulns    int         `json:"vulns"`
	Secrets  int         `json:"secrets"`
	Issues   int         `json:"issues"`
	Tests    *TestMetric `json:"tests,omitempty"`
}

// TestMetric is the per-test-function tally promoted from the test report.
type TestMetric struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Total  int `json:"total"`
}

// buildHealth rolls the run-phase outputs into the health headline: the status
// tally, the rating-engine grade, the slowest check, and the promoted metrics.
// durationMs is the run's wall-clock, measured by the caller around the runner.
func buildHealth(runs []mend.Rendered, durationMs int64, grading rating.Config) Health {
	h := Health{DurationMs: durationMs}

	checks := make([]rating.Check, 0, len(runs))
	for _, o := range runs {
		switch o.Status() {
		case mend.StatusOK:
			h.OK++
		case mend.StatusWarn:
			h.Warn++
		case mend.StatusFail:
			h.Fail++
		case mend.StatusSkip:
			h.Skip++
		}
		checks = append(checks, rating.Check{Feature: o.Feature(), Outcome: outcome(o.Status())})
		if h.Slowest == nil || o.DurationMs() > h.Slowest.DurationMs {
			h.Slowest = &Slowest{Feature: o.Feature(), DurationMs: o.DurationMs()}
		}
		h.accrueMetrics(o)
	}

	grade := rating.Evaluate(checks, grading)
	h.Score, h.Rating, h.Verdict = grade.Score, grade.Rating, grade.Verdict
	return h
}

// accrueMetrics lifts the headline numbers off one check's typed payload.
func (h *Health) accrueMetrics(o mend.Rendered) {
	switch d := o.Data().(type) {
	case mend.TestReport:
		c := d.Cases
		h.Metrics.Tests = &TestMetric{Passed: c.Passed, Failed: c.Failed, Total: c.Passed + c.Failed + c.Skipped}
		if d.WithCoverage {
			cov := d.Total
			h.Metrics.Coverage = &cov
		}
	case mend.SecretReport:
		h.Metrics.Secrets += len(d.Findings)
	case mend.VulnReport:
		h.Metrics.Vulns += len(d.Findings)
	case mend.BuildReport:
		h.Metrics.Issues += len(d.Errors)
	case mend.VetReport:
		h.Metrics.Issues += len(d.Issues)
	case mend.FormatReport:
		h.Metrics.Issues += len(d.Files)
	case mend.QualityReport:
		h.Metrics.Issues += len(d.Issues)
	case mend.DepsReport:
		h.Metrics.Issues += len(d.Issues)
	}
}

// outcome maps a core Status onto the rating engine's Outcome.
func outcome(s mend.Status) rating.Outcome {
	switch s {
	case mend.StatusOK:
		return rating.Pass
	case mend.StatusWarn:
		return rating.Warn
	case mend.StatusFail:
		return rating.Fail
	default:
		return rating.Skip
	}
}
