package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/toaweme/cli"
	"github.com/toaweme/http"

	"github.com/toaweme/mend/cmd/mend/output"
	"github.com/toaweme/mend/eco/shared/sync"
)

// SetupConfig drives the generic `mend setup` file sync. With no --from it falls
// back to listing the scaffolding subcommands.
type SetupConfig struct {
	From  string `arg:"from" short:"f" env:"MEND_SETUP_FROM" help:"Source to sync from: a local path, a bundled template name, a github/gist url, or the owner/repo/path shorthand"`
	Out   string `arg:"out" short:"o" env:"MEND_SETUP_OUT" help:"Destination path to write, relative to cwd"`
	Token string `arg:"token" short:"t" env:"GITHUB_TOKEN" help:"GitHub token for private sources; defaults to the GITHUB_TOKEN env"`
	Force bool   `arg:"force" env:"MEND_SETUP_FORCE" default:"false" help:"Overwrite an existing destination file"`
}

// SetupCommand is the parent of the repo-scaffolding subcommands (lint, ...) and
// the generic file-sync entry point. Run with --from it syncs one file; run bare
// it lists its subcommands.
type SetupCommand struct {
	cli.BaseCommand[SetupConfig]
	client http.Client
	embed  sync.EmbedFunc
}

var _ cli.Command[SetupConfig] = (*SetupCommand)(nil)

func NewSetupCommand(client http.Client, embed sync.EmbedFunc) *SetupCommand {
	return &SetupCommand{
		BaseCommand: cli.NewBaseCommand[SetupConfig](),
		client:      client,
		embed:       embed,
	}
}

func (c *SetupCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	if c.Inputs.From == "" {
		return cli.ErrDisplaySubCommands
	}
	if c.Inputs.Out == "" {
		return fmt.Errorf("a destination is required: pass --out <path>")
	}

	engine := sync.NewEngine(sync.NewFetcher(c.client, c.Inputs.Token), c.embed)
	dest := resolveDest(options.Cwd, c.Inputs.Out)
	res, err := engine.Sync(context.Background(), sync.Request{Spec: c.Inputs.From, Dest: dest, Force: c.Inputs.Force})
	if err != nil {
		return fmt.Errorf("failed to sync %s: %w", c.Inputs.From, err)
	}
	reportSync(res)
	return nil
}

func (c *SetupCommand) Help() string {
	return "Sync a config file into the current repo, either from a named preset (subcommands: lint) or a remote source via --from <src> --out <path> (github owner/repo/path, gist:<id>, builtin:<name>)."
}

// resolveDest joins a destination relative to cwd, leaving absolute paths as-is.
func resolveDest(cwd, out string) string {
	if filepath.IsAbs(out) {
		return out
	}
	return filepath.Join(cwd, out)
}

// reportSync prints the outcome of a sync in the shared setup style.
func reportSync(res sync.Result) {
	if res.Skipped {
		fmt.Printf("%s %s already exists; pass --force to overwrite\n", output.WarnStyle.Render("•"), res.Dest)
		return
	}
	fmt.Printf("%s wrote %s\n", output.OKStyle.Render("✓"), res.Dest)
	fmt.Printf("%s\n", output.DimStyle.Render("source: "+res.Source))
}
