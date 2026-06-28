package changelog

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// fieldSep separates the fields in a git-log record. A NUL (0x00) can't be
// passed in an exec argument, so we use the unit-separator control byte, which
// never appears in a one-line subject or an author name.
const fieldSep = "\x1f"

// Git is the host-neutral commit backend: it reads tags and commits straight
// from the local repository via the git CLI, with no git-host involvement. It is
// always available and is the fallback whenever a GitHost is absent or fails.
type Git struct {
	dir string
}

// NewGit builds a git backend rooted at dir.
func NewGit(dir string) *Git {
	return &Git{dir: filepath.Clean(dir)}
}

// Tags lists every tag, newest version first.
func (g *Git) Tags(ctx context.Context) ([]string, error) {
	out, err := g.run(ctx, "tag", "--list", "--sort=-version:refname")
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	var tags []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			tags = append(tags, line)
		}
	}
	return tags, nil
}

// LatestTag returns the newest tag, or "" when the repo has none.
func (g *Git) LatestTag(ctx context.Context) (string, error) {
	tags, err := g.Tags(ctx)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", nil
	}
	return tags[0], nil
}

// PreviousTag returns the nearest tag in ref's history before ref itself (the
// previous release), or "" when there is none (a first release). ref may be any
// git ref: a tag, branch, or HEAD. It is purely ancestor-based, so it works with
// any tag naming and never assumes a version scheme.
func (g *Git) PreviousTag(ctx context.Context, ref string) (string, error) {
	out, err := g.run(ctx, "describe", "--tags", "--abbrev=0", ref+"^")
	if err != nil {
		// no earlier tag (or ref has no parent): not an error, it's a first release.
		return "", nil //nolint:nilerr // a missing predecessor tag is the first-release signal, not a failure
	}
	return strings.TrimSpace(out), nil
}

// CommitsInRange returns the commits in (from, to], newest first, parsed for
// their conventional fields. Author is the git author name (the host-neutral
// identity); no handle is set. from is "" to include every commit reachable from
// to (a first release). Merge commits are excluded.
func (g *Git) CommitsInRange(ctx context.Context, from, to string) ([]Commit, error) {
	format := "%H" + fieldSep + "%s" + fieldSep + "%an"
	args := []string{"log", "--no-merges", "--format=" + format}
	if from == "" {
		args = append(args, to)
	} else {
		args = append(args, from+".."+to)
	}
	out, err := g.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to read commits for %s..%s: %w", from, to, err)
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, fieldSep, 3)
		if len(fields) != 3 {
			continue
		}
		c := Commit{Hash: fields[0], Subject: fields[1], Author: fields[2]}
		Parse(&c)
		commits = append(commits, c)
	}
	return commits, nil
}

// TagDate returns the commit date (YYYY-MM-DD) the tag points at, for the
// version section header. A missing tag yields an empty string, not an error.
func (g *Git) TagDate(ctx context.Context, tag string) string {
	out, err := g.run(ctx, "log", "-1", "--format=%cd", "--date=short", tag)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (g *Git) run(ctx context.Context, args ...string) (string, error) {
	var out bytes.Buffer
	// the binary is the constant "git"; args are git operands, never shell.
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}
