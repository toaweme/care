package gomod

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_ReplaceDirectives(t *testing.T) {
	tests := []struct {
		name  string
		gomod string
		want  []string
	}{
		{
			name:  "no replaces",
			gomod: "module example.com/x\n\ngo 1.22\n",
			want:  []string{},
		},
		{
			name:  "single replace",
			gomod: "module example.com/x\n\ngo 1.22\n\nreplace example.com/a => ../a\n",
			want:  []string{"example.com/a"},
		},
		{
			name:  "block of replaces",
			gomod: "module example.com/x\n\ngo 1.22\n\nreplace (\n\texample.com/a => ../a\n\texample.com/b => ../b\n)\n",
			want:  []string{"example.com/a", "example.com/b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(tt.gomod), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := ReplaceDirectives(dir)
			if err != nil {
				t.Fatalf("ReplaceDirectives: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// writeCachedMod lays a dependency's go.mod into a fake module cache at the path
// ReadDepFloor expects (cache/download/<path>/@v/<version>.mod).
func writeCachedMod(t *testing.T, modcache, path, ver, goVer string) {
	t.Helper()
	dir := filepath.Join(modcache, "cache", "download", path, "@v")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "module " + path + "\n\ngo " + goVer + "\n"
	if err := os.WriteFile(filepath.Join(dir, ver+".mod"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func Test_ReadDepFloor(t *testing.T) {
	dir := t.TempDir()
	modcache := t.TempDir()

	gomod := "module example.com/x\n\ngo 1.20\n\nrequire (\n" +
		"\texample.com/a v1.0.0\n" +
		"\texample.com/b v1.2.0\n" +
		"\texample.com/c v0.1.0 // indirect\n" +
		")\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	// a needs 1.21, b needs 1.23 (the floor), c is not in the cache (missing).
	writeCachedMod(t, modcache, "example.com/a", "v1.0.0", "1.21")
	writeCachedMod(t, modcache, "example.com/b", "v1.2.0", "1.23")

	floor, err := ReadDepFloor(dir, modcache)
	if err != nil {
		t.Fatalf("ReadDepFloor: %v", err)
	}
	if floor.Version != "1.23" || floor.Module != "example.com/b" {
		t.Fatalf("floor = %q from %q, want 1.23 from example.com/b", floor.Version, floor.Module)
	}
	if floor.Requires != 3 || floor.Missing != 1 {
		t.Fatalf("requires=%d missing=%d, want 3 and 1", floor.Requires, floor.Missing)
	}
}

func Test_ReadDepFloor_escapedPath(t *testing.T) {
	dir := t.TempDir()
	modcache := t.TempDir()
	gomod := "module example.com/x\n\ngo 1.20\n\nrequire github.com/Azure/go-autorest v1.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	// uppercase letters escape to !<lower> in the cache path.
	writeCachedMod(t, modcache, "github.com/!azure/go-autorest", "v1.0.0", "1.22")

	floor, err := ReadDepFloor(dir, modcache)
	if err != nil {
		t.Fatalf("ReadDepFloor: %v", err)
	}
	if floor.Version != "1.22" || floor.Missing != 0 {
		t.Fatalf("floor=%q missing=%d, want 1.22 and 0 (escaped path not found)", floor.Version, floor.Missing)
	}
}
