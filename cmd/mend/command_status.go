package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/toaweme/cli"

	"github.com/toaweme/mend"
	"github.com/toaweme/mend/cmd/mend/output"
	"github.com/toaweme/mend/internal/rating"
)

// errChecksFailed marks a run that completed but had failing checks. The failures
// are already rendered, so main exits non-zero without logging it as an app error.
var errChecksFailed = errors.New("check(s) failed")

// StatusFlags selects which checks a status run includes. There is one flag per
// feature shown in the report, named after that feature, plus the --quality and
// --security convenience umbrellas that flip on a whole family at once. Passing no
// feature flag runs everything.
type StatusFlags struct {
	VersionControl  bool `arg:"version-control" short:"vc" env:"MEND_STATUS_VERSION_CONTROL" default:"false" help:"Report git working-tree and upstream-sync state"`
	Build           bool `arg:"build" short:"b" env:"MEND_STATUS_BUILD" default:"false" help:"Check that the module compiles"`
	Lint            bool `arg:"lint" short:"l" env:"MEND_STATUS_LINT" default:"false" help:"Run the linter (golangci-lint, or go vet + gofmt fallback)"`
	Dependencies    bool `arg:"dependencies" short:"d" env:"MEND_STATUS_DEPENDENCIES" default:"false" help:"Check dependencies are tidy with no replace directives"`
	Runtime         bool `arg:"runtime" short:"r" env:"MEND_STATUS_RUNTIME" default:"false" help:"Check declared vs code vs dependency runtime versions"`
	Docs            bool `arg:"docs" short:"dc" env:"MEND_STATUS_DOCS" default:"false" help:"Check documentation coverage"`
	Tests           bool `arg:"tests" short:"t" env:"MEND_STATUS_TESTS" default:"false" help:"Run tests"`
	Benchmarks      bool `arg:"benchmarks" short:"bn" env:"MEND_STATUS_BENCHMARKS" default:"false" help:"Run benchmarks (go test -bench)"`
	Secrets         bool `arg:"secrets" short:"s" env:"MEND_STATUS_SECRETS" default:"false" help:"Scan for committed secrets"`
	Vulnerabilities bool `arg:"vulnerabilities" short:"vu" env:"MEND_STATUS_VULNERABILITIES" default:"false" help:"Scan dependencies for known vulnerabilities"`

	// Quality and Security are convenience umbrellas over their feature families;
	// each is equivalent to passing every per-feature flag it covers.
	Quality  bool `arg:"quality" short:"q" env:"MEND_STATUS_QUALITY" default:"false" help:"Umbrella for build, lint, dependencies, runtime and docs"`
	Security bool `arg:"security" short:"se" env:"MEND_STATUS_SECURITY" default:"false" help:"Umbrella for secrets and vulnerabilities"`

	Coverage bool `arg:"coverage" short:"c" env:"MEND_STATUS_COVERAGE" default:"false" help:"Collect coverage when running tests (implies --tests)"`
	Race     bool `arg:"race" short:"ra" env:"MEND_STATUS_RACE" default:"false" help:"Run tests with -race"`
	Fix      bool `arg:"fix" env:"MEND_STATUS_FIX" default:"false" help:"Apply auto-fixes before checking"`
	// Amend is a fast, one-shot update for external live-tracking tooling to loop: a
	// single call re-runs only the working-tree (version-control) state and amends it
	// into the --output JSON file, re-grading from the preserved heavy-check results,
	// then exits. With no file yet it falls through to a full run that seeds it. mend
	// never loops itself; the caller (a watcher/cron/dashboard) repeats the call.
	Amend bool `arg:"amend" short:"a" env:"MEND_STATUS_AMEND" default:"false" help:"Amend the json report file with -vc information. Used with --json and --output."`
}

// OutputFlags holds the flags that shape how a run's results are rendered.
type OutputFlags struct {
	JSON          bool   `arg:"json" short:"j" env:"MEND_JSON" default:"false" help:"Output results as JSON"`
	ExpandInstall bool   `arg:"expand-install" short:"ei" env:"MEND_EXPAND_INSTALL" default:"false" help:"Expand the per-tool install phase into its own section instead of folding it into the repo header"`
	Output        string `arg:"output" short:"o" env:"MEND_OUTPUT" help:"Write the JSON report to a file instead of stdout."`
	// Pretty renders the human report to stdout in addition to the --output JSON file,
	// so a CI run shows the report in its log while still producing the machine
	// artifact. Without --output it is a no-op (stdout already renders).
	Pretty bool `arg:"pretty" short:"p" env:"MEND_PRETTY" default:"false" help:"With --output, also render the human report to stdout (e.g. for CI logs)"`
}

// StatusConfig is the full flag set for a status run: which checks to run, how to
// render them, and the verbosity level.
type StatusConfig struct {
	StatusFlags
	OutputFlags
	cli.Verbosity
}

// StatusCommand runs the configured checks against the current repo and renders
// their results.
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

// NewStatusCommand wires the status command to its ecosystem, runner, and the
// resolvers for module identity, disabled features, version control, and grading.
func NewStatusCommand(eco *mend.Ecosystem, runner mend.Runner, module func(dir string) string, disabled func(feature string) bool, vc func(dir string) *output.VCInfo, grading rating.Config) *StatusCommand {
	return &StatusCommand{eco: eco, runner: runner, module: module, disabled: disabled, vc: vc, grading: grading}
}

// Help returns the status command's usage text.
func (c *StatusCommand) Help() string {
	return "Report status for the current repository."
}

// Run executes the selected checks against the cwd and either renders the report or
// writes it to --output. A --amend against an existing --output file takes the fast
// amend path instead. Returns errChecksFailed when any check in a full run fails.
func (c *StatusCommand) Run(options cli.GlobalFlags, _ cli.Unknowns) error {
	in := c.Inputs.StatusFlags
	out := c.Inputs.OutputFlags

	// fast path: amend an existing report in place. a missing file falls through to a
	// full run below, which seeds it.
	if in.Amend && out.Output != "" && fileExists(out.Output) {
		return c.amend(options.Cwd, out.Output)
	}

	config, runOptions := mapEcosystemConfigs(in)
	applyDisabled(&config, c.disabled)
	tasks := c.eco.Tasks(config)
	start := time.Now()
	outputs := c.runner.Run(context.Background(), tasks, options.Cwd, runOptions)
	info := c.runInfo(options.Cwd, start)

	// --output writes the JSON report to a file (and seeds the --amend target);
	// otherwise render to stdout in the selected format.
	if out.Output != "" {
		if err := output.WriteReportFile(out.Output, output.BuildReport(outputs, info, c.grading)); err != nil {
			return fmt.Errorf("failed to write report to %q: %w", out.Output, err)
		}
		// --pretty additionally renders the human report to stdout so a CI run shows
		// what happened while the JSON file feeds downstream tooling.
		if c.Inputs.Pretty {
			renderOptions := output.RenderOptions{Verbosity: c.Inputs.Level(), JSON: false, ExpandInstall: c.Inputs.ExpandInstall, Grading: c.grading}
			if err := output.Render(outputs, info, renderOptions); err != nil {
				return err
			}
		}
	} else {
		renderOptions := output.RenderOptions{Verbosity: c.Inputs.Level(), JSON: c.Inputs.JSON, ExpandInstall: c.Inputs.ExpandInstall, Grading: c.grading}
		if err := output.Render(outputs, info, renderOptions); err != nil {
			return err
		}
	}
	if fails := output.Failures(outputs); fails > 0 {
		return fmt.Errorf("%d %w", fails, errChecksFailed)
	}
	return nil
}

// amend runs only the working-tree (version-control) state and merges it into the
// existing report at path, re-grading from the merged check set, then returns. It is
// the fast one-shot update external tooling loops for live tracking, so it never
// returns errChecksFailed: it updates the file rather than gating on the (preserved)
// heavy-check results.
func (c *StatusCommand) amend(cwd, path string) error {
	config := mend.EcosystemConfig{VersionControl: true}
	applyDisabled(&config, c.disabled)
	start := time.Now()
	outputs := c.runner.Run(context.Background(), c.eco.Tasks(config), cwd, mend.RunOptions{})
	info := c.runInfo(cwd, start)

	existing, err := output.ReadReport(path)
	if err != nil {
		return fmt.Errorf("failed to read report %q: %w", path, err)
	}
	if err := output.WriteReportFile(path, output.AmendReport(existing, outputs, info, c.grading)); err != nil {
		return fmt.Errorf("failed to write report to %q: %w", path, err)
	}
	return nil
}

// runInfo resolves the caller-side report header (created stamp, repo dir, module
// identity, version-control state) for a run that started at start.
func (c *StatusCommand) runInfo(cwd string, start time.Time) output.RunInfo {
	info := output.RunInfo{Created: start.UTC(), Repo: cwd, DurationMs: time.Since(start).Milliseconds()}
	if c.module != nil {
		info.Module = c.module(cwd)
	}
	if c.vc != nil {
		info.VC = c.vc(cwd)
	}
	return info
}

// fileExists reports whether path exists and is statable.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// mapEcosystemConfigs maps the status flags onto the EcosystemConfig and the per-run
// RunOptions, defaulting to everything (with coverage) when no feature flag is set.
// Each feature has its own flag; the --quality umbrella additionally turns on build,
// lint, dependencies, runtime and docs, and --security turns on secrets and
// vulnerabilities.
func mapEcosystemConfigs(in StatusFlags) (mend.EcosystemConfig, mend.RunOptions) {
	cfg := mend.EcosystemConfig{
		VersionControl:  in.VersionControl,
		Build:           in.Build || in.Quality,
		Quality:         in.Lint || in.Quality,
		Dependencies:    in.Dependencies || in.Quality,
		Runtime:         in.Runtime || in.Quality,
		Docs:            in.Docs || in.Quality,
		Tests:           in.Tests || in.Coverage,
		Benchmark:       in.Benchmarks,
		Secrets:         in.Secrets || in.Security,
		Vulnerabilities: in.Vulnerabilities || in.Security,
	}
	coverage := in.Coverage
	allOff := cfg == (mend.EcosystemConfig{})
	if allOff {
		cfg = mend.EcosystemConfig{
			VersionControl:  true,
			Build:           true,
			Quality:         true,
			Dependencies:    true,
			Runtime:         true,
			Docs:            true,
			Tests:           true,
			Benchmark:       true,
			Secrets:         true,
			Vulnerabilities: true,
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
