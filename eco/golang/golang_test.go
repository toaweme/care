package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/toaweme/mend"
)

func Test_HasGolangciConfig(t *testing.T) {
	tests := []struct {
		name    string
		files   []string // paths to create, relative to the temp root
		checkAt string   // dir to query, relative to the temp root ("" = root)
		want    bool
	}{
		{name: "none", files: nil, checkAt: "", want: false},
		{name: "yml in dir", files: []string{".golangci.yml"}, checkAt: "", want: true},
		{name: "yaml in dir", files: []string{".golangci.yaml"}, checkAt: "", want: true},
		{name: "toml in dir", files: []string{".golangci.toml"}, checkAt: "", want: true},
		{name: "json in dir", files: []string{".golangci.json"}, checkAt: "", want: true},
		{name: "config in parent governs nested dir", files: []string{".golangci.yml"}, checkAt: "sub/mod", want: true},
		{name: "unrelated yml does not count", files: []string{"config.yml"}, checkAt: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tt.files {
				writeFile(t, filepath.Join(root, f))
			}
			dir := root
			if tt.checkAt != "" {
				dir = filepath.Join(root, tt.checkAt)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
			}
			if got := hasGolangciConfig(dir); got != tt.want {
				t.Fatalf("hasGolangciConfig(%q) = %v, want %v", dir, got, tt.want)
			}
		})
	}
}

// Test_VetFormat_StepAside checks the philosophy-B gate: vet and format apply on a
// Go module only when no golangci-lint config governs the dir.
func Test_VetFormat_StepAside(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"))

	vet := NewVet(mend.Tool{})
	format := NewFormat(mend.Tool{})

	if !vet.Applies(root) {
		t.Fatalf("vet should apply on a Go module with no golangci config")
	}
	if !format.Applies(root) {
		t.Fatalf("format should apply on a Go module with no golangci config")
	}

	writeFile(t, filepath.Join(root, ".golangci.yml"))

	if vet.Applies(root) {
		t.Fatalf("vet should step aside when a golangci config is present")
	}
	if format.Applies(root) {
		t.Fatalf("format should step aside when a golangci config is present")
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}
}
