// Package git inspects a git repository's working-tree status, upstream sync
// state, and identity header for mend's report.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo inspects a git repository's working tree and upstream sync state.
type Repo struct {
	Dir string
}

var _ Repository = (*Repo)(nil)

// NewRepository creates a repository for status and sync checks.
func NewRepository(dir string) Repository {
	return &Repo{Dir: extractTildePath(dir)}
}

func extractTildePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// getGitChangedFiles returns the changed files reported by git status in repoDir.
func getGitChangedFiles(repoDir string) ([]File, error) {
	var out bytes.Buffer

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = filepath.Clean(repoDir)
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var files []File
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.ReplaceAll(line, "  ", " ")
		var parts []string
		if strings.Contains(line, "->") {
			parts = strings.Split(line, "->")
			parts = []string{parts[0], parts[len(parts)-1]}
		} else {
			parts = strings.Split(line, " ")
		}
		statusCode := strings.TrimSpace(parts[0])
		filePath := strings.TrimSpace(parts[1])
		files = append(files, File{
			Name:   filepath.Base(filePath),
			Status: parseStatus(statusCode),
		})
	}

	return files, nil
}

func parseStatus(code string) FileStatus {
	switch {
	case strings.Contains(code, "??"):
		return Untracked
	case strings.Contains(code, "M"):
		return Modified
	case strings.Contains(code, "A"):
		return Added
	case strings.Contains(code, "D"):
		return Deleted
	case strings.Contains(code, "R"):
		return Renamed
	default:
		return Unmodified
	}
}

func revListCount(repoDir, refSpec string) (behind, ahead int, ok bool) {
	var out bytes.Buffer

	cmd := exec.Command("git", "rev-list", "--left-right", "--count", refSpec)
	cmd.Dir = filepath.Clean(repoDir)
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return 0, 0, false
	}

	fields := strings.Fields(out.String())
	if len(fields) != 2 {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(fields[0], "%d", &behind); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(fields[1], "%d", &ahead); err != nil {
		return 0, 0, false
	}
	return behind, ahead, true
}

func getGitSyncStatus(repoDir string) (SyncStatus, error) {
	// prefer the configured upstream
	if behind, ahead, ok := revListCount(repoDir, "@{upstream}...HEAD"); ok {
		return SyncStatus{HasUpstream: true, Ahead: ahead, Behind: behind}, nil
	}
	// fall back to common default branches when no upstream is configured
	for _, ref := range []string{"origin/main", "origin/master"} {
		if behind, ahead, ok := revListCount(repoDir, ref+"...HEAD"); ok {
			return SyncStatus{HasUpstream: true, Ahead: ahead, Behind: behind}, nil
		}
	}
	return SyncStatus{HasUpstream: false}, nil
}

// SyncStatus reports how the working tree's branch relates to its upstream,
// returning a zero-value (no upstream) status when the directory is not a repo.
func (p *Repo) SyncStatus() (SyncStatus, error) {
	_, err := os.Stat(filepath.Join(p.Dir, ".git"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SyncStatus{HasUpstream: false}, nil
		}
		return SyncStatus{}, fmt.Errorf("failed to check .git directory: %w", err)
	}

	return getGitSyncStatus(p.Dir)
}

// gitLine runs a git command in repoDir and returns its trimmed single-line
// stdout, or "" and an error when the command fails.
func gitLine(repoDir string, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.Clean(repoDir)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// Info returns the repository's identity header (branch, commit, commit count,
// dirty and sync state, last commit time), best-effort per field.
func (p *Repo) Info() (Info, error) {
	_, err := os.Stat(filepath.Join(p.Dir, ".git"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Info{}, nil
		}
		return Info{}, fmt.Errorf("failed to check .git directory: %w", err)
	}

	var info Info
	// each field is best-effort: a fresh repo with no commits has no HEAD, so a
	// failing probe leaves its field zero rather than failing the whole header.
	if branch, err := gitLine(p.Dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = branch
	}
	if commit, err := gitLine(p.Dir, "rev-parse", "--short", "HEAD"); err == nil {
		info.Commit = commit
	}
	if count, err := gitLine(p.Dir, "rev-list", "--count", "HEAD"); err == nil {
		// a malformed count just leaves Commits zero; not worth failing the header
		if _, serr := fmt.Sscanf(count, "%d", &info.Commits); serr != nil {
			info.Commits = 0
		}
	}
	if when, err := gitLine(p.Dir, "log", "-1", "--format=%cI"); err == nil && when != "" {
		if t, perr := time.Parse(time.RFC3339, when); perr == nil {
			info.LastCommit = t
		}
	}

	files, err := getGitChangedFiles(p.Dir)
	if err != nil {
		return info, fmt.Errorf("failed to get git changed files: %w", err)
	}
	info.Dirty = len(files) > 0

	sync, err := getGitSyncStatus(p.Dir)
	if err != nil {
		return info, fmt.Errorf("failed to get git sync status: %w", err)
	}
	info.HasUpstream = sync.HasUpstream
	info.Ahead = sync.Ahead
	info.Behind = sync.Behind

	return info, nil
}

// Status returns the working tree's changed files, or an empty slice when the
// directory is not a git repository.
func (p *Repo) Status() ([]File, error) {
	_, err := os.Stat(filepath.Join(p.Dir, ".git"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make([]File, 0), nil
		}
		return nil, fmt.Errorf("failed to check .git directory: %w", err)
	}

	files, err := getGitChangedFiles(p.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get git changed files: %w", err)
	}

	return files, nil
}
