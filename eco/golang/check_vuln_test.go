package golang

import (
	"testing"

	"github.com/toaweme/mend"
)

// govulncheck emits a stream of newline-delimited json objects. these fixtures
// mirror the osv/finding shape the parser decodes: an osv message naming the
// vulnerability, then one finding per call site (trace[0] is the affected symbol).
const (
	osvDep = `{"osv":{"id":"GO-2024-0001","summary":"a dependency bug"}}
{"finding":{"osv":"GO-2024-0001","fixed_version":"v1.2.3","trace":[{"module":"github.com/foo/bar","version":"v1.2.0","package":"github.com/foo/bar","function":"Vuln"}]}}`

	osvStdlib = `{"osv":{"id":"GO-2024-0002","summary":"a stdlib bug"}}
{"finding":{"osv":"GO-2024-0002","fixed_version":"go1.25.1","trace":[{"module":"stdlib","version":"go1.25.0","package":"net/http","function":"Serve"}]}}`

	osvImportsOnly = `{"osv":{"id":"GO-2024-0003","summary":"imported but not called"}}
{"finding":{"osv":"GO-2024-0003","trace":[{"module":"github.com/foo/baz","version":"v0.1.0","package":"github.com/foo/baz"}]}}`

	osvOwnCode = `{"osv":{"id":"GO-2024-0004","summary":"a bug in our own module"}}
{"finding":{"osv":"GO-2024-0004","fixed_version":"v2.0.0","trace":[{"module":"github.com/toaweme/mend","version":"v1.0.0","package":"github.com/toaweme/mend/eco","function":"Boom"}]}}`
)

func Test_ParseGovulncheckJSON(t *testing.T) {
	const mod = "github.com/toaweme/mend"
	type want struct {
		id  string
		cat string
	}
	tests := []struct {
		name string
		out  string
		want []want
	}{
		{name: "empty stream", out: "", want: nil},
		{name: "dependency finding tagged deps", out: osvDep, want: []want{{"GO-2024-0001", mend.VulnDeps}}},
		{name: "stdlib finding tagged runtime", out: osvStdlib, want: []want{{"GO-2024-0002", mend.VulnRuntime}}},
		{name: "imports-only finding dropped", out: osvImportsOnly, want: nil},
		{name: "own-module finding tagged code", out: osvOwnCode, want: []want{{"GO-2024-0004", mend.VulnCode}}},
		{
			name: "mixed: deps + runtime kept, imports-only dropped",
			out:  osvDep + "\n" + osvStdlib + "\n" + osvImportsOnly,
			want: []want{{"GO-2024-0001", mend.VulnDeps}, {"GO-2024-0002", mend.VulnRuntime}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := parseGovulncheckJSON([]byte(tt.out), mod)
			if len(findings) != len(tt.want) {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), len(tt.want), findings)
			}
			for i, w := range tt.want {
				if findings[i].ID != w.id {
					t.Errorf("finding[%d].ID = %q, want %q", i, findings[i].ID, w.id)
				}
				if findings[i].Category != w.cat {
					t.Errorf("finding[%d].Category = %q, want %q", i, findings[i].Category, w.cat)
				}
			}
		})
	}
}

func Test_VulnReport_ActionableAndRuntime(t *testing.T) {
	r := mend.VulnReport{Findings: []mend.VulnFinding{
		{ID: "a", Category: mend.VulnDeps},
		{ID: "b", Category: mend.VulnRuntime},
		{ID: "c", Category: mend.VulnCode},
		{ID: "d", Category: mend.VulnRuntime},
	}}
	if got := r.Actionable(); got != 2 {
		t.Errorf("Actionable() = %d, want 2", got)
	}
	if got := r.RuntimeVulns(); got != 2 {
		t.Errorf("RuntimeVulns() = %d, want 2", got)
	}
}
