package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/toaweme/mend"
)

type secrets struct {
	mend.BaseCheck
	tool    mend.Tool
	history bool
}

var _ mend.Secrets = (*secrets)(nil)

// NewBetterleaks scans a repo for leaked secrets via the injected betterleaks tool.
// history scans git history too (the default is the working tree only). It is
// language-agnostic: betterleaks works on any repository.
func NewBetterleaks(tool mend.Tool, history bool) mend.Secrets {
	return &secrets{
		BaseCheck: mend.NewBaseCheck("betterleaks", tool),
		tool:      tool,
		history:   history,
	}
}

func (f *secrets) Applies(string) bool { return true }

func (f *secrets) Run(ctx context.Context, dir string, _ mend.RunOptions) mend.Output[mend.SecretReport] {
	report, err := os.CreateTemp("", "mend-betterleaks-*.json")
	if err != nil {
		return mend.Errored[mend.SecretReport]("setup failed", fmt.Errorf("failed to create betterleaks report file: %w", err))
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
		return mend.Pass(mend.SecretReport{})
	}
	findings := parseBetterleaksJSON(reportPath)
	if len(findings) == 0 {
		return mend.Errored[mend.SecretReport]("tool failed", fmt.Errorf("failed to run betterleaks: %w", err))
	}
	return mend.Fail(mend.SecretReport{Findings: findings})
}

// parseBetterleaksJSON reads betterleaks' JSON report file into structured
// findings. A missing or unparseable report yields nil so the caller falls back
// to a note.
func parseBetterleaksJSON(path string) []mend.SecretFinding {
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
	findings := make([]mend.SecretFinding, 0, len(raw))
	for _, r := range raw {
		findings = append(findings, mend.SecretFinding{
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
