package care

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"regexp"
)

// semverRe extracts the first dotted version (e.g. 8.18.2, go1.22.0 -> 1.22.0) from
// a tool's version output, so one probe handles every tool's own format.
var semverRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// ToolSpec is a tool binary's identity and install coordinates. Installer selects
// how the runner provisions it; Version is a pin the runner installs to on drift.
// A tool may carry coordinates for several methods (e.g. Release plus GoPath) so the
// runner can fall back from one to the next.
type ToolSpec struct {
	Name      string       `json:"name"`
	Installer Installer    `json:"installer,omitempty"`
	Brew      string       `json:"brew,omitempty"`
	GoPath    string       `json:"go_path,omitempty"`
	Release   *ReleaseSpec `json:"release,omitempty"`
	Version   string       `json:"version,omitempty"`
}

// ReleaseSpec locates a tool's prebuilt release archive for the download installer:
// the URL and filename templates (expanding {tag}, {version}, {os}, {arch}), the
// binary's path inside the archive, per-platform token overrides, and optional
// cosign signer identity. Requires a pinned Version, since a release URL names an
// exact tag.
type ReleaseSpec struct {
	BaseURL   string            `json:"base_url"`
	Asset     string            `json:"asset"`
	Checksums string            `json:"checksums"`
	BinPath   string            `json:"bin_path,omitempty"`
	OS        map[string]string `json:"os,omitempty"`
	Arch      map[string]string `json:"arch,omitempty"`
	Cosign    *CosignVerify     `json:"cosign,omitempty"`
}

// CosignVerify is the keyless signer identity the download installer verifies a
// release's checksums sigstore bundle against, when a cosign binary is present.
type CosignVerify struct {
	IdentityRegexp string `json:"identity_regexp"`
	Issuer         string `json:"issuer"`
}

// Tool is an external binary a Feature runs through. It wraps a ToolSpec the
// runner installs from and runs the binary in a repo dir via Exec/ExecStdout. The
// zero Tool (Name "") is a feature with no tool dependency.
type Tool struct {
	spec ToolSpec
}

// NewTool builds a Tool from its spec.
func NewTool(spec ToolSpec) Tool { return Tool{spec: spec} }

// Name is the tool's binary name on PATH.
func (t Tool) Name() string { return t.spec.Name }

// Spec returns the install descriptor the runner provisions from.
func (t Tool) Spec() ToolSpec { return t.spec }

// Exec runs the tool's binary in dir with args and returns its combined output.
func (t Tool) Exec(ctx context.Context, dir string, args ...string) ([]byte, error) {
	//nolint:gosec // tool name and args are program-controlled, from a registered ToolSpec
	cmd := exec.CommandContext(ctx, t.spec.Name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// Version probes the binary for its semantic version, trying the common version
// invocations and extracting the first semver from the output. It returns "" when
// the tool reports no recognizable version (so the caller falls back to a bare name).
func (t Tool) Version(ctx context.Context) string {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-version"}} {
		out, _ := t.Exec(ctx, ".", args...)
		if m := semverRe.Find(out); m != nil {
			return string(m)
		}
	}
	return ""
}

// ExecStdout runs the tool's binary in dir with args and returns only stdout,
// discarding stderr. It suits tools whose machine-readable output (e.g.
// govulncheck -json) goes to stdout while progress noise goes to stderr.
func (t Tool) ExecStdout(ctx context.Context, dir string, args ...string) ([]byte, error) {
	//nolint:gosec // tool name and args are program-controlled, from a registered ToolSpec
	cmd := exec.CommandContext(ctx, t.spec.Name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return out.Bytes(), err
}
