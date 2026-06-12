package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/toaweme/cli"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/cmd/mend/output"
	"github.com/toaweme/mend/internal/rating"
)

// errChecksFailed marks a run that completed but had failing checks. The failures
// are already rendered, so main exits non-zero without logging it as an app error.
var errChecksFailed = errors.New("check(s) failed")

// StatusFlags selects which checks a status run includes.
type StatusFlags struct {
	Git      bool `arg:"git" short:"gi" env:"MEND_STATUS_GIT" default:"false" help:"Report git working-tree and upstream-sync state"`
	Quality  bool `arg:"quality" short:"q" env:"MEND_STATUS_QUALITY" default:"false" help:"Run quality checks (lint, config sync, go.mod)"`
	Tests    bool `arg:"tests" short:"t" env:"MEND_STATUS_TESTS" default:"false" help:"Run tests"`
	Coverage bool `arg:"coverage" short:"c" env:"MEND_STATUS_COVERAGE" default:"false" help:"Collect coverage when running tests (implies --tests)"`
	Race     bool `arg:"race" short:"r" env:"MEND_STATUS_RACE" default:"false" help:"Run tests with -race"`
	Bench    bool `arg:"bench" short:"b" env:"MEND_STATUS_BENCH" default:"false" help:"Run benchmarks (go test -bench)"`
	Security bool `arg:"security" short:"s" env:"MEND_STATUS_SECURITY" default:"false" help:"Run security checks (secrets, vulnerabilities)"`
	Fix      bool `arg:"fix" env:"MEND_STATUS_FIX" default:"false" help:"Apply auto-fixes before checking"`
}

// OutputFlags holds the flags that shape how a run's results are rendered.
type OutputFlags struct {
	JSON          bool `arg:"json" short:"j" env:"MEND_JSON" default:"false" help:"Output results as JSON"`
	ExpandInstall bool `arg:"expand-install" short:"ei" env:"MEND_EXPAND_INSTALL" default:"false" help:"Expand the per-tool install phase into its own section instead of folding it into the repo header"`
}

type StatusConfig struct {
	StatusFlags
	OutputFlags
	cli.Verbosity
}

type StatusCommand struct {
	cli.BaseCommand[StatusConfig]
	eco    *mend.Ecosystem
	runner mend.Runner
	// module resolves the repo's module / project identity for the report header
	// (golang: go.mod path, node: package.json name). nil when unknown; returns ""
	// when it cannot be determined for a given dir.
	module func(dir string) string
	// disabled reports whether a feature is turned off in config, so an operator can
	// drop a check from the everything-on default without disabling its tool.
	disabled func(feature string) bool
	// vc resolves the repo's version-control identity for the report header (branch,
	// commit, commit count, sync state). nil when unavailable.
	vc func(dir string) *output.VCInfo
	// grading is the health policy (weights + caps) the score is computed against.
	grading rating.Config
}

var _ cli.Command[StatusConfig] = (*StatusCommand)(nil)

func NewStatusCommand(eco *mend.Ecosystem, runner mend.Runner, module func(dir string) string, disabled func(feature string) bool, vc func(dir string) *output.VCInfo, grading rating.Config) *StatusCommand {
	return &StatusCommand{eco: eco, runner: runner, module: module, disabled: disabled, vc: vc, grading: grading}
}

func (c *StatusCommand) Help() string {
	return "Report status for the current repository: --git state, -q quality, -t tests, -b benchmarks, -s security (default: all, with coverage). --coverage forces coverage on a -t run; --fix applies fixes first."
}

func (c *StatusCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	in := c.Inputs.StatusFlags
	config, runOptions := mapEcosystemConfigs(in)
	applyDisabled(&config, c.disabled)
	tasks := c.eco.Tasks(config)
	start := time.Now()
	outputs := c.runner.Run(context.Background(), tasks, options.Cwd, runOptions)
	info := output.RunInfo{Created: start.UTC(), Repo: options.Cwd, DurationMs: time.Since(start).Milliseconds()}
	if c.module != nil {
		info.Module = c.module(options.Cwd)
	}
	if c.vc != nil {
		info.VC = c.vc(options.Cwd)
	}
	renderOptions := output.RenderOptions{Verbosity: c.Inputs.Level(), JSON: c.Inputs.JSON, ExpandInstall: c.Inputs.ExpandInstall, Grading: c.grading}
	if err := output.Render(outputs, info, renderOptions); err != nil {
		return err
	}
	if fails := output.Failures(outputs); fails > 0 {
		return fmt.Errorf("%d %w", fails, errChecksFailed)
	}
	return nil
}

// mapEcosystemConfigs maps the status flags onto the EcosystemConfig and the per-run
// RunOptions, defaulting to everything (with coverage) when no feature flag is set.
// The quality type covers lint + dependencies; the security type covers secrets +
// vulnerabilities.
func mapEcosystemConfigs(in StatusFlags) (mend.EcosystemConfig, mend.RunOptions) {
	// the quality group covers the compile/style/dependency family: build, vet,
	// format, lint, dependencies, docs.
	cfg := mend.EcosystemConfig{
		VersionControl:  in.Git,
		Build:           in.Quality,
		Quality:         in.Quality,
		Dependencies:    in.Quality,
		Runtime:         in.Quality,
		Docs:            in.Quality,
		Tests:           in.Tests || in.Coverage,
		Benchmark:       in.Bench,
		Secrets:         in.Security,
		Vulnerabilities: in.Security,
	}
	coverage := in.Coverage
	if !in.Git && !in.Quality && !in.Tests && !in.Coverage && !in.Security && !in.Bench {
		cfg = mend.EcosystemConfig{
			VersionControl: true,
			Build:          true, Quality: true, Dependencies: true, Runtime: true, Docs: true,
			Tests: true, Benchmark: true,
			Secrets: true, Vulnerabilities: true,
		}
		coverage = true // default status run reports coverage
	}
	return cfg, mend.RunOptions{
		Tests:        mend.TestRunOptions{Race: in.Race, Coverage: coverage},
		Quality:      mend.QualityRunOptions{Fix: in.Fix},
		Dependencies: mend.DepsRunOptions{Fix: in.Fix},
	}
}

// applyDisabled zeroes the EcosystemConfig slots an operator turned off in config.
// Everything runs by default, so this is the off switch per check.
func applyDisabled(cfg *mend.EcosystemConfig, disabled func(feature string) bool) {
	if disabled == nil {
		return
	}
	for feature, slot := range map[string]*bool{
		mend.FeatureVersionControl:  &cfg.VersionControl,
		mend.FeatureBuild:           &cfg.Build,
		mend.FeatureLint:            &cfg.Quality,
		mend.FeatureDependencies:    &cfg.Dependencies,
		mend.FeatureRuntime:         &cfg.Runtime,
		mend.FeatureDocs:            &cfg.Docs,
		mend.FeatureTests:           &cfg.Tests,
		mend.FeatureBenchmark:       &cfg.Benchmark,
		mend.FeatureSecrets:         &cfg.Secrets,
		mend.FeatureVulnerabilities: &cfg.Vulnerabilities,
	} {
		if disabled(feature) {
			*slot = false
		}
	}
}
