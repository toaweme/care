// Package install provides tool-binary installers. Each install method (brew, go
// install, prebuilt release download) is its own Installer; a Tool carries the
// coordinates each method needs plus an optional version pin. The runner pairs each
// tool with an installer and may fall back from one method to the next.
package install

import (
	"context"
	"io"
)

// Tool is the minimal descriptor for a tool binary: the binary name on PATH and
// the per-method coordinates needed to install it. Version is an optional pin;
// empty means latest (and is required for the release download method).
type Tool struct {
	Bin     string       // binary name on PATH, e.g. "golangci-lint"
	Brew    string       // brew formula name (empty: not installable via brew)
	GoPath  string       // `go install` import path (empty: not installable via go)
	Release *ReleaseSpec // prebuilt release coordinates (nil: not installable via download)
	Version string       // optional version pin; empty means latest
}

// ReleaseSpec locates a tool's prebuilt release archive so the download installer can
// fetch, checksum-verify, and extract its binary without compiling from source. The
// templates expand {tag} (version with a leading v), {version} (without it), {os}
// and {arch} (the GOOS/GOARCH the asset names use, after OS/Arch token mapping).
type ReleaseSpec struct {
	BaseURL   string            // release asset directory, e.g. https://github.com/o/r/releases/download/{tag}
	Asset     string            // archive filename template, e.g. tool_{version}_{os}_{arch}.tar.gz
	Checksums string            // checksums filename template, sha256-verified against the asset
	BinPath   string            // binary path inside the archive (empty: the binary sits at the root under Bin)
	OS        map[string]string // GOOS -> asset token (e.g. identity); GOOS used verbatim when absent
	Arch      map[string]string // GOARCH -> asset token (e.g. amd64->x64); GOARCH used verbatim when absent
	Cosign    *Cosign           // when set, best-effort cosign verification of the checksums sigstore bundle
}

// Cosign is the signer identity the release installer verifies the checksums file's
// keyless sigstore bundle (<checksums>.sigstore.json) against, when a cosign binary
// is available. Verification is best-effort: a missing cosign binary or absent bundle
// is skipped, but a present-and-failing verification rejects the download.
type Cosign struct {
	IdentityRegexp string // --certificate-identity-regexp anchoring the release workflow
	Issuer         string // --certificate-oidc-issuer, e.g. https://token.actions.githubusercontent.com
}

// Installer installs a single tool via one method. Available reports whether the
// method is usable on this platform; the caller picks an installer per tool.
type Installer interface {
	Available() bool
	IsInstalled(tool Tool) bool
	Install(ctx context.Context, tool Tool) error
}

// CommandRunner executes shell commands. Installers shell out to brew, go, and cosign.
type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

// Downloader streams a URL's body to w without buffering it in memory, so large
// release archives never round-trip through a []byte. It is satisfied by a thin
// adapter over github.com/toaweme/http and stubbed in tests.
type Downloader interface {
	Download(ctx context.Context, url string, w io.Writer) error
}
