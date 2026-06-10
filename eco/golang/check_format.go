package golang

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	"github.com/toaweme/mend"
)

type formatCheck struct {
	mend.BaseCheck
	tool mend.Tool
}

var _ mend.Format = (*formatCheck)(nil)

// NewFormat is the Format feature for Go: it lists the files gofmt would reformat
// (`gofmt -l`), a read-only check that needs no linter config. Unformatted files are
// a warning, not a failure, so they never block a run on their own.
func NewFormat(tool mend.Tool) mend.Format {
	return &formatCheck{BaseCheck: mend.NewBaseCheck("gofmt", tool), tool: tool}
}

// Applies runs gofmt only as the fallback baseline: when a golangci-lint config
// governs dir its formatters (gofmt/goimports) subsume this check, so the
// standalone pass steps aside and runs only when no config is present.
func (f *formatCheck) Applies(dir string) bool { return hasGoMod(dir) && !hasGolangciConfig(dir) }

func (f *formatCheck) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.FormatReport] {
	out, err := f.tool.Exec(ctx, dir, "-l", ".")
	files := parseGofmtList(out)
	if err != nil && len(files) == 0 {
		return mend.Errored[mend.FormatReport]("tool failed", err)
	}
	if len(files) == 0 {
		return mend.Pass(mend.FormatReport{})
	}
	return mend.Warn(mend.FormatReport{Files: files})
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
