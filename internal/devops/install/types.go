// Package install provides tool-binary installers. Each install method (brew, go
// install) is its own Installer; a Tool carries the coordinates each method needs
// plus an optional version pin. The runner pairs each tool with an installer.
package install

import "context"

// Tool is the minimal descriptor for a tool binary: the binary name on PATH and
// the per-method coordinates needed to install it. Version is an optional pin;
// empty means latest.
type Tool struct {
	Bin     string // binary name on PATH, e.g. "golangci-lint"
	Brew    string // brew formula name (empty: not installable via brew)
	GoPath  string // `go install` import path (empty: not installable via go)
	Version string // optional version pin; empty means latest
}

// Installer installs a single tool via one method. Available reports whether the
// method is usable on this platform; the caller picks an installer per tool.
type Installer interface {
	Available() bool
	IsInstalled(tool Tool) bool
	Install(ctx context.Context, tool Tool) error
}

// CommandRunner executes shell commands. Installers shell out to brew and go.
type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}
