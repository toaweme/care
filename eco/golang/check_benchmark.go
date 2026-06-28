package golang

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/toaweme/care"
	"github.com/toaweme/care/eco/golang/inspect"
)

type benchmarkCheck struct {
	care.BaseCheck
	tool     care.Tool
	profiles []care.RunProfile
}

var (
	_ care.Benchmark = (*benchmarkCheck)(nil)
	_ care.Profiled  = (*benchmarkCheck)(nil)
)

// NewBenchmark runs the repo's benchmarks (`go test -bench`) through the injected go tool
// and reports each benchmark's throughput and allocations under the active run-profile.
// A profile can vary -benchtime, -count, -cpu, and build tags; profiles selects the named
// flag-sets to run, and an empty list runs once with defaults. Benchmark numbers never fail
// the run; only a benchmark run that errors out does.
//
// `go test -bench` has no machine-readable form (even under -json the figures arrive as
// text in Output events), so this is the one feature that parses tool stdout rather than
// structured JSON.
func NewBenchmark(tool care.Tool, profiles []care.RunProfile) care.Benchmark {
	return &benchmarkCheck{BaseCheck: care.NewBaseCheck("go-test-bench", tool), tool: tool, profiles: profiles}
}

// Applies skips a repo with no benchmark functions outright. Otherwise `go test -bench`
// compiles every test binary just to discover there is nothing to run, billing a
// multi-second SKIP, so benchmarks are detected by a cheap source scan (no compilation)
// rather than by running the tool.
func (f *benchmarkCheck) Applies(dir string) bool {
	return hasGoMod(dir) && hasBenchmarks(dir)
}

// benchFuncRe matches a top-level benchmark function declaration. Benchmarks must be
// `func BenchmarkXxx(...)` at file scope, so anchoring to the start of a line keeps the
// indented occurrences inside comments or strings from matching.
var benchFuncRe = regexp.MustCompile(`(?m)^func Benchmark`)

// errStopWalk halts the directory walk as soon as the first benchmark is found.
var errStopWalk = errors.New("benchmark found")

// hasBenchmarks reports whether any _test.go file under dir declares a benchmark. It
// walks the tree (skipping vendor, testdata, and dot dirs) and stops at the first hit,
// reading files but never compiling, so it is cheap even on a large module.
func hasBenchmarks(dir string) bool {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // an unreadable entry just isn't a benchmark
		}
		if d.IsDir() {
			if name := d.Name(); path != dir && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// this scans the working tree of a repo the operator pointed care at, where a
		// symlink TOCTOU race has no attacker and no privilege to escalate to.
		//nolint:gosec // G122: read-only local scan of a trusted working tree
		if b, err := os.ReadFile(path); err == nil && benchFuncRe.Match(b) {
			return errStopWalk
		}
		return nil
	})
	return errors.Is(err, errStopWalk)
}

// Profiles returns the configured benchmark profiles, or a single default.
func (f *benchmarkCheck) Profiles() []care.RunProfile {
	if len(f.profiles) == 0 {
		return []care.RunProfile{{Name: "default"}}
	}
	return f.profiles
}

func (f *benchmarkCheck) Run(ctx context.Context, dir string, opts care.RunOptions) care.Output[care.BenchReport] {
	out, err := f.tool.Exec(ctx, dir, benchArgs(opts.Profile)...)
	results := parseBenchOutput(out)
	if err != nil && len(results) == 0 {
		return care.Errored[care.BenchReport]("tool failed", fmt.Errorf("failed to run benchmarks in %q: %w\n%s", dir, err, trimOutput(out)))
	}
	if len(results) == 0 {
		return care.Skip[care.BenchReport]("no benchmarks")
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Package != results[j].Package {
			return results[i].Package < results[j].Package
		}
		return results[i].Name < results[j].Name
	})
	report := care.BenchReport{Benchmarks: results}
	if mod, err := inspect.ReadModulePath(dir); err == nil {
		report.ModulePath = mod
	}
	return care.Pass(report)
}

// benchArgs builds the `go test -bench` invocation for a run-profile: always
// -benchmem, plus the profile's benchtime/count/cpu/tags and any raw extra flags.
func benchArgs(p care.RunProfile) []string {
	args := []string{"test", "-run", "^$", "-bench", ".", "-benchmem"}
	if p.Benchtime != "" {
		args = append(args, "-benchtime", p.Benchtime)
	}
	if p.Count > 0 {
		args = append(args, "-count", strconv.Itoa(p.Count))
	}
	if p.CPU != "" {
		args = append(args, "-cpu", p.CPU)
	}
	if p.Tags != "" {
		args = append(args, "-tags", p.Tags)
	}
	args = append(args, p.Args...)
	args = append(args, "./...")
	return args
}

// parseBenchOutput distills `go test -bench -benchmem` output into one BenchResult
// per benchmark line, tracking the current package from the `pkg:` header lines
// that precede each package's results.
func parseBenchOutput(out []byte) []care.BenchResult {
	var results []care.BenchResult
	var pkg string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if rest, ok := strings.CutPrefix(line, "pkg:"); ok {
			pkg = strings.TrimSpace(rest)
			continue
		}
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		runs, err := strconv.Atoi(fields[1])
		if err != nil {
			continue // not a result row
		}
		name := fields[0]
		// strip the trailing "-GOMAXPROCS" suffix go appends to benchmark names.
		if i := strings.LastIndex(name, "-"); i > 0 {
			if _, err := strconv.Atoi(name[i+1:]); err == nil {
				name = name[:i]
			}
		}
		res := care.BenchResult{Name: name, Package: pkg, Runs: runs}
		for i := 2; i+1 < len(fields); i += 2 {
			val, unit := fields[i], fields[i+1]
			switch unit {
			case "ns/op":
				res.NsPerOp, _ = strconv.ParseFloat(val, 64)
			case "B/op":
				res.BytesPerOp, _ = strconv.Atoi(val)
			case "allocs/op":
				res.AllocsPerOp, _ = strconv.Atoi(val)
			default:
				// MB/s (b.SetBytes) and custom b.ReportMetric units ride the same
				// line; keep them rather than drop them, since care is general-purpose
				// and a given repo's benchmarks may report any of them.
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					res.Extra = append(res.Extra, care.BenchMetric{Unit: unit, Value: v})
				}
			}
		}
		results = append(results, res)
	}
	return results
}
