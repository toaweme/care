package install

import (
	"context"
	"os/exec"
	"runtime"
)

// Option configures an installer at construction. The zero configuration uses a
// real exec-based command runner and the host OS.
type Option func(*options)

type options struct {
	runner CommandRunner
	goos   string
}

func newOptions(opts ...Option) options {
	o := options{
		runner: execRunner{},
		goos:   runtime.GOOS,
	}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithRunner injects a command runner (used in tests to avoid shelling out).
func WithRunner(r CommandRunner) Option { return func(o *options) { o.runner = r } }

// WithGOOS overrides the detected operating system (used by the brew installer).
func WithGOOS(goos string) Option { return func(o *options) { o.goos = goos } }

// execRunner shells out to the real process tree.
type execRunner struct{}

var _ CommandRunner = execRunner{}

func (execRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func (execRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }
