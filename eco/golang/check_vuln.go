package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/toaweme/mend"
)

type vulnCheck struct {
	mend.BaseCheck
	tool mend.Tool
	db   string
}

var _ mend.Vulnerabilities = (*vulnCheck)(nil)

// NewGovulncheck reports vulnerabilities the code is affected by, via the injected
// govulncheck tool. db overrides the vulnerability database URL.
func NewGovulncheck(tool mend.Tool, db string) mend.Vulnerabilities {
	return &vulnCheck{
		BaseCheck: mend.NewBaseCheck("govulncheck", tool),
		tool:      tool,
		db:        db,
	}
}

func (f *vulnCheck) Applies(dir string) bool { return hasGoMod(dir) }

func (f *vulnCheck) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.VulnReport] {
	args := []string{"-json"}
	if f.db != "" {
		args = append(args, "-db", f.db)
	}
	args = append(args, "./...")
	out, err := f.tool.ExecStdout(ctx, dir, args...)
	findings := parseGovulncheckJSON(out)
	if len(findings) > 0 {
		return mend.Fail(mend.VulnReport{Findings: findings})
	}
	if err != nil {
		return mend.Errored[mend.VulnReport]("tool failed", fmt.Errorf("failed to run govulncheck: %w\n%s", err, trimOutput(out)))
	}
	return mend.Pass(mend.VulnReport{})
}

// parseGovulncheckJSON distills govulncheck's `-json` message stream into one
// VulnFinding per OSV id, keeping only vulnerabilities the code actually calls (a
// finding with a symbol-level trace frame). Imports-only findings, which
// govulncheck reports separately as non-actionable, are dropped so the report
// matches govulncheck's "your code is affected by" set.
func parseGovulncheckJSON(out []byte) []mend.VulnFinding {
	dec := json.NewDecoder(bytes.NewReader(out))
	summaries := make(map[string]string)
	called := make(map[string]mend.VulnFinding)
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
		trace := make([]mend.VulnFrame, 0, len(msg.Finding.Trace))
		for _, fr := range msg.Finding.Trace {
			trace = append(trace, mend.VulnFrame{Module: fr.Module, Version: fr.Version, Package: fr.Package, Function: fr.Function})
		}
		called[msg.Finding.OSV] = mend.VulnFinding{
			ID: msg.Finding.OSV, Package: t.Package, Found: t.Version, Fixed: msg.Finding.FixedVersion, Symbol: t.Function, Trace: trace,
		}
	}
	findings := make([]mend.VulnFinding, 0, len(called))
	for id, vf := range called {
		vf.Summary = summaries[id]
		findings = append(findings, vf)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	return findings
}
