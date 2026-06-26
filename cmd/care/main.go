package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/toaweme/cli"
	builtinCommands "github.com/toaweme/cli/commands/help"
	"github.com/toaweme/cli/config"
	yamlcodec "github.com/toaweme/cli/config/addons/yaml"
	"github.com/toaweme/http"

	"github.com/toaweme/care"
	"github.com/toaweme/care/cmd/care/output"
	"github.com/toaweme/care/eco/golang"
	gotools "github.com/toaweme/care/eco/golang/tools"
	"github.com/toaweme/care/eco/shared"
	sharedtools "github.com/toaweme/care/eco/shared/tools"
	"github.com/toaweme/care/internal/devops/git"
	"github.com/toaweme/care/internal/rating"
	"github.com/toaweme/care/templates"
)

var version = "0.0.0"

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		slog.Error("care", "error", fmt.Errorf("failed to get cwd: %w", err))
		os.Exit(1)
	}
	if err := run(cwd, os.Args[1:]); err != nil {
		// failing checks are already rendered; exit non-zero without an app-error log.
		if !errors.Is(err, errChecksFailed) {
			slog.Error("care", "error", fmt.Errorf("failed to run: %w", err))
		}
		os.Exit(1)
	}
}

// run loads config, builds and injects the tools, registers the features by role,
// and wires the status command. A help request is treated as success.
func run(cwd string, args []string) error {
	// optional: a missing or unreadable .env is not an error.
	_ = cli.LoadDotEnv()

	yml := yamlcodec.New(".yml")
	stores := []config.Store{
		config.NewFileStore(config.HomePath(".care"), "care", true, yml),
		config.NewFileStore(cwd, "care", true, yml),
	}
	cfg := care.Defaults()
	for _, store := range stores {
		// optional: a missing config store layers nothing and is not an error.
		_ = store.Read(&cfg)
	}
	app := cli.NewApp(cli.Config{Name: "care", Version: version}, cli.GlobalFlags{Cwd: cwd})

	// build the tools at the top (with any operator version pin), then inject them
	// into the features that fill the ecosystem's feature slots.
	golangci := gotools.NewGolangCiLint(cfg.ToolVersion("golangci-lint"))
	betterleaks := sharedtools.NewBetterleaks(cfg.ToolVersion("betterleaks"))
	govulncheck := gotools.NewGovulncheck(cfg.ToolVersion("govulncheck"))
	gotool := gotools.Go()
	gofmt := gotools.Gofmt()

	eco := &care.Ecosystem{
		VersionControl:  shared.NewVersionControl(),
		Build:           golang.NewBuild(gotool),
		Quality:         golang.NewQuality(golangci, gotool, gofmt),
		Dependencies:    golang.NewGoMod(gotool),
		Runtime:         golang.NewRuntime(gotool),
		Docs:            golang.NewDocs(floatOption(cfg, "docs", "min")),
		Tests:           golang.NewTests(gotool, cfg.Profiles.Tests),
		Benchmark:       golang.NewBenchmark(gotool, cfg.Profiles.Bench),
		Secrets:         shared.NewBetterleaks(betterleaks, boolOption(cfg, "sec.secrets", "history")),
		Vulnerabilities: golang.NewGovulncheck(govulncheck, cfg.CheckOption("sec.vuln", "db")),
		Fixer:           golang.NewFixer(golangci, gotool),
	}

	runner := care.NewRunner(cfg.AutoInstall, cfg.Tools)
	grading := rating.FromConfig(cfg.Health.Weights, cfg.Health.Caps)
	statusCommand := NewStatusCommand(eco, runner, golang.ModulePath, cfg.CheckDisabled, resolveVC, grading)
	helpCommand := builtinCommands.NewHelpCommand(app.Config, app.Commands, app.OutputFormats, app.DefaultCommand)
	app.Help(helpCommand)
	app.Default(statusCommand)
	app.Add("status", statusCommand)

	// an empty base URL lets the fetcher GET fully-qualified raw URLs verbatim.
	httpClient := http.NewClient(http.Config{UserAgent: "care"})
	getCommand := NewGetCommand(httpClient, templates.FS.ReadFile)
	app.Add("get", getCommand)
	getCommand.Add("lint", NewGetLintCommand(httpClient, templates.FS.ReadFile, golang.ModulePath))

	if err := app.Run(args); cli.IsRealError(err) {
		return err
	}

	return nil
}

// resolveVC reads the repo's version-control identity for the report header. A
// non-git dir (or a probe failure) yields an empty header rather than an error: the
// report still renders, just without the VC line.
func resolveVC(dir string) *output.VCInfo {
	info, err := git.NewRepository(dir).Info(context.Background())
	if err != nil {
		return nil
	}
	branch, tag := ciRef(info.Branch, info.Tag)
	// a tagged CI build checks out a detached HEAD, so branch is empty; a tag (or a
	// commit) is still enough identity to emit the header.
	if branch == "" && tag == "" && info.Commit == "" {
		return nil
	}
	vc := &output.VCInfo{
		Branch:       branch,
		Tag:          tag,
		Commit:       info.Commit,
		CommitFull:   info.CommitFull,
		Commits:      info.Commits,
		Dirty:        info.Dirty,
		HasUpstream:  info.HasUpstream,
		Ahead:        info.Ahead,
		Behind:       info.Behind,
		LinesAdded:   info.LinesAdded,
		LinesDeleted: info.LinesDeleted,
	}
	if !info.CommittedAt.IsZero() {
		t := info.CommittedAt
		vc.CommittedAt = &t
	}
	if !info.TouchedAt.IsZero() {
		t := info.TouchedAt
		vc.TouchedAt = &t
	}
	return vc
}

// ciRef fills branch and tag from the CI runner's environment when git cannot
// supply them, which is the normal case for a tagged release: the runner checks out
// a detached HEAD, so git reports no branch, but the env names the ref. GitHub
// Actions sets GITHUB_REF_TYPE + GITHUB_REF_NAME; GitLab sets CI_COMMIT_TAG and
// CI_COMMIT_REF_NAME. Git-derived values win when present, so local runs are
// unaffected.
func ciRef(branch, tag string) (string, string) {
	if tag == "" {
		if t := os.Getenv("CI_COMMIT_TAG"); t != "" {
			tag = t
		} else if os.Getenv("GITHUB_REF_TYPE") == "tag" {
			tag = os.Getenv("GITHUB_REF_NAME")
		}
	}
	if branch == "" {
		if os.Getenv("GITHUB_REF_TYPE") == "branch" {
			branch = os.Getenv("GITHUB_REF_NAME")
		} else if b := os.Getenv("CI_COMMIT_BRANCH"); b != "" {
			branch = b
		}
	}
	return branch, tag
}

func boolOption(cfg care.Config, check, option string) bool {
	v, err := strconv.ParseBool(cfg.CheckOption(check, option))
	return err == nil && v
}

func floatOption(cfg care.Config, check, option string) float64 {
	v, err := strconv.ParseFloat(cfg.CheckOption(check, option), 64)
	if err != nil {
		return 0
	}
	return v
}
