package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Engine syncs files into the working tree from a source spec. It owns the
// ordered resolution chain - local filesystem, then embedded templates, then the
// remote host providers - and the writing rules. It is the agnostic core both the
// generic `care setup` and the named presets call.
type Engine struct {
	fetcher   Fetcher
	providers []Provider
}

// NewEngine builds the sync engine. embed reads bundled templates by name (nil
// disables the embed step); fetcher pulls remote sources.
func NewEngine(fetcher Fetcher, embed EmbedFunc) *Engine {
	return &Engine{
		fetcher: fetcher,
		providers: []Provider{
			localProvider{},
			embedProvider{read: embed},
			gistProvider{},
			githubProvider{},
		},
	}
}

// Request is one file sync: resolve Spec's bytes and write them to Dest. When the
// source names only a repo (no file path), Dest's basename supplies the filename.
type Request struct {
	Spec  string
	Dest  string
	Force bool
}

// Result reports the outcome of a sync. Skipped is true when an existing dest was
// left untouched for want of Force.
type Result struct {
	Dest    string
	Source  string
	Bytes   int
	Skipped bool
}

// Resolve walks the provider chain for spec, filling a missing filename (a bare
// remote repo) from fillName. The first provider that claims the spec wins, so
// the local filesystem shadows an embedded template of the same name, which in
// turn shadows a remote shorthand.
func (e *Engine) Resolve(spec, fillName string) (Source, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		return Source{}, errors.New("empty source spec")
	}
	for _, p := range e.providers {
		src, ok, err := p.Resolve(raw)
		if err != nil {
			return Source{}, err
		}
		if !ok {
			continue
		}
		src.Raw = raw
		if !src.HasFile() {
			if fillName == "" {
				return Source{}, fmt.Errorf("source %s names no file and no destination filename was given", src)
			}
			src = src.WithFile(fillName)
		}
		return src, nil
	}
	return Source{}, fmt.Errorf("unrecognized source %q (expected a path, an embedded template name, a github/gist url, or owner/repo/path)", raw)
}

// Bytes returns a resolved source's content: read from disk, taken from the embed,
// or fetched from its URL.
func (e *Engine) Bytes(ctx context.Context, src Source) ([]byte, error) {
	switch src.kind {
	case kindLocal:
		data, err := os.ReadFile(src.path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", src, err)
		}
		return data, nil
	case kindEmbed:
		return src.data, nil
	default:
		return e.fetcher.Fetch(ctx, src)
	}
}

// Sync resolves the request's source and writes it to the destination, honoring
// the existing-file/Force rule.
func (e *Engine) Sync(ctx context.Context, req Request) (Result, error) {
	if req.Dest == "" {
		return Result{}, errors.New("no destination given (use --out)")
	}
	src, err := e.Resolve(req.Spec, filepath.Base(req.Dest))
	if err != nil {
		return Result{}, err
	}
	content, err := e.Bytes(ctx, src)
	if err != nil {
		return Result{}, err
	}

	wrote, err := WriteFile(req.Dest, content, req.Force)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Dest:    req.Dest,
		Source:  src.String(),
		Bytes:   len(content),
		Skipped: !wrote,
	}, nil
}

// WriteFile writes content to dest, creating parent directories. An existing dest
// is left untouched unless force is set; the bool reports whether a write
// happened.
func WriteFile(dest string, content []byte, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(dest); err == nil {
			return false, nil
		}
	}
	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, fmt.Errorf("failed to create directory %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(dest, content, 0o644); err != nil { //nolint:gosec // synced configs/templates (e.g. .golangci.yml) must stay world-readable
		return false, fmt.Errorf("failed to write %q: %w", dest, err)
	}
	return true, nil
}
