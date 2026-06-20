// Package tests runs the Go test suite once via `go test ./... -json` and parses
// the test2json event stream into per-package, per-test, and coverage results.
package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

// GoTestRunner runs the repo's tests once via `go test ./... -json` and parses
// the test2json event stream into per-package and per-test results. It is a
// single invocation: -json structures the output and -coverprofile adds a
// coverage profile to the same run, neither splits it.
type GoTestRunner struct {
	race     bool
	coverage bool
	tags     string
	args     []string
}

var _ Runner = (*GoTestRunner)(nil)

// Options configures the test runner. Coverage is opt-in. Tags/Args carry a
// profile's build tags and any raw escape-hatch flags.
type Options struct {
	Race     bool
	Coverage bool
	Tags     string
	Args     []string
}

// NewRunner creates a GoTestRunner configured from the given Options.
func NewRunner(opts Options) *GoTestRunner {
	return &GoTestRunner{race: opts.Race, coverage: opts.Coverage, tags: opts.Tags, args: opts.Args}
}

// Run executes the test suite in dir and returns the parsed Report.
func (r *GoTestRunner) Run(ctx context.Context, dir string) (Report, error) {
	args := []string{"test", "-json"}

	var profilePath string
	if r.coverage {
		f, err := os.CreateTemp("", "mend-cover-*.out")
		if err != nil {
			return Report{}, fmt.Errorf("failed to create coverage profile: %w", err)
		}
		profilePath = f.Name()
		f.Close()
		defer os.Remove(profilePath)
		// -coverprofile implies -cover and writes one merged profile for ./...,
		// so we keep the single invocation while gaining per-file statement counts.
		args = append(args, "-coverprofile="+profilePath)
	}
	if r.race {
		args = append(args, "-race")
	}
	if r.tags != "" {
		args = append(args, "-tags", r.tags)
	}
	args = append(args, r.args...)
	args = append(args, "./...")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	out, _ := cmd.Output() // non-zero exit on test failure is expected; parse the stream

	packages, err := parseTestJSON(out)
	if err != nil {
		return Report{}, fmt.Errorf("failed to parse test output: %w", err)
	}
	if profilePath != "" {
		if cov, err := parseCoverProfile(profilePath); err == nil {
			applyCoverage(packages, cov)
		}
		// per-function coverage reads the same profile (no recompile, fast), so the
		// report can name the uncovered functions rather than just a file percentage.
		if funcs, err := coverFuncs(ctx, dir, profilePath); err == nil {
			applyFuncs(packages, funcs)
		}
	}
	return Report{Packages: packages, Total: computeTotal(packages)}, nil
}

// testEvent is one test2json record.
type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

// parseTestJSON folds the test2json event stream into one PackageResult per
// package, each carrying its per-test cases, coverage and duration. Output events
// are buffered per package and per test so a failing package or test keeps its
// log text; the buffers are dropped for anything that passed.
func parseTestJSON(out []byte) ([]PackageResult, error) {
	pkgs := map[string]*PackageResult{}
	order := []string{}
	pkgOut := map[string][]string{}
	testOut := map[string][]string{}
	pkg := func(name string) *PackageResult {
		p, ok := pkgs[name]
		if !ok {
			p = &PackageResult{Name: name, Passed: true}
			pkgs[name] = p
			order = append(order, name)
		}
		return p
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var ev testEvent
		if err := dec.Decode(&ev); err != nil {
			break
		}
		if ev.Package == "" {
			continue
		}
		p := pkg(ev.Package)
		if ev.Test != "" {
			key := ev.Package + "\x00" + ev.Test
			switch ev.Action {
			case "output":
				testOut[key] = append(testOut[key], ev.Output)
			case "pass", "skip":
				p.Tests = append(p.Tests, TestCase{Name: ev.Test, Action: ev.Action, ElapsedMs: ms(ev.Elapsed)})
			case "fail":
				p.Tests = append(p.Tests, TestCase{Name: ev.Test, Action: ev.Action, ElapsedMs: ms(ev.Elapsed), Output: cleanOutput(testOut[key])})
			}
			continue
		}
		switch ev.Action {
		case "pass":
			p.DurationMs = ms(ev.Elapsed)
		case "fail":
			p.Passed = false
			p.DurationMs = ms(ev.Elapsed)
			p.Output = cleanOutput(pkgOut[ev.Package])
		case "skip":
			p.Skipped = true
			p.DurationMs = ms(ev.Elapsed)
		case "output":
			if cov, ok := parseCoverage(ev.Output); ok {
				p.Coverage = cov
			}
			if strings.Contains(ev.Output, "[no test files]") {
				p.NoTestFiles = true
			}
			pkgOut[ev.Package] = append(pkgOut[ev.Package], ev.Output)
		}
	}

	results := make([]PackageResult, 0, len(order))
	for _, name := range order {
		p := pkgs[name]
		sort.SliceStable(p.Tests, func(i, j int) bool { return p.Tests[i].Name < p.Tests[j].Name })
		results = append(results, *p)
	}
	return results, nil
}

// cleanOutput joins buffered test2json output lines into a compact failure
// snippet, dropping the test2json control and summary lines that carry no signal
// and capping the tail so a runaway panic does not flood the report.
func cleanOutput(lines []string) string {
	const maxLines = 30
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		t := strings.TrimRight(l, "\n")
		ts := strings.TrimSpace(t)
		switch {
		case ts == "":
			continue
		case strings.HasPrefix(ts, "=== "): // RUN / PAUSE / CONT control lines
			continue
		case ts == "PASS" || ts == "FAIL" || strings.HasPrefix(ts, "ok  ") || strings.HasPrefix(ts, "FAIL\t"):
			continue
		}
		kept = append(kept, t)
	}
	if len(kept) > maxLines {
		kept = append([]string{"... (truncated)"}, kept[len(kept)-maxLines:]...)
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// parseCoverage extracts the percentage from a "coverage: 84.2% of statements"
// output line.
func parseCoverage(out string) (float64, bool) {
	i := strings.Index(out, "coverage:")
	if i < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(out[i+len("coverage:"):])
	pct, _, ok := strings.Cut(rest, "%")
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(pct), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// pkgCoverage is the statement coverage aggregated for one package from a profile.
type pkgCoverage struct {
	statements int
	covered    int
	files      []FileCoverage
}

// parseCoverProfile reads a `go test -coverprofile` file and sums statements and
// covered statements per package and per file. The profile keys blocks by full
// import path + file, so the package import path is the directory of that path,
// which matches the test2json Package field.
func parseCoverProfile(profilePath string) (map[string]*pkgCoverage, error) {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read coverage profile: %w", err)
	}
	pkgs := map[string]*pkgCoverage{}
	files := map[string]*FileCoverage{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		// format: import/path/file.go:7.34,9.2 numStmt count
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		colon := strings.LastIndex(fields[0], ":")
		if colon < 0 {
			continue
		}
		file := fields[0][:colon]
		stmts, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		pkgName := path.Dir(file)
		pc := pkgs[pkgName]
		if pc == nil {
			pc = &pkgCoverage{}
			pkgs[pkgName] = pc
		}
		fc := files[file]
		if fc == nil {
			fc = &FileCoverage{Path: file}
			files[file] = fc
		}
		pc.statements += stmts
		fc.Statements += stmts
		if count > 0 { // any positive count (set/count/atomic mode) means covered
			pc.covered += stmts
			fc.Covered += stmts
		} else if r, ok := blockLines(fields[0][colon+1:]); ok {
			fc.Uncovered = append(fc.Uncovered, r)
		}
	}
	// attach finalized files to their packages, sorted for determinism
	for name, pc := range pkgs {
		var fs []FileCoverage
		for fpath, fc := range files {
			if path.Dir(fpath) == name {
				fs = append(fs, *fc)
			}
		}
		sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
		pc.files = fs
	}
	return pkgs, nil
}

// applyCoverage folds profile-derived statement counts onto the parsed packages,
// overriding the text-scraped percentage with the exact statement ratio.
func applyCoverage(packages []PackageResult, cov map[string]*pkgCoverage) {
	for i := range packages {
		pc := cov[packages[i].Name]
		if pc == nil || pc.statements == 0 {
			continue
		}
		packages[i].Statements = pc.statements
		packages[i].Covered = pc.covered
		packages[i].Coverage = float64(pc.covered) / float64(pc.statements) * 100
		packages[i].Files = pc.files
	}
}

// blockLines parses a coverprofile block range ("start.col,end.col") into its
// inclusive source-line span, dropping the column parts.
func blockLines(rng string) (LineRange, bool) {
	start, end, ok := strings.Cut(rng, ",")
	if !ok {
		return LineRange{}, false
	}
	s := lineOf(start)
	e := lineOf(end)
	if s == 0 || e == 0 {
		return LineRange{}, false
	}
	return LineRange{Start: s, End: e}, true
}

// lineOf extracts the line number from a "line.col" coordinate.
func lineOf(coord string) int {
	line, _, _ := strings.Cut(coord, ".")
	n, err := strconv.Atoi(line)
	if err != nil {
		return 0
	}
	return n
}

// coverFuncs runs `go tool cover -func` over the profile and returns the per-function
// coverage grouped by package import path (the directory of the reported file path).
func coverFuncs(ctx context.Context, dir, profilePath string) (map[string][]FuncCoverage, error) {
	cmd := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profilePath) //nolint:gosec // profilePath is an internal temp file, not user input
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run go tool cover: %w", err)
	}
	byPkg := map[string][]FuncCoverage{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "total:") {
			continue
		}
		// format: import/path/file.go:7:\tFuncName\t82.4%
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		loc := strings.TrimSuffix(fields[0], ":")
		colon := strings.LastIndex(loc, ":")
		if colon < 0 {
			continue
		}
		file := loc[:colon]
		lineNo, _ := strconv.Atoi(loc[colon+1:])
		pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil {
			continue
		}
		pkgName := path.Dir(file)
		byPkg[pkgName] = append(byPkg[pkgName], FuncCoverage{
			File: file, Line: lineNo, Name: fields[len(fields)-2], Coverage: pct,
		})
	}
	return byPkg, nil
}

// applyFuncs attaches per-function coverage to its package.
func applyFuncs(packages []PackageResult, funcs map[string][]FuncCoverage) {
	for i := range packages {
		if fs := funcs[packages[i].Name]; len(fs) > 0 {
			packages[i].Funcs = fs
		}
	}
}

func ms(seconds float64) int { return int(seconds * 1000) }

func computeTotal(packages []PackageResult) float64 {
	var totalStmts, coveredStmts int
	var sumPct float64
	var counted int

	for _, p := range packages {
		if p.Skipped {
			continue
		}
		if p.Statements > 0 {
			totalStmts += p.Statements
			coveredStmts += p.Covered
		} else {
			sumPct += p.Coverage
			counted++
		}
	}

	if totalStmts > 0 {
		return float64(coveredStmts) / float64(totalStmts) * 100
	}
	if counted > 0 {
		return sumPct / float64(counted)
	}
	return 0
}
