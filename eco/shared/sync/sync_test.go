package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// stubFetcher returns canned bytes, recording the source it was asked for.
type stubFetcher struct {
	body []byte
	err  error
	got  Source
}

func (s *stubFetcher) Fetch(_ context.Context, src Source) ([]byte, error) {
	s.got = src
	return s.body, s.err
}

func Test_Engine_Sync_WritesNewFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "nested", ".golangci.yml")
	fetcher := &stubFetcher{body: []byte("linters: []\n")}
	engine := NewEngine(fetcher, nil)

	res, err := engine.Sync(t.Context(), Request{Spec: "toaweme/common/.golangci.yml", Dest: dst})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if res.Skipped {
		t.Fatalf("expected a write, got skipped")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("dest not written: %v", err)
	}
	if string(got) != "linters: []\n" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func Test_Engine_Sync_SkipsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, ".golangci.yml")
	if err := os.WriteFile(dst, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(&stubFetcher{body: []byte("remote")}, nil)

	res, err := engine.Sync(t.Context(), Request{Spec: "toaweme/common/.golangci.yml", Dest: dst})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if !res.Skipped {
		t.Fatalf("expected skip on existing file")
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "original" {
		t.Fatalf("existing file was overwritten: %q", got)
	}
}

func Test_Engine_Sync_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, ".golangci.yml")
	if err := os.WriteFile(dst, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(&stubFetcher{body: []byte("remote")}, nil)

	if _, err := engine.Sync(t.Context(), Request{Spec: "toaweme/common/.golangci.yml", Dest: dst, Force: true}); err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "remote" {
		t.Fatalf("force did not overwrite: %q", got)
	}
}

func Test_Engine_Sync_BareRepoFillsFilenameFromDest(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, ".golangci.yml")
	fetcher := &stubFetcher{body: []byte("x")}
	engine := NewEngine(fetcher, nil)

	if _, err := engine.Sync(t.Context(), Request{Spec: "toaweme/common", Dest: dst}); err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	want := "https://raw.githubusercontent.com/toaweme/common/HEAD/.golangci.yml"
	if fetcher.got.URL() != want {
		t.Fatalf("filename not filled from dest:\n got: %s\nwant: %s", fetcher.got.URL(), want)
	}
}

func Test_Engine_Sync_RequiresDest(t *testing.T) {
	engine := NewEngine(&stubFetcher{body: []byte("x")}, nil)
	if _, err := engine.Sync(t.Context(), Request{Spec: "toaweme/common/.golangci.yml"}); err == nil {
		t.Fatalf("expected error for empty dest")
	}
}
