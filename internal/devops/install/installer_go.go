package install

import (
	"context"
	"fmt"
)

// goInstall installs tools via `go install`.
type goInstall struct {
	runner CommandRunner
}

var _ Installer = (*goInstall)(nil)

// Go returns an installer that provisions tools via `go install`.
func Go(opts ...Option) Installer {
	o := newOptions(opts...)
	return &goInstall{runner: o.runner}
}

func (g *goInstall) Available() bool {
	_, err := g.runner.LookPath("go")
	return err == nil
}

func (g *goInstall) IsInstalled(tool Tool) bool {
	_, err := g.runner.LookPath(tool.Bin)
	return err == nil
}

func (g *goInstall) Install(ctx context.Context, tool Tool) error {
	if tool.GoPath == "" {
		return fmt.Errorf("failed to install %q via go install: no import path configured", tool.Bin)
	}
	version := tool.Version
	if version == "" {
		version = "latest"
	}
	out, err := g.runner.Run(ctx, ".", "go", "install", tool.GoPath+"@"+version)
	if err != nil {
		return fmt.Errorf("failed to go install %q (output: %s): %w", tool.Bin, trimOutput(out), err)
	}
	return nil
}
