// Package tools builds the language-agnostic external tools with their install
// coordinates and an optional version pin. These scan any repository regardless of
// toolchain (e.g. betterleaks), so they live alongside the agnostic features in
// eco/shared rather than in a language's tools package.
package tools

import "github.com/toaweme/mend"

// NewBetterleaks builds the betterleaks tool, pinned to version when non-empty.
// betterleaks scans any repo for leaked secrets, so it is shared, not Go-specific.
func NewBetterleaks(version string) mend.Tool {
	return mend.NewTool(mend.ToolSpec{
		Name:      "betterleaks",
		Installer: mend.InstallerBrew,
		Brew:      "betterleaks",
		GoPath:    "github.com/betterleaks/betterleaks",
		Version:   version,
	})
}
