// Package tools builds the Go ecosystem's external tools with their install
// coordinates and an optional version pin. Main calls these to construct the tools
// before injecting them into feature constructors; the runner provisions each from
// its ToolSpec. Agnostic tools (e.g. betterleaks) live in eco/shared/tools, not here.
package tools

import "github.com/toaweme/mend"

// NewGolangCiLint builds the golangci-lint tool, pinned to version when non-empty.
func NewGolangCiLint(version string) mend.Tool {
	return mend.NewTool(mend.ToolSpec{
		Name:      "golangci-lint",
		Installer: mend.InstallerBrew,
		Brew:      "golangci-lint",
		GoPath:    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		Version:   version,
	})
}

// NewGovulncheck builds the govulncheck tool, pinned to version when non-empty.
func NewGovulncheck(version string) mend.Tool {
	return mend.NewTool(mend.ToolSpec{
		Name:      "govulncheck",
		Installer: mend.InstallerGo,
		GoPath:    "golang.org/x/vuln/cmd/govulncheck",
		Version:   version,
	})
}

// Go builds the go toolchain handle. It ships with the toolchain, so the runner
// never installs it; features shell out through it (go test, go mod, go get).
func Go() mend.Tool {
	return mend.NewTool(mend.ToolSpec{Name: "go", Installer: mend.InstallerBuiltin})
}

// Gofmt builds the gofmt handle. It ships with the Go toolchain, so the runner never
// installs it; the format check shells out through it (gofmt -l).
func Gofmt() mend.Tool {
	return mend.NewTool(mend.ToolSpec{Name: "gofmt", Installer: mend.InstallerBuiltin})
}
