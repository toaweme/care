package golang

import (
	"context"

	"github.com/toaweme/mend"
)

type vetCheck struct {
	mend.BaseCheck
	tool mend.Tool
}

var _ mend.Vet = (*vetCheck)(nil)

// NewVet is the Vet feature for Go: it runs `go vet ./...` and reports each
// diagnostic (printf format mismatches, lost cancel, suspicious constructs). vet
// writes its findings to stderr in the canonical file:line:col form.
func NewVet(tool mend.Tool) mend.Vet {
	return &vetCheck{BaseCheck: mend.NewBaseCheck("go-vet", tool), tool: tool}
}

// Applies runs vet only as the fallback baseline: when a golangci-lint config
// governs dir the repo delegates static analysis to golangci-lint (whose govet
// linter subsumes `go vet`), so this standalone pass steps aside to avoid a
// redundant type-check.
func (f *vetCheck) Applies(dir string) bool { return hasGoMod(dir) && !hasGolangciConfig(dir) }

func (f *vetCheck) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.VetReport] {
	out, _ := f.tool.Exec(ctx, dir, "vet", "./...") // non-zero exit on findings is expected; parse the output
	diags := parseDiagnostics(out)
	if len(diags) == 0 {
		return mend.Pass(mend.VetReport{})
	}
	report := mend.VetReport{Issues: make([]mend.VetIssue, 0, len(diags))}
	for _, d := range diags {
		report.Issues = append(report.Issues, mend.VetIssue{File: d.File, Line: d.Line, Col: d.Col, Message: d.Message})
	}
	return mend.Fail(report)
}
