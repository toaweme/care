package golang

import (
	"context"

	"github.com/toaweme/mend"
)

type fixer struct {
	mend.BaseCheck
	golangci mend.Tool
	gotool   mend.Tool
}

var _ mend.Fixer = (*fixer)(nil)

// NewFixer is the Fixer feature for Go: when --fix is set it applies the fixable
// features' fixes (golangci-lint --fix, gofmt -w, go mod tidy) before the read-only
// features report what is left.
func NewFixer(golangci, gotool mend.Tool) mend.Fixer {
	return &fixer{
		BaseCheck: mend.NewBaseCheck("go-fixer", golangci, gotool),
		golangci:  golangci,
		gotool:    gotool,
	}
}

func (f *fixer) Applies(dir string) bool { return hasGoMod(dir) }

func (f *fixer) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.FixReport] {
	var report mend.FixReport

	if hasGolangciConfig(dir) {
		report.Fixes = append(report.Fixes, run(ctx, f.golangci, dir, "golangci-lint --fix", "run", "--fix", "./..."))
	}
	report.Fixes = append(report.Fixes, run(ctx, f.gotool, dir, "gofmt", "fmt", "./..."))
	report.Fixes = append(report.Fixes, run(ctx, f.gotool, dir, "go mod tidy", "mod", "tidy"))

	return mend.Pass(report)
}

// run applies one fix command and records the outcome as a FixResult.
func run(ctx context.Context, tool mend.Tool, dir, action string, args ...string) mend.FixResult {
	out, err := tool.Exec(ctx, dir, args...)
	if err != nil {
		return mend.FixResult{Action: action, Changed: false, Detail: trimOutput(out)}
	}
	return mend.FixResult{Action: action, Changed: true}
}
