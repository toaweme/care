package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDownloader serves canned bytes per URL.
type fakeDownloader struct {
	files map[string][]byte
}

func (d *fakeDownloader) Download(_ context.Context, url string, w io.Writer) error {
	body, ok := d.files[url]
	if !ok {
		return fmt.Errorf("not found: %s", url)
	}
	_, err := w.Write(body)
	return err
}

// tarGz builds a gzipped tar carrying one regular file at name with the given body.
func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

// zipArchive builds a zip carrying one file at name with the given body.
func zipArchive(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := f.Write(body); err != nil {
		t.Fatalf("write zip body: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func Test_Release_Install(t *testing.T) {
	binBody := []byte("#!/bin/sh\necho fake-tool\n")

	tests := []struct {
		name    string
		spec    *ReleaseSpec
		version string
		goos    string
		goarch  string
		// archive builds the asset bytes and returns (assetName, bytes).
		archive func(t *testing.T) (string, []byte)
		// checksumsName is the published checksums filename for this tool.
		checksumsName string
		wantErr       string
	}{
		{
			name:    "nested binary in tar.gz (golangci style)",
			version: "v2.12.2",
			goos:    "linux",
			goarch:  "amd64",
			spec: &ReleaseSpec{
				BaseURL:   "https://example.com/r/{tag}",
				Asset:     "tool-{version}-{os}-{arch}.tar.gz",
				Checksums: "tool-{version}-checksums.txt",
				BinPath:   "tool-{version}-{os}-{arch}/tool",
			},
			checksumsName: "tool-2.12.2-checksums.txt",
			archive: func(t *testing.T) (string, []byte) {
				t.Helper()
				return "tool-2.12.2-linux-amd64.tar.gz", tarGz(t, "tool-2.12.2-linux-amd64/tool", binBody)
			},
		},
		{
			name:    "root binary in tar.gz with arch token map (betterleaks style)",
			version: "v1.6.0",
			goos:    "linux",
			goarch:  "amd64",
			spec: &ReleaseSpec{
				BaseURL:   "https://example.com/r/{tag}",
				Asset:     "tool_{version}_{os}_{arch}.tar.gz",
				Checksums: "checksums.txt",
				Arch:      map[string]string{"amd64": "x64"},
			},
			checksumsName: "checksums.txt",
			archive: func(t *testing.T) (string, []byte) {
				t.Helper()
				return "tool_1.6.0_linux_x64.tar.gz", tarGz(t, "tool", binBody)
			},
		},
		{
			name:    "zip archive (windows style)",
			version: "v1.0.0",
			goos:    "windows",
			goarch:  "amd64",
			spec: &ReleaseSpec{
				BaseURL:   "https://example.com/r/{tag}",
				Asset:     "tool_{version}_{os}_{arch}.zip",
				Checksums: "checksums.txt",
			},
			checksumsName: "checksums.txt",
			archive: func(t *testing.T) (string, []byte) {
				t.Helper()
				return "tool_1.0.0_windows_amd64.zip", zipArchive(t, "tool.exe", binBody)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assetName, assetBytes := tt.archive(t)
			base := strings.NewReplacer("{tag}", tt.version, "{version}", strings.TrimPrefix(tt.version, "v")).Replace(tt.spec.BaseURL)
			checksums := fmt.Sprintf("%s  %s\n", sha256Hex(assetBytes), assetName)

			dl := &fakeDownloader{files: map[string][]byte{
				base + "/" + assetName:        assetBytes,
				base + "/" + tt.checksumsName: []byte(checksums),
			}}

			dir := t.TempDir()
			r := Release(
				WithDownloader(dl),
				WithRunner(&mockRunner{lookPath: map[string]bool{}}),
				WithGOOS(tt.goos),
				WithGOARCH(tt.goarch),
				WithDir(dir),
			)

			err := r.Install(t.Context(), Tool{Bin: "tool", Version: tt.version, Release: tt.spec})
			if tt.wantErr != "" {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Install() error = %v", err)
			}

			dest := filepath.Join(dir, binName("tool", tt.goos))
			got, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("read installed binary: %v", err)
			}
			if !bytes.Equal(got, binBody) {
				t.Fatalf("installed binary = %q, want %q", got, binBody)
			}
			info, err := os.Stat(dest)
			if err != nil {
				t.Fatalf("stat installed binary: %v", err)
			}
			if info.Mode().Perm()&0o100 == 0 {
				t.Fatalf("installed binary not executable: mode %v", info.Mode())
			}
		})
	}
}

func Test_Release_Install_ChecksumMismatch(t *testing.T) {
	spec := &ReleaseSpec{
		BaseURL:   "https://example.com/r/{tag}",
		Asset:     "tool_{version}_{os}_{arch}.tar.gz",
		Checksums: "checksums.txt",
	}
	asset := tarGz(t, "tool", []byte("real-bytes"))
	dl := &fakeDownloader{files: map[string][]byte{
		"https://example.com/r/v1.0.0/tool_1.0.0_linux_amd64.tar.gz": asset,
		"https://example.com/r/v1.0.0/checksums.txt":                 []byte("deadbeef  tool_1.0.0_linux_amd64.tar.gz\n"),
	}}
	r := Release(WithDownloader(dl), WithRunner(&mockRunner{}), WithGOOS("linux"), WithGOARCH("amd64"), WithDir(t.TempDir()))

	err := r.Install(t.Context(), Tool{Bin: "tool", Version: "v1.0.0", Release: spec})
	if err == nil || !contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum mismatch error, got %v", err)
	}
}

func Test_Release_Install_Requires(t *testing.T) {
	r := Release(WithDownloader(&fakeDownloader{}), WithRunner(&mockRunner{}), WithDir(t.TempDir()))

	if err := r.Install(t.Context(), Tool{Bin: "tool", Version: "v1.0.0"}); err == nil || !contains(err.Error(), "no release coordinates") {
		t.Fatalf("want missing-coordinates error, got %v", err)
	}
	spec := &ReleaseSpec{BaseURL: "https://x/{tag}", Asset: "a", Checksums: "c"}
	if err := r.Install(t.Context(), Tool{Bin: "tool", Release: spec}); err == nil || !contains(err.Error(), "pinned version is required") {
		t.Fatalf("want missing-version error, got %v", err)
	}
}

func Test_Release_Install_CosignVerifies(t *testing.T) {
	bin := []byte("tool-bin")
	asset := tarGz(t, "tool", bin)
	checksums := sha256Hex(asset) + "  tool_1.6.0_linux_x64.tar.gz\n"
	base := "https://example.com/r/v1.6.0"
	dl := &fakeDownloader{files: map[string][]byte{
		base + "/tool_1.6.0_linux_x64.tar.gz": asset,
		base + "/checksums.txt":               []byte(checksums),
		base + "/checksums.txt.sigstore.json": []byte("{bundle}"),
	}}
	runner := &mockRunner{lookPath: map[string]bool{"cosign": true}}
	r := Release(WithDownloader(dl), WithRunner(runner), WithGOOS("linux"), WithGOARCH("amd64"), WithDir(t.TempDir()))

	spec := &ReleaseSpec{
		BaseURL:   "https://example.com/r/{tag}",
		Asset:     "tool_{version}_{os}_{arch}.tar.gz",
		Checksums: "checksums.txt",
		Arch:      map[string]string{"amd64": "x64"},
		Cosign:    &Cosign{IdentityRegexp: "^https://github.com/x/", Issuer: "https://token.actions.githubusercontent.com"},
	}
	if err := r.Install(t.Context(), Tool{Bin: "tool", Version: "v1.6.0", Release: spec}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	var verified bool
	for _, c := range runner.runCalls {
		if c.name == "cosign" && len(c.args) > 0 && c.args[0] == "verify-blob" {
			verified = true
		}
	}
	if !verified {
		t.Fatalf("expected cosign verify-blob to run, calls: %v", runner.runCalls)
	}
}

func Test_Release_Install_CosignFailsClosed(t *testing.T) {
	bin := []byte("tool-bin")
	asset := tarGz(t, "tool", bin)
	checksums := sha256Hex(asset) + "  tool_1.6.0_linux_amd64.tar.gz\n"
	base := "https://example.com/r/v1.6.0"
	dl := &fakeDownloader{files: map[string][]byte{
		base + "/tool_1.6.0_linux_amd64.tar.gz": asset,
		base + "/checksums.txt":                 []byte(checksums),
		base + "/checksums.txt.sigstore.json":   []byte("{bundle}"),
	}}
	// cosign present but verification fails.
	runner := &mockRunner{lookPath: map[string]bool{"cosign": true}, runErr: errors.New("bad signature")}
	r := Release(WithDownloader(dl), WithRunner(runner), WithGOOS("linux"), WithGOARCH("amd64"), WithDir(t.TempDir()))

	spec := &ReleaseSpec{
		BaseURL:   "https://example.com/r/{tag}",
		Asset:     "tool_{version}_{os}_{arch}.tar.gz",
		Checksums: "checksums.txt",
		Cosign:    &Cosign{IdentityRegexp: "^x", Issuer: "y"},
	}
	if err := r.Install(t.Context(), Tool{Bin: "tool", Version: "v1.6.0", Release: spec}); err == nil || !contains(err.Error(), "cosign-verify") {
		t.Fatalf("want cosign verification failure, got %v", err)
	}
}
