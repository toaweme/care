// Package git inspects a git repository's working-tree status, upstream sync
// state, and identity header for care's report.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	stat := getGitDiffStat(filepath.Clean(repoDir))

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
		file := File{
			Name:   filepath.Base(filePath),
			Path:   filePath,
			Status: parseStatus(statusCode),
		}
		// the working-tree mtime is the "last touched" signal; a deleted file has
		// nothing to stat, so its ModTime stays zero.
		if st, err := os.Stat(filepath.Join(repoDir, filePath)); err == nil {
			file.ModTime = st.ModTime()
		}
		// the line delta comes from the diff against HEAD; an untracked file is not in
		// that diff, so every line it carries counts as added.
		if ls, ok := stat[filePath]; ok {
			file.Added, file.Deleted = ls.added, ls.deleted
		} else if file.Status == Untracked {
			file.Added = countLines(filepath.Join(repoDir, filePath))
		}
		files = append(files, file)
	}

	return files, nil
}

// lineStat is one path's added/deleted line counts in the diff against HEAD.
type lineStat struct{ added, deleted int }

// getGitDiffStat returns the per-path uncommitted line delta against HEAD (staged
// and unstaged), keyed by repo-root-relative path. Rename detection is on (-M), so a
// renamed-and-edited file reports its real intra-file delta (not a whole-file
// add/delete), keyed by the new path to match the status entry. Binary files report
// no counts and are left zero. A repo with no HEAD (or any failure) yields a nil map.
func getGitDiffStat(repoDir string) map[string]lineStat {
	var out bytes.Buffer
	cmd := exec.Command("git", "diff", "HEAD", "--numstat", "-M", "-z")
	cmd.Dir = filepath.Clean(repoDir)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	stats := make(map[string]lineStat)
	// -z emits NUL-terminated records. a normal record is one token
	// "added\tdeleted\tpath"; a rename splits across three tokens, the first being
	// "added\tdeleted\t" (empty path) followed by the old then the new path.
	tokens := strings.Split(out.String(), "\x00")
	for i := 0; i < len(tokens); i++ {
		fields := strings.SplitN(tokens[i], "\t", 3)
		if len(fields) != 3 {
			continue
		}
		// a binary file reports "-" for both counts; Atoi fails and leaves it zero.
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		path := fields[2]
		if path == "" && i+2 < len(tokens) {
			// rename/copy: key by the new path so it matches the status entry, which
			// keeps the new name; skip the consumed old/new path tokens.
			path = tokens[i+2]
			i += 2
		}
		if path == "" {
			continue
		}
		stats[path] = lineStat{added: added, deleted: deleted}
	}
	return stats
}

// countLines counts the lines in a file, treating a final line without a trailing
// newline as a line. It returns 0 when the path cannot be read (e.g. an untracked
// directory entry).
func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
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

// IgnoredFiles returns the subset of paths that git ignores in repoDir, as a
// set keyed by the path exactly as passed. It lets callers drop working-tree
// findings in intentionally gitignored files (e.g. a local .env), which are not
// leaks. `git check-ignore` exits 1 when no path matches, which is not an error,
// so any failure just yields an empty set and nothing is filtered.
func IgnoredFiles(repoDir string, paths []string) map[string]bool {
	ignored := make(map[string]bool, len(paths))
	if len(paths) == 0 {
		return ignored
	}
	var out bytes.Buffer
	// the binary is the constant "git" and `--` separates flags from the path
	// operands, so the variadic paths cannot inject options or another command.
	//nolint:gosec // G204: fixed "git" binary, paths are operands after `--`
	cmd := exec.Command("git", append([]string{"check-ignore", "--"}, paths...)...)
	cmd.Dir = filepath.Clean(repoDir)
	cmd.Stdout = &out
	_ = cmd.Run()
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line != "" {
			ignored[line] = true
		}
	}
	return ignored
}

// Info returns the repository's identity header (branch, commit, commit count,
// dirty and sync state, commit time, and working-tree touched time), best-effort
// per field.
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
	if branch, err := gitLine(p.Dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && branch != "HEAD" {
		// a detached HEAD (tagged CI checkout, rebase, bisect) reports the literal
		// "HEAD"; treat that as no branch so a caller can fill it from CI instead.
		info.Branch = branch
	}
	if tag, err := gitLine(p.Dir, "tag", "--points-at", "HEAD"); err == nil && tag != "" {
		// multiple tags on one commit come back newline-separated; the first is enough
		// to identify the release.
		info.Tag = strings.SplitN(tag, "\n", 2)[0]
	}
	if commit, err := gitLine(p.Dir, "rev-parse", "--short", "HEAD"); err == nil {
		info.Commit = commit
	}
	if commit, err := gitLine(p.Dir, "rev-parse", "HEAD"); err == nil {
		info.CommitFull = commit
	}
	if count, err := gitLine(p.Dir, "rev-list", "--count", "HEAD"); err == nil {
		// a malformed count just leaves Commits zero; not worth failing the header
		if _, serr := fmt.Sscanf(count, "%d", &info.Commits); serr != nil {
			info.Commits = 0
		}
	}
	if when, err := gitLine(p.Dir, "log", "-1", "--format=%cI"); err == nil && when != "" {
		if t, perr := time.Parse(time.RFC3339, when); perr == nil {
			info.CommittedAt = t
		}
	}

	files, err := getGitChangedFiles(p.Dir)
	if err != nil {
		return info, fmt.Errorf("failed to get git changed files: %w", err)
	}
	info.Dirty = len(files) > 0
	// touched-at is the newest working-tree mtime: when this repo was last worked on.
	// it stays zero for a clean tree so a consumer can sort it out of the active set.
	for _, f := range files {
		if f.ModTime.After(info.TouchedAt) {
			info.TouchedAt = f.ModTime
		}
		info.LinesAdded += f.Added
		info.LinesDeleted += f.Deleted
	}

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
