package golang

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/toaweme/care"
)

type qualityCheck struct {
	care.BaseCheck
	golangci care.Tool
	gotool   care.Tool
	gofmt    care.Tool
}

var _ care.Quality = (*qualityCheck)(nil)

// NewQuality is the single static-analysis feature for Go. When a .golangci.yml governs the
// repo it runs golangci-lint (which already bundles govet and the gofmt/goimports
// formatters); otherwise it falls back to the toolchain-native `go vet` + `gofmt -l`,
// reported as the same QualityReport.
//
// This is one "lint" feature rather than three overlapping ones: vet and format used to be
// separate slots that simply self-skipped whenever golangci governed the repo (which, given
// `care setup lint`, is every repo). In fix mode (QualityRunOptions.Fix) it formats and
// applies golangci's auto-fixes, or `go fmt` as the fallback.
func NewQuality(golangci, gotool, gofmt care.Tool) care.Quality {
	return &qualityCheck{
		BaseCheck: care.NewBaseCheck("golangci-lint", golangci, gotool, gofmt),
		golangci:  golangci,
		gotool:    gotool,
		gofmt:     gofmt,
	}
}

func (f *qualityCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *qualityCheck) Run(ctx context.Context, dir string, opts care.RunOptions) care.Output[care.QualityReport] {
	hasCfg := hasGolangciConfig(dir)
	if opts.Quality.Fix {
		if err := f.format(ctx, dir, hasCfg); err != nil {
			return care.Errored[care.QualityReport]("tool failed", err)
		}
	}
	if hasCfg {
		return f.runGolangci(ctx, dir, opts)
	}
	return f.runFallback(ctx, dir)
}

// format applies the formatters in fix mode: golangci-lint fmt when a config is
// present, falling back to `go fmt` when there is none or golangci cannot format.
func (f *qualityCheck) format(ctx context.Context, dir string, hasCfg bool) error {
	if hasCfg {
		if _, err := f.golangci.Exec(ctx, dir, "fmt", "./..."); err == nil {
			return nil
		}
	}
	if out, err := f.gotool.Exec(ctx, dir, "fmt", "./..."); err != nil {
		return fmt.Errorf("failed to run go fmt: %w\n%s", err, trimOutput(out))
	}
	return nil
}

func (f *qualityCheck) runGolangci(ctx context.Context, dir string, opts care.RunOptions) care.Output[care.QualityReport] {
	// golangci-lint v2 emits the default text formatter to stdout alongside any JSON
	// formatter, so JSON on stdout is interleaved with code snippets. Route the JSON
	// report into its own file to keep it clean; stdout/stderr then carry only the
	// human-readable text surfaced when a run fails for a non-lint reason.
	report, err := os.CreateTemp("", "care-golangci-*.json")
	if err != nil {
		return care.Errored[care.QualityReport]("tool failed", fmt.Errorf("failed to create golangci-lint report file: %w", err))
	}
	defer os.Remove(report.Name())
	report.Close()

	args := []string{"run", "--output.json.path", report.Name()}
	if opts.Quality.Fix {
		args = append(args, "--fix")
	}
	args = append(args, "./...")
	out, err := f.golangci.Exec(ctx, dir, args...)
	if err == nil {
		return care.Pass(care.QualityReport{})
	}
	issues := parseGolangciJSON(report.Name())
	if len(issues) == 0 {
		return care.Errored[care.QualityReport]("tool failed", fmt.Errorf("failed to run golangci-lint: %w\n%s", err, trimOutput(out)))
	}
	return care.Fail(care.QualityReport{Issues: issues})
}

// runFallback is the no-config baseline: `go vet` (a failure when it finds anything)
// and `gofmt -l` (a warning), merged into one QualityReport with each finding tagged
// by its source so the row reads the same as a golangci run.
func (f *qualityCheck) runFallback(ctx context.Context, dir string) care.Output[care.QualityReport] {
	var issues []care.QualityIssue
	vetOut, _ := f.gotool.Exec(ctx, dir, "vet", "./...") // non-zero exit on findings is expected
	for _, d := range parseDiagnostics(vetOut) {
		issues = append(issues, care.QualityIssue{File: d.File, Line: d.Line, Col: d.Col, Linter: "govet", Message: d.Message})
	}
	vetFailed := len(issues) > 0

	fmtOut, _ := f.gofmt.Exec(ctx, dir, "-l", ".")
	for _, file := range parseGofmtList(fmtOut) {
		issues = append(issues, care.QualityIssue{File: file, Linter: "gofmt", Message: "not gofmt'd"})
	}

	switch {
	case len(issues) == 0:
		return care.Pass(care.QualityReport{})
	case vetFailed:
		return care.Fail(care.QualityReport{Issues: issues})
	default:
		return care.Warn(care.QualityReport{Issues: issues}) // formatting only, never blocks
	}
}

func parseGolangciJSON(path string) []care.QualityIssue {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var parsed struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Severity   string `json:"Severity"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
				Column   int    `json:"Column"`
			} `json:"Pos"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	issues := make([]care.QualityIssue, 0, len(parsed.Issues))
	for _, i := range parsed.Issues {
		issues = append(issues, care.QualityIssue{File: i.Pos.Filename, Line: i.Pos.Line, Col: i.Pos.Column, Linter: i.FromLinter, Severity: i.Severity, Message: i.Text})
	}
	return issues
}

// parseGofmtList reads the newline-separated file list `gofmt -l` prints, dropping
// vendored and testdata files (which are not ours to format).
func parseGofmtList(out []byte) []string {
	var files []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.TrimSpace(sc.Text())
		if f == "" || isVendored(f) {
			continue
		}
		files = append(files, f)
	}
	return files
}

// isVendored reports whether a path is under a vendor or testdata directory.
func isVendored(p string) bool {
	return strings.HasPrefix(p, "vendor/") || strings.Contains(p, "/vendor/") ||
		strings.HasPrefix(p, "testdata/") || strings.Contains(p, "/testdata/")
}
