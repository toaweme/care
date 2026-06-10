package golang

import "github.com/toaweme/mend/eco/golang/inspect"

// ModulePath returns the repo's module path for the report header, or "" when it
// cannot be determined (not a Go module, unreadable go.mod). It fills the
// Ecosystem.Module slot so the report header is resolved once at the caller.
func ModulePath(dir string) string {
	mod, err := inspect.ReadModulePath(dir)
	if err != nil {
		return ""
	}
	return mod
}
