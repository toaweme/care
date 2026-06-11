package golang

import (
	"context"
	"fmt"
	"go/version"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/eco/golang/gomod"
	"github.com/toaweme/mend/eco/golang/minver"
)

type runtimeCheck struct {
	mend.BaseCheck
	tool   mend.Tool
	policy RuntimePolicy
}

var _ mend.Runtime = (*runtimeCheck)(nil)

// NewRuntime is the Runtime feature for Go: it reports the module's `go` directive
// against the lowest it could declare, max(code floor, dependency floor). Go is
// backwards-compatible, so it fills only the minimum; there is no maximum. The
// dependency floor is read from the local module cache (never downloaded) and the
// code floor from the minver scan; both stay silent when they cannot be authoritative.
// policy governs the verdict when the directive is above the minimum: warn
// (default), strict (fail), or off (skip).
func NewRuntime(tool mend.Tool, policy RuntimePolicy) mend.Runtime {
	return &runtimeCheck{BaseCheck: mend.NewBaseCheck("go-runtime", tool), tool: tool, policy: policy}
}

func (f *runtimeCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *runtimeCheck) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.RuntimeReport] {
	if f.policy == RuntimeOff {
		return mend.Skip[mend.RuntimeReport]("disabled")
	}
	d, err := gomod.ReadDirectives(dir)
	if err != nil {
		return mend.Errored[mend.RuntimeReport]("read failed", fmt.Errorf("failed to read go.mod directives: %w", err))
	}
	report := mend.RuntimeReport{Declared: d.GoVersion, Toolchain: d.Toolchain}

	if floor, err := gomod.ReadDepFloor(dir, f.modCache(ctx, dir)); err == nil {
		report.DepMin = floor.Version
		report.DepModule = floor.Module
		report.CacheComplete = floor.Missing == 0
		report.Deps = runtimeDeps(floor.Deps)
	}
	if codeVer, reason, ok := codeFloor(ctx, dir); ok {
		report.CodeMin = codeVer
		report.CodeReason = reason
	}
	report.Minimum = maxGoVer(report.CodeMin, report.DepMin)
	report.ToolchainNote = toolchainNote(d)

	// reducible only when provable: a complete cache (the dependency floor is exact,
	// not a lower bound) and a known code floor, with the directive above their max.
	report.Reducible = report.CacheComplete && report.CodeMin != "" &&
		report.Minimum != "" && goLess(report.Minimum, report.Declared)

	switch {
	case report.Reducible && f.policy == RuntimeStrict:
		return mend.Fail(report)
	case report.Reducible || report.ToolchainNote != "":
		return mend.Warn(report)
	default:
		return mend.Pass(report)
	}
}

// runtimeDeps maps the cache-read dependency list to the report shape, sorted by
// declared version descending (the floor-setting deps first) then module name, so
// the verbose view leads with what constrains the floor.
func runtimeDeps(deps []gomod.DepGo) []mend.RuntimeDep {
	out := make([]mend.RuntimeDep, 0, len(deps))
	for _, d := range deps {
		out = append(out, mend.RuntimeDep{Module: d.Module, Version: d.Version, Min: d.Go})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Min != out[j].Min {
			return version.Compare("go"+out[i].Min, "go"+out[j].Min) > 0
		}
		return out[i].Module < out[j].Module
	})
	return out
}

// toolchainNote describes a `toolchain` directive that sits at or above the `go`
// directive's minor (the reappearing-under-GOTOOLCHAIN=auto case), or "" when there
// is no toolchain line or it is genuinely older.
func toolchainNote(d gomod.Directives) string {
	if d.Toolchain == "" {
		return ""
	}
	tcMin, ok := goMinor(d.Toolchain)
	if !ok {
		return ""
	}
	goMin, ok := goMinor(d.GoVersion)
	if !ok {
		return ""
	}
	switch {
	case tcMin > goMin:
		return "raises the build floor; `go get toolchain@none` + GOTOOLCHAIN=local stops it reappearing"
	case tcMin == goMin:
		return "redundant; remove with `go get toolchain@none`"
	default:
		return ""
	}
}

// codeFloor runs the minver scan and returns the lowest Go version the module's own
// code can declare ("1.N") with the construct that forces it. ok is false when the
// scan cannot run ($GOROOT/api absent, or the module does not type-check).
func codeFloor(ctx context.Context, dir string) (ver, reason string, ok bool) {
	hist, err := minver.LoadHistory()
	if err != nil {
		return "", "", false
	}
	res, err := minver.NewScanner(hist).ScanDir(ctx, dir)
	if err != nil {
		return "", "", false
	}
	ver = fmt.Sprintf("1.%d", res.Min)
	reason = "the code uses nothing newer than Go " + ver
	if len(res.Reasons) > 0 {
		r := res.Reasons[0]
		reason = "highest requirement: " + r.Desc
		if r.Pos != "" {
			reason += " at " + filepath.Base(r.Pos)
		}
	}
	return ver, reason, true
}

// modCache resolves the module cache directory via the injected go tool, falling
// back to $GOMODCACHE. An empty result makes ReadDepFloor count every dep as missing,
// which keeps the check from claiming a (then-unprovable) reducible directive.
func (f *runtimeCheck) modCache(ctx context.Context, dir string) string {
	out, err := f.tool.ExecStdout(ctx, dir, "env", "GOMODCACHE")
	if err != nil {
		return os.Getenv("GOMODCACHE")
	}
	return strings.TrimSpace(string(out))
}

// maxGoVer returns the higher of two Go version strings, treating "" as the lowest.
func maxGoVer(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case version.Compare("go"+a, "go"+b) >= 0:
		return a
	default:
		return b
	}
}

// goLess reports whether Go version a is lower than b.
func goLess(a, b string) bool { return version.Compare("go"+a, "go"+b) < 0 }

// goMinor extracts the minor version N from a Go version string ("1.25.0", "1.25"
// or "go1.26.0" -> 25/25/26). It returns false for anything not on the 1.x line.
func goMinor(v string) (int, bool) {
	v = strings.TrimPrefix(v, "go")
	parts := strings.Split(v, ".")
	if len(parts) < 2 || parts[0] != "1" {
		return 0, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// RuntimePolicy controls how the Runtime check reacts when the declared version is
// higher than the module's true minimum.
type RuntimePolicy int

const (
	// RuntimeWarn reports a non-minimal directive as an advisory WARN. Default.
	RuntimeWarn RuntimePolicy = iota
	// RuntimeStrict fails the check when the directive is above the minimum, for
	// modules (typically dependency-light libraries) that must declare the lowest
	// version they support so they do not over-constrain consumers.
	RuntimeStrict
	// RuntimeOff disables the check entirely.
	RuntimeOff
)

// ParseRuntimePolicy maps a config string to a policy, defaulting to warn. It
// accepts strict/enforce/fail for strict and off/none/false for off.
func ParseRuntimePolicy(s string) RuntimePolicy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict", "enforce", "fail", "true":
		return RuntimeStrict
	case "off", "none", "false", "disable", "disabled":
		return RuntimeOff
	default:
		return RuntimeWarn
	}
}
