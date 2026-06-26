package minver

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// History maps a stdlib symbol to the Go minor version that introduced it, parsed
// from $GOROOT/api/go1.N.txt. ids covers package-level symbols (funcs, types,
// consts, vars); members covers methods and struct/interface members keyed by
// their owning type.
type History struct {
	// max is the highest go1.N file seen; the analyzer can detect features only up
	// to this version (the same bound mingo enforces).
	max int
	// ids[pkgPath][name] = minor version.
	ids map[string]map[string]int
	// members[pkgPath][typeName][memberName] = minor version.
	members map[string]map[string]map[string]int
}

// Max reports the newest Go minor version this History knows about.
func (h *History) Max() int { return h.max }

// lookup returns the minor version a package-level symbol was introduced in.
func (h *History) lookup(pkgPath, name string) (int, bool) {
	if m, ok := h.ids[pkgPath]; ok {
		v, ok := m[name]
		return v, ok
	}
	return 0, false
}

// lookupMember returns the minor version a method or struct/interface member of a
// named type was introduced in.
func (h *History) lookupMember(pkgPath, typeName, member string) (int, bool) {
	if t, ok := h.members[pkgPath]; ok {
		if m, ok := t[typeName]; ok {
			v, ok := m[member]
			return v, ok
		}
	}
	return 0, false
}

// apiDir returns the api directory of the active Go installation, honoring $GOROOT
// then falling back to `go env GOROOT` (the non-deprecated way to locate the root
// of the toolchain actually in use).
func apiDir(ctx context.Context) string {
	root := os.Getenv("GOROOT")
	if root == "" {
		root = goEnvRoot(ctx)
	}
	if root == "" {
		return ""
	}
	return filepath.Join(root, "api")
}

// goEnvRoot asks the go tool for its GOROOT, returning "" when go is unavailable.
func goEnvRoot(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "go", "env", "GOROOT").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// LoadHistory reads $GOROOT/api/go1.N.txt into a History. It returns ErrNoAPI when
// the api directory is absent so the caller can skip the stdlib-symbol analysis
// rather than produce a wrong answer.
func LoadHistory(ctx context.Context) (*History, error) {
	dir := apiDir(ctx)
	if dir == "" {
		return nil, ErrNoAPI
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoAPI
		}
		return nil, fmt.Errorf("failed to read api directory %q: %w", dir, err)
	}
	h := &History{
		ids:     map[string]map[string]int{},
		members: map[string]map[string]map[string]int{},
	}
	var found bool
	for _, e := range entries {
		minor, ok := apiFileMinor(e.Name())
		if !ok {
			continue
		}
		found = true
		if minor > h.max {
			h.max = minor
		}
		if err := h.readFile(filepath.Join(dir, e.Name()), minor); err != nil {
			return nil, err
		}
	}
	if !found {
		return nil, ErrNoAPI
	}
	return h, nil
}

// apiFileMinor extracts the minor version from an api filename: "go1.txt" is the
// 1.0 baseline (minor 0) and "go1.N.txt" is minor N. Any other name (except.txt,
// next/) is not a release snapshot.
func apiFileMinor(name string) (int, bool) {
	if name == "go1.txt" {
		return 0, true
	}
	if !strings.HasPrefix(name, "go1.") || !strings.HasSuffix(name, ".txt") {
		return 0, false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(name, "go1."), ".txt")
	n, err := strconv.Atoi(mid)
	if err != nil {
		return 0, false
	}
	return n, true
}

var (
	// rePkg strips the "pkg <path>, " prefix every feature line carries.
	rePkg = regexp.MustCompile(`^pkg (\S+), (.*)$`)

	reFunc   = regexp.MustCompile(`^func (\w+)`)
	reConst  = regexp.MustCompile(`^const (\w+)`)
	reVar    = regexp.MustCompile(`^var (\w+)`)
	reField  = regexp.MustCompile(`^type (\w+) struct, (\w+)`)
	reIface  = regexp.MustCompile(`^type (\w+) interface, (\w+)`)
	reMethod = regexp.MustCompile(`^method \(\*?(\w+)[^)]*\) (\w+)`)
	reType   = regexp.MustCompile(`^type (\w+)`)
)

// readFile parses one api snapshot, recording the earliest version each symbol is
// seen at (api snapshots are frozen and additive, so the first file a symbol
// appears in is its introduction version).
func (h *History) readFile(path string, minor int) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open api file %q: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		m := rePkg.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pkg, rest := m[1], m[2]
		h.recordLine(pkg, rest, minor)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("failed to scan api file %q: %w", path, err)
	}
	return nil
}

// recordLine classifies one feature line and records its symbol at minor. Member
// forms (struct field, interface method, method) are matched before the bare type
// form, since they share the "type <name>" lead-in.
func (h *History) recordLine(pkg, rest string, minor int) {
	switch {
	case strings.HasPrefix(rest, "method "):
		if m := reMethod.FindStringSubmatch(rest); m != nil {
			h.putMember(pkg, m[1], m[2], minor)
		}
	case reField.MatchString(rest):
		m := reField.FindStringSubmatch(rest)
		h.putMember(pkg, m[1], m[2], minor)
	case reIface.MatchString(rest):
		m := reIface.FindStringSubmatch(rest)
		h.putMember(pkg, m[1], m[2], minor)
	case strings.HasPrefix(rest, "func "):
		if m := reFunc.FindStringSubmatch(rest); m != nil {
			h.putID(pkg, m[1], minor)
		}
	case strings.HasPrefix(rest, "const "):
		if m := reConst.FindStringSubmatch(rest); m != nil {
			h.putID(pkg, m[1], minor)
		}
	case strings.HasPrefix(rest, "var "):
		if m := reVar.FindStringSubmatch(rest); m != nil {
			h.putID(pkg, m[1], minor)
		}
	case strings.HasPrefix(rest, "type "):
		if m := reType.FindStringSubmatch(rest); m != nil {
			h.putID(pkg, m[1], minor)
		}
	}
}

func (h *History) putID(pkg, name string, minor int) {
	m := h.ids[pkg]
	if m == nil {
		m = map[string]int{}
		h.ids[pkg] = m
	}
	if cur, ok := m[name]; !ok || minor < cur {
		m[name] = minor
	}
}

func (h *History) putMember(pkg, typeName, member string, minor int) {
	t := h.members[pkg]
	if t == nil {
		t = map[string]map[string]int{}
		h.members[pkg] = t
	}
	m := t[typeName]
	if m == nil {
		m = map[string]int{}
		t[typeName] = m
	}
	if cur, ok := m[member]; !ok || minor < cur {
		m[member] = minor
	}
}
