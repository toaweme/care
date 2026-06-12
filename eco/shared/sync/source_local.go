package sync

import (
	"os"
	"path/filepath"
	"strings"
)

const providerLocal = "local"

// localProvider resolves a filesystem path, read later with os.ReadFile.
type localProvider struct{}

var _ Provider = localProvider{}

func (localProvider) Name() string { return providerLocal }

func (localProvider) Resolve(spec string) (Source, bool, error) {
	if !looksLocal(spec) {
		return Source{}, false, nil
	}
	return Source{Provider: providerLocal, kind: kindLocal, path: expandLocal(spec)}, true, nil
}

// looksLocal recognizes a filesystem path: an explicit path marker, a file://
// URL, or any spec that already names an existing file on disk.
func looksLocal(spec string) bool {
	if strings.Contains(spec, "://") {
		return strings.HasPrefix(spec, "file://")
	}
	switch {
	case strings.HasPrefix(spec, "/"),
		strings.HasPrefix(spec, "./"),
		strings.HasPrefix(spec, "../"),
		spec == "~",
		strings.HasPrefix(spec, "~/"):
		return true
	}
	if info, err := os.Stat(spec); err == nil && !info.IsDir() {
		return true
	}
	return false
}

func expandLocal(spec string) string {
	p := strings.TrimPrefix(spec, "file://")
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p[1:], "/"))
		}
	}
	return p
}
