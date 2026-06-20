// Package mend is the language-agnostic core: it models ecosystems, features, and
// checks, runs them against a repo, and renders the unified typed results.
package mend

// Defaults returns a Config with sensible defaults applied.
func Defaults() Config {
	return Config{AutoInstall: true}
}

// Config is the global mend configuration loaded from mend.yml. The yaml tags
// carry omitempty so `mend init` can marshal a Config back into a clean,
// minimal mend.yml without emitting every zero-valued field.
type Config struct {
	// AutoInstall provisions missing tool binaries when running commands. Defaults to true.
	AutoInstall bool `yaml:"auto_install" default:"true"`
	// Checks configures individual checks by a config key (e.g. "sec.secrets",
	// "sec.vuln"): a free-form options bag the composition root passes to the
	// feature that owns those keys, so adding a check option never changes this
	// struct.
	Checks map[string]CheckConfig `yaml:"checks,omitempty"`
	// Tools overrides tool binaries by name: pin a version or disable a tool.
	// Install coordinates (brew formula, go import path) live in code, not config.
	Tools map[string]ToolConfig `yaml:"tools,omitempty"`
	// Profiles configures the run-profiles a profiled feature runs under: a feature
	// with N profiles runs N times (e.g. tests plain + -race + a build-tag variant),
	// each producing its own result row. An empty list keeps the synthesized default.
	Profiles ProfilesConfig `yaml:"profiles,omitempty"`
	// Health tunes the repo health grade (score + rating). Empty maps keep the
	// built-in weights and caps.
	Health HealthConfig `yaml:"health,omitempty"`
}

// HealthConfig tunes how the run's check outcomes roll up into a health grade. Both
// maps overlay the built-in defaults key-by-key, so setting one weight keeps the
// rest. The keys are feature names (e.g. "build", "secrets").
type HealthConfig struct {
	// Weights is the relative importance of each feature; 0 makes a feature
	// informational (excluded from the score).
	Weights map[string]int `yaml:"weights,omitempty"`
	// Caps is the worst-case score a failing feature is allowed to leave standing
	// (e.g. a committed secret caps the whole grade).
	Caps map[string]int `yaml:"caps,omitempty"`
}

// ProfilesConfig holds the per-feature run-profiles. Only the profiled features
// (tests, benchmarks) take profiles; both shell out to `go test`.
type ProfilesConfig struct {
	Tests []RunProfile `yaml:"tests,omitempty"`
	Bench []RunProfile `yaml:"bench,omitempty"`
}

// CheckConfig is the operator-facing configuration for a single check, keyed by a
// config key in Config.Checks. Options is a free-form bag the composition root
// passes to the feature that owns those keys.
type CheckConfig struct {
	Options map[string]string `yaml:"options,omitempty"`
	// Disabled removes this check from a run even when its feature would otherwise be
	// selected. Everything runs by default, so this is how an operator turns a check
	// off without disabling the underlying tool (which other checks may share).
	Disabled bool `yaml:"disabled,omitempty"`
}

// CheckOption returns the value of a single option for a named check, or "" when
// the check or option is not configured.
func (c Config) CheckOption(check, option string) string {
	return c.Checks[check].Options[option]
}

// CheckDisabled reports whether a check is disabled in config, keyed by its feature
// name (e.g. "build", "vet").
func (c Config) CheckDisabled(feature string) bool {
	return c.Checks[feature].Disabled
}

// ToolConfig is the operator-facing override for a tool binary: a version pin
// and an enable/disable switch. It deliberately carries no install coordinates.
type ToolConfig struct {
	Version  string `yaml:"version"`
	Disabled bool   `yaml:"disabled"`
}
