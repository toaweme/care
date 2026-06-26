package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// release installs a tool by downloading its prebuilt release archive, verifying it
// against the published checksums (and, best-effort, a cosign sigstore bundle), and
// extracting the binary into a care-managed directory kept on PATH. It avoids
// compiling from source, which is what `go install` would otherwise pay for.
type release struct {
	dl     Downloader
	runner CommandRunner
	goos   string
	goarch string
	dir    string
}

var _ Installer = (*release)(nil)

// Release returns an installer that provisions tools from prebuilt release archives.
func Release(opts ...Option) Installer {
	o := newOptions(opts...)
	dir := o.dir
	if dir == "" {
		dir = defaultBinDir()
	}
	r := &release{dl: o.dl, runner: o.runner, goos: o.goos, goarch: o.goarch, dir: dir}
	ensureOnPath(dir)
	return r
}

// Available reports the download method usable whenever a destination directory is
// known. Network or asset failures surface from Install, so the caller can fall back.
func (r *release) Available() bool { return r.dir != "" }

// IsInstalled reports whether the binary is already resolvable, either on the system
// PATH or in the care-managed download directory from a previous run.
func (r *release) IsInstalled(tool Tool) bool {
	if _, err := r.runner.LookPath(tool.Bin); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(r.dir, binName(tool.Bin, r.goos))); err == nil {
		return true
	}
	return false
}

// Install downloads, verifies, and extracts the tool's release binary into r.dir.
func (r *release) Install(ctx context.Context, tool Tool) error {
	rel := tool.Release
	if rel == nil {
		return fmt.Errorf("failed to install %q via download: no release coordinates configured", tool.Bin)
	}
	if tool.Version == "" {
		return fmt.Errorf("failed to install %q via download: a pinned version is required", tool.Bin)
	}

	tag, version := tagAndVersion(tool.Version)
	osTok := token(rel.OS, r.goos)
	archTok := token(rel.Arch, r.goarch)
	expand := strings.NewReplacer("{tag}", tag, "{version}", version, "{os}", osTok, "{arch}", archTok).Replace

	asset := expand(rel.Asset)
	base := strings.TrimSuffix(expand(rel.BaseURL), "/")
	assetURL := base + "/" + asset
	checksums := expand(rel.Checksums)
	checksumsURL := base + "/" + checksums

	work, err := os.MkdirTemp("", "care-release-")
	if err != nil {
		return fmt.Errorf("failed to create work dir for %q: %w", tool.Bin, err)
	}
	defer os.RemoveAll(work)

	assetPath := filepath.Join(work, asset)
	checksumsPath := filepath.Join(work, checksums)
	if err := r.fetch(ctx, assetURL, assetPath); err != nil {
		return err
	}
	if err := r.fetch(ctx, checksumsURL, checksumsPath); err != nil {
		return err
	}

	if err := r.verifyCosign(ctx, rel, work, checksums, checksumsURL); err != nil {
		return err
	}
	if err := verifySHA256(assetPath, checksumsPath, asset); err != nil {
		return fmt.Errorf("failed to verify %q download: %w", tool.Bin, err)
	}

	binPath := ""
	if rel.BinPath != "" {
		binPath = expand(rel.BinPath)
	}
	dest := filepath.Join(r.dir, binName(tool.Bin, r.goos))
	if err := extractBinary(assetPath, asset, binPath, tool.Bin, r.goos, dest); err != nil {
		return fmt.Errorf("failed to extract %q from %s: %w", tool.Bin, asset, err)
	}
	ensureOnPath(r.dir)
	return nil
}

// fetch downloads url into a freshly created file at dest.
func (r *release) fetch(ctx context.Context, url, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Base(dest), err)
	}
	defer f.Close()
	return r.dl.Download(ctx, url, f)
}

// verifyCosign best-effort verifies the checksums file's keyless sigstore bundle.
// An absent bundle or missing cosign binary is skipped; a present-and-failing
// verification rejects the download.
func (r *release) verifyCosign(ctx context.Context, rel *ReleaseSpec, work, checksums, checksumsURL string) error {
	if rel.Cosign == nil {
		return nil
	}
	// best-effort: with no cosign binary or no published bundle, the sha256 check
	// still gates the asset, so a missing prerequisite is a skip, not a failure.
	cosignAvailable := r.runner != nil
	if cosignAvailable {
		_, lookErr := r.runner.LookPath("cosign")
		cosignAvailable = lookErr == nil
	}
	if !cosignAvailable {
		return nil
	}
	bundlePath := filepath.Join(work, checksums+".sigstore.json")
	// no published bundle for this release: sha256 still gates the asset.
	if r.fetch(ctx, checksumsURL+".sigstore.json", bundlePath) == nil {
		out, err := r.runner.Run(ctx, work, "cosign", "verify-blob",
			"--bundle", bundlePath,
			"--certificate-identity-regexp", rel.Cosign.IdentityRegexp,
			"--certificate-oidc-issuer", rel.Cosign.Issuer,
			filepath.Join(work, checksums),
		)
		if err != nil {
			return fmt.Errorf("failed to cosign-verify %s (output: %s): %w", checksums, trimOutput(out), err)
		}
	}
	return nil
}

// verifySHA256 computes the asset's sha256 and compares it to the matching entry in
// the checksums file (lines of "<hex>  <filename>"), failing on mismatch or absence.
func verifySHA256(assetPath, checksumsPath, asset string) error {
	want, err := checksumFor(checksumsPath, asset)
	if err != nil {
		return err
	}
	f, err := os.Open(assetPath)
	if err != nil {
		return fmt.Errorf("failed to open downloaded asset: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash downloaded asset: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, want, got)
	}
	return nil
}

// checksumFor returns the hex sha256 listed for asset in a checksums file.
func checksumFor(checksumsPath, asset string) (string, error) {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read checksums file: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s not listed in checksums file", asset)
}

// extractBinary pulls the tool binary out of a .tar.gz or .zip archive and writes it
// to dest with an executable mode. binPath, when set, is the exact archive entry to
// extract; otherwise the entry whose base name matches bin (the toolchain binary at
// the archive root) is used.
func extractBinary(archivePath, asset, binPath, bin, goos, dest string) error {
	wantName := binName(bin, goos)
	match := func(name string) bool {
		if binPath != "" {
			return path.Clean(name) == path.Clean(binPath)
		}
		return path.Base(name) == wantName || path.Base(name) == bin
	}
	switch {
	case strings.HasSuffix(asset, ".tar.gz") || strings.HasSuffix(asset, ".tgz"):
		return extractTarGz(archivePath, match, dest)
	case strings.HasSuffix(asset, ".zip"):
		return extractZip(archivePath, match, dest)
	default:
		return fmt.Errorf("unsupported archive format: %s", asset)
	}
}

func extractTarGz(archivePath string, match func(string) bool, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to open gzip stream: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || !match(hdr.Name) {
			continue
		}
		return writeBinary(dest, tr)
	}
	return errors.New("binary not found in archive")
}

func extractZip(archivePath string, match func(string) bool, dest string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer zr.Close()
	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() || !match(entry.Name) {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry: %w", err)
		}
		err = writeBinary(dest, rc)
		rc.Close()
		return err
	}
	return errors.New("binary not found in archive")
}

// writeBinary streams an archive entry to dest with an executable mode, replacing any
// stale binary from a previous run.
func writeBinary(dest string, src io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create binary file: %w", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return fmt.Errorf("failed to write binary: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to flush binary: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}
	return nil
}

// tagAndVersion splits a pin into the tag form (leading v) used in release URLs and
// the bare form used in asset filenames.
func tagAndVersion(pin string) (tag, version string) {
	version = strings.TrimPrefix(pin, "v")
	return "v" + version, version
}

// token maps a GOOS/GOARCH value to the token a tool's asset names use, defaulting
// to the value itself when the tool follows Go's own naming.
func token(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return key
}

// binName appends .exe on Windows so PATH lookups and archive matching line up.
func binName(bin, goos string) string {
	if goos == "windows" {
		return bin + ".exe"
	}
	return bin
}

// defaultBinDir is the care-managed directory downloaded binaries live in, under the
// user cache dir when available and a temp dir otherwise.
func defaultBinDir() string {
	if cache, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cache, "care", "bin")
	}
	return filepath.Join(os.TempDir(), "care-bin")
}

// ensureOnPath prepends dir to the process PATH if absent, so binaries downloaded
// this run are resolvable by the exec-based run stage that follows.
func ensureOnPath(dir string) {
	if dir == "" {
		return
	}
	current := os.Getenv("PATH")
	for _, p := range filepath.SplitList(current) {
		if p == dir {
			return
		}
	}
	if current == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+current)
}
