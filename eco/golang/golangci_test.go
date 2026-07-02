package golang

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_FindGolangciConfig(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}

	if _, found := FindGolangciConfig(sub); found {
		t.Fatalf("expected no config before any is written")
	}

	cfg := filepath.Join(root, golangciConfigNames[0])
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
