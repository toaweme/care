package main

import (
	"errors"
	"os"
	"strconv"

	"github.com/toaweme/cli"
	builtinCommands "github.com/toaweme/cli/commands/help"
	"github.com/toaweme/cli/config"
	yamlcodec "github.com/toaweme/cli/config/addons/yaml"
	"github.com/toaweme/log"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/cmd/mend/output"
	"github.com/toaweme/mend/eco/golang"
	gotools "github.com/toaweme/mend/eco/golang/tools"
	"github.com/toaweme/mend/eco/shared"
	sharedtools "github.com/toaweme/mend/eco/shared/tools"
	"github.com/toaweme/mend/internal/devops/git"
	"github.com/toaweme/mend/internal/rating"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Error("error getting working directory", "error", err)
		os.Exit(1)
	}
	if err := run(cwd, os.Args[1:]); err != nil {
		// failing checks are already rendered; exit non-zero without an app-error log.
		if !errors.Is(err, errChecksFailed) {
			log.Error("error running app", "error", err)
		}
		os.Exit(1)
	}
}

// run loads config, builds and injects the tools, registers the features by role,
// and wires the status command. A help request is treated as success.
func run(cwd string, args []string) error {
	if err := cli.LoadDotEnv(); err != nil {
		log.Warn("error loading .env", "error", err)
	}

	yml := yamlcodec.New(".yml")
	stores := []config.Store{
		config.NewFileStore(config.HomePath(".mend"), "mend", yml),
		config.NewFileStore(cwd, "mend", yml),
	}
	cfg := mend.Defaults()
	for _, store := range stores {
		if err := store.Read(&cfg); err != nil {
			log.Warn("error loading config", "error", err)
		}
	}
	app := cli.NewApp(cli.Config{Name: "mend", Version: "1.0.0"}, cli.GlobalFlags{Cwd: cwd})

	// build the tools at the top (with any operator version pin), then inject them
	// into the features that fill the ecosystem's feature slots.
	golangci := gotools.NewGolangCiLint(cfg.Tools["golangci-lint"].Version)
	betterleaks := sharedtools.NewBetterleaks(cfg.Tools["betterleaks"].Version)
	govulncheck := gotools.NewGovulncheck(cfg.Tools["govulncheck"].Version)
	gotool := gotools.Go()
	gofmt := gotools.Gofmt()

	eco := &mend.Ecosystem{
		VersionControl:  shared.NewVersionControl(),
		Build:           golang.NewBuild(gotool),
		Vet:             golang.NewVet(gotool),
		Format:          golang.NewFormat(gofmt),
		Quality:         golang.NewGolangciLint(golangci, gotool),
		Dependencies:    golang.NewGoMod(gotool),
		Runtime:         golang.NewRuntime(gotool, golang.RuntimeWarn),
		Docs:            golang.NewDocs(floatOption(cfg, "docs", "min")),
		Tests:           golang.NewTests(gotool, cfg.Profiles.Tests),
		Benchmark:       golang.NewBenchmark(gotool, cfg.Profiles.Bench),
		Secrets:         shared.NewBetterleaks(betterleaks, boolOption(cfg, "sec.secrets", "history")),
		Vulnerabilities: golang.NewGovulncheck(govulncheck, cfg.CheckOption("sec.vuln", "db")),
		Fixer:           golang.NewFixer(golangci, gotool),
	}

	runner := mend.NewRunner(cfg.AutoInstall, cfg.Tools)
	grading := rating.FromConfig(cfg.Health.Weights, cfg.Health.Caps)
	statusCommand := NewStatusCommand(eco, runner, golang.ModulePath, cfg.CheckDisabled, resolveVC, grading)
	helpCommand := builtinCommands.NewHelpCommand(app.Config, app.Commands, app.OutputFormats)
	app.Help(helpCommand)
	app.Default(statusCommand)
	app.Add("status", statusCommand)

	setupCommand := NewSetupCommand()
	app.Add("setup", setupCommand)
	setupCommand.Add("lint", NewSetupLintCommand(golang.ModulePath))

	if err := app.Run(args); cli.IsRealError(err) {
		return err
	}

	return nil
}

// resolveVC reads the repo's version-control identity for the report header. A
// non-git dir (or a probe failure) yields an empty header rather than an error: the
// report still renders, just without the VC line.
func resolveVC(dir string) *output.VCInfo {
	info, err := git.NewRepository(dir).Info()
	if err != nil || info.Branch == "" {
		return nil
	}
	vc := &output.VCInfo{
		Branch:      info.Branch,
		Commit:      info.Commit,
		Commits:     info.Commits,
		Dirty:       info.Dirty,
		HasUpstream: info.HasUpstream,
		Ahead:       info.Ahead,
		Behind:      info.Behind,
	}
	if !info.LastCommit.IsZero() {
		t := info.LastCommit
		vc.LastCommit = &t
	}
	return vc
}

func boolOption(cfg mend.Config, check, option string) bool {
	v, err := strconv.ParseBool(cfg.CheckOption(check, option))
	return err == nil && v
}

func floatOption(cfg mend.Config, check, option string) float64 {
	v, err := strconv.ParseFloat(cfg.CheckOption(check, option), 64)
	if err != nil {
		return 0
	}
	return v
}
