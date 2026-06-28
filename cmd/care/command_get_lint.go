package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/toaweme/cli"
	"github.com/toaweme/http"

	"github.com/toaweme/care/cmd/care/output"
	"github.com/toaweme/care/eco/golang"
	"github.com/toaweme/care/eco/shared/sync"
)

// GetLintConfig configures the lint scaffolding subcommand: the shared sync flags plus the
// lint-only import-sort prefixes. With no --from the canonical golangci config is rendered
// from the bundled template; --from pulls it from a source verbatim (no placeholder expansion).
type GetLintConfig struct {
	GetConfig
	ImportSortPrefixes []string `arg:"isp" short:"i" sep:"," env:"CARE_GET_IMPORT_SORT_PREFIXES" help:"Import path prefixes grouped right after stdlib (goimports local-prefixes); comma-separated. Defaults to the repo's module path. Ignored with --from"`
}

// GetLintCommand writes a golangci-lint config into the current repository. Without --from it
// renders the canonical bundled config (expanding the goimports local-prefixes placeholder);
// with --from it syncs a config from a source verbatim. When a config already governs the dir
// (here or in a parent, as golangci-lint resolves upward) it reports and skips unless --force
// is passed.
type GetLintCommand struct {
	cli.BaseCommand[GetLintConfig]
	client http.Client
	// embed reads the bundled templates by name.
	embed sync.EmbedFunc
	// module resolves the repo's module path so the goimports local-prefixes placeholder can
	// be pinned to it. Returns "" when the dir is not a Go module.
	module func(dir string) string
}

var _ cli.Command[GetLintConfig] = (*GetLintCommand)(nil)

// NewGetLintCommand builds the lint scaffolding subcommand from the http client,
// embed reader, and module-path resolver.
func NewGetLintCommand(client http.Client, embed sync.EmbedFunc, module func(dir string) string) *GetLintCommand {
	return &GetLintCommand{client: client, embed: embed, module: module}
}

// Help returns the lint subcommand's usage text.
func (c *GetLintCommand) Help() string {
	return "Write a golangci-lint config into the current repo: the canonical bundled config by default, or one synced from --from (a local path, bundled template name, or github/gist url). Reports and skips when a config already governs the dir (--force to overwrite)."
}

// Run writes a golangci-lint config into the current repo, rendering the bundled template
// or syncing one from --from, skipping when a config already governs the dir.
func (c *GetLintCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	dir := options.Cwd

	if existing, found := golang.FindGolangciConfig(dir); found && !c.Inputs.Force {
		fmt.Printf("%s golangci-lint config already governs this repo: %s\n", output.WarnStyle.Render("•"), existing)
		fmt.Printf("%s\n", output.DimStyle.Render("pass --force to overwrite the local config"))
		return nil
	}

	// the shared --out has no default; lint writes the canonical filename unless the operator
	// points it elsewhere.
	out := c.Inputs.Out
	if out == "" {
		out = golang.GolangciConfigName
	}
	dst := filepath.Join(dir, out)

	content, source, err := c.resolve(dir, dst)
	if err != nil {
		return err
	}

	if _, err := sync.WriteFile(dst, content, true); err != nil {
		return fmt.Errorf("failed to write golangci-lint config: %w", err)
	}

	fmt.Printf("%s wrote %s\n", output.OKStyle.Render("✓"), dst)
	fmt.Printf("%s\n", output.DimStyle.Render(source))
	return nil
}

// resolve returns the config bytes and a one-line source description, either from a remote
// --from source or the rendered builtin template.
func (c *GetLintCommand) resolve(dir, dst string) ([]byte, string, error) {
	if c.Inputs.From != "" {
		engine := sync.NewEngine(sync.NewFetcher(c.client, c.Inputs.Token), c.embed)
		src, err := engine.Resolve(c.Inputs.From, filepath.Base(dst))
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve source %q: %w", c.Inputs.From, err)
		}
		content, err := engine.Bytes(context.Background(), src)
		if err != nil {
			return nil, "", fmt.Errorf("failed to sync golangci config from %s: %w", c.Inputs.From, err)
		}
		return content, "source: " + src.String(), nil
	}

	// explicit --isp wins; otherwise fall back to the repo's module path, leaving the block
	// empty (dropped) when neither is available.
	prefixes := c.Inputs.ImportSortPrefixes
	if len(prefixes) == 0 && c.module != nil {
		if module := c.module(dir); module != "" {
			prefixes = []string{module}
		}
	}

	content, err := golang.RenderGolangciConfig(prefixes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render golangci-lint config: %w", err)
	}
	source := "source: builtin"
	if len(prefixes) > 0 {
		source = "source: builtin · import-sort prefixes: " + strings.Join(prefixes, ", ")
	}
	return content, source, nil
}
