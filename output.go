package care

import (
	"context"
	"time"
)

// Output is a feature's typed result: the feature fills Status and Data (its typed
// payload); the runner stamps the metadata. T is the feature's payload type, so
// authoring a feature and its tests stays statically typed. Output satisfies
// Rendered, so a heterogeneous run collects into one []Rendered.
type Output[T Report] struct {
	status  Status
	data    T
	hasData bool
	note    string // short text for a skip / tool-failure with no rich payload
	err     error

	phase      Phase
	feature    string
	check      string
	tool       string
	profile    string
	dir        string
	durationMs int64

	// weight and cap are the grading policy stamped from the check's WithRating
	// registration: weight is the feature's importance in the score, capScore the
	// ceiling it imposes on a failure (active only when hasCap). They ride on the
	// result so the rating engine grades from per-check policy.
	weight   int
	capScore int
	hasCap   bool
}

// Pass is an OK outcome carrying its typed payload.
func Pass[T Report](data T) Output[T] { return Output[T]{status: StatusOK, data: data, hasData: true} }

// Warn is a non-failing outcome that still has something to report.
func Warn[T Report](data T) Output[T] {
	return Output[T]{status: StatusWarn, data: data, hasData: true}
}

// Fail is a failing outcome carrying its typed payload (the findings that failed it).
func Fail[T Report](data T) Output[T] {
	return Output[T]{status: StatusFail, data: data, hasData: true}
}

// Skip is a not-applicable / nothing-to-do outcome; note is the short reason.
func Skip[T Report](note string) Output[T] { return Output[T]{status: StatusSkip, note: note} }

// Errored is a failing outcome with no usable payload (the tool itself failed);
// note is the short label, err the detail surfaced at higher verbosity.
func Errored[T Report](note string, err error) Output[T] {
	return Output[T]{status: StatusFail, note: note, err: err}
}

// Phase returns the pipeline stage this result belongs to.
func (o Output[T]) Phase() Phase { return o.phase }

// Feature returns the language-agnostic feature this result was produced for.
func (o Output[T]) Feature() string { return o.feature }

// Check returns the name of the check that produced this result.
func (o Output[T]) Check() string { return o.check }

// Tool returns the name of the tool this result ran through.
func (o Output[T]) Tool() string { return o.tool }

// Profile returns the run-profile name this result ran under.
func (o Output[T]) Profile() string { return o.profile }

// Dir returns the repo directory this result was produced in.
func (o Output[T]) Dir() string { return o.dir }

// Status returns the outcome status of this result.
func (o Output[T]) Status() Status { return o.status }

// Err returns the underlying error for a tool-failure outcome, or nil.
func (o Output[T]) Err() error { return o.err }

// DurationMs returns how long the check took, in milliseconds.
func (o Output[T]) DurationMs() int64 { return o.durationMs }

// Weight returns the feature's grading weight, stamped from its WithRating
// registration (0 for an unweighted, informational feature).
func (o Output[T]) Weight() int { return o.weight }

// Cap returns the score ceiling this check imposes on a failure; ok is false when the
// check imposes no cap.
func (o Output[T]) Cap() (int, bool) { return o.capScore, o.hasCap }

// WithWeight returns a copy carrying the grading weight, so a caller (or a test)
// synthesizing a result stream outside the runner can stamp the policy WithRating
// would have attached.
func (o Output[T]) WithWeight(weight int) Output[T] { o.weight = weight; return o }

// WithCap returns a copy carrying the failure score cap, the companion to WithWeight
// for the critical features (secrets, vulnerabilities) that impose a ceiling.
func (o Output[T]) WithCap(score int) Output[T] { o.capScore, o.hasCap = score, true; return o }

// Version returns the tool version for this result (empty for typed outputs).
func (o Output[T]) Version() string { return "" }

// Source returns the result source (empty for typed outputs).
func (o Output[T]) Source() string { return "" }

// Summary renders the result's payload as a one-line terminal summary, or the note
// when there is no rich payload.
func (o Output[T]) Summary(verbosity int) string {
	if !o.hasData {
		return o.note
	}
	return o.data.Summary(verbosity)
}

// Rows renders the result's payload as detail rows, or nil when there is no payload.
func (o Output[T]) Rows(verbosity int) [][]string {
	if !o.hasData {
		return nil
	}
	return o.data.Rows(verbosity)
}

// Data returns the typed payload for the JSON wire, or nil when the outcome has no
// rich payload (a skip or a tool failure).
func (o Output[T]) Data() any {
	if !o.hasData {
		return nil
	}
	return o.data
}

// Result builds a stamped run-phase Output from explicit parts, so a caller (or a
// test) outside this package can synthesize an output stream without the runner.
func Result[T Report](feature, check, dir string, status Status, data T) Output[T] {
	return Output[T]{
		phase: PhaseRun, feature: feature, check: check, dir: dir,
		status: status, data: data, hasData: true,
	}
}

// ErroredResult builds a stamped run-phase tool-failure Output: note is the short
// label ("tool failed"), err the underlying detail. It mirrors Result so a caller
// (or a test) outside this package can synthesize an errored outcome.
func ErroredResult[T Report](feature, check, dir, note string, err error) Output[T] {
	return Output[T]{
		phase: PhaseRun, feature: feature, check: check, dir: dir,
		status: StatusFail, note: note, err: err,
	}
}

// InstallResult builds an install-phase outcome for one tool; note is its short
// state text ("present", "installed", ...).
func InstallResult(tool string, status Status, note string) Rendered {
	return simpleOutput{phase: PhaseInstall, tool: tool, status: status, note: note}
}

// Rendered is the non-generic view the runner collects and the renderer + JSON
// consume. Every concrete result (a typed Output[T], a skip, an install outcome)
// satisfies it, so the unified output stream is one []Rendered.
type Rendered interface {
	Phase() Phase
	Feature() string
	Check() string
	Tool() string
	// Profile is the run-profile this result ran under (e.g. "race"); "" for the
	// implicit default profile and for non-profiled checks.
	Profile() string
	Dir() string
	Status() Status
	Summary(verbosity int) string
	Rows(verbosity int) [][]string
	Data() any
	Err() error
	DurationMs() int64
	// Weight is the feature's grading weight (0 for an unweighted feature); Cap is the
	// score ceiling it imposes on a failure, ok false when it imposes none. Both are
	// stamped from the check's WithRating registration so the grade reads per-check.
	Weight() int
	Cap() (score int, ok bool)
	// Version is the resolved tool version captured at install (install outputs
	// only); "" for run-phase checks, which reference their tool by bare name.
	Version() string
	// Source is the installer that provisions the tool (its Installer const:
	// "brew", "go-install", "builtin"); install outputs only, "" otherwise.
	Source() string
}

// simpleOutput is a runner-emitted Rendered with no typed payload: a skipped task
// or an install-phase tool outcome. note carries its one-line text.
type simpleOutput struct {
	phase      Phase
	feature    string
	check      string
	tool       string
	profile    string
	version    string
	source     string
	dir        string
	status     Status
	note       string
	err        error
	durationMs int64
}

var _ Rendered = simpleOutput{}

func (s simpleOutput) Phase() Phase        { return s.phase }
func (s simpleOutput) Feature() string     { return s.feature }
func (s simpleOutput) Check() string       { return s.check }
func (s simpleOutput) Tool() string        { return s.tool }
func (s simpleOutput) Profile() string     { return s.profile }
func (s simpleOutput) Dir() string         { return s.dir }
func (s simpleOutput) Status() Status      { return s.status }
func (s simpleOutput) Version() string     { return s.version }
func (s simpleOutput) Source() string      { return s.source }
func (s simpleOutput) Summary(int) string  { return s.note }
func (s simpleOutput) Rows(int) [][]string { return nil }
func (s simpleOutput) Data() any           { return nil }
func (s simpleOutput) Err() error          { return s.err }
func (s simpleOutput) DurationMs() int64   { return s.durationMs }
func (s simpleOutput) Weight() int         { return 0 }
func (s simpleOutput) Cap() (int, bool)    { return 0, false }

// Task is a selected feature erased for the runner: enough to install its tools and
// run it, with its feature identity attached. It can only be built inside this
// package (via Ecosystem.Tasks), so the feature label always comes from a registry
// slot, never from arbitrary caller input.
type Task interface {
	Name() string
	Feature() string
	Profile() string
	Tools() []Tool
	Applies(dir string) bool
	run(ctx context.Context, dir string, opts RunOptions) Rendered
}

// task binds a typed Check[T] to its feature and erases it to Task: run() calls
// the typed Run and stamps the metadata onto the returned Output[T]. profile is the
// run-profile this task runs under (the zero RunProfile for a non-profiled feature);
// run() injects it into RunOptions so the check builds the right tool invocation.
type task[T Report] struct {
	feature string
	check   Check[T]
	profile RunProfile
}

func newTask[T Report](feature string, c Check[T]) Task {
	return task[T]{feature: feature, check: c}
}

// newProfileTask binds a check to one of its run-profiles, so a profiled feature
// expands into one task per profile.
func newProfileTask[T Report](feature string, c Check[T], p RunProfile) Task {
	return task[T]{feature: feature, check: c, profile: p}
}

func (t task[T]) Name() string            { return t.check.Name() }
func (t task[T]) Feature() string         { return t.feature }
func (t task[T]) Profile() string         { return t.profile.Name }
func (t task[T]) Tools() []Tool           { return t.check.Tools() }
func (t task[T]) Applies(dir string) bool { return t.check.Applies(dir) }

func (t task[T]) run(ctx context.Context, dir string, opts RunOptions) Rendered {
	opts.Profile = t.profile
	start := time.Now()
	o := t.check.Run(ctx, dir, opts)
	o.durationMs = time.Since(start).Milliseconds()
	o.phase = PhaseRun
	o.feature = t.feature
	o.check = t.check.Name()
	o.profile = t.profile.Name
	o.dir = dir
	// stamp the grading policy registered with WithRating, so the result carries its
	// own weight and cap into the rating engine. An unwrapped check grades at weight 0.
	if r, ok := t.check.(Rated); ok {
		o.weight = r.Weight()
		o.capScore, o.hasCap = r.Cap()
	}
	if o.tool == "" {
		if tools := t.check.Tools(); len(tools) > 0 && tools[0].Name() != "" {
			o.tool = tools[0].Name()
		}
	}
	return o
}
