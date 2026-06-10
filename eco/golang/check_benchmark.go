package golang

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/eco/golang/inspect"
)

type benchmarkCheck struct {
	mend.BaseCheck
	tool     mend.Tool
	profiles []mend.RunProfile
}

var (
	_ mend.Benchmark = (*benchmarkCheck)(nil)
	_ mend.Profiled  = (*benchmarkCheck)(nil)
)

// NewBenchmark runs the repo's benchmarks (`go test -bench`) through the injected
// go tool and reports each benchmark's throughput and allocations under the active
// run-profile (a profile can vary -benchtime, -count, -cpu, build tags). profiles
// configures the named flag-sets to run; an empty list runs once with defaults.
// Benchmark numbers never fail the run; only a benchmark run that errors out does.
//
// `go test -bench` has no machine-readable form (even under -json the figures
// arrive as text in Output events), so this is the one feature that parses tool
// stdout rather than structured JSON.
func NewBenchmark(tool mend.Tool, profiles []mend.RunProfile) mend.Benchmark {
	return &benchmarkCheck{BaseCheck: mend.NewBaseCheck("go-test-bench", tool), tool: tool, profiles: profiles}
}

func (f *benchmarkCheck) Applies(dir string) bool { return hasGoMod(dir) }

// Profiles returns the configured benchmark profiles, or a single default.
func (f *benchmarkCheck) Profiles() []mend.RunProfile {
	if len(f.profiles) == 0 {
		return []mend.RunProfile{{Name: "default"}}
	}
	return f.profiles
}

func (f *benchmarkCheck) Run(ctx context.Context, dir string, opts mend.RunOptions) mend.Output[mend.BenchReport] {
	out, err := f.tool.Exec(ctx, dir, benchArgs(opts.Profile)...)
	results := parseBenchOutput(out)
	if err != nil && len(results) == 0 {
		return mend.Errored[mend.BenchReport]("tool failed", fmt.Errorf("failed to run benchmarks in %q: %w\n%s", dir, err, trimOutput(out)))
	}
	if len(results) == 0 {
		return mend.Skip[mend.BenchReport]("no benchmarks")
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Package != results[j].Package {
			return results[i].Package < results[j].Package
		}
		return results[i].Name < results[j].Name
	})
	report := mend.BenchReport{Benchmarks: results}
	if mod, err := inspect.ReadModulePath(dir); err == nil {
		report.ModulePath = mod
	}
	return mend.Pass(report)
}

// benchArgs builds the `go test -bench` invocation for a run-profile: always
// -benchmem, plus the profile's benchtime/count/cpu/tags and any raw extra flags.
func benchArgs(p mend.RunProfile) []string {
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
func parseBenchOutput(out []byte) []mend.BenchResult {
	var results []mend.BenchResult
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
		res := mend.BenchResult{Name: name, Package: pkg, Runs: runs}
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
				// line; keep them rather than drop them, since mend is general-purpose
				// and a given repo's benchmarks may report any of them.
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					res.Extra = append(res.Extra, mend.BenchMetric{Unit: unit, Value: v})
				}
			}
		}
		results = append(results, res)
	}
	return results
}
