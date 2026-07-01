package output

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/toaweme/care"
	"github.com/toaweme/care/internal/rating"
)

// RenderOptions controls how a run's outputs are rendered.
type RenderOptions struct {
	Verbosity int
	JSON      bool
	// ExpandInstall prints the per-tool install phase as its own section. Off by default: the
	// tool count is folded into the repo header instead.
	ExpandInstall bool
	// Explain prints the per-check grading breakdown beneath the report (what each
	// weighted feature cost the score, and any cap that lowered it). Off by default.
	Explain bool
}

// Render writes a run's outputs as JSON or a structured text report. It consumes the
// phase-tagged Rendered stream the runner produces (install outputs then run outputs); info
// carries the caller-resolved repo header for the JSON shape.
func Render(outputs []care.Rendered, info RunInfo, opts RenderOptions) error {
	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(buildJSON(outputs, info)); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
		return nil
	}
	renderPretty(outputs, info, opts)
	return nil
}

func renderPretty(outputs []care.Rendered, info RunInfo, opts RenderOptions) {
	p := NewPretty()

	var installs, runs []care.Rendered
	for _, o := range outputs {
		if o.Phase() == care.PhaseInstall {
			installs = append(installs, o)
		} else {
			runs = append(runs, o)
		}
	}

	headerTools := len(installs)
	if opts.ExpandInstall && len(installs) > 0 {
		p.Section("install", DimStyle.Render(plural(len(installs), "tool", "tools")))
		width := labelWidth(installs, func(o care.Rendered) string { return o.Tool() })
		for _, o := range installs {
			p.CheckRow(icon(o.Status()), durFmt(o.DurationMs()), o.Tool(), width, o.Summary(0))
		}
		p.Newline()
		headerTools = 0 // already shown as its own section
	}

	if len(runs) == 0 {
		return
	}
	health := buildHealth(runs, info.DurationMs)
	p.Section(shortenRepo(runs[0].Dir()), repoMeta(health, headerTools)...)
	if vc := vcMeta(info.VC); vc != "" {
		p.SubHeader(vc)
	}
	renderChecks(p, runs, opts)
	if opts.Explain {
		renderBreakdown(p, health, runs)
	}
	p.Newline()
}

// renderBreakdown prints the per-check grading explanation beneath the report: each
// weighted feature, the points it cost the score, and any cap that actually lowered
// the grade. It reads the breakdown the rating engine already computed, so it shows
// exactly what moved the headline number. Passing checks (zero deduction) are dropped
// so only what bumped the score down is listed. runs supplies each feature's duration,
// since rating.Contribution (built for the JSON/score contract) does not carry timing.
func renderBreakdown(p *Pretty, h Health, runs []care.Rendered) {
	var dents []rating.Contribution
	for _, c := range h.Breakdown {
		if c.Deduction > 0 || c.Cap != nil {
			dents = append(dents, c)
		}
	}
	if len(dents) == 0 {
		return
	}
	durByFeature := make(map[string]int64, len(runs))
	for _, o := range runs {
		durByFeature[o.Feature()] = o.DurationMs()
	}
	p.Newline()
	p.Section("grading", DimStyle.Render(fmt.Sprintf("%d/100", h.Score)))
	width := 0
	for _, c := range dents {
		if n := len(featureLabel(c.Feature)); n > width {
			width = n
		}
	}
	for _, c := range dents {
		detail := WarnStyle.Render(fmt.Sprintf("-%.1f", c.Deduction)) + DimStyle.Render(fmt.Sprintf(" · weight %d · %s", c.Weight, c.Outcome))
		if c.Cap != nil {
			ceiling := fmt.Sprintf(" · caps at %d", *c.Cap)
			if c.Binding {
				ceiling += " (binding)"
			}
			detail += ErrorStyle.Render(ceiling)
		}
		p.CheckRow(outcomeIcon(c.Outcome), durFmt(durByFeature[c.Feature]), featureLabel(c.Feature), width, detail)
	}
}

// outcomeIcon returns the styled status glyph for a grading outcome label.
func outcomeIcon(outcome string) string {
	switch outcome {
	case "fail":
		return ErrorStyle.Render("✗")
	case "warn":
		return WarnStyle.Render("!")
	default:
		return OKStyle.Render("✓")
	}
}

// repoMeta builds a repo header's meta segments: the graded score + rating, the colored
// pass/warn/fail/skip breakdown, the wall-clock duration, then the tool count (unless install
// was expanded into its own section).
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

// gradeMeta renders the health grade as "B+ 88/100 needs-attention", colored by the verdict
// tier so the headline reads at a glance.
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

// vcMeta renders the version-control identity line: branch, short commit, commit count,
// dirty/clean, unpushed delta, and how long ago the tree was last touched (dirty) or HEAD was
// committed (clean).
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
	// a dirty tree reads by when it was last touched (active work); a clean tree by when it
	// was last committed.
	if vc.Dirty && vc.TouchedAt != nil {
		parts = append(parts, "touched "+relativeTime(*vc.TouchedAt))
	} else if vc.CommittedAt != nil {
		parts = append(parts, relativeTime(*vc.CommittedAt))
	}
	return strings.Join(parts, DimStyle.Render(" · "))
}

// durFmt renders a single check's duration for the per-row duration column: tenths of a
// second below 1s ("0.4s"), whole seconds below a minute ("12s"), and minutes:seconds beyond
// that. Unlike formatDuration (the run's total wall-clock, which favors ms precision for sub-
// second runs), this rounds aggressively so every value fits the row's fixed-width column.
// Zero or negative (no timing, e.g. a skipped check) renders as "".
func durFmt(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 950 {
		tenths := int(math.Round(float64(ms) / 100))
		return fmt.Sprintf("0.%ds", tenths)
	}
	sec := int64(math.Round(float64(ms) / 1000))
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	return fmt.Sprintf("%dm%ds", sec/60, sec%60)
}

// formatDuration renders a run's wall-clock as a compact human string.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// relativeTime renders how long ago t was, coarsely (the report header wants "2h ago", not a
// precise span).
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

// statusMeta renders a tally as a colored, comma-separated breakdown that names what each
// count is ("6 passed, 2 failed"), so the numbers are never ambiguous. Zero categories are
// omitted.
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

// renderChecks renders one repo's features (passing first, failures last). Each feature is one
// row labeled by its name; a composite feature's sub-checks expand into its item rows. Order is
// stable because the runner delivers outputs sorted before this re-sorts by status.
func renderChecks(p *Pretty, checks []care.Rendered, opts RenderOptions) {
	width := labelWidth(checks, rowLabel)

	sortByStatus(checks)
	for _, o := range checks {
		renderCheck(p, o, width, opts)
	}
}

// featureLabel humanizes a feature id into its display label
// ("version_control" -> "version control").
func featureLabel(feature string) string {
	return strings.ReplaceAll(feature, "_", " ")
}

// rowLabel is a check's display label: its humanized feature, suffixed with the run-profile
// when it ran under a named (non-default) one, e.g. "tests (race)".
func rowLabel(o care.Rendered) string {
	label := featureLabel(o.Feature())
	if p := profileLabel(o.Profile()); p != "" {
		return label + " (" + p + ")"
	}
	return label
}

// sortByStatus orders rows passing-first, failures last (nearest the footer),
// stable within each status so feature order is otherwise preserved.
func sortByStatus(outputs []care.Rendered) {
	sort.SliceStable(outputs, func(i, j int) bool {
		return statusRank(outputs[i].Status()) < statusRank(outputs[j].Status())
	})
}

func renderCheck(p *Pretty, o care.Rendered, width int, opts RenderOptions) {
	label := rowLabel(o)
	icn := icon(o.Status())
	summary := o.Summary(opts.Verbosity)

	if o.Status() == care.StatusSkip {
		detail := "skipped"
		if summary != "" {
			detail = "skipped: " + summary
		}
		// a skipped check did no work, so its duration column stays blank rather than
		// showing a meaningless near-zero timing.
		p.CheckRow(icn, "", label, width, detail)
		return
	}
	// the summary line always prints; expanded item rows follow beneath it. Passing
	// checks stay collapsed (summary only) at default verbosity and expand at -v. Duration
	// shows unconditionally, independent of verbosity.
	p.CheckRow(icn, durFmt(o.DurationMs()), label, width, summary)
	if o.Status() == care.StatusOK && opts.Verbosity == 0 {
		return
	}
	rows := o.Rows(opts.Verbosity)
	// an errored check (a tool failure) carries no payload rows, so surface its
	// underlying error inline rather than hiding it behind -vv: "tool failed" on
	// its own is undiagnosable. Each line of a multi-line tool error is its own row.
	if o.Status() == care.StatusFail && o.Err() != nil {
		rows = append(rows, errorRows(o.Err())...)
	}
	p.ItemRows(rows)
}

// errorRows splits an errored check's underlying error into one item row per line,
// dropping blank lines, so a multi-line tool failure (a wrapped message plus the
// tool's own output) reads as a clean block beneath the summary.
func errorRows(err error) [][]string {
	var rows [][]string
	for _, line := range strings.Split(err.Error(), "\n") {
		if strings.TrimSpace(line) != "" {
			rows = append(rows, []string{strings.TrimRight(line, "\r")})
		}
	}
	return rows
}

// statusRank orders check rows for display: passing first, then skipped, warnings,
// and failures last (closest to the footer summary).
func statusRank(s care.Status) int {
	switch s {
	case care.StatusOK:
		return 0
	case care.StatusSkip:
		return 1
	case care.StatusWarn:
		return 2
	case care.StatusFail:
		return 3
	default:
		return 4
	}
}

// icon returns the styled status glyph for a row.
func icon(s care.Status) string {
	switch s {
	case care.StatusOK:
		return OKStyle.Render("✓")
	case care.StatusWarn:
		return WarnStyle.Render("!")
	case care.StatusFail:
		return ErrorStyle.Render("✗")
	default:
		return DimStyle.Render("○")
	}
}

// labelWidth returns the longest label among outputs, for column alignment.
func labelWidth(outputs []care.Rendered, label func(care.Rendered) string) int {
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
func Failures(outputs []care.Rendered) int {
	var n int
	for _, o := range outputs {
		if o.Status() == care.StatusFail {
			n++
		}
	}
	return n
}
