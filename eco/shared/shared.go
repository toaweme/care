// Package shared holds the language-agnostic feature implementations: features
// that apply to any repository regardless of its toolchain (version control,
// secret scanning). Each constructor takes its injected tool (if any) and returns
// the feature's mend interface, so main fills the Ecosystem slots next to the
// language-specific features.
package shared
