package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/toaweme/http"
)

// Option configures an installer at construction. The zero configuration uses a
// real exec-based command runner, an http-backed downloader, and the host platform.
type Option func(*options)

type options struct {
	runner CommandRunner
	dl     Downloader
	goos   string
	goarch string
	dir    string
}

func newOptions(opts ...Option) options {
	o := options{
		runner: execRunner{},
		dl:     newHTTPDownloader(),
		goos:   runtime.GOOS,
		goarch: runtime.GOARCH,
	}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithRunner injects a command runner (used in tests to avoid shelling out).
func WithRunner(r CommandRunner) Option { return func(o *options) { o.runner = r } }

// WithDownloader injects a downloader (used in tests to serve release assets locally).
func WithDownloader(d Downloader) Option { return func(o *options) { o.dl = d } }

// WithGOOS overrides the detected operating system (used by the brew and release installers).
func WithGOOS(goos string) Option { return func(o *options) { o.goos = goos } }

// WithGOARCH overrides the detected architecture (used by the release installer).
func WithGOARCH(goarch string) Option { return func(o *options) { o.goarch = goarch } }

// WithDir overrides the directory downloaded binaries are installed into (used in tests).
func WithDir(dir string) Option { return func(o *options) { o.dir = dir } }

// httpDownloader streams release archives through github.com/toaweme/http, bounding
// each download with its own timeout so a stalled transfer never blocks a run.
type httpDownloader struct {
	client  http.Client
	timeout time.Duration
}

var _ Downloader = httpDownloader{}

func newHTTPDownloader() httpDownloader {
	return httpDownloader{client: http.NewClient(http.Config{}), timeout: 5 * time.Minute}
}

func (d httpDownloader) Download(ctx context.Context, url string, w io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	resp, err := d.client.Get(ctx, http.GetRequest{Request: http.Request{Path: url, Stream: true}})
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp, 512))
		return fmt.Errorf("failed to download %s: http %d: %s", url, resp.StatusCode, bytes.TrimSpace(snippet))
	}
	if _, err := io.Copy(w, resp); err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	return nil
}

// execRunner runs commands as real OS processes via os/exec, the production
// counterpart to the command runner stubbed in tests.
type execRunner struct{}

var _ CommandRunner = execRunner{}

func (execRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func (execRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }
