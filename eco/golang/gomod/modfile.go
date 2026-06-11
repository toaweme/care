// Package gomod reads facts out of a repo's go.mod without invoking the go command:
// its go/toolchain directives, replace directives, and the dependency go-directive
// floor (computed from each required module's go.mod in the local cache).
package gomod

import (
	"fmt"
	"go/version"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Directives is the version state of a go.mod's `go` and `toolchain` lines, parsed
// for the minimal-version check. GoVersion is the bare `go` value ("1.25.0");
// Toolchain is the `toolchain` value ("go1.26.0") or empty when there is none.
type Directives struct {
	GoVersion string
	Toolchain string
}

// ReadDirectives parses <repo>/go.mod and returns its go/toolchain directives.
func ReadDirectives(repo string) (Directives, error) {
	path := repo + "/go.mod"
	raw, err := os.ReadFile(path)
	if err != nil {
		return Directives{}, fmt.Errorf("failed to read go.mod: %w", err)
	}
	f, err := modfile.Parse(path, raw, nil)
	if err != nil {
		return Directives{}, fmt.Errorf("failed to parse go.mod: %w", err)
	}
	var d Directives
	if f.Go != nil {
		d.GoVersion = f.Go.Version
	}
	if f.Toolchain != nil {
		d.Toolchain = f.Toolchain.Name
	}
	return d, nil
}

// DepFloor is the highest `go` directive across a module's dependencies: the
// lowest version the main module's own `go` directive is allowed to declare,
// independent of its code. Module names which dependency set the floor. Requires
// is the number of require entries considered and Missing the number whose go.mod
// was not in the local cache, so a caller can tell whether the floor is complete.
type DepFloor struct {
	Version  string
	Module   string
	Requires int
	Missing  int
	// Deps is every required module with its version and `go` directive (the go
	// value empty when its go.mod was missing or declared none), for a caller that
	// wants to list the whole graph, not just the floor-setting module.
	Deps []DepGo
}

// DepGo is one dependency's identity and its `go` directive.
type DepGo struct {
	Module  string
	Version string
	Go      string
}

// ReadDepFloor computes the dependency go-directive floor for repo by reading each
// required module's go.mod straight from the local module cache (modcache,
// typically `go env GOMODCACHE`). It never downloads: a dependency whose go.mod is
// not cached is counted in Missing and skipped, so the caller can require a
// complete cache (the user runs `go mod download`) before trusting the floor.
func ReadDepFloor(repo, modcache string) (DepFloor, error) {
	path := repo + "/go.mod"
	raw, err := os.ReadFile(path)
	if err != nil {
		return DepFloor{}, fmt.Errorf("failed to read go.mod: %w", err)
	}
	f, err := modfile.Parse(path, raw, nil)
	if err != nil {
		return DepFloor{}, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	var floor DepFloor
	for _, r := range f.Require {
		floor.Requires++
		dep := DepGo{Module: r.Mod.Path, Version: r.Mod.Version}
		modPath := depModPath(modcache, r.Mod)
		if modPath == "" {
			floor.Missing++
			floor.Deps = append(floor.Deps, dep)
			continue
		}
		b, err := os.ReadFile(modPath)
		if err != nil {
			floor.Missing++
			floor.Deps = append(floor.Deps, dep)
			continue
		}
		if df, err := modfile.Parse(modPath, b, nil); err == nil && df.Go != nil {
			dep.Go = df.Go.Version
			if floor.Version == "" || version.Compare("go"+dep.Go, "go"+floor.Version) > 0 {
				floor.Version = dep.Go
				floor.Module = r.Mod.Path
			}
		}
		floor.Deps = append(floor.Deps, dep)
	}
	return floor, nil
}

// depModPath returns the cache location of a dependency's go.mod, applying the
// module-cache path escaping (uppercase letters become "!<lower>"). It returns ""
// when modcache is empty or the path/version cannot be escaped.
func depModPath(modcache string, m module.Version) string {
	if modcache == "" {
		return ""
	}
	escPath, err := module.EscapePath(m.Path)
	if err != nil {
		return ""
	}
	escVer, err := module.EscapeVersion(m.Version)
	if err != nil {
		return ""
	}
	return filepath.Join(modcache, "cache", "download", escPath, "@v", escVer+".mod")
}

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
