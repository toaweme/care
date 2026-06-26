package output

import (
	"testing"
	"time"

	"github.com/toaweme/care"
	"github.com/toaweme/care/internal/rating"
)

// Test_BuildJSON_SingleRepo checks the single-repo wire shape: a flat check list
// under one header (author + created), each check keyed by its type/feature, install
// outputs lifted into Tools, and a check's typed payload carried as Data.
func Test_BuildJSON_SingleRepo(t *testing.T) {
	outputs := []care.Rendered{
		care.InstallResult("golangci-lint", care.StatusOK, "present"),
		care.Result(care.FeatureLint, "golangci-lint", "/src/repoA", care.StatusFail,
			care.QualityReport{Issues: []care.QualityIssue{{File: "main.go", Line: 3, Col: 1, Message: "ineffassign", Linter: "ineffassign"}}}),
		care.Result(care.FeatureDependencies, "go-mod", "/src/repoA", care.StatusOK, care.DepsReport{}),
	}

	created := time.Date(2026, 6, 9, 14, 23, 1, 0, time.UTC)
	rep := buildJSON(outputs, RunInfo{Created: created, Repo: "/src/repoA"}, rating.Default())
	if rep.Author != "care" || rep.Created != "2026-06-09T14:23:01Z" {
		t.Fatalf("header = author %q created %q, want care / RFC3339", rep.Author, rep.Created)
	}
	if len(rep.Tools) != 1 || rep.Tools[0].Install != "present" {
		t.Fatalf("tools = %+v, want one present golangci-lint", rep.Tools)
	}
	if rep.Dir != "/src/repoA" || len(rep.Checks) != 2 {
		t.Fatalf("repo = %q with %d checks, want /src/repoA with 2", rep.Dir, len(rep.Checks))
	}
	if rep.Health.OK != 1 || rep.Health.Fail != 1 {
		t.Fatalf("health tally = %d ok %d fail, want 1 ok 1 fail", rep.Health.OK, rep.Health.Fail)
	}
	// lint (weight 20) failed, dependencies (weight 8) passed: weighted average is
	// 8*100/(20+8) = 29, no critical cap, a failing grade.
	if rep.Health.Score != 29 || rep.Health.Rating != "F" {
		t.Fatalf("health grade = %d/%s, want 29/F", rep.Health.Score, rep.Health.Rating)
	}
	lint := rep.Checks[0]
	if lint.Type != "quality" || lint.Feature != care.FeatureLint || lint.Status != "FAIL" {
		t.Errorf("lint check = %+v, want FAIL lint under quality", lint)
	}
	data, ok := lint.Data.(care.QualityReport)
	if !ok || len(data.Issues) != 1 || data.Issues[0].File != "main.go" {
		t.Errorf("lint data = %+v, want one main.go issue", lint.Data)
	}
}

// Test_ToolID checks the tools-array identity: the resolved version embeds in the
// tool name, and a versionless tool stays bare.
func Test_ToolID(t *testing.T) {
	if got := toolID("betterleaks", "8.18.2"); got != "betterleaks:8.18.2" {
		t.Errorf("toolID with version = %q, want betterleaks:8.18.2", got)
	}
	if got := toolID("git", ""); got != "git" {
		t.Errorf("toolID without version = %q, want git", got)
	}
}

// Test_SourceID checks the installer-to-wire mapping: go-install reads as goinstall,
// brew and builtin pass through.
func Test_SourceID(t *testing.T) {
	cases := map[string]string{"go-install": "goinstall", "brew": "brew", "builtin": "builtin", "": ""}
	for in, want := range cases {
		if got := sourceID(in); got != want {
			t.Errorf("sourceID(%q) = %q, want %q", in, got, want)
		}
	}
}

// Test_BuildJSON_Module checks that the repo header carries the caller-resolved
// module, independent of any check payload.
func Test_BuildJSON_Module(t *testing.T) {
	outputs := []care.Rendered{
		care.Result(care.FeatureTests, "go-test", "/src/repoA", care.StatusOK,
			care.TestReport{ModulePath: "example.com/a", WithCoverage: true,
				Suites: []care.TestSuite{{Name: "example.com/a", Passed: true, Coverage: 87.5}}}),
	}

	rep := buildJSON(outputs, RunInfo{Repo: "/src/repoA", Module: "example.com/a"}, rating.Default())
	if rep.Dir != "/src/repoA" || rep.Module != "example.com/a" || len(rep.Checks) != 1 {
		t.Fatalf("report = %+v, want repoA module example.com/a with 1 check", rep)
	}
	data, ok := rep.Checks[0].Data.(care.TestReport)
	if !ok || !data.WithCoverage || data.Suites[0].Coverage != 87.5 {
		t.Errorf("test data = %+v, want coverage 87.5", rep.Checks[0].Data)
	}
}
