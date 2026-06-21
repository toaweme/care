package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/internal/rating"
)

// RenderOptions controls how a run's outputs are rendered.
type RenderOptions struct {
	Verbosity int
	JSON      bool
	// ExpandInstall prints the per-tool install phase as its own section. Off by
	// default: the tool count is folded into the repo header instead.
	ExpandInstall bool
	// Grading is the health policy (weights + caps) the score is computed against.
	// The zero value grades with the built-in defaults.
	Grading rating.Config
}

// Render writes a run's outputs as JSON or a structured text report. It consumes
// the phase-tagged Rendered stream the runner produces (install outputs then run
// outputs); info carries the caller-resolved repo header for the JSON shape.
func Render(outputs []mend.Rendered, info RunInfo, opts RenderOptions) error {
	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(buildJSON(outputs, info, opts.Grading)); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
		return nil
	}
	renderPretty(outputs, info, opts)
	return nil
}

func renderPretty(outputs []mend.Rendered, info RunInfo, opts RenderOptions) {
	p := NewPretty()

	var installs, runs []mend.Rendered
	for _, o := range outputs {
		if o.Phase() == mend.PhaseInstall {
			installs = append(installs, o)
		} else {
			runs = append(runs, o)
		}
	}

	headerTools := len(installs)
	if opts.ExpandInstall && len(installs) > 0 {
		p.Section("install", DimStyle.Render(plural(len(installs), "tool", "tools")))
		width := labelWidth(installs, func(o mend.Rendered) string { return o.Tool() })
		for _, o := range installs {
			p.CheckRow(icon(o.Status()), o.Tool(), width, o.Summary(0))
		}
		p.Newline()
		headerTools = 0 // already shown as its own section
	}

	if len(runs) == 0 {
		return
	}
	health := buildHealth(runs, info.DurationMs, opts.Grading)
	p.Section(shortenRepo(runs[0].Dir()), repoMeta(health, headerTools)...)
	if vc := vcMeta(info.VC); vc != "" {
		p.SubHeader(vc)
	}
	renderChecks(p, runs, opts)
	p.Newline()
}

// repoMeta builds a repo header's meta segments: the graded score + rating, the
// colored pass/warn/fail/skip breakdown, the wall-clock duration, then the tool
// count (unless install was expanded into its own section).
func repoMeta(h Health, tools int) []string {
	meta := []string{gradeMeta(h), statusMeta(tally{h.OK, h.Warn, h.Fail, h.Skip})}
	if h.DurationMs > 0 {
		meta = append(meta, DimStyle.Render(formatDuration(h.DurationMs)))
	}
	if tools > 0 {
		meta = append(meta, DimStyle.Render(plural(tools, "tool", "tools")))
	}
	return meta
}

// gradeMeta renders the health grade as "B+ 88/100 needs-attention", colored by the
// verdict tier so the headline reads at a glance.
func gradeMeta(h Health) string {
	style := gradeStyle(h.Verdict)
	return style.Render(fmt.Sprintf("%s %d/100", h.Rating, h.Score)) + DimStyle.Render(" "+h.Verdict)
}

func gradeStyle(verdict string) lipgloss.Style {
	switch verdict {
	case "healthy":
		return OKStyle
	case "needs-attention":
		return WarnStyle
	default:
		return ErrorStyle
	}
}

// vcMeta renders the version-control identity line: branch, short commit, commit
// count, dirty/clean, unpushed delta, and how long ago the tree was last touched
// (dirty) or HEAD was committed (clean).
func vcMeta(vc *VCInfo) string {
	if vc == nil || vc.Branch == "" {
		return ""
	}
	parts := []string{vc.Branch}
	if vc.Commit != "" {
		parts = append(parts, vc.Commit)
	}
	if vc.Commits > 0 {
		parts = append(parts, plural(vc.Commits, "commit", "commits"))
	}
	if vc.Dirty {
		dirty := "dirty"
		if vc.LinesAdded != 0 || vc.LinesDeleted != 0 {
			dirty += fmt.Sprintf(" +%d -%d", vc.LinesAdded, vc.LinesDeleted)
		}
		parts = append(parts, dirty)
	} else {
		parts = append(parts, "clean")
	}
	if vc.HasUpstream && (vc.Ahead != 0 || vc.Behind != 0) {
		parts = append(parts, fmt.Sprintf("+%d -%d", vc.Ahead, vc.Behind))
	}
	// a dirty tree reads by when it was last touched (active work); a clean tree by
	// when it was last committed.
	if vc.Dirty && vc.TouchedAt != nil {
		parts = append(parts, "touched "+relativeTime(*vc.TouchedAt))
	} else if vc.CommittedAt != nil {
		parts = append(parts, relativeTime(*vc.CommittedAt))
	}
	return strings.Join(parts, DimStyle.Render(" · "))
}

// formatDuration renders a run's wall-clock as a compact human string.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// relativeTime renders how long ago t was, coarsely (the report header wants "2h
// ago", not a precise span).
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// tally counts run outcomes for a header or footer summary.
type tally struct{ ok, warn, fail, skip int }

// statusMeta renders a tally as a colored, comma-separated breakdown that names
// what each count is ("6 passed, 2 failed"), so the numbers are never ambiguous.
// Zero categories are omitted.
func statusMeta(t tally) string {
	var parts []string
	if t.ok > 0 {
		parts = append(parts, OKStyle.Render(fmt.Sprintf("%d passed", t.ok)))
	}
	if t.warn > 0 {
		parts = append(parts, WarnStyle.Render(plural(t.warn, "warning", "warnings")))
	}
	if t.fail > 0 {
		parts = append(parts, ErrorStyle.Render(fmt.Sprintf("%d failed", t.fail)))
	}
	if t.skip > 0 {
		parts = append(parts, DimStyle.Render(fmt.Sprintf("%d skipped", t.skip)))
	}
	if len(parts) == 0 {
		return DimStyle.Render("no checks")
	}
	return strings.Join(parts, DimStyle.Render(", "))
}

// plural renders a count with its singular or plural noun.
func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// renderChecks renders one repo's features (passing first, failures last). Each
// feature is one row labeled by its name; a composite feature's sub-checks expand
// into its item rows. Order is stable because the runner delivers outputs sorted
// before this re-sorts by status.
func renderChecks(p *Pretty, checks []mend.Rendered, opts RenderOptions) {
	width := labelWidth(checks, rowLabel)

	sortByStatus(checks)
	for _, o := range checks {
		renderCheck(p, o, width, opts)
	}
}

// featureLabel humanizes a feature id into its display label ("version_control" ->
// "version control").
func featureLabel(feature string) string {
	return strings.ReplaceAll(feature, "_", " ")
}

// rowLabel is a check's display label: its humanized feature, suffixed with the
// run-profile when it ran under a named (non-default) one, e.g. "tests (race)".
func rowLabel(o mend.Rendered) string {
	label := featureLabel(o.Feature())
	if p := profileLabel(o.Profile()); p != "" {
		return label + " (" + p + ")"
	}
	return label
}

// sortByStatus orders rows passing-first, failures last (nearest the footer),
// stable within each status so feature order is otherwise preserved.
func sortByStatus(outputs []mend.Rendered) {
	sort.SliceStable(outputs, func(i, j int) bool {
		return statusRank(outputs[i].Status()) < statusRank(outputs[j].Status())
	})
}

func renderCheck(p *Pretty, o mend.Rendered, width int, opts RenderOptions) {
	label := rowLabel(o)
	icn := icon(o.Status())
	summary := o.Summary(opts.Verbosity)

	if o.Status() == mend.StatusSkip {
		detail := "skipped"
		if summary != "" {
			detail = "skipped: " + summary
		}
		p.CheckRow(icn, label, width, detail)
		return
	}
	// the summary line always prints; expanded item rows follow beneath it. Passing
	// checks stay collapsed (summary only) at default verbosity and expand at -v.
	p.CheckRow(icn, label, width, summary)
	if o.Status() == mend.StatusOK && opts.Verbosity == 0 {
		return
	}
	rows := o.Rows(opts.Verbosity)
	if o.Status() == mend.StatusFail && o.Err() != nil && opts.Verbosity > 1 {
		rows = append(rows, []string{"err: " + o.Err().Error()})
	}
	p.ItemRows(rows)
}

// statusRank orders check rows for display: passing first, then skipped, warnings,
// and failures last (closest to the footer summary).
func statusRank(s mend.Status) int {
	switch s {
	case mend.StatusOK:
		return 0
	case mend.StatusSkip:
		return 1
	case mend.StatusWarn:
		return 2
	case mend.StatusFail:
		return 3
	default:
		return 4
	}
}

// icon returns the styled status glyph for a row.
func icon(s mend.Status) string {
	switch s {
	case mend.StatusOK:
		return OKStyle.Render("✓")
	case mend.StatusWarn:
		return WarnStyle.Render("!")
	case mend.StatusFail:
		return ErrorStyle.Render("✗")
	default:
		return DimStyle.Render("○")
	}
}

// labelWidth returns the longest label among outputs, for column alignment.
func labelWidth(outputs []mend.Rendered, label func(mend.Rendered) string) int {
	w := 0
	for _, o := range outputs {
		if n := len(label(o)); n > w {
			w = n
		}
	}
	return w
}

// shortenRepo returns the last 3 path segments of a repo path for display.
func shortenRepo(p string) string {
	parts := strings.Split(strings.TrimRight(p, "/"), "/")
	if len(parts) <= 3 {
		return p
	}
	return strings.Join(parts[len(parts)-3:], "/")
}

// Failures returns the number of failing outputs (commands use it for exit code).
func Failures(outputs []mend.Rendered) int {
	var n int
	for _, o := range outputs {
		if o.Status() == mend.StatusFail {
			n++
		}
	}
	return n
}
