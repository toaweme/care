// Package tools builds the Go ecosystem's external tools with their install
// coordinates and an optional version pin. Main calls these to construct the tools
// before injecting them into feature constructors; the runner provisions each from
// its ToolSpec. Agnostic tools (e.g. betterleaks) live in eco/shared/tools, not here.
package tools

import "github.com/toaweme/care"

// defaultGolangCiVersion is the golangci-lint release the download installer pins to
// when the operator configures none. It tracks the version the shipped .golangci.yml
// is validated against (mirrored by GOLANGCI_VERSION in the CI workflow); bump both
// together. A pin is required for the download method, which names an exact tag.
const defaultGolangCiVersion = "v2.12.2"

// NewGolangCiLint builds the golangci-lint tool, pinned to version when non-empty and
// to defaultGolangCiVersion otherwise. It installs from the verified prebuilt release
// by default (fast, no compile), falling back to `go install` and brew when a
// download is not possible.
func NewGolangCiLint(version string) care.Tool {
	if version == "" {
		version = defaultGolangCiVersion
	}
	return care.NewTool(care.ToolSpec{
		Name:      "golangci-lint",
		Installer: care.InstallerRelease,
		Brew:      "golangci-lint",
		GoPath:    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		Release: &care.ReleaseSpec{
			BaseURL:   "https://github.com/golangci/golangci-lint/releases/download/{tag}",
			Asset:     "golangci-lint-{version}-{os}-{arch}.tar.gz",
			Checksums: "golangci-lint-{version}-checksums.txt",
			BinPath:   "golangci-lint-{version}-{os}-{arch}/golangci-lint",
		},
		Version: version,
	})
}

// NewGovulncheck builds the govulncheck tool, pinned to version when non-empty.
func NewGovulncheck(version string) care.Tool {
	return care.NewTool(care.ToolSpec{
		Name:      "govulncheck",
		Installer: care.InstallerGo,
		GoPath:    "golang.org/x/vuln/cmd/govulncheck",
		Version:   version,
	})
}

// Go builds the go toolchain handle. It ships with the toolchain, so the runner
// never installs it; features shell out through it (go test, go mod, go get).
func Go() care.Tool {
	return care.NewTool(care.ToolSpec{Name: "go", Installer: care.InstallerBuiltin})
}

// Gofmt builds the gofmt handle. It ships with the Go toolchain, so the runner never
// installs it; the format check shells out through it (gofmt -l).
func Gofmt() care.Tool {
	return care.NewTool(care.ToolSpec{Name: "gofmt", Installer: care.InstallerBuiltin})
}
