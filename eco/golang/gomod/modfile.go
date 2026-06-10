package gomod

import (
	"fmt"
	"os"

	"golang.org/x/mod/modfile"
)

// ReplaceDirectives parses <repo>/go.mod and returns the module paths that have a
// replace directive, so callers need not import golang.org/x/mod/modfile directly.
func ReplaceDirectives(repo string) ([]string, error) {
	path := repo + "/go.mod"
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}
	f, err := modfile.Parse(path, raw, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}
	out := make([]string, 0, len(f.Replace))
	for _, r := range f.Replace {
		out = append(out, r.Old.Path)
	}
	return out, nil
}
