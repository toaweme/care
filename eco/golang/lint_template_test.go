package golang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test_RenderGolangciConfig asserts the invariants that matter regardless of which
// template version is shipped: a rendered config never carries the raw placeholder,
// and every supplied prefix is expanded into its own local-prefixes item.
func Test_RenderGolangciConfig(t *testing.T) {
	const placeholder = "__IMPORT_SORT_PREFIXES__"
	prefixes := []string{"github.com/toaweme", "github.com/awee-ai"}

	withPrefixes, err := RenderGolangciConfig(prefixes)
	if err != nil {
		t.Fatalf("failed to render golangci-lint config: %v", err)
	}
	if strings.Contains(string(withPrefixes), placeholder) {
		t.Fatalf("rendered config still contains the raw placeholder %q", placeholder)
	}
	// if the template uses local-prefixes, every prefix must be expanded into an item.
	if strings.Contains(string(withPrefixes), "local-prefixes") {
		for _, p := range prefixes {
			if !strings.Contains(string(withPrefixes), "        - "+p+"\n") {
				t.Fatalf("local-prefixes template did not expand prefix %q", p)
			}
		}
	}

	none, err := RenderGolangciConfig(nil)
	if err != nil {
		t.Fatalf("failed to render golangci-lint config without prefixes: %v", err)
	}
	if strings.Contains(string(none), placeholder) {
		t.Fatalf("rendered config without prefixes still contains %q", placeholder)
	}
	for _, want := range []string{"version: \"2\"", "linters:", "formatters:"} {
		if !strings.Contains(string(none), want) {
			t.Fatalf("rendered config missing %q", want)
		}
	}
}

func Test_FindGolangciConfig(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}

	if _, found := FindGolangciConfig(sub); found {
		t.Fatalf("expected no config before any is written")
	}

	cfg := filepath.Join(root, GolangciConfigName)
	if err := os.WriteFile(cfg, []byte("version: \"2\"\n"), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// a config at the root governs nested dirs, mirroring golangci-lint discovery.
	got, found := FindGolangciConfig(sub)
	if !found {
		t.Fatalf("expected to find the root config from a nested dir")
	}
	if got != cfg {
		t.Fatalf("found %q, want %q", got, cfg)
	}
}
