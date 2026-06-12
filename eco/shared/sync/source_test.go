package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_Resolve_Remote(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		provider string
		url      string
	}{
		{
			name:     "shorthand owner/repo/path implies github",
			spec:     "toaweme/structs/assert_test.go",
			provider: providerGitHub,
			url:      "https://raw.githubusercontent.com/toaweme/structs/HEAD/assert_test.go",
		},
		{
			name:     "shorthand nested path",
			spec:     "toaweme/common/config/.golangci.yml",
			provider: providerGitHub,
			url:      "https://raw.githubusercontent.com/toaweme/common/HEAD/config/.golangci.yml",
		},
		{
			name:     "github blob url",
			spec:     "https://github.com/toaweme/structs/blob/main/assert_test.go",
			provider: providerGitHub,
			url:      "https://raw.githubusercontent.com/toaweme/structs/main/assert_test.go",
		},
		{
			name:     "github raw url with refs",
			spec:     "https://github.com/toaweme/structs/raw/refs/heads/main/assert_test.go",
			provider: providerGitHub,
			url:      "https://raw.githubusercontent.com/toaweme/structs/refs/heads/main/assert_test.go",
		},
		{
			name:     "raw githubusercontent url",
			spec:     "https://raw.githubusercontent.com/toaweme/structs/refs/heads/main/assert_test.go",
			provider: providerGitHub,
			url:      "https://raw.githubusercontent.com/toaweme/structs/refs/heads/main/assert_test.go",
		},
		{
			name:     "gist raw url fetched verbatim",
			spec:     "https://gist.githubusercontent.com/iberflow/e692af6c090076a85c6b9e21e54afd4c/raw/df2af825dda096f72ca1d357d6dc3c8bbfec3616/gistfile1.txt",
			provider: providerGist,
			url:      "https://gist.githubusercontent.com/iberflow/e692af6c090076a85c6b9e21e54afd4c/raw/df2af825dda096f72ca1d357d6dc3c8bbfec3616/gistfile1.txt",
		},
		{
			name:     "gist web url maps to raw",
			spec:     "https://gist.github.com/iberflow/e692af6c090076a85c6b9e21e54afd4c",
			provider: providerGist,
			url:      "https://gist.githubusercontent.com/iberflow/e692af6c090076a85c6b9e21e54afd4c/raw",
		},
	}

	engine := NewEngine(nil, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.Resolve(tt.spec, "")
			if err != nil {
				t.Fatalf("Resolve(%q) error: %v", tt.spec, err)
			}
			if got.Provider != tt.provider {
				t.Fatalf("provider = %q, want %q", got.Provider, tt.provider)
			}
			if got.URL() != tt.url {
				t.Fatalf("url\n got: %s\nwant: %s", got.URL(), tt.url)
			}
		})
	}
}

func Test_Resolve_BareRepoFillsFile(t *testing.T) {
	engine := NewEngine(nil, nil)
	got, err := engine.Resolve("toaweme/common", ".golangci.yml")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	want := "https://raw.githubusercontent.com/toaweme/common/HEAD/.golangci.yml"
	if got.URL() != want {
		t.Fatalf("url\n got: %s\nwant: %s", got.URL(), want)
	}
}

func Test_Resolve_LocalPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(file, []byte("local: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(nil, nil)

	// an absolute path and an existing bare name both resolve to the local file.
	for _, spec := range []string{file, "./" + filepath.Base(file)} {
		t.Run(spec, func(t *testing.T) {
			if spec[0] == '.' {
				t.Chdir(dir)
			}
			src, err := engine.Resolve(spec, "")
			if err != nil {
				t.Fatalf("Resolve(%q) error: %v", spec, err)
			}
			if src.Provider != providerLocal {
				t.Fatalf("provider = %q, want local", src.Provider)
			}
			got, err := engine.Bytes(t.Context(), src)
			if err != nil {
				t.Fatalf("Bytes error: %v", err)
			}
			if string(got) != "local: true\n" {
				t.Fatalf("unexpected content: %q", got)
			}
		})
	}
}

func Test_Resolve_EmbedByName(t *testing.T) {
	embed := func(name string) ([]byte, error) {
		if name != "taskfile.test.go.yml" {
			return nil, os.ErrNotExist
		}
		return []byte("tasks: {}"), nil
	}
	engine := NewEngine(nil, embed)

	src, err := engine.Resolve("taskfile.test.go.yml", "")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if src.Provider != providerEmbed {
		t.Fatalf("provider = %q, want embed", src.Provider)
	}
	got, err := engine.Bytes(t.Context(), src)
	if err != nil {
		t.Fatalf("Bytes error: %v", err)
	}
	if string(got) != "tasks: {}" {
		t.Fatalf("unexpected embed content: %q", got)
	}
}

func Test_Resolve_LocalShadowsEmbed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "taskfile.test.go.yml"), []byte("from-disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	embed := func(name string) ([]byte, error) { return []byte("from-embed"), nil }
	engine := NewEngine(nil, embed)

	t.Chdir(dir)
	src, err := engine.Resolve("taskfile.test.go.yml", "")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if src.Provider != providerLocal {
		t.Fatalf("an on-disk file must shadow the embed; provider = %q", src.Provider)
	}
}

func Test_Resolve_Errors(t *testing.T) {
	engine := NewEngine(nil, nil)
	tests := []struct {
		name string
		spec string
	}{
		{name: "empty", spec: "   "},
		{name: "owner only", spec: "totallynotafile"},
		{name: "foreign host url rejected", spec: "https://gitlab.com/toaweme/structs/blob/main/x.go"},
		{name: "github tree url unsupported", spec: "https://github.com/toaweme/structs/tree/main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := engine.Resolve(tt.spec, ""); err == nil {
				t.Fatalf("Resolve(%q) expected error, got nil", tt.spec)
			}
		})
	}
}
