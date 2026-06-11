package mend

// Ecosystem is the registry of every feature mend checks, one typed slot per
// feature. The struct is the living documentation of mend's capabilities: each
// field names a language-agnostic feature and holds the active ecosystem's check
// implementation (e.g. golang.NewGoMod() for Dependencies). A second ecosystem
// (Node) fills the same slots with its own checks. An empty slot is a feature
// this ecosystem does not implement; it is skipped.
type Ecosystem struct {
	VersionControl  VersionControl
	Build           Build
	Vet             Vet
	Format          Format
	Quality         Quality
	Dependencies    Dependencies
	Runtime         Runtime
	Docs            Docs
	Tests           Tests
	Benchmark       Benchmark
	Secrets         Secrets
	Vulnerabilities Vulnerabilities
	// Fixer applies the fixable features' fixes (lint autofix, gofmt, go.mod tidy)
	// when --fix is set; it runs before the read-only features report what is left.
	Fixer Fixer
}

// EcosystemConfig picks which feature slots a run includes. The status command maps its
// flags onto these; the Ecosystem turns them into the runnable Task list.
type EcosystemConfig struct {
	VersionControl  bool
	Build           bool
	Vet             bool
	Format          bool
	Quality         bool
	Dependencies    bool
	Runtime         bool
	Docs            bool
	Tests           bool
	Benchmark       bool
	Secrets         bool
	Vulnerabilities bool
	// Fix selects the Fixer slot (when present) ahead of the read-only features.
	Fix bool
}

// Tasks turns a EcosystemConfig into the runnable Task list: the Fixer first (when
// requested and present), then each selected, filled feature slot. Empty slots are
// dropped, so a feature this ecosystem does not implement never reaches the runner.
// A profiled feature (one whose check implements Profiled) expands into one task per
// configured run-profile.
func (e *Ecosystem) Tasks(s EcosystemConfig) []Task {
	var tasks []Task
	if s.Fix && e.Fixer != nil {
		tasks = append(tasks, newTask(FeatureFixer, e.Fixer))
	}
	if s.VersionControl && e.VersionControl != nil {
		tasks = append(tasks, newTask(FeatureVersionControl, e.VersionControl))
	}
	if s.Build && e.Build != nil {
		tasks = append(tasks, newTask(FeatureBuild, e.Build))
	}
	if s.Vet && e.Vet != nil {
		tasks = append(tasks, newTask(FeatureVet, e.Vet))
	}
	if s.Format && e.Format != nil {
		tasks = append(tasks, newTask(FeatureFormat, e.Format))
	}
	if s.Quality && e.Quality != nil {
		tasks = append(tasks, newTask(FeatureLint, e.Quality))
	}
	if s.Dependencies && e.Dependencies != nil {
		tasks = append(tasks, newTask(FeatureDependencies, e.Dependencies))
	}
	if s.Runtime && e.Runtime != nil {
		tasks = append(tasks, newTask(FeatureRuntime, e.Runtime))
	}
	if s.Docs && e.Docs != nil {
		tasks = append(tasks, newTask(FeatureDocs, e.Docs))
	}
	if s.Tests && e.Tests != nil {
		tasks = append(tasks, profileTasks(FeatureTests, e.Tests)...)
	}
	if s.Benchmark && e.Benchmark != nil {
		tasks = append(tasks, profileTasks(FeatureBenchmark, e.Benchmark)...)
	}
	if s.Secrets && e.Secrets != nil {
		tasks = append(tasks, newTask(FeatureSecrets, e.Secrets))
	}
	if s.Vulnerabilities && e.Vulnerabilities != nil {
		tasks = append(tasks, newTask(FeatureVulnerabilities, e.Vulnerabilities))
	}
	return tasks
}

// profileTasks expands a feature into one task per run-profile when its check
// implements Profiled; otherwise it emits the single non-profiled task. A profiled
// check with a lone unnamed profile collapses to a single, unlabeled task.
func profileTasks[T Report](feature string, c Check[T]) []Task {
	p, ok := c.(Profiled)
	if !ok {
		return []Task{newTask(feature, c)}
	}
	profiles := p.Profiles()
	if len(profiles) == 0 {
		return []Task{newTask(feature, c)}
	}
	tasks := make([]Task, 0, len(profiles))
	for _, prof := range profiles {
		tasks = append(tasks, newProfileTask(feature, c, prof))
	}
	return tasks
}
