// Package golang holds the Go ecosystem's checks, one per file
// (check_<name>.go). Each pairs a mend role implementation with the typed
// mend.Report it produces and the parser for its tool's output. Constructors take
// their injected tools; main registers each into the mend.Ecosystem. The engines
// they call live in subpackages: tests, gomod, inspect.
package golang

import (
	"os"
	"path/filepath"
	"strings"
)

// hasGoMod reports whether dir carries a go.mod. Go features use it for Applies so
// they self-skip in a non-Go repo during a mixed sweep.
func hasGoMod(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod"))
}

// fileExists reports whether path is present (a stat succeeds).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// golangciConfigNames are the config filenames golangci-lint reads, in the order
// it resolves them.
var golangciConfigNames = []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}

// hasGolangciConfig reports whether a golangci-lint config governs dir. When one
// does, the repo delegates static analysis to golangci-lint, so the standalone
// vet and format features step aside (golangci's govet and formatters subsume
// them); they run only as the fallback baseline when no config is present.
//
// It mirrors golangci-lint's own discovery: walk up from dir through every parent
// to the filesystem root, since a config placed at the repo root governs nested
// module dirs too.
func hasGolangciConfig(dir string) bool {
	_, found := FindGolangciConfig(dir)
	return found
}

// firstLine returns the first non-empty line of tool output, trimmed, or a default
// when there is none. It distills a multi-line tool message into one issue detail.
func firstLine(b []byte) string {
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "verification failed"
}

func trimOutput(b []byte) string {
	s := string(b)
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}
