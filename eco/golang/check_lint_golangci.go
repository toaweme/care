package golang

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/toaweme/mend"
)

type golangciLint struct {
	mend.BaseCheck
	tool   mend.Tool
	gotool mend.Tool
}

var _ mend.Quality = (*golangciLint)(nil)

// NewGolangciLint runs golangci-lint through the injected tool and reports the
// issues it finds. In fix mode (QualityRunOptions.Fix) it first formats, then
// applies the auto-fixable lints (run --fix) and reports what is left; otherwise
// it is a read-only check. The go tool is the formatting fallback (go fmt) when
// there is no .golangci.yml or golangci-lint cannot format.
func NewGolangciLint(tool, gotool mend.Tool) mend.Quality {
	return &golangciLint{BaseCheck: mend.NewBaseCheck("golangci-lint", tool), tool: tool, gotool: gotool}
}

func (f *golangciLint) Applies(dir string) bool { return hasGoMod(dir) }

// format applies the formatters in fix mode: golangci-lint fmt when a config is
// present, falling back to `go fmt` when there is no .golangci.yml or golangci-lint
// cannot format (so formatting still happens without a linter config).
func (f *golangciLint) format(ctx context.Context, dir string, hasCfg bool) error {
	if hasCfg {
		if _, err := f.tool.Exec(ctx, dir, "fmt", "./..."); err == nil {
			return nil
		}
	}
	if out, err := f.gotool.Exec(ctx, dir, "fmt", "./..."); err != nil {
		return fmt.Errorf("failed to run go fmt: %w\n%s", err, trimOutput(out))
	}
	return nil
}

func (f *golangciLint) Run(ctx context.Context, dir string, opts mend.RunOptions) mend.Output[mend.QualityReport] {
	hasCfg := hasGolangciConfig(dir)
	if opts.Quality.Fix {
		if err := f.format(ctx, dir, hasCfg); err != nil {
			return mend.Errored[mend.QualityReport]("tool failed", err)
		}
	}
	if !hasCfg {
		return mend.Skip[mend.QualityReport]("no .golangci.yml")
	}
	// golangci-lint v2 emits the default text formatter to stdout alongside any
	// JSON formatter, so JSON on stdout is interleaved with code snippets. Route
	// the JSON report into its own file to keep it clean; stdout/stderr then carry
	// only human-readable text we surface if the run fails for a non-lint reason.
	report, err := os.CreateTemp("", "mend-golangci-*.json")
	if err != nil {
		return mend.Errored[mend.QualityReport]("tool failed", fmt.Errorf("failed to create golangci-lint report file: %w", err))
	}
	defer os.Remove(report.Name())
	report.Close()

	args := []string{"run", "--output.json.path", report.Name()}
	if opts.Quality.Fix {
		args = append(args, "--fix")
	}
	args = append(args, "./...")
	out, err := f.tool.Exec(ctx, dir, args...)
	if err == nil {
		return mend.Pass(mend.QualityReport{})
	}
	issues := parseGolangciJSON(report.Name())
	if len(issues) == 0 {
		return mend.Errored[mend.QualityReport]("tool failed", fmt.Errorf("failed to run golangci-lint: %w\n%s", err, trimOutput(out)))
	}
	return mend.Fail(mend.QualityReport{Issues: issues})
}

func parseGolangciJSON(path string) []mend.QualityIssue {
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
	issues := make([]mend.QualityIssue, 0, len(parsed.Issues))
	for _, i := range parsed.Issues {
		issues = append(issues, mend.QualityIssue{File: i.Pos.Filename, Line: i.Pos.Line, Col: i.Pos.Column, Linter: i.FromLinter, Severity: i.Severity, Message: i.Text})
	}
	return issues
}
