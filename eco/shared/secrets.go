package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/toaweme/care"
	"github.com/toaweme/care/internal/devops/git"
)

type secrets struct {
	care.BaseCheck
	tool    care.Tool
	history bool
}

var _ care.Secrets = (*secrets)(nil)

// NewBetterleaks scans a repo for leaked secrets via the injected betterleaks tool.
// history scans git history too (the default is the working tree only). It is
// language-agnostic: betterleaks works on any repository.
func NewBetterleaks(tool care.Tool, history bool) care.Secrets {
	return &secrets{
		BaseCheck: care.NewBaseCheck("betterleaks", tool),
		tool:      tool,
		history:   history,
	}
}

func (f *secrets) Applies(string) bool { return true }

func (f *secrets) Run(ctx context.Context, dir string, _ care.RunOptions) care.Output[care.SecretReport] {
	report, err := os.CreateTemp("", "care-betterleaks-*.json")
	if err != nil {
		return care.Errored[care.SecretReport]("setup failed", fmt.Errorf("failed to create betterleaks report file: %w", err))
	}
	reportPath := report.Name()
	report.Close()
	defer os.Remove(reportPath)

	// betterleaks uses `git` to scan commit history and `dir` to scan only the
	// working tree; the positional "." is the repo (cmd runs in dir).
	scan := "dir"
	if f.history {
		scan = "git"
	}
	// --redact is a percent (Uint), not a bool, so it must carry a value;
	// --redact=100 fully masks secrets in the report.
	args := []string{scan, ".", "--no-banner", "--redact=100", "--report-format", "json", "--report-path", reportPath}
	_, err = f.tool.Exec(ctx, dir, args...)
	if err == nil {
		return care.Pass(care.SecretReport{})
	}
	findings := parseBetterleaksJSON(reportPath)
	if len(findings) == 0 {
		return care.Errored[care.SecretReport]("tool failed", fmt.Errorf("failed to run betterleaks: %w", err))
	}
	// a gitignored file (e.g. a local .env) is the correct home for secrets, not a
	// leak, so drop working-tree findings in ignored files. History scans keep
	// everything: a secret committed in the past is a real exposure even if the
	// file is gitignored at HEAD.
	if !f.history {
		findings = dropIgnored(ctx, dir, findings)
		if len(findings) == 0 {
			return care.Pass(care.SecretReport{})
		}
	}
	return care.Fail(care.SecretReport{Findings: findings})
}

// dropIgnored removes findings whose file git ignores in dir.
func dropIgnored(ctx context.Context, dir string, findings []care.SecretFinding) []care.SecretFinding {
	files := make([]string, 0, len(findings))
	for _, fnd := range findings {
		files = append(files, fnd.File)
	}
	ignored := git.IgnoredFiles(ctx, dir, files)
	if len(ignored) == 0 {
		return findings
	}
	kept := make([]care.SecretFinding, 0, len(findings))
	for _, fnd := range findings {
		if !ignored[fnd.File] {
			kept = append(kept, fnd)
		}
	}
	return kept
}

// parseBetterleaksJSON reads betterleaks' JSON report file into structured
// findings. A missing or unparseable report yields nil so the caller falls back
// to a note.
func parseBetterleaksJSON(path string) []care.SecretFinding {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw []struct {
		RuleID      string   `json:"RuleID"`
		Description string   `json:"Description"`
		File        string   `json:"File"`
		StartLine   int      `json:"StartLine"`
		Commit      string   `json:"Commit"`
		Entropy     float64  `json:"Entropy"`
		Tags        []string `json:"Tags"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	findings := make([]care.SecretFinding, 0, len(raw))
	for _, r := range raw {
		findings = append(findings, care.SecretFinding{
			Rule:        r.RuleID,
			Description: r.Description,
			File:        r.File,
			Line:        r.StartLine,
			Commit:      r.Commit,
			Entropy:     r.Entropy,
			Tags:        r.Tags,
		})
	}
	return findings
}
