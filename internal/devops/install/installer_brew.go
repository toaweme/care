package install

import (
	"context"
	"fmt"
)

// brew installs tools via Homebrew.
type brew struct {
	runner CommandRunner
	goos   string
}

var _ Installer = (*brew)(nil)

// Brew returns an installer that provisions tools via Homebrew.
func Brew(opts ...Option) Installer {
	o := newOptions(opts...)
	return &brew{runner: o.runner, goos: o.goos}
}

func (b *brew) Available() bool {
	if b.goos == "windows" {
		return false
	}
	_, err := b.runner.LookPath("brew")
	return err == nil
}

func (b *brew) IsInstalled(tool Tool) bool {
	_, err := b.runner.LookPath(tool.Bin)
	return err == nil
}

func (b *brew) Install(ctx context.Context, tool Tool) error {
	if tool.Brew == "" {
		return fmt.Errorf("failed to install %q via brew: no brew formula configured", tool.Bin)
	}
	out, err := b.runner.Run(ctx, ".", "brew", "install", tool.Brew)
	if err != nil {
		return fmt.Errorf("failed to brew install %q (output: %s): %w", tool.Bin, trimOutput(out), err)
	}
	return nil
}

func trimOutput(b []byte) string {
	const maxLen = 200
	s := string(b)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
