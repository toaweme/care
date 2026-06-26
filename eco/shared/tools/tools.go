// Package tools builds the language-agnostic external tools with their install
// coordinates and an optional version pin. These scan any repository regardless of
// toolchain (e.g. betterleaks), so they live alongside the agnostic features in
// eco/shared rather than in a language's tools package.
package tools

import "github.com/toaweme/care"

// defaultBetterleaksVersion is the betterleaks release the download installer pins to
// when the operator configures none. A pin is required for the download method, which
// names an exact tag.
const defaultBetterleaksVersion = "v1.6.0"

// NewBetterleaks builds the betterleaks tool, pinned to version when non-empty.
// betterleaks scans any repo for leaked secrets, so it is shared, not Go-specific.
// It installs from the verified prebuilt release by default (its checksums carry a
// cosign signature, verified when cosign is present), falling back to `go install`
// and brew when a download is not possible.
func NewBetterleaks(version string) care.Tool {
	if version == "" {
		version = defaultBetterleaksVersion
	}
	return care.NewTool(care.ToolSpec{
		Name:      "betterleaks",
		Installer: care.InstallerRelease,
		Brew:      "betterleaks",
		GoPath:    "github.com/betterleaks/betterleaks",
		Release: &care.ReleaseSpec{
			BaseURL:   "https://github.com/betterleaks/betterleaks/releases/download/{tag}",
			Asset:     "betterleaks_{version}_{os}_{arch}.tar.gz",
			Checksums: "checksums.txt",
			Arch:      map[string]string{"amd64": "x64"},
			Cosign: &care.CosignVerify{
				IdentityRegexp: `^https://github\.com/betterleaks/betterleaks/\.github/workflows/release\.yml@refs/tags/`,
				Issuer:         "https://token.actions.githubusercontent.com",
			},
		},
		Version: version,
	})
}
