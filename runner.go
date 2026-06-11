package mend

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/toaweme/mend/internal/devops/install"
)

// runner installs the tools the tasks need, then runs the tasks against a repo
// through a bounded worker pool. The output is sorted after collection, so running
// in parallel never changes what gets rendered.
type runner struct {
	autoInstall bool
	tools       map[string]ToolConfig
	concurrency int
	installers  map[Installer]install.Installer
}

// Runner runs selected feature tasks against a repository.
type Runner interface {
	Run(ctx context.Context, tasks []Task, dir string, opts RunOptions) []Rendered
}

var _ Runner = (*runner)(nil)

// NewRunner builds a runner. autoInstall controls whether missing tools are
// installed (vs the feature being skipped); tools carries operator overrides
// (version pin / disable) keyed by tool name. Concurrency defaults to NumCPU.
func NewRunner(autoInstall bool, tools map[string]ToolConfig) Runner {
	return &runner{
		autoInstall: autoInstall,
		tools:       tools,
		concurrency: runtime.NumCPU(),
		installers: map[Installer]install.Installer{
			InstallerBrew: install.Brew(),
			InstallerGo:   install.Go(),
		},
	}
}

// toolState is the install-phase outcome for one tool: whether the run stage may
// use it, and a note when it may not.
type toolState struct {
	available bool
	note      string
}

// Run skips tasks that do not apply to the repo, installs the tools the rest need,
// runs them (the Fixer first and serially, then the read-only features
// concurrently), then collects and sorts the output.
func (r *runner) Run(ctx context.Context, tasks []Task, dir string, opts RunOptions) []Rendered {
	var runnable, fixers []Task
	var skips []Rendered
	for _, t := range tasks {
		if !t.Applies(dir) {
			skips = append(skips, simpleOutput{
				phase:   PhaseRun,
				feature: t.Feature(),
				check:   t.Name(),
				profile: t.Profile(),
				dir:     dir,
				status:  StatusSkip,
				note:    "not applicable",
			})
			continue
		}
		if t.Feature() == FeatureFixer {
			fixers = append(fixers, t)
			continue
		}
		runnable = append(runnable, t)
	}

	installOuts, state := r.install(ctx, append(append([]Task{}, fixers...), runnable...))

	var results []Rendered
	// the Fixer mutates the repo, so it runs before the read-only features and never
	// concurrently with them.
	for _, f := range fixers {
		results = append(results, r.runOne(ctx, f, dir, state, opts))
	}
	results = append(results, r.dispatch(ctx, runnable, dir, state, opts)...)

	out := make([]Rendered, 0, len(installOuts)+len(skips)+len(results))
	out = append(out, installOuts...)
	out = append(out, skips...)
	out = append(out, results...)
	sortOutputs(out)
	return out
}

// install provisions each unique tool the tasks reference exactly once, returning
// one install-phase Rendered per tool and a map of tool name to its availability.
func (r *runner) install(ctx context.Context, tasks []Task) ([]Rendered, map[string]toolState) {
	state := make(map[string]toolState)
	var outs []Rendered
	for _, t := range tasks {
		for _, tool := range t.Tools() {
			name := tool.Name()
			if name == "" {
				continue
			}
			if _, done := state[name]; done {
				continue
			}
			start := time.Now()
			o, st := r.ensureTool(ctx, tool)
			o.durationMs = time.Since(start).Milliseconds()
			state[name] = st
			outs = append(outs, o)
		}
	}
	sort.Slice(outs, func(i, j int) bool { return outs[i].Tool() < outs[j].Tool() })
	return outs, state
}

// ensureTool provisions a single tool, returning its install-phase Rendered and
// availability. It honors operator config: a disabled tool is unavailable, a
// version pin is applied before checking/installing.
func (r *runner) ensureTool(ctx context.Context, t Tool) (simpleOutput, toolState) {
	spec := t.Spec()
	out := func(status Status, note string) simpleOutput {
		return simpleOutput{phase: PhaseInstall, tool: spec.Name, source: string(spec.Installer), status: status, note: note}
	}
	// ok stamps the resolved tool version onto an available outcome, so the tools
	// array can report it (falling back to the configured pin when the probe finds
	// nothing).
	ok := func(note string) (simpleOutput, toolState) {
		o := out(StatusOK, note)
		if o.version = t.Version(ctx); o.version == "" {
			o.version = spec.Version
		}
		return o, toolState{available: true}
	}
	cfg := r.tools[spec.Name]
	if cfg.Disabled {
		return out(StatusSkip, "disabled"), toolState{note: "disabled"}
	}
	if cfg.Version != "" {
		spec.Version = cfg.Version
	}
	inst := r.installers[spec.Installer]
	if inst == nil {
		note := "present"
		if spec.Installer == InstallerBuiltin {
			note = "builtin"
		}
		return ok(note)
	}
	it := install.Tool{Bin: spec.Name, Brew: spec.Brew, GoPath: spec.GoPath, Version: spec.Version}
	if inst.IsInstalled(it) {
		return ok("present")
	}
	if !r.autoInstall {
		return out(StatusSkip, "not installed (auto-install off)"), toolState{note: "auto-install off"}
	}
	if !inst.Available() {
		return out(StatusSkip, "not installed (installer unavailable)"), toolState{note: "installer unavailable"}
	}
	if err := inst.Install(ctx, it); err != nil {
		o := out(StatusFail, "install failed")
		o.err = fmt.Errorf("failed to install %q: %w", spec.Name, err)
		return o, toolState{note: "install failed"}
	}
	return ok("installed")
}

// dispatch runs the tasks through a bounded worker pool.
func (r *runner) dispatch(ctx context.Context, tasks []Task, dir string, state map[string]toolState, opts RunOptions) []Rendered {
	if len(tasks) == 0 {
		return nil
	}

	limit := r.concurrency
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	results := make([]Rendered, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t Task) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = r.runOne(ctx, t, dir, state, opts)
		}(i, t)
	}
	wg.Wait()

	return results
}

// runOne runs a single task: it skips when a tool the task needs is unavailable,
// otherwise runs the task and returns its stamped result.
func (r *runner) runOne(ctx context.Context, t Task, dir string, state map[string]toolState, opts RunOptions) Rendered {
	if name, note, ok := unavailableTool(t, state); ok {
		return simpleOutput{
			phase: PhaseRun, feature: t.Feature(), check: t.Name(), profile: t.Profile(),
			tool: name, dir: dir, status: StatusSkip, note: note,
		}
	}
	return t.run(ctx, dir, opts)
}

// sortOutputs orders the stream deterministically so parallel completion never
// changes what is rendered: install outputs first (by tool), then run outputs by
// feature then check name.
func sortOutputs(out []Rendered) {
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Phase() == PhaseInstall) != (b.Phase() == PhaseInstall) {
			return a.Phase() == PhaseInstall
		}
		if a.Phase() == PhaseInstall {
			return a.Tool() < b.Tool()
		}
		if a.Feature() != b.Feature() {
			return a.Feature() < b.Feature()
		}
		if a.Check() != b.Check() {
			return a.Check() < b.Check()
		}
		return a.Profile() < b.Profile()
	})
}

// unavailableTool returns the first of a task's tools the install stage could not
// make available, with the note explaining why.
func unavailableTool(t Task, state map[string]toolState) (name, note string, ok bool) {
	for _, tool := range t.Tools() {
		if tool.Name() == "" {
			continue
		}
		if st := state[tool.Name()]; !st.available {
			return tool.Name(), st.note, true
		}
	}
	return "", "", false
}
