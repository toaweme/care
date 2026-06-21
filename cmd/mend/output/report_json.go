package output

import (
	"time"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/internal/rating"
)

// RunInfo is the repo context the caller resolves once and the renderer stamps onto
// the report header: when the report was produced, the repo dir, its module /
// project identity, the version-control state, and the run's wall-clock cost. The
// module and VC identity are resolved at the caller (never from a payload).
type RunInfo struct {
	Created    time.Time
	Repo       string
	Module     string
	VC         *VCInfo
	DurationMs int64
}

// VCInfo is the repo's version-control identity for the report header: the current
// branch and commit, the total commit count on HEAD, whether the tree is dirty, the
// upstream sync state, when HEAD was committed, and when the working tree was last
// touched. TouchedAt is the newest uncommitted-file mtime, the repo-level "last
// worked on" signal; it is absent when the tree is clean.
type VCInfo struct {
	Branch      string     `json:"branch,omitempty"`
	Commit      string     `json:"commit,omitempty"`
	Commits     int        `json:"commits,omitempty"`
	Dirty       bool       `json:"dirty"`
	HasUpstream bool       `json:"has_upstream"`
	Ahead       int        `json:"ahead,omitempty"`
	Behind      int        `json:"behind,omitempty"`
	CommittedAt *time.Time `json:"committed_at,omitempty"`
	TouchedAt   *time.Time `json:"touched_at,omitempty"`
	// LinesAdded / LinesDeleted are the repo's total uncommitted line delta against
	// HEAD, the quick "how much is in flight" signal for a dashboard.
	LinesAdded   int `json:"lines_added,omitempty"`
	LinesDeleted int `json:"lines_deleted,omitempty"`
}

// ToolResult is one install-phase tool outcome: the resolved tool identity, its
// status, how it is provisioned (Source: brew / goinstall / builtin), and the short
// install state text.
type ToolResult struct {
	Tool       string `json:"tool"`
	Status     string `json:"status"`
	Source     string `json:"source,omitempty"`
	Install    string `json:"install,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// Check is one feature's result against the repo: its umbrella Type, the
// Feature (capability) checked, the Tool that ran it, the status, and the typed
// payload. Data is the single structured contract; any table a UI renders is derived
// from it. A consumer decodes Data by the Feature it is keyed to.
type Check struct {
	Type       string `json:"type"`
	Feature    string `json:"feature"`
	Profile    string `json:"profile,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Data       any    `json:"data,omitempty"`
}

// Report is mend's public JSON wire shape: a single repo's checks under a flat header,
// the shared install-phase tools, and a graded health headline. Author is the
// generating tool ("mend"); a version embeds in it ("mend:1.1") only if ever needed.
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

// buildJSON shapes the phase-tagged output stream into the wire format: install
// outputs become Tools, run outputs become Checks and feed the graded Health
// headline, and the repo header comes from the caller-resolved RunInfo.
func buildJSON(outputs []mend.Rendered, info RunInfo, grading rating.Config) Report {
	rep := Report{Author: "mend", Dir: info.Repo, Module: info.Module, VersionControl: info.VC, Checks: []Check{}}
	if !info.Created.IsZero() {
		rep.Created = info.Created.Format(time.RFC3339)
	}
	var runs []mend.Rendered
	for _, o := range outputs {
		if o.Phase() == mend.PhaseInstall {
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

func checkOf(o mend.Rendered) Check {
	return Check{
		Type:       typeOf(o.Feature()),
		Feature:    o.Feature(),
		Profile:    profileLabel(o.Profile()),
		Tool:       o.Tool(),
		Status:     o.Status().String(),
		DurationMs: o.DurationMs(),
		Data:       o.Data(),
	}
}

// profileLabel returns the run-profile name to display, or "" for the implicit
// default (an unnamed or "default" profile is the unlabeled one).
func profileLabel(name string) string {
	if name == "default" {
		return ""
	}
	return name
}

// typeOf groups a feature under its umbrella type for the wire (security|quality|
// tests|repo). This grouping is purely a wire concern, so it lives here and not in
// the core.
func typeOf(feature string) string {
	switch feature {
	case mend.FeatureSecrets, mend.FeatureVulnerabilities:
		return "security"
	case mend.FeatureBuild, mend.FeatureLint, mend.FeatureDependencies, mend.FeatureRuntime, mend.FeatureDocs, mend.FeatureFixer:
		return "quality"
	case mend.FeatureTests, mend.FeatureBenchmark:
		return "tests"
	case mend.FeatureVersionControl:
		return "repo"
	default:
		return ""
	}
}

// toolID renders a tool's identity for the tools array, embedding the resolved
// version when known ("betterleaks:8.18.2"), bare otherwise.
func toolID(name, version string) string {
	if version == "" {
		return name
	}
	return name + ":" + version
}

// sourceID maps a tool's Installer const onto the wire source value. go-install
// reads as "goinstall" on the wire; brew and builtin pass through.
func sourceID(installer string) string {
	switch installer {
	case string(mend.InstallerGo):
		return "goinstall"
	default:
		return installer
	}
}
