package golang

import (
	"context"

	"github.com/toaweme/mend"
)

type buildCheck struct {
	mend.BaseCheck
	tool mend.Tool
}

var _ mend.Build = (*buildCheck)(nil)

// NewBuild is the Build feature for Go: it reports whether the module compiles via
// `go build ./...`, surfacing each compiler diagnostic. A repo that does not build is
// the single strongest quality signal, so this fails the run on any error.
func NewBuild(tool mend.Tool) mend.Build {
	return &buildCheck{BaseCheck: mend.NewBaseCheck("go-build", tool), tool: tool}
}

func (f *buildCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *buildCheck) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.BuildReport] {
	out, _ := f.tool.Exec(ctx, dir, "build", "./...") // non-zero exit on a build error is expected; parse the output
	errs := parseDiagnostics(out)
	if len(errs) == 0 {
		return mend.Pass(mend.BuildReport{})
	}
	report := mend.BuildReport{Errors: make([]mend.BuildError, 0, len(errs))}
	for _, d := range errs {
		report.Errors = append(report.Errors, mend.BuildError{File: d.File, Line: d.Line, Col: d.Col, Message: d.Message})
	}
	return mend.Fail(report)
}
