package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/toaweme/care"
)

type vulnCheck struct {
	care.BaseCheck
	tool care.Tool
	db   string
}

var _ care.Vulnerabilities = (*vulnCheck)(nil)

// NewGovulncheck reports vulnerabilities the code is affected by, via the injected
// govulncheck tool. db overrides the vulnerability database URL.
func NewGovulncheck(tool care.Tool, db string) care.Vulnerabilities {
	return &vulnCheck{
		BaseCheck: care.NewBaseCheck("govulncheck", tool),
		tool:      tool,
		db:        db,
	}
}

func (f *vulnCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *vulnCheck) Run(ctx context.Context, dir string, _ care.RunOptions) care.Output[care.VulnReport] {
	args := []string{"-json"}
	if f.db != "" {
		args = append(args, "-db", f.db)
	}
	args = append(args, "./...")
	out, err := f.tool.ExecStdout(ctx, dir, args...)
	report := care.VulnReport{Findings: parseGovulncheckJSON(out, ModulePath(dir))}
	// govulncheck exits non-zero whenever it finds any vulnerability (toolchain
	// included), so a non-zero err alongside findings is the normal "found something"
	// signal, not a tool failure. Only an empty result with a non-zero err is a real
	// failure to run. Runtime (stdlib) findings are informational: they track the
	// installed Go toolchain, not the code, so they never fail the check.
	if report.Actionable() > 0 {
		return care.Fail(report)
	}
	if err != nil && len(report.Findings) == 0 {
		return care.Errored[care.VulnReport]("tool failed", fmt.Errorf("failed to run govulncheck: %w\n%s", err, trimOutput(out)))
	}
	return care.Pass(report)
}

// parseGovulncheckJSON distills govulncheck's `-json` message stream into one
// VulnFinding per OSV id, keeping only vulnerabilities the code actually calls (a
// finding with a symbol-level trace frame). Imports-only findings, which
// govulncheck reports separately as non-actionable, are dropped so the report
// matches govulncheck's "your code is affected by" set. Each kept finding is tagged
// by origin (mod is the caller's own module path): a stdlib finding is VulnRuntime
// (a toolchain property, informational), a finding in the module's own code is
// VulnCode, and everything else is VulnDeps.
func parseGovulncheckJSON(out []byte, mod string) []care.VulnFinding {
	dec := json.NewDecoder(bytes.NewReader(out))
	summaries := make(map[string]string)
	called := make(map[string]care.VulnFinding)
	for {
		var msg struct {
			OSV *struct {
				ID      string `json:"id"`
				Summary string `json:"summary"`
			} `json:"osv"`
			Finding *struct {
				OSV          string `json:"osv"`
				FixedVersion string `json:"fixed_version"`
				Trace        []struct {
					Module   string `json:"module"`
					Version  string `json:"version"`
					Package  string `json:"package"`
					Function string `json:"function"`
				} `json:"trace"`
			} `json:"finding"`
		}
		if err := dec.Decode(&msg); err != nil {
			break
		}
		if msg.OSV != nil {
			summaries[msg.OSV.ID] = msg.OSV.Summary
		}
		if msg.Finding == nil || len(msg.Finding.Trace) == 0 {
			continue
		}
		t := msg.Finding.Trace[0]
		if t.Function == "" {
			continue // imports-only finding; not actionable
		}
		trace := make([]care.VulnFrame, 0, len(msg.Finding.Trace))
		for _, fr := range msg.Finding.Trace {
			trace = append(trace, care.VulnFrame{Module: fr.Module, Version: fr.Version, Package: fr.Package, Function: fr.Function})
		}
		called[msg.Finding.OSV] = care.VulnFinding{
			ID: msg.Finding.OSV, Category: vulnCategory(t.Module, mod), Package: t.Package, Found: t.Version, Fixed: msg.Finding.FixedVersion, Symbol: t.Function, Trace: trace,
		}
	}
	findings := make([]care.VulnFinding, 0, len(called))
	for id, vf := range called {
		vf.Summary = summaries[id]
		findings = append(findings, vf)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	return findings
}

// vulnCategory classifies a finding by the module its affected symbol lives in:
// the Go stdlib is a toolchain (runtime) concern, the caller's own module (or a
// submodule of it) is own-code, and anything else is a third-party dependency.
func vulnCategory(module, mod string) string {
	switch {
	case module == "stdlib":
		return care.VulnRuntime
	case mod != "" && (module == mod || strings.HasPrefix(module, mod+"/")):
		return care.VulnCode
	default:
		return care.VulnDeps
	}
}
