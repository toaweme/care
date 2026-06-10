package golang

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_DocCoverage(t *testing.T) {
	dir := t.TempDir()
	src := `package sample

// Documented is documented.
func Documented() {}

func Undocumented() {}

// Thing is a documented type.
type Thing struct{}

func (t Thing) Exported() {}

func (t Thing) unexported() {} // not exported, ignored

// Exported const, documented.
const Answer = 42

var Loose = 1

func unexportedFunc() {} // ignored
`
	write(t, dir, "sample.go", src)
	// a _test.go and a vendored file must be ignored entirely.
	write(t, dir, "sample_test.go", "package sample\n\nfunc HelperExported() {}\n")
	write(t, filepath.Join(dir, "vendor", "dep"), "dep.go", "package dep\n\nfunc Vendored() {}\n")

	rep, err := docCoverage(dir)
	if err != nil {
		t.Fatalf("docCoverage: %v", err)
	}
	// exported decls: Documented, Undocumented, Thing, Thing.Exported, Answer, Loose = 6
	if rep.Total != 6 {
		t.Fatalf("total exported = %d, want 6 (got missing %+v)", rep.Total, rep.Missing)
	}
	// documented: Documented, Thing, Answer = 3
	if rep.Documented != 3 {
		t.Fatalf("documented = %d, want 3", rep.Documented)
	}
	missing := map[string]string{}
	for _, m := range rep.Missing {
		missing[m.Name] = m.Kind
	}
	wantMissing := map[string]string{"Undocumented": "func", "Thing.Exported": "method", "Loose": "var"}
	for name, kind := range wantMissing {
		if missing[name] != kind {
			t.Errorf("missing[%q] = %q, want %q", name, missing[name], kind)
		}
	}
	if len(rep.Missing) != 3 {
		t.Errorf("missing count = %d, want 3 (%+v)", len(rep.Missing), rep.Missing)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
