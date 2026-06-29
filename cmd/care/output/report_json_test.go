package output

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/toaweme/care"
)

// Test_BuildJSON_SingleRepo checks the single-repo wire shape: a flat check list under one
// header (author + created), each check keyed by its type/feature, install outputs lifted into
// Tools, and a check's typed payload carried as Data.
func Test_BuildJSON_SingleRepo(t *testing.T) {
	outputs := []care.Rendered{
		care.InstallResult("golangci-lint", care.StatusOK, "present"),
		care.Result(care.FeatureLint, "golangci-lint", "/src/repoA", care.StatusFail,
			care.QualityReport{Issues: []care.QualityIssue{{File: "main.go", Line: 3, Col: 1, Message: "ineffassign", Linter: "ineffassign"}}}).WithWeight(20),
		care.Result(care.FeatureDependencies, "go-mod", "/src/repoA", care.StatusOK, care.DepsReport{}).WithWeight(8),
	}

	created := time.Date(2026, 6, 9, 14, 23, 1, 0, time.UTC)
	rep := buildJSON(outputs, RunInfo{Created: created, Repo: "/src/repoA"})
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

// Test_BuildJSON_ErroredCheck checks that a tool-failure outcome carries its
// underlying error into the wire (the error field), so a consumer sees why a check
// errored rather than a bare FAIL with no payload.
func Test_BuildJSON_ErroredCheck(t *testing.T) {
	outputs := []care.Rendered{
		care.ErroredResult[care.VulnReport](care.FeatureVulnerabilities, "govulncheck", "/src/repoA",
			"tool failed", errors.New("failed to run govulncheck: exit status 1\ngovulncheck: no required module provides package")),
	}
	rep := buildJSON(outputs, RunInfo{Repo: "/src/repoA"})
	if len(rep.Checks) != 1 {
		t.Fatalf("got %d checks, want 1", len(rep.Checks))
	}
	c := rep.Checks[0]
	if c.Status != "FAIL" || c.Data != nil {
		t.Errorf("errored check = status %q data %v, want FAIL with no data", c.Status, c.Data)
	}
	if !strings.Contains(c.Error, "failed to run govulncheck") || !strings.Contains(c.Error, "no required module") {
		t.Errorf("error field = %q, want the wrapped govulncheck failure", c.Error)
	}
}

// Test_RenderPretty_ErroredCheckShowsError checks the default (verbosity 0) pretty
// output surfaces an errored check's underlying error beneath the summary, so
// "tool failed" is actionable without needing -vv.
func Test_RenderPretty_ErroredCheckShowsError(t *testing.T) {
	outputs := []care.Rendered{
		care.ErroredResult[care.VulnReport](care.FeatureVulnerabilities, "govulncheck", "/src/repoA",
			"tool failed", errors.New("failed to run govulncheck: exit status 1\ngovulncheck: no required module provides package")),
	}
	out := capture(t, func() {
		renderPretty(outputs, RunInfo{Repo: "/src/repoA"}, RenderOptions{Verbosity: 0})
	})
	if !strings.Contains(out, "tool failed") {
		t.Errorf("output missing the summary note:\n%s", out)
	}
	if !strings.Contains(out, "failed to run govulncheck") || !strings.Contains(out, "no required module") {
		t.Errorf("output missing the error detail at verbosity 0:\n%s", out)
	}
}

// Test_ToolID checks the tools-array identity: the resolved version embeds in the tool name,
// and a versionless tool stays bare.
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

// Test_BuildJSON_Module checks that the repo header carries the caller-resolved module,
// independent of any check payload.
func Test_BuildJSON_Module(t *testing.T) {
	outputs := []care.Rendered{
		care.Result(care.FeatureTests, "go-test", "/src/repoA", care.StatusOK,
			care.TestReport{ModulePath: "example.com/a", WithCoverage: true,
				Suites: []care.TestSuite{{Name: "example.com/a", Passed: true, Coverage: 87.5}}}),
	}

	rep := buildJSON(outputs, RunInfo{Repo: "/src/repoA", Module: "example.com/a"})
	if rep.Dir != "/src/repoA" || rep.Module != "example.com/a" || len(rep.Checks) != 1 {
		t.Fatalf("report = %+v, want repoA module example.com/a with 1 check", rep)
	}
	data, ok := rep.Checks[0].Data.(care.TestReport)
	if !ok || !data.WithCoverage || data.Suites[0].Coverage != 87.5 {
		t.Errorf("test data = %+v, want coverage 87.5", rep.Checks[0].Data)
	}
}
