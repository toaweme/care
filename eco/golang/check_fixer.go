package golang

import (
	"context"

	"github.com/toaweme/care"
)

type fixer struct {
	care.BaseCheck
	golangci care.Tool
	gotool   care.Tool
}

var _ care.Fixer = (*fixer)(nil)

// NewFixer is the Fixer feature for Go: when --fix is set it applies the fixable
// features' fixes (golangci-lint --fix, gofmt -w, go mod tidy) before the read-only
// features report what is left.
func NewFixer(golangci, gotool care.Tool) care.Fixer {
	return &fixer{
		BaseCheck: care.NewBaseCheck("go-fixer", golangci, gotool),
		golangci:  golangci,
		gotool:    gotool,
	}
}

func (f *fixer) Applies(dir string) bool { return hasGoMod(dir) }

func (f *fixer) Run(ctx context.Context, dir string, _ care.RunOptions) care.Output[care.FixReport] {
	var report care.FixReport

	if hasGolangciConfig(dir) {
		report.Fixes = append(report.Fixes, run(ctx, f.golangci, dir, "golangci-lint --fix", "run", "--fix", "./..."))
	}
	report.Fixes = append(report.Fixes,
		run(ctx, f.gotool, dir, "gofmt", "fmt", "./..."),
		run(ctx, f.gotool, dir, "go mod tidy", "mod", "tidy"),
	)

	return care.Pass(report)
}

// run applies one fix command and records the outcome as a FixResult.
func run(ctx context.Context, tool care.Tool, dir, action string, args ...string) care.FixResult {
	out, err := tool.Exec(ctx, dir, args...)
	if err != nil {
		return care.FixResult{Action: action, Changed: false, Detail: trimOutput(out)}
	}
	return care.FixResult{Action: action, Changed: true}
}
