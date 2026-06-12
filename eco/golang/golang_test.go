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

// Test_Quality_ConfigDrivesEngine checks the merged Quality feature: it applies on
// any Go module (golangci when a config governs, the go vet + gofmt fallback
// otherwise), and the config-detection gate that selects the engine still works.
func Test_Quality_ConfigDrivesEngine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"))

	quality := NewQuality(mend.Tool{}, mend.Tool{}, mend.Tool{})
	if !quality.Applies(root) {
		t.Fatalf("quality should apply on any Go module")
	}
	if hasGolangciConfig(root) {
		t.Fatalf("no config present: engine should be the vet+gofmt fallback")
	}

	writeFile(t, filepath.Join(root, ".golangci.yml"))
	if !hasGolangciConfig(root) {
		t.Fatalf("config present: engine should be golangci-lint")
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}
}
