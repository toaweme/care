package golang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/toaweme/care"
	"github.com/toaweme/care/eco/golang/gomod"
)

type goModCheck struct {
	care.BaseCheck
	tool care.Tool
}

var _ care.Dependencies = (*goModCheck)(nil)

// NewGoMod is the Dependencies feature for Go: it reports whether `go mod tidy`
// would change go.mod/go.sum and any replace directives present, as one rolled-up
// result. In fix mode (DepsRunOptions.Fix) it applies `go mod tidy` for real instead
// of running the non-mutating tidy check. (The go/toolchain directive minimality
// check is a separate feature, NewRuntime.)
func NewGoMod(tool care.Tool) care.Dependencies {
	return &goModCheck{BaseCheck: care.NewBaseCheck("go-mod", tool), tool: tool}
}

func (f *goModCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *goModCheck) Run(ctx context.Context, dir string, opts care.RunOptions) care.Output[care.DepsReport] {
	var issues []care.DepIssue

	if opts.Dependencies.Fix {
		if out, err := f.tool.Exec(ctx, dir, "mod", "tidy"); err != nil {
			return care.Errored[care.DepsReport]("tool failed", fmt.Errorf("failed to run go mod tidy: %w\n%s", err, trimOutput(out)))
		}
	} else {
		diff, err := f.checkTidy(ctx, dir)
		if err != nil {
			return care.Errored[care.DepsReport]("tool failed", err)
		}
		if len(diff) > 0 {
			issues = append(issues, care.DepIssue{Check: "tidy", Detail: "go.mod/go.sum not tidy"})
			for _, d := range diff {
				issues = append(issues, care.DepIssue{Check: "tidy", Detail: d})
			}
		}
	}

	replaces, err := gomod.ReplaceDirectives(dir)
	if err != nil {
		return care.Errored[care.DepsReport]("read failed", fmt.Errorf("failed to read replace directives: %w", err))
	}
	for _, d := range replaces {
		issues = append(issues, care.DepIssue{Check: "replace", Detail: d})
	}

	// go mod verify checks the module cache against go.sum offline; anything but the
	// "all modules verified" line means a dependency was tampered with.
	if out, err := f.tool.Exec(ctx, dir, "mod", "verify"); err != nil || !strings.Contains(string(out), "all modules verified") {
		issues = append(issues, care.DepIssue{Check: "verify", Detail: firstLine(out)})
	}

	report := care.DepsReport{Issues: issues}
	// what the graph demands: the runtime-version floor the dependencies force, read
	// from the local module cache (never downloaded), plus the per-dep table.
	if floor, err := gomod.ReadDepFloor(dir, goModCache(ctx, f.tool, dir)); err == nil {
		report.RuntimeFloor = floor.Version
		report.RuntimeFloorBy = floor.Module
		report.Deps = runtimeDeps(floor.Deps)
	}

	if len(issues) > 0 {
		return care.Fail(report)
	}
	return care.Pass(report)
}

// checkTidy returns the changes `go mod tidy` would make to go.mod/go.sum as
// human-readable diff lines (empty when already tidy). It runs tidy against a
// snapshot it restores afterward so the check never mutates the repo.
func (f *goModCheck) checkTidy(ctx context.Context, dir string) ([]string, error) {
	modPath := filepath.Join(dir, "go.mod")
	sumPath := filepath.Join(dir, "go.sum")
	beforeMod, beforeSum, err := snapshot(modPath, sumPath)
	if err != nil {
		return nil, err
	}
	out, runErr := f.tool.Exec(ctx, dir, "mod", "tidy")
	restoreErr := restore(modPath, beforeMod, sumPath, beforeSum)
	if runErr != nil {
		return nil, fmt.Errorf("failed to run go mod tidy: %w\n%s", runErr, trimOutput(out))
	}
	if restoreErr != nil {
		return nil, fmt.Errorf("failed to restore go.mod/go.sum after tidy: %w", restoreErr)
	}
	afterMod, afterSum, err := snapshot(modPath, sumPath)
	if err != nil {
		return nil, err
	}
	return tidyDiff(beforeMod, afterMod, beforeSum, afterSum), nil
}

// tidyDiff describes what `go mod tidy` would change: each added/removed go.mod
// line (where the dependency edits are legible) as "+ ..."/"- ...", plus a
// one-line tally of go.sum entry churn (which is too noisy to list in full).
func tidyDiff(beforeMod, afterMod, beforeSum, afterSum []byte) []string {
	diff := lineDelta(beforeMod, afterMod)
	added, removed := countDelta(beforeSum, afterSum)
	if added > 0 || removed > 0 {
		diff = append(diff, fmt.Sprintf("go.sum: +%d / -%d entries", added, removed))
	}
	return diff
}

// lineDelta returns the meaningful go.mod line changes between before and after
// as "+ line" (added) and "- line" (removed), skipping blank lines.
func lineDelta(before, after []byte) []string {
	old := lineSet(before)
	cur := lineSet(after)
	var out []string
	for _, l := range splitLines(after) {
		if l != "" && !old[l] {
			out = append(out, "+ "+l)
		}
	}
	for _, l := range splitLines(before) {
		if l != "" && !cur[l] {
			out = append(out, "- "+l)
		}
	}
	return out
}

func countDelta(before, after []byte) (added, removed int) {
	old := lineSet(before)
	cur := lineSet(after)
	for _, l := range splitLines(after) {
		if l != "" && !old[l] {
			added++
		}
	}
	for _, l := range splitLines(before) {
		if l != "" && !cur[l] {
			removed++
		}
	}
	return added, removed
}

func splitLines(b []byte) []string {
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return lines
}

func lineSet(b []byte) map[string]bool {
	set := map[string]bool{}
	for _, l := range splitLines(b) {
		if l != "" {
			set[l] = true
		}
	}
	return set
}

func snapshot(modPath, sumPath string) (mod, sum []byte, err error) {
	mod, err = os.ReadFile(modPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read go.mod: %w", err)
	}
	sum, err = os.ReadFile(sumPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to read go.sum: %w", err)
	}
	return mod, sum, nil
}

func restore(modPath string, mod []byte, sumPath string, sum []byte) error {
	if err := os.WriteFile(modPath, mod, 0o644); err != nil { //nolint:gosec // go.mod must be world-readable
		return fmt.Errorf("failed to restore go.mod: %w", err)
	}
	if sum == nil {
		_ = os.Remove(sumPath)
		return nil
	}
	if err := os.WriteFile(sumPath, sum, 0o644); err != nil { //nolint:gosec // go.sum must be world-readable
		return fmt.Errorf("failed to restore go.sum: %w", err)
	}
	return nil
}
