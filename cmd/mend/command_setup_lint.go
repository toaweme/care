package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/toaweme/cli"

	"github.com/toaweme/mend/cmd/mend/output"
	"github.com/toaweme/mend/eco/golang"
)

// SetupLintFlags configures the lint scaffolding subcommand.
type SetupLintFlags struct {
	Force              bool     `arg:"force" short:"f" env:"MEND_SETUP_FORCE" default:"false" help:"Write the config even when one already governs the repo"`
	ImportSortPrefixes []string `arg:"isp" short:"i" sep:"," env:"MEND_SETUP_IMPORT_SORT_PREFIXES" help:"Import path prefixes grouped right after stdlib (goimports local-prefixes); comma-separated. Defaults to the repo's module path"`
}

// SetupLintCommand writes the canonical golangci-lint config into the current
// repository. When a config already governs the dir (here or in a parent, as
// golangci-lint resolves upward) it reports and skips unless --force is passed.
type SetupLintCommand struct {
	cli.BaseCommand[SetupLintFlags]
	// module resolves the repo's module path so the goimports local-prefixes
	// placeholder can be pinned to it. Returns "" when the dir is not a Go module.
	module func(dir string) string
}

var _ cli.Command[SetupLintFlags] = (*SetupLintCommand)(nil)

func NewSetupLintCommand(module func(dir string) string) *SetupLintCommand {
	return &SetupLintCommand{module: module}
}

func (c *SetupLintCommand) Help() string {
	return "Write the canonical golangci-lint config (.golangci.yml) into the current repo; reports and skips when one already governs the dir (--force to overwrite)."
}

func (c *SetupLintCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	dir := options.Cwd

	if existing, found := golang.FindGolangciConfig(dir); found && !c.Inputs.Force {
		fmt.Printf("%s golangci-lint config already governs this repo: %s\n", output.WarnStyle.Render("•"), existing)
		fmt.Printf("%s\n", output.DimStyle.Render("pass --force to overwrite the local config"))
		return nil
	}

	// explicit --isp wins; otherwise fall back to the repo's module path, leaving the
	// block empty (dropped) when neither is available.
	prefixes := c.Inputs.ImportSortPrefixes
	if len(prefixes) == 0 && c.module != nil {
		if module := c.module(dir); module != "" {
			prefixes = []string{module}
		}
	}

	content, err := golang.RenderGolangciConfig(prefixes)
	if err != nil {
		return fmt.Errorf("failed to render golangci-lint config: %w", err)
	}

	dst := filepath.Join(dir, golang.GolangciConfigName)
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return fmt.Errorf("failed to write golangci-lint config: %w", err)
	}

	fmt.Printf("%s wrote %s\n", output.OKStyle.Render("✓"), dst)
	if len(prefixes) > 0 {
		fmt.Printf("%s\n", output.DimStyle.Render("import-sort prefixes: "+strings.Join(prefixes, ", ")))
	}
	return nil
}
