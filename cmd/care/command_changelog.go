package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/toaweme/cli"

	"github.com/toaweme/care/cmd/care/output"
	"github.com/toaweme/care/internal/changelog"
)

// ChangelogConfig is the flag set for `care changelog`. By default it prints a
// ref range's release notes to stdout, reading curated prose from --file for the
// natural range; with --write it instead maintains the CHANGELOG.md at --file.
type ChangelogConfig struct {
	Tag     string `arg:"0" env:"CARE_CHANGELOG_TAG" help:"Range end to describe (defaults to GITHUB_REF_NAME, the latest tag, then HEAD)"`
	Since   string `arg:"since" env:"CARE_CHANGELOG_SINCE" help:"Exclusive start of the range, any ref (defaults to the previous tag)"`
	Full    bool   `arg:"full" env:"CARE_CHANGELOG_FULL" default:"false" help:"Start from the first commit, ignoring tags (mutually exclusive with --since)"`
	File    string `arg:"file" short:"f" env:"CARE_CHANGELOG_FILE" default:"./CHANGELOG.md" help:"CHANGELOG.md path read for curated notes and written by --write"`
	Write   bool   `arg:"write" env:"CARE_CHANGELOG_WRITE" default:"false" help:"Maintain the CHANGELOG.md at --file instead of printing notes"`
	Release string `arg:"release" env:"CARE_CHANGELOG_RELEASE" help:"With --write, stage a section for an upcoming untagged version (push the branch first so handles resolve)"`
	Remote  string `arg:"remote" env:"CARE_CHANGELOG_REMOTE" help:"Remote URL overriding git-host detection (an unknown host degrades to the git-log path)"`
	Token   string `arg:"token" short:"t" env:"CARE_GIT_HOST_TOKEN" help:"Git-host API token for author handles and contributor extras (falls back to GITHUB_TOKEN/GH_TOKEN)"`
	Plain   bool   `arg:"plain" env:"CARE_CHANGELOG_PLAIN" default:"false" help:"Drop commit/PR links and author attribution, leaving only the cleaned subjects"`
}

// ChangelogCommand prints conventional-commit release notes and maintains a
// CHANGELOG.md.
type ChangelogCommand struct {
	cli.BaseCommand[ChangelogConfig]
}

var _ cli.Command[ChangelogConfig] = (*ChangelogCommand)(nil)

// NewChangelogCommand builds the changelog command.
func NewChangelogCommand() *ChangelogCommand {
	return &ChangelogCommand{BaseCommand: cli.NewBaseCommand[ChangelogConfig]()}
}

// Run maintains CHANGELOG.md when --write is set, otherwise prints a ref range's
// release notes to stdout.
func (c *ChangelogCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	ctx := context.Background()
	cwd := options.Cwd
	cfg := *c.Inputs
	engine := detectEngine(ctx, cwd, cfg)

	if cfg.Write {
		content, err := c.write(ctx, cwd, cfg, engine)
		if err != nil {
			return err
		}
		return writeFile(cwd, cfg.File, content)
	}
	return c.printNotes(ctx, cwd, cfg, engine)
}

// Help returns the changelog command's one-line listing summary.
func (c *ChangelogCommand) Help() string {
	return "Print release notes for a tag from your conventional commits, or maintain CHANGELOG.md with --write."
}

// Description returns the richer body shown in detailed help.
func (c *ChangelogCommand) Description() string {
	return "Derives release notes straight from git: groups conventional commits, strips their prefixes, and links each entry to its PR and commit. The range defaults to the latest tag and its predecessor; name a tag as the argument, set the start with --since, or span all history with --full. For the natural range, a matching curated section in --file (default ./CHANGELOG.md) is used verbatim. Pass --write to maintain that CHANGELOG.md instead of printing (adds missing versions, keeps your edits); add --release v0.2.0 to stage a section for the upcoming, not-yet-tagged version so you commit the changelog and then tag that commit. Pass --plain for a link-free, attribution-free body."
}

// Examples returns usage examples shown in detailed and agent help.
func (c *ChangelogCommand) Examples() [][]string {
	return [][]string{
		{"care changelog", "notes for the latest tag and its predecessor"},
		{"care changelog v1.2.0", "notes for a specific tag's range"},
		{"care changelog --since v1.0.0 v2.0.0", "notes across an explicit range"},
		{"care changelog --full --plain", "all history, no links or attribution"},
		{"care changelog --write", "create or update ./CHANGELOG.md from tags"},
		{"care changelog --write --release v0.2.0", "stage an untagged v0.2.0 section before tagging"},
	}
}

// write produces the maintained CHANGELOG.md content: the tag-driven Update by
// default, or InsertVersion when --release stages an as-yet-untagged version from
// the resolved range.
func (c *ChangelogCommand) write(ctx context.Context, cwd string, cfg ChangelogConfig, engine *changelog.Engine) (string, error) {
	existing := readChangelog(cwd, cfg.File)
	if cfg.Release == "" {
		content, err := engine.Update(ctx, existing)
		if err != nil {
			return "", fmt.Errorf("failed to update changelog: %w", err)
		}
		return content, nil
	}
	git := changelog.NewGit(cwd)
	// the staged version describes work past the latest tag, so the range ends at
	// the named ref (default HEAD), not the latest tag.
	to := cfg.Tag
	if to == "" {
		to = "HEAD"
	}
	from, err := resolveSince(ctx, git, cfg, to)
	if err != nil {
		return "", err
	}
	content, err := engine.InsertVersion(ctx, from, to, cfg.Release, today(), existing)
	if err != nil {
		return "", fmt.Errorf("failed to stage release %q in changelog: %w", cfg.Release, err)
	}
	return content, nil
}

// today is the local date stamped on a staged release section, in the Keep a
// Changelog YYYY-MM-DD format.
func today() string {
	return time.Now().Format("2006-01-02")
}

// printNotes resolves the ref range and prints its notes to stdout (redirect to
// capture).
func (c *ChangelogCommand) printNotes(ctx context.Context, cwd string, cfg ChangelogConfig, engine *changelog.Engine) error {
	git := changelog.NewGit(cwd)
	to, err := resolveTo(ctx, git, cfg.Tag)
	if err != nil {
		return err
	}
	if to == "" {
		return errors.New("nothing to describe: the repo has no tags; name a tag or commit as the argument")
	}
	from, err := resolveSince(ctx, git, cfg, to)
	if err != nil {
		return err
	}
	// only the natural range reads curated prose from the file; an explicit
	// --since/--full owns its range and must derive from git.
	var existing string
	if cfg.Since == "" && !cfg.Full {
		existing = readChangelog(cwd, cfg.File)
	}
	notes, err := engine.ExtractNotes(ctx, from, to, existing)
	if err != nil {
		return fmt.Errorf("failed to extract release notes for %s..%s: %w", from, to, err)
	}
	fmt.Print(notes)
	return nil
}

// detectEngine builds the engine: a git backend plus the detected git host (which
// degrades to the git-log path when the host is unknown or its API fails).
func detectEngine(ctx context.Context, cwd string, cfg ChangelogConfig) *changelog.Engine {
	git := changelog.NewGit(cwd)
	host := changelog.DetectGitHost(ctx, cwd, cfg.Remote, cfg.Token)
	return changelog.NewEngine(git, host, changelog.DefaultGroups, cfg.Plain)
}

// resolveTo picks the range end: the tag argument, else the CI tag ref, else the
// latest tag, else HEAD.
func resolveTo(ctx context.Context, git *changelog.Git, tag string) (string, error) {
	if tag != "" {
		return tag, nil
	}
	if os.Getenv("GITHUB_REF_TYPE") == "tag" {
		if ref := os.Getenv("GITHUB_REF_NAME"); ref != "" {
			return ref, nil
		}
	}
	latest, err := git.LatestTag(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve latest tag: %w", err)
	}
	if latest != "" {
		return latest, nil
	}
	return "HEAD", nil
}

// resolveSince picks the range start: empty (the first commit) with --full, the
// explicit --since ref, else the tag before to. --full and --since conflict.
func resolveSince(ctx context.Context, git *changelog.Git, cfg ChangelogConfig, to string) (string, error) {
	if cfg.Full && cfg.Since != "" {
		return "", errors.New("--full and --since are mutually exclusive")
	}
	if cfg.Full {
		return "", nil
	}
	if cfg.Since != "" {
		return cfg.Since, nil
	}
	// auto: the previous tag, or "" (first commit) when there is none.
	return git.PreviousTag(ctx, to)
}

// readChangelog reads the CHANGELOG.md, returning "" when it does not exist so
// the engine takes the fileless path.
func readChangelog(cwd, file string) string {
	data, err := os.ReadFile(resolveDest(cwd, file))
	if err != nil {
		return ""
	}
	return string(data)
}

// writeFile writes the maintained CHANGELOG.md to dest (relative to cwd) and
// prints a confirmation. Notes go straight to stdout instead, so a caller
// redirects them with the shell.
func writeFile(cwd, dest, content string) error {
	path := resolveDest(cwd, dest)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // a changelog file must stay world-readable
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	fmt.Printf("%s wrote %s\n", output.OKStyle.Render("✓"), filepath.Clean(path))
	return nil
}
