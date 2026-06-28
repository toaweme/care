package tests

import "context"

// Report holds one module's test outcome.
type Report struct {
	Module   string
	Packages []PackageResult
	Total    float64
}

// PackageResult holds one package's test result, with its per-test cases. Output
// carries package-scoped failure text (e.g. a compile/build error) when the
// package failed; it is empty on success. Files holds per-file coverage when the
// run collected a coverage profile.
type PackageResult struct {
	Name        string
	Coverage    float64
	Statements  int
	Covered     int
	Passed      bool
	Skipped     bool
	NoTestFiles bool // package has no _test.go files (reported "[no test files]")
	DurationMs  int
	Output      string
	Tests       []TestCase
	Files       []FileCoverage
	Funcs       []FuncCoverage
}

// TestCase is one test function's outcome. Output carries the test's own log
// output when it failed; it is empty for passing and skipped tests.
type TestCase struct {
	Name      string
	Action    string // pass|fail|skip
	ElapsedMs int
	Output    string
}

// FileCoverage is one source file's statement coverage within a package, with the
// line ranges left uncovered (the count==0 blocks of the coverage profile).
type FileCoverage struct {
	Path       string
	Statements int
	Covered    int
	Uncovered  []LineRange
}

// LineRange is an inclusive span of source lines (e.g. an uncovered block).
type LineRange struct {
	Start int
	End   int
}

// FuncCoverage is one function's statement coverage, from `go tool cover -func`.
type FuncCoverage struct {
	File     string
	Line     int
	Name     string
	Coverage float64
}

// Runner runs a module's test suite in dir and returns the parsed Report.
type Runner interface {
	Run(ctx context.Context, dir string) (Report, error)
}
