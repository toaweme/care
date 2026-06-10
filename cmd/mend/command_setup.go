package main

import (
	"github.com/toaweme/cli"
)

// SetupConfig is the (empty) config for the setup group, which only namespaces its
// subcommands and takes no inputs of its own.
type SetupConfig struct{}

// SetupCommand is the parent of the repo-scaffolding subcommands (lint, ...). Run
// directly, it lists its subcommands rather than doing work.
type SetupCommand struct {
	cli.BaseCommand[SetupConfig]
}

var _ cli.Command[SetupConfig] = (*SetupCommand)(nil)

func NewSetupCommand() *SetupCommand {
	return &SetupCommand{BaseCommand: cli.NewBaseCommand[SetupConfig]()}
}

func (c *SetupCommand) Run(_ cli.GlobalFlags, _ cli.Unknowns) error {
	return cli.ErrDisplaySubCommands
}

func (c *SetupCommand) Help() string {
	return "Scaffold canonical project config into the current repository. Subcommands: lint (golangci-lint config)."
}
