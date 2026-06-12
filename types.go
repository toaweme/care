package mend

import (
	"context"
	"strconv"
)

// Phase is the pipeline stage a result belongs to. The runner installs every
// task's tools (install phase) before running the tasks (run phase), and tags
// each result so a single renderer can section the unified stream.
type Phase string

const (
	PhaseInstall Phase = "install" // ensuring a tool binary is present
	PhaseRun     Phase = "run"     // running a check against a repo
)

// Installer selects how the runner provisions a Tool's binary. The zero value
// (empty) means nothing to provision.
type Installer string

const (
	InstallerBrew    Installer = "brew"       // homebrew formula
	InstallerGo      Installer = "go-install" // go install <path>@<version>
	InstallerBuiltin Installer = "builtin"    // ships with the toolchain (e.g. go)
)

// Feature names a language-agnostic capability mend checks: the wire `feature` and
// the terminal header. A result's feature comes from the Ecosystem slot it was
// selected from, never from the check itself.
const (
	FeatureVersionControl  = "version_control"
	FeatureBuild           = "build"
	FeatureLint            = "lint"
	FeatureDependencies    = "dependencies"
	FeatureRuntime         = "runtime"
	FeatureDocs            = "docs"
	FeatureTests           = "tests"
	FeatureBenchmark       = "benchmarks"
	FeatureSecrets         = "secrets"
	FeatureVulnerabilities = "vulnerabilities"
	FeatureFixer           = "fixer"
)

// RunOptions carries the per-invocation modes a check reads at run time, so a feature
// slot is constructed once at registration and parameterized per run. Each check
// reads only its own namespace, so a mode meant for one feature can never leak into
// another and the struct stays coherent as features gain options.
type RunOptions struct {
	Tests        TestRunOptions
	Quality      QualityRunOptions
	Dependencies DepsRunOptions
	// Profile is the active run-profile for this task: a profiled feature (tests,
	// benchmarks) expands into one task per profile, and the runner sets this to the
	// profile that task runs under so the check builds the right tool invocation.
	Profile RunProfile
}

// RunProfile is one named set of `go test`-based flags a profiled feature runs
// under. Both profiled features (tests, benchmarks) shell out to `go test`, so a
// single shape covers both: a feature with N configured profiles runs N times, once
// per profile, each producing its own result row. The zero value is the implicit
// "default" profile.
type RunProfile struct {
	Name      string   `yaml:"name"`
	Race      bool     `yaml:"race,omitempty"`
	Coverage  bool     `yaml:"coverage,omitempty"`
	Tags      string   `yaml:"tags,omitempty"`      // -tags
	Benchtime string   `yaml:"benchtime,omitempty"` // benchmarks: -benchtime
	Count     int      `yaml:"count,omitempty"`     // -count
	CPU       string   `yaml:"cpu,omitempty"`       // -cpu
	Args      []string `yaml:"args,omitempty"`      // raw escape-hatch flags appended to the invocation
}

// Profiled is implemented by a check that runs under multiple named profiles. The
// Ecosystem expands such a slot into one Task per profile; a check that does not
// implement it (or returns a single profile) runs once. The returned slice is never
// empty: a check with no configured profiles returns its synthesized default.
type Profiled interface {
	Profiles() []RunProfile
}

// TestRunOptions are the run modes the Tests check reads.
type TestRunOptions struct {
	Race     bool
	Coverage bool
}

// QualityRunOptions are the run modes the Quality check reads. Fix runs the
// linter in apply mode (golangci-lint run --fix) instead of read-only reporting.
type QualityRunOptions struct {
	Fix bool
}

// DepsRunOptions are the run modes the Dependencies check reads. Fix applies
// `go mod tidy` for real instead of the non-mutating tidy check.
type DepsRunOptions struct {
	Fix bool
}

// Report is a check's structured payload rendered to human text. Every outcome
// carries one: the concrete type is the JSON wire data (via its struct tags) and
// the source of the terminal presentation. English lives here, never on the
// metadata fields that reach the wire.
type Report interface {
	Summary(verbosity int) string
	Rows(verbosity int) [][]string
}

// Check is one feature's implementation for an ecosystem: it declares the tools it
// needs, whether it applies to a repo, and runs to produce a typed Output. T is the
// feature's payload, so a check's Run is statically typed end to end.
type Check[T Report] interface {
	Name() string
	Tools() []Tool
	Applies(dir string) bool
	Run(ctx context.Context, dir string, opts RunOptions) Output[T]
}

// The per-feature interfaces the Ecosystem slots are typed with. Each is a Check
// bound to that feature's payload, so a slot can only hold the right kind of check
// and a constructor returns a precisely typed value.
type (
	VersionControl  = Check[VCReport]
	Build           = Check[BuildReport]
	Quality         = Check[QualityReport]
	Dependencies    = Check[DepsReport]
	Runtime         = Check[RuntimeReport]
	Docs            = Check[DocsReport]
	Tests           = Check[TestReport]
	Benchmark       = Check[BenchReport]
	Secrets         = Check[SecretReport]
	Vulnerabilities = Check[VulnReport]
	Fixer           = Check[FixReport]
)

// BaseCheck supplies the Name/Tools accessors and the default Applies (false:
// opt-in), so an implementation only adds Run and overrides Applies for the repos
// it handles.
type BaseCheck struct {
	name  string
	tools []Tool
}

// NewBaseCheck builds the identity a check embeds from its name and injected tools.
func NewBaseCheck(name string, tools ...Tool) BaseCheck {
	return BaseCheck{name: name, tools: tools}
}

func (b BaseCheck) Name() string  { return b.name }
func (b BaseCheck) Tools() []Tool { return b.tools }

// Applies defaults to false: a check must opt in to the repos it handles by
// overriding this, so nothing runs (or installs tools) where it does not belong.
func (b BaseCheck) Applies(string) bool { return false }

// Status is the outcome of running a check against a repo.
type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusFail
	StatusSkip
)

// String returns the short label for a status.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusWarn:
		return "WARN"
	case StatusFail:
		return "FAIL"
	case StatusSkip:
		return "SKIP"
	default:
		return "?"
	}
}

// MarshalJSON encodes a status as its short label.
func (s Status) MarshalJSON() ([]byte, error) { return []byte(strconv.Quote(s.String())), nil }
