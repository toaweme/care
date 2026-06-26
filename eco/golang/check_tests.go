package golang

import (
	"context"
	"fmt"

	"github.com/toaweme/care"
	"github.com/toaweme/care/eco/golang/inspect"
	"github.com/toaweme/care/eco/golang/tests"
)

type testsCheck struct {
	care.BaseCheck
	profiles []care.RunProfile
}

var (
	_ care.Tests    = (*testsCheck)(nil)
	_ care.Profiled = (*testsCheck)(nil)
)

// NewTests runs the repo's tests, collecting coverage and running with -race per the
// active run-profile. profiles configures the named flag-sets the suite runs under
// (e.g. plain + -race + a build-tag variant); an empty list runs once under a
// synthesized default profile that honors the CLI --race/--coverage flags. The
// injected go tool is its install dependency.
func NewTests(tool care.Tool, profiles []care.RunProfile) care.Tests {
	return &testsCheck{BaseCheck: care.NewBaseCheck("go-test", tool), profiles: profiles}
}

func (f *testsCheck) Applies(dir string) bool { return hasGoMod(dir) }

// Profiles returns the configured run-profiles, or a single default when none are
// configured (so the feature runs exactly once with today's behavior).
func (f *testsCheck) Profiles() []care.RunProfile {
	if len(f.profiles) == 0 {
		return []care.RunProfile{{Name: "default"}}
	}
	return f.profiles
}

func (f *testsCheck) Run(ctx context.Context, dir string, opts care.RunOptions) care.Output[care.TestReport] {
	prof := opts.Profile
	// CLI --race/--coverage OR into every profile, so a global toggle applies across
	// all configured runs; an explicit profile flag is never turned off by them.
	race := prof.Race || opts.Tests.Race
	coverage := prof.Coverage || opts.Tests.Coverage

	rep, err := tests.NewRunner(tests.Options{Race: race, Coverage: coverage, Tags: prof.Tags, Args: prof.Args}).Run(ctx, dir)
	if err != nil {
		return care.Errored[care.TestReport]("tool failed", fmt.Errorf("failed to run tests in %q: %w", dir, err))
	}
	report := care.TestReport{ModulePath: rep.Module, Total: rep.Total, WithCoverage: coverage}
	if report.ModulePath == "" {
		if mod, err := inspect.ReadModulePath(dir); err == nil {
			report.ModulePath = mod
		}
	}
	var failed int
	for _, p := range rep.Packages {
		suite := care.TestSuite{
			Name: p.Name, Coverage: p.Coverage, Statements: p.Statements, Covered: p.Covered,
			Passed: p.Passed, Skipped: p.Skipped, DurationMs: p.DurationMs, Output: p.Output,
		}
		for _, tc := range p.Tests {
			suite.Tests = append(suite.Tests, care.TestCase{Name: tc.Name, Action: tc.Action, ElapsedMs: tc.ElapsedMs, Output: tc.Output})
			countCase(&report.Cases, tc.Action)
		}
		for _, fc := range p.Files {
			suite.Files = append(suite.Files, care.FileCoverage{
				Path: fc.Path, Statements: fc.Statements, Covered: fc.Covered, Uncovered: lineRanges(fc.Uncovered),
			})
		}
		for _, fn := range p.Funcs {
			suite.Funcs = append(suite.Funcs, care.FuncCoverage{File: fn.File, Line: fn.Line, Name: fn.Name, Coverage: fn.Coverage})
		}
		report.Suites = append(report.Suites, suite)
		if untested(p, coverage) {
			report.Untested = append(report.Untested, p.Name)
		}
		if !p.Passed && !p.Skipped {
			failed++
		}
	}
	if failed > 0 {
		return care.Fail(report)
	}
	return care.Pass(report)
}

// untested reports whether a package has no effective test coverage: no test files,
// or (when coverage ran) statements that are entirely uncovered.
func untested(p tests.PackageResult, coverage bool) bool {
	if p.NoTestFiles {
		return true
	}
	return coverage && p.Statements > 0 && p.Covered == 0
}

// countCase tallies one test function's outcome into the report's case counts.
func countCase(c *care.TestCounts, action string) {
	switch action {
	case "pass":
		c.Passed++
	case "fail":
		c.Failed++
	case "skip":
		c.Skipped++
	}
}

func lineRanges(in []tests.LineRange) []care.LineRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]care.LineRange, 0, len(in))
	for _, r := range in {
		out = append(out, care.LineRange{Start: r.Start, End: r.End})
	}
	return out
}
