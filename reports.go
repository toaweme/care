package mend

import (
	"fmt"
	"sort"
	"strings"
)

// This file is the wire contract and the living documentation of what mend checks:
// one payload type per feature, language-agnostic, filled by an ecosystem's
// features. Each implements Report, so it is both the JSON data and the source of
// its own terminal presentation.

// VCReport is the version-control state: uncommitted files plus how far the branch
// is ahead of / behind its upstream.
type VCReport struct {
	Files       []RepoFile `json:"files,omitempty"`
	HasUpstream bool       `json:"has_upstream"`
	Ahead       int        `json:"ahead,omitempty"`
	Behind      int        `json:"behind,omitempty"`
}

// RepoFile is one changed file in the working tree: a short status word and the path.
type RepoFile struct {
	Status string `json:"status"`
	Name   string `json:"name"`
}

func (r VCReport) Summary(int) string {
	var parts []string
	if len(r.Files) > 0 {
		parts = append(parts, fmt.Sprintf("%d uncommitted", len(r.Files)))
	}
	if r.HasUpstream && (r.Ahead != 0 || r.Behind != 0) {
		parts = append(parts, fmt.Sprintf("unpushed +%d -%d", r.Ahead, r.Behind))
	}
	if !r.HasUpstream {
		parts = append(parts, "no upstream")
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, ", ")
}

func (r VCReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Files))
	for _, file := range r.Files {
		rows = append(rows, []string{file.Status, file.Name})
	}
	return rows
}

// QualityReport is the linter findings for one repo.
type QualityReport struct {
	Issues []QualityIssue `json:"issues,omitempty"`
}

// QualityIssue is one structured linter finding.
type QualityIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Col      int    `json:"col,omitempty"`
	Linter   string `json:"linter,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
}

func (r QualityReport) Summary(int) string {
	if len(r.Issues) == 0 {
		return "no issues"
	}
	return plural(len(r.Issues), "issue", "issues")
}

// Rows lists each issue as file/location/message, blanking the file cell on runs of
// the same file so it reads grouped without leaving the flat grid.
func (r QualityReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Issues))
	var lastFile string
	for _, is := range r.Issues {
		file := is.File
		if file == lastFile {
			file = ""
		} else {
			lastFile = is.File
		}
		msg := is.Message
		if is.Linter != "" {
			msg += " (" + is.Linter + ")"
		}
		rows = append(rows, []string{file, issueLoc(is), msg})
	}
	return rows
}

func issueLoc(is QualityIssue) string {
	if is.Line > 0 {
		return fmt.Sprintf("%d:%d", is.Line, is.Col)
	}
	return "-"
}

// DepsReport is the dependency-graph state for one repo: hygiene findings (module
// not tidy, replace directives, failed verification) plus what the graph demands -
// the runtime-version floor the dependencies force (RuntimeFloor, set by
// RuntimeFloorBy) and the per-dependency runtime versions (Deps, verbose only).
type DepsReport struct {
	Issues []DepIssue `json:"issues,omitempty"`
	// RuntimeFloor is the highest runtime version any dependency requires (Go: the
	// max `go` directive across the graph); RuntimeFloorBy is the module that sets
	// it. This is a demand the graph places on the project, surfaced next to the
	// hygiene findings. Deps is every dependency with its declared runtime version.
	RuntimeFloor   string       `json:"runtime_floor,omitempty"`
	RuntimeFloorBy string       `json:"runtime_floor_by,omitempty"`
	Deps           []RuntimeDep `json:"deps,omitempty"`
}

// DepIssue is one dependency finding: which sub-check raised it and the detail.
type DepIssue struct {
	Check  string `json:"check"` // tidy | replace | verify
	Detail string `json:"detail"`
}

// RuntimeDep is one dependency's identity and the runtime version it declares.
type RuntimeDep struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Min     string `json:"min,omitempty"` // the dep's own declared runtime version
}

func (r DepsReport) Summary(verbosity int) string {
	base := "tidy, no replace directives"
	if len(r.Issues) > 0 {
		base = plural(len(r.Issues), "issue", "issues")
	}
	// the runtime floor the graph forces is context, not a finding: shown from -v.
	if verbosity >= 1 && r.RuntimeFloor != "" {
		floor := "deps require " + r.RuntimeFloor
		if r.RuntimeFloorBy != "" {
			floor += " (" + r.RuntimeFloorBy + ")"
		}
		base += " · " + floor
	}
	return base
}

func (r DepsReport) Rows(verbosity int) [][]string {
	// findings always render (they are actionable, independent of verbosity); the
	// full per-dependency runtime-version table is exhaustive detail, shown at -vv.
	rows := make([][]string, 0, len(r.Issues))
	for _, it := range r.Issues {
		rows = append(rows, []string{it.Check, it.Detail})
	}
	if verbosity >= 2 {
		for _, d := range r.Deps {
			rows = append(rows, []string{d.Module, d.Version, d.Min})
		}
	}
	return rows
}

// RuntimeReport is the execution environment a project targets and what its own
// code needs - not what its dependencies demand, which is DepsReport. It is
// language-agnostic: an ecosystem fills only the parts it has. Go fills the version
// floor and the toolchain; Node additionally fills the module system and platform
// targets.
type RuntimeReport struct {
	// Version is the declared language version against what the code actually needs.
	Version RuntimeVersion `json:"version"`
	// Toolchain is the toolchain that builds and runs the project.
	Toolchain RuntimeToolchain `json:"toolchain"`
	// Targets is the module system and platforms the project builds for.
	Targets RuntimeTargets `json:"targets"`
}

// RuntimeVersion is the language version a project declares against the version its
// own code needs, with the verdict on whether the declaration could be lowered.
type RuntimeVersion struct {
	// Declared is the version range the manifest claims to support (Go `go`
	// directive, Node engines, Python requires-python).
	Declared Bound `json:"declared"`
	// Required is the version range the project's own code needs, by static analysis.
	Required Bound `json:"required"`
	// RequiredReason names the construct that sets Required's lower bound.
	RequiredReason string `json:"required_reason,omitempty"`
	// DependencyFloor is the lowest version the dependency graph allows.
	DependencyFloor string `json:"dependency_floor,omitempty"`
	// Minimum is the lowest version Declared could be set to: the higher of the code
	// requirement and the dependency floor.
	Minimum string `json:"minimum,omitempty"`
	// Reducible reports whether Declared's lower bound sits above Minimum and so
	// could be lowered to it.
	Reducible bool `json:"reducible"`
}

// RuntimeToolchain is the toolchain or interpreter that builds and runs a project:
// the version executing now against the version the manifest pins.
type RuntimeToolchain struct {
	// Running is the toolchain version currently executing (Go `go env GOVERSION`,
	// the running node binary's version).
	Running string `json:"running,omitempty"`
	// Pinned is the toolchain version the manifest declares (Go `toolchain`
	// directive, a packageManager field); empty when nothing is pinned.
	Pinned string `json:"pinned,omitempty"`
	// PinNote flags a notable Pinned value - one that is redundant or raises the
	// build floor; empty when the pin is unremarkable.
	PinNote string `json:"pin_note,omitempty"`
}

// RuntimeTargets is what a project targets at the environment level. Both fields
// are empty for ecosystems without these concepts.
type RuntimeTargets struct {
	// ModuleSystem is the module format the project emits (Node "esm"/"commonjs").
	ModuleSystem string `json:"module_system,omitempty"`
	// Platforms are the os/arch targets the project builds for.
	Platforms []string `json:"platforms,omitempty"`
}

// Bound is a version range [Min, Max]. An empty Max means unbounded - as Go always
// is, being backwards-compatible, so a Go bound never carries a Max; a language that
// removes features (or declares an upper engines bound) fills it.
type Bound struct {
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
}

// String renders a bound: "1.25" (min only), "<=22" (max only), "18..22" (both).
func (b Bound) String() string {
	switch {
	case b.Min == "" && b.Max == "":
		return ""
	case b.Max == "":
		return b.Min
	case b.Min == "":
		return "<=" + b.Max
	default:
		return b.Min + ".." + b.Max
	}
}

func (r RuntimeReport) Summary(int) string {
	v := r.Version
	// each version is labeled by what it is, so the line reads unambiguously:
	// "declared 1.25.0 · code 1.22 · deps 1.25.0". The declared field carries the
	// only verdict - a "(min X)" marker when the declared version could drop.
	declared := "declared " + v.Declared.String()
	if v.Reducible && v.Minimum != "" {
		declared += " (min " + v.Minimum + ")"
	}
	parts := []string{declared}
	if req := v.Required.String(); req != "" {
		parts = append(parts, "code "+req)
	}
	if v.DependencyFloor != "" {
		parts = append(parts, "deps "+v.DependencyFloor)
	}
	if r.Targets.ModuleSystem != "" {
		parts = append(parts, r.Targets.ModuleSystem)
	}
	// a noteworthy (redundant/floor-raising) toolchain pin earns a place; a normal
	// one is detail, shown in the rows.
	if r.Toolchain.PinNote != "" {
		parts = append(parts, "toolchain "+r.Toolchain.Pinned+" "+r.Toolchain.PinNote)
	}
	return strings.Join(parts, " · ")
}

func (r RuntimeReport) Rows(verbosity int) [][]string {
	if verbosity < 1 {
		return nil
	}
	v := r.Version
	var rows [][]string
	add := func(k, val string) {
		if val != "" {
			rows = append(rows, []string{k, val})
		}
	}
	add("running", r.Toolchain.Running)
	add("declared", v.Declared.String())
	if req := v.Required.String(); req != "" {
		detail := req
		if v.RequiredReason != "" {
			detail += " (" + v.RequiredReason + ")"
		}
		add("code", detail)
	}
	add("deps", v.DependencyFloor)
	if r.Toolchain.Pinned != "" {
		tc := r.Toolchain.Pinned
		if r.Toolchain.PinNote != "" {
			tc += " (" + r.Toolchain.PinNote + ")"
		}
		add("toolchain", tc)
	}
	add("module", r.Targets.ModuleSystem)
	if len(r.Targets.Platforms) > 0 {
		add("platforms", strings.Join(r.Targets.Platforms, ", "))
	}
	return rows
}

// TestReport is the structured result of a test run: per-suite and per-test
// outcomes plus aggregate coverage.
type TestReport struct {
	ModulePath string `json:"-"` // repo metadata lives on the report header; kept only to shorten package names in Rows

	Suites       []TestSuite `json:"suites,omitempty"`
	Total        float64     `json:"total,omitempty"`
	WithCoverage bool        `json:"with_coverage"`
	// Cases is the per-test-function tally across all suites (distinct from the
	// per-suite pass/fail, which Summary derives from Suites).
	Cases TestCounts `json:"cases"`
	// Untested lists packages with no test coverage (no test files, or 0% covered
	// when coverage ran), shortened against ModulePath.
	Untested []string `json:"untested,omitempty"`
}

// TestCounts is a pass/fail/skip tally of individual test functions.
type TestCounts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped,omitempty"`
}

// TestSuite is one suite's (Go package's) test result with its per-test cases.
// Output carries package-scoped failure text (e.g. a build error) when the suite
// failed to even run; Files holds per-file coverage when coverage was collected.
type TestSuite struct {
	Name       string         `json:"name"`
	Coverage   float64        `json:"coverage,omitempty"`
	Statements int            `json:"statements,omitempty"`
	Covered    int            `json:"covered,omitempty"`
	Passed     bool           `json:"passed"`
	Skipped    bool           `json:"skipped,omitempty"`
	DurationMs int            `json:"duration_ms,omitempty"`
	Output     string         `json:"output,omitempty"`
	Tests      []TestCase     `json:"tests,omitempty"`
	Files      []FileCoverage `json:"files,omitempty"`
	Funcs      []FuncCoverage `json:"funcs,omitempty"`
}

// FuncCoverage is one function's statement coverage within a suite.
type FuncCoverage struct {
	File     string  `json:"file"`
	Line     int     `json:"line,omitempty"`
	Name     string  `json:"name"`
	Coverage float64 `json:"coverage"`
}

// TestCase is one test function's outcome. Output carries the test's own log
// output when it failed.
type TestCase struct {
	Name      string `json:"name"`
	Action    string `json:"action"` // pass|fail|skip
	ElapsedMs int    `json:"elapsed_ms,omitempty"`
	Output    string `json:"output,omitempty"`
}

// FileCoverage is one source file's statement coverage within a suite, with the
// line ranges left uncovered.
type FileCoverage struct {
	Path       string      `json:"path"`
	Statements int         `json:"statements,omitempty"`
	Covered    int         `json:"covered,omitempty"`
	Uncovered  []LineRange `json:"uncovered,omitempty"`
}

// LineRange is an inclusive span of source lines.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (r TestReport) Summary(int) string {
	c := r.Cases
	total := c.Passed + c.Failed + c.Skipped
	parts := []string{plural(total, "test", "tests")}
	if c.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", c.Failed))
	}
	if c.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", c.Skipped))
	}
	// a suite that fails to build carries no test cases, so report failing suites
	// when the case tally would otherwise read all-green.
	if fs := r.failedSuites(); fs > 0 && c.Failed == 0 {
		parts = append(parts, fmt.Sprintf("%s failed to build", plural(fs, "suite", "suites")))
	}
	if r.WithCoverage {
		parts = append(parts, fmt.Sprintf("%.1f%% coverage", r.Total))
	}
	if n := len(r.Untested); n > 0 {
		parts = append(parts, fmt.Sprintf("%d untested", n))
	}
	return strings.Join(parts, ", ")
}

// failedSuites counts suites that did not pass and were not skipped.
func (r TestReport) failedSuites() int {
	var n int
	for _, s := range r.Suites {
		if !s.Passed && !s.Skipped {
			n++
		}
	}
	return n
}

// Rows lists each suite (name, result, coverage), expanding any failed suite
// inline with the failure text so a red run is actionable without a flag; at -vv
// it expands every suite to its individual test functions.
func (r TestReport) Rows(verbosity int) [][]string {
	if verbosity >= 2 {
		return r.testRows()
	}
	rows := make([][]string, 0, len(r.Suites))
	for _, s := range r.Suites {
		// at v0 only failing suites are findings; passing/skipped suites are context
		// shown from -v, so a failed run stays focused on what broke.
		if verbosity < 1 && (s.Passed || s.Skipped) {
			continue
		}
		cells := []string{shortenPkg(s.Name, r.ModulePath), suiteResult(s)}
		if r.WithCoverage {
			cov := "-"
			if !s.Skipped {
				cov = fmt.Sprintf("%.1f%%", s.Coverage)
			}
			cells = append(cells, cov)
		}
		rows = append(rows, cells)
		if !s.Passed && !s.Skipped {
			rows = append(rows, suiteFailureRows(s)...)
		}
	}
	if verbosity >= 1 {
		rows = append(rows, r.extraRows()...)
	}
	return rows
}

// extraRows surfaces the coverage-derived signals at -v: packages with no coverage
// and the slowest test functions, each as its own labeled row.
func (r TestReport) extraRows() [][]string {
	var rows [][]string
	for _, pkg := range r.Untested {
		rows = append(rows, []string{shortenPkg(pkg, r.ModulePath), "untested"})
	}
	for _, tc := range r.slowestTests(5) {
		rows = append(rows, []string{tc.name, fmt.Sprintf("%dms", tc.ms)})
	}
	return rows
}

// slowTest is one test's elapsed time, qualified by its suite for display.
type slowTest struct {
	name string
	ms   int
}

// slowestTests returns the n slowest individual test functions across all suites,
// each prefixed with its (shortened) suite, so a -v run flags the time sinks.
func (r TestReport) slowestTests(n int) []slowTest {
	var all []slowTest
	for _, s := range r.Suites {
		short := shortenPkg(s.Name, r.ModulePath)
		for _, tc := range s.Tests {
			if tc.Action == "pass" && tc.ElapsedMs > 0 {
				all = append(all, slowTest{name: short + " " + tc.Name, ms: tc.ElapsedMs})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ms > all[j].ms })
	if len(all) > n {
		all = all[:n]
	}
	return all
}

func (r TestReport) testRows() [][]string {
	var rows [][]string
	for _, s := range r.Suites {
		short := shortenPkg(s.Name, r.ModulePath)
		if s.Output != "" {
			rows = append(rows, indentLines(s.Output, "  ")...)
		}
		for _, tc := range s.Tests {
			rows = append(rows, []string{short + " " + tc.Name, tc.Action})
			if tc.Action == "fail" && tc.Output != "" {
				rows = append(rows, indentLines(tc.Output, "    ")...)
			}
		}
	}
	return rows
}

// suiteFailureRows renders a failed suite's reason: a package-level build error,
// then each failing test name with its captured output, all indented under the
// suite row.
func suiteFailureRows(s TestSuite) [][]string {
	var rows [][]string
	if s.Output != "" {
		rows = append(rows, indentLines(s.Output, "  ")...)
	}
	for _, tc := range s.Tests {
		if tc.Action != "fail" {
			continue
		}
		rows = append(rows, []string{"  " + tc.Name, "fail"})
		if tc.Output != "" {
			rows = append(rows, indentLines(tc.Output, "    ")...)
		}
	}
	return rows
}

// indentLines turns a multi-line block into one single-cell row per line, each
// prefixed with indent so it nests under its suite or test in the flat block.
func indentLines(block, indent string) [][]string {
	var rows [][]string
	for _, l := range strings.Split(block, "\n") {
		rows = append(rows, []string{indent + l})
	}
	return rows
}

func suiteResult(s TestSuite) string {
	switch {
	case s.Skipped:
		return "SKIP"
	case !s.Passed:
		return "FAIL"
	default:
		return "PASS"
	}
}

// BenchReport holds one repo's benchmark results.
type BenchReport struct {
	ModulePath string        `json:"-"` // repo metadata lives on the report header; kept only to shorten package names in Rows
	Benchmarks []BenchResult `json:"benchmarks,omitempty"`
}

// BenchResult is one benchmark's result. Ns/Bytes/Allocs are the standard columns
// `go test -benchmem` always emits; Extra carries any further columns the same line
// reports (MB/s from b.SetBytes, custom units from b.ReportMetric), in output order.
type BenchResult struct {
	Name        string        `json:"name"`
	Package     string        `json:"package,omitempty"`
	Runs        int           `json:"runs,omitempty"`
	NsPerOp     float64       `json:"ns_per_op,omitempty"`
	BytesPerOp  int           `json:"bytes_per_op,omitempty"`
	AllocsPerOp int           `json:"allocs_per_op,omitempty"`
	Extra       []BenchMetric `json:"extra,omitempty"`
}

// BenchMetric is one non-standard benchmark column: a value and its unit, as the
// benchmark emitted it (e.g. SetBytes throughput "MB/s", a ReportMetric custom unit).
type BenchMetric struct {
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
}

func (r BenchReport) Summary(int) string {
	return plural(len(r.Benchmarks), "benchmark", "benchmarks")
}

func (r BenchReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Benchmarks))
	for _, b := range r.Benchmarks {
		name := strings.TrimPrefix(strings.TrimPrefix(b.Name, "Benchmark"), "_")
		if b.Package != "" {
			name = shortenPkg(b.Package, r.ModulePath) + " " + name
		}
		var parts []string
		if b.NsPerOp > 0 {
			parts = append(parts, fmt.Sprintf("%.1f ns/op", b.NsPerOp))
		}
		if b.BytesPerOp > 0 {
			parts = append(parts, fmt.Sprintf("%d B/op", b.BytesPerOp))
		}
		if b.AllocsPerOp > 0 {
			parts = append(parts, fmt.Sprintf("%d allocs/op", b.AllocsPerOp))
		}
		for _, m := range b.Extra {
			parts = append(parts, fmt.Sprintf("%g %s", m.Value, m.Unit))
		}
		rows = append(rows, []string{name, strings.Join(parts, "  ")})
	}
	return rows
}

// SecretReport lists leaked-secret findings.
type SecretReport struct {
	Findings []SecretFinding `json:"findings,omitempty"`
}

// SecretFinding is one leaked-secret finding: the rule that matched, where, and
// the entropy and tags the scanner attached to it.
type SecretFinding struct {
	Rule        string   `json:"rule"`
	Description string   `json:"description,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Commit      string   `json:"commit,omitempty"`
	Entropy     float64  `json:"entropy,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (r SecretReport) Summary(int) string {
	return plural(len(r.Findings), "secret", "secrets")
}

func (r SecretReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		loc := "-"
		if f.Line > 0 {
			loc = fmt.Sprintf("%d", f.Line)
		}
		rows = append(rows, []string{f.Rule, f.File, loc})
	}
	return rows
}

// VulnReport lists the vulnerabilities the code is affected by.
type VulnReport struct {
	Findings []VulnFinding `json:"findings,omitempty"`
}

// VulnFinding is one structured vulnerability: the OSV id, the affected package and
// the versions found vs fixed, a one-line summary, the symbol the code reaches, and
// the call-chain trace.
type VulnFinding struct {
	ID      string      `json:"id"`
	Package string      `json:"package,omitempty"`
	Found   string      `json:"found,omitempty"`
	Fixed   string      `json:"fixed,omitempty"`
	Summary string      `json:"summary,omitempty"`
	Symbol  string      `json:"symbol,omitempty"`
	Trace   []VulnFrame `json:"trace,omitempty"`
}

// VulnFrame is one frame of a vulnerability's call-chain trace.
type VulnFrame struct {
	Module   string `json:"module,omitempty"`
	Version  string `json:"version,omitempty"`
	Package  string `json:"package,omitempty"`
	Function string `json:"function,omitempty"`
}

func (r VulnReport) Summary(int) string {
	return plural(len(r.Findings), "vulnerability", "vulnerabilities")
}

func (r VulnReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Findings))
	for _, v := range r.Findings {
		var ver string
		switch {
		case v.Found != "" && v.Fixed != "":
			ver = v.Found + " -> " + v.Fixed
		case v.Found != "":
			ver = v.Found + " (no fix)"
		}
		rows = append(rows, []string{v.ID, v.Package, ver, v.Summary})
	}
	return rows
}

// BuildReport is the compile state of the repo: the compiler errors `go build`
// emitted. An empty Errors list means the module compiles.
type BuildReport struct {
	Errors []BuildError `json:"errors,omitempty"`
}

// BuildError is one compiler diagnostic: where it occurred and the message.
type BuildError struct {
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	Message string `json:"message"`
}

func (r BuildReport) Summary(int) string {
	if len(r.Errors) == 0 {
		return "compiles"
	}
	return plural(len(r.Errors), "error", "errors")
}

func (r BuildReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		rows = append(rows, []string{e.File, lineCol(e.Line, e.Col), e.Message})
	}
	return rows
}

// DocsReport is the doc-comment coverage of exported symbols: how many of the
// repo's exported declarations carry a doc comment, and which do not.
type DocsReport struct {
	Total      int         `json:"total"`
	Documented int         `json:"documented"`
	Missing    []DocSymbol `json:"missing,omitempty"`
}

// DocSymbol is one undocumented exported declaration: its kind, name and location.
type DocSymbol struct {
	File string `json:"file"`
	Line int    `json:"line,omitempty"`
	Kind string `json:"kind"` // func | type | const | var | method
	Name string `json:"name"`
}

func (r DocsReport) Summary(int) string {
	if r.Total == 0 {
		return "no exported symbols"
	}
	pct := float64(r.Documented) / float64(r.Total) * 100
	return fmt.Sprintf("%.0f%% documented (%d/%d, %d undocumented)", pct, r.Documented, r.Total, len(r.Missing))
}

// Rows is intentionally empty: docs reports general stats only (the Summary), never
// the per-symbol list, which would flood the terminal. The full undocumented set
// still rides on the JSON payload for machine consumers.
func (r DocsReport) Rows(verbosity int) [][]string {
	// the summary carries the coverage stats; the per-symbol undocumented list is
	// exhaustive detail, shown only at -vv, grouped by file (blank-first-cell).
	if verbosity < 2 || len(r.Missing) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(r.Missing))
	last := ""
	for _, s := range r.Missing {
		file := s.File
		if file == last {
			file = ""
		} else {
			last = s.File
		}
		rows = append(rows, []string{file, lineCol(s.Line, 0), s.Kind + " " + s.Name})
	}
	return rows
}

// lineCol renders a line:col location, or "-" when no line is known.
func lineCol(line, col int) string {
	if line <= 0 {
		return "-"
	}
	if col > 0 {
		return fmt.Sprintf("%d:%d", line, col)
	}
	return fmt.Sprintf("%d", line)
}

// FixReport lists the fixes a Fixer applied across the fixable features.
type FixReport struct {
	Fixes []FixResult `json:"fixes,omitempty"`
}

// FixResult is one applied fix: the action taken and a short result detail.
type FixResult struct {
	Action  string `json:"action"`
	Changed bool   `json:"changed"`
	Detail  string `json:"detail,omitempty"`
}

func (r FixReport) Summary(int) string {
	var changed int
	for _, f := range r.Fixes {
		if f.Changed {
			changed++
		}
	}
	return fmt.Sprintf("%d applied", changed)
}

func (r FixReport) Rows(int) [][]string {
	rows := make([][]string, 0, len(r.Fixes))
	for _, f := range r.Fixes {
		detail := f.Detail
		if detail == "" {
			if f.Changed {
				detail = "changed"
			} else {
				detail = "no change"
			}
		}
		rows = append(rows, []string{f.Action, detail})
	}
	return rows
}

// plural renders a count with its singular or plural noun.
func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// shortenPkg trims the module prefix from a package path for a compact display.
func shortenPkg(name, module string) string {
	if module == "" {
		return name
	}
	if name == module {
		return "."
	}
	if trimmed := strings.TrimPrefix(name, module+"/"); trimmed != name {
		return trimmed
	}
	return name
}
