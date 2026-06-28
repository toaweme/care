package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/toaweme/care"
	"github.com/toaweme/care/internal/rating"
)

// RunInfo is the repo context the caller resolves once and the renderer stamps onto the report
// header: when the report was produced, the repo dir, its module / project identity, the
// version-control state, and the run's wall-clock cost. The module and VC identity are resolved
// at the caller (never from a payload).
type RunInfo struct {
	Created    time.Time
	Repo       string
	Module     string
	VC         *VCInfo
	DurationMs int64
}

// VCInfo is the repo's version-control identity for the report header: the current branch and
// commit, the total commit count on HEAD, whether the tree is dirty, the upstream sync state,
// when HEAD was committed, and when the working tree was last touched. TouchedAt is the newest
// uncommitted-file mtime, the repo-level "last worked on" signal; it is absent when the tree is
// clean.
type VCInfo struct {
	Branch string `json:"branch,omitempty"`
	// Tag is the release tag at HEAD, absent when HEAD is not tagged. It is the release
	// identity for a tagged CI build, where Branch is unavailable.
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
	// CommitFull is the full 40-char HEAD SHA; Commit is its short form.
	CommitFull  string     `json:"commit_full,omitempty"`
	Commits     int        `json:"commits,omitempty"`
	Dirty       bool       `json:"dirty"`
	HasUpstream bool       `json:"has_upstream"`
	Ahead       int        `json:"ahead,omitempty"`
	Behind      int        `json:"behind,omitempty"`
	CommittedAt *time.Time `json:"committed_at,omitempty"`
	TouchedAt   *time.Time `json:"touched_at,omitempty"`
	// LinesAdded / LinesDeleted are the repo's total uncommitted line delta against HEAD, the
	// quick "how much is in flight" signal for a dashboard.
	LinesAdded   int `json:"lines_added,omitempty"`
	LinesDeleted int `json:"lines_deleted,omitempty"`
}

// ToolResult is one install-phase tool outcome: the resolved tool identity, its status, how it
// is provisioned (Source: brew / goinstall / builtin), and the short install state text.
type ToolResult struct {
	Tool       string `json:"tool"`
	Status     string `json:"status"`
	Source     string `json:"source,omitempty"`
	Install    string `json:"install,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// Check is one feature's result against the repo: its umbrella Type, the Feature (capability)
// checked, the Tool that ran it, the status, and the typed payload. Data is the single
// structured contract; any table a UI renders is derived from it. A consumer decodes Data by
// the Feature it is keyed to.
type Check struct {
	Type       string `json:"type"`
	Feature    string `json:"feature"`
	Profile    string `json:"profile,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Data       any    `json:"data,omitempty"`
	// Error is the underlying failure detail for an errored check (a tool that
	// failed to run, with no payload). Absent for normal pass/fail outcomes, whose
	// detail lives in Data.
	Error string `json:"error,omitempty"`
}

// Report is care's public JSON wire shape: a single repo's checks under a flat header, the
// shared install-phase tools, and a graded health headline. Author is the generating tool
// ("care"); a version embeds in it ("care:1.1") only if ever needed.
type Report struct {
	Author         string       `json:"author"`
	Created        string       `json:"created,omitempty"`
	Dir            string       `json:"dir,omitempty"`
	Module         string       `json:"module,omitempty"`
	VersionControl *VCInfo      `json:"version_control,omitempty"`
	Health         Health       `json:"health"`
	Tools          []ToolResult `json:"tools,omitempty"`
	Checks         []Check      `json:"checks"`
}

// buildJSON shapes the phase-tagged output stream into the wire format: install outputs become
// Tools, run outputs become Checks and feed the graded Health headline, and the repo header
// comes from the caller-resolved RunInfo.
func buildJSON(outputs []care.Rendered, info RunInfo, grading rating.Config) Report {
	rep := Report{Author: "care", Dir: info.Repo, Module: info.Module, VersionControl: info.VC, Checks: []Check{}}
	if !info.Created.IsZero() {
		rep.Created = info.Created.Format(time.RFC3339)
	}
	var runs []care.Rendered
	for _, o := range outputs {
		if o.Phase() == care.PhaseInstall {
			rep.Tools = append(rep.Tools, ToolResult{
				Tool:       toolID(o.Tool(), o.Version()),
				Status:     o.Status().String(),
				Source:     sourceID(o.Source()),
				Install:    o.Summary(0),
				DurationMs: o.DurationMs(),
			})
			continue
		}
		rep.Checks = append(rep.Checks, checkOf(o))
		runs = append(runs, o)
	}
	rep.Health = buildHealth(runs, info.DurationMs, grading)
	return rep
}

// BuildReport shapes a run's outputs into the public JSON report. It is the exported entry
// point for callers that write the report themselves (e.g. to a file) rather than through
// Render.
func BuildReport(outputs []care.Rendered, info RunInfo, grading rating.Config) Report {
	return buildJSON(outputs, info, grading)
}

// ReadReport reads and decodes a previously written JSON report from path.
func ReadReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("failed to read report file: %w", err)
	}
	var rep Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return Report{}, fmt.Errorf("failed to decode report file: %w", err)
	}
	return rep, nil
}

// WriteReportFile encodes rep as indented JSON to path, replacing any existing file.
func WriteReportFile(path string, rep Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode report: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil { //nolint:gosec // the report is polled/served by external live-tracking tooling (possibly a different user) and carries no secrets
		return fmt.Errorf("failed to write report file: %w", err)
	}
	return nil
}

// AmendReport merges a fast incremental run's outputs into an existing report: it refreshes the
// caller-resolved header (created stamp, version_control), replaces each re-run check in place
// (matched by feature+profile, appended when new), and re-grades health from the merged check
// set. The promoted metrics, run duration and slowest check from the last full run are
// preserved, since a fast pass does not recompute them. The install-phase tools are left
// untouched.
func AmendReport(existing Report, outputs []care.Rendered, info RunInfo, grading rating.Config) Report {
	rep := existing
	if !info.Created.IsZero() {
		rep.Created = info.Created.Format(time.RFC3339)
	}
	if info.Repo != "" {
		rep.Dir = info.Repo
	}
	if info.Module != "" {
		rep.Module = info.Module
	}
	rep.VersionControl = info.VC

	for _, o := range outputs {
		if o.Phase() == care.PhaseInstall {
			continue
		}
		c := checkOf(o)
		if i := indexOfCheck(rep.Checks, c.Feature, c.Profile); i >= 0 {
			rep.Checks[i] = c
		} else {
			rep.Checks = append(rep.Checks, c)
		}
	}

	rep.Health = regrade(rep.Health, rep.Checks, grading)
	return rep
}

// indexOfCheck returns the index of the check matching feature+profile, or -1.
func indexOfCheck(checks []Check, feature, profile string) int {
	for i, c := range checks {
		if c.Feature == feature && c.Profile == profile {
			return i
		}
	}
	return -1
}

// regrade recomputes the status tally and the rating-engine grade from a merged set of wire
// checks, preserving the metrics/duration/slowest carried on prev (a fast pass does not
// recompute those). Each check's outcome is re-derived from its status string, the inverse of
// Status.String().
func regrade(prev Health, checks []Check, grading rating.Config) Health {
	h := prev
	h.OK, h.Warn, h.Fail, h.Skip = 0, 0, 0, 0
	rc := make([]rating.Check, 0, len(checks))
	for _, c := range checks {
		o := outcomeOf(c.Status)
		switch o {
		case rating.Pass:
			h.OK++
		case rating.Warn:
			h.Warn++
		case rating.Fail:
			h.Fail++
		case rating.Skip:
			h.Skip++
		}
		rc = append(rc, rating.Check{Feature: c.Feature, Outcome: o})
	}
	grade := rating.Evaluate(rc, grading)
	h.Score, h.Rating, h.Verdict = grade.Score, grade.Rating, grade.Verdict
	return h
}

// outcomeOf maps a wire status string back onto the rating engine's Outcome, the inverse of
// care.Status.String().
func outcomeOf(status string) rating.Outcome {
	switch status {
	case care.StatusOK.String():
		return rating.Pass
	case care.StatusWarn.String():
		return rating.Warn
	case care.StatusFail.String():
		return rating.Fail
	default:
		return rating.Skip
	}
}

func checkOf(o care.Rendered) Check {
	c := Check{
		Type:       typeOf(o.Feature()),
		Feature:    o.Feature(),
		Profile:    profileLabel(o.Profile()),
		Tool:       o.Tool(),
		Status:     o.Status().String(),
		DurationMs: o.DurationMs(),
		Data:       o.Data(),
	}
	if err := o.Err(); err != nil {
		c.Error = err.Error()
	}
	return c
}

// profileLabel returns the run-profile name to display, or "" for the implicit default (an
// unnamed or "default" profile is the unlabeled one).
func profileLabel(name string) string {
	if name == "default" {
		return ""
	}
	return name
}

// typeOf groups a feature under its umbrella type for the wire (security|quality|tests|repo).
// This grouping is purely a wire concern, so it lives here and not in the core.
func typeOf(feature string) string {
	switch feature {
	case care.FeatureSecrets, care.FeatureVulnerabilities:
		return "security"
	case care.FeatureBuild, care.FeatureLint, care.FeatureDependencies, care.FeatureRuntime, care.FeatureDocs, care.FeatureFixer:
		return "quality"
	case care.FeatureTests, care.FeatureBenchmark:
		return "tests"
	case care.FeatureVersionControl:
		return "repo"
	default:
		return ""
	}
}

// toolID renders a tool's identity for the tools array, embedding the resolved version when
// known ("betterleaks:8.18.2"), bare otherwise.
func toolID(name, version string) string {
	if version == "" {
		return name
	}
	return name + ":" + version
}

// sourceID maps a tool's Installer const onto the wire source value. go-install reads as
// "goinstall" on the wire; brew and builtin pass through.
func sourceID(installer string) string {
	switch installer {
	case string(care.InstallerGo):
		return "goinstall"
	default:
		return installer
	}
}
