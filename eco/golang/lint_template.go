package golang

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/toaweme/care/templates"
)

// golangciTemplateFile is the canonical golangci-lint config shipped in the
// embedded templates filesystem.
const golangciTemplateFile = ".golangci.yml"

// golangciPrefixPlaceholder is the token the goimports local-prefixes list item in
// the template carries; the item it sits on expands into one YAML item per
// import-sort prefix on write.
const golangciPrefixPlaceholder = "__IMPORT_SORT_PREFIXES__"

// golangciLocalPrefixesKey is the parent key the placeholder item lives under,
// removed alongside the item when there are no prefixes so no empty key is left.
const golangciLocalPrefixesKey = "local-prefixes:"

// GolangciConfigName is the filename setup writes the canonical config to (the
// first name golangci-lint resolves).
const GolangciConfigName = ".golangci.yml"

// FindGolangciConfig walks up from dir to the filesystem root looking for a
// golangci-lint config, mirroring golangci-lint's own discovery: a config at the
// repo root governs nested module dirs too. It returns the path of the first
// config found and whether one governs dir.
func FindGolangciConfig(dir string) (string, bool) {
	for {
		for _, name := range golangciConfigNames {
			p := filepath.Join(dir, name)
			if fileExists(p) {
				return p, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// RenderGolangciConfig returns the canonical golangci-lint config ready to write
// to a repo. When the template carries the goimports local-prefixes block, the
// placeholder item expands into one YAML item per prefix; with no prefixes the
// whole block is dropped. A template without the block is returned verbatim. Either
// way the result never contains the raw placeholder.
func RenderGolangciConfig(prefixes []string) ([]byte, error) {
	raw, err := templates.FS.ReadFile(golangciTemplateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read golangci-lint template: %w", err)
	}

	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines)+len(prefixes))
	for _, line := range lines {
		if !strings.Contains(line, golangciPrefixPlaceholder) {
			out = append(out, line)
			continue
		}
		// reuse the placeholder line's own indentation so the expansion is correct no
		// matter how the template is indented (spaces, tabs, width).
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if len(prefixes) == 0 {
			// no prefixes: drop the item and the now-empty local-prefixes: key above it.
			if n := len(out); n > 0 && strings.TrimSpace(out[n-1]) == golangciLocalPrefixesKey {
				out = out[:n-1]
			}
			continue
		}
		for _, p := range prefixes {
			out = append(out, indent+"- "+p)
		}
	}
	return []byte(strings.Join(out, "\n")), nil
}
