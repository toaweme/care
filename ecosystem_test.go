package care

import (
	"context"
	"reflect"
	"testing"
)

// vcStub and fixStub fill two typed slots so Select can be exercised end to end.
type vcStub struct{ BaseCheck }

func (vcStub) Applies(string) bool { return true }
func (vcStub) Run(context.Context, string, RunOptions) Output[VCReport] {
	return Pass(VCReport{})
}

type fixStub struct{ BaseCheck }

func (fixStub) Applies(string) bool { return true }
func (fixStub) Run(context.Context, string, RunOptions) Output[FixReport] {
	return Pass(FixReport{})
}

func features(tasks []Task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Feature())
	}
	return out
}

// testsStub fills the Tests slot and optionally advertises run-profiles, so profile
// expansion can be exercised end to end.
type testsStub struct {
	BaseCheck
	profiles []RunProfile
}

func (testsStub) Applies(string) bool { return true }
func (testsStub) Run(context.Context, string, RunOptions) Output[TestReport] {
	return Pass(TestReport{})
}
func (s testsStub) Profiles() []RunProfile { return s.profiles }

func profiles(tasks []Task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Profile())
	}
	return out
}

func Test_Ecosystem_ProfileExpansion(t *testing.T) {
	eco := &Ecosystem{Tests: testsStub{NewBaseCheck("go-test"), []RunProfile{{Name: "default"}, {Name: "race"}}}}
	tasks := eco.Tasks(EcosystemConfig{Tests: true})
	if got := profiles(tasks); !reflect.DeepEqual(got, []string{"default", "race"}) {
		t.Fatalf("profiles = %v, want [default race]", got)
	}
	for _, tk := range tasks {
		if tk.Feature() != FeatureTests {
			t.Errorf("task feature = %q, want tests", tk.Feature())
		}
	}
}

func Test_Ecosystem_ProfileExpansion_NoneConfigured(t *testing.T) {
	// a profiled check with no configured profiles runs exactly once, unlabeled.
	eco := &Ecosystem{Tests: testsStub{BaseCheck: NewBaseCheck("go-test")}}
	tasks := eco.Tasks(EcosystemConfig{Tests: true})
	if len(tasks) != 1 || tasks[0].Profile() != "" {
		t.Fatalf("tasks = %v (profiles %v), want one unlabeled task", features(tasks), profiles(tasks))
	}
}

func Test_Ecosystem_SelectFillsTasks(t *testing.T) {
	eco := &Ecosystem{
		VersionControl: vcStub{NewBaseCheck("git")},
		Fixer:          fixStub{NewBaseCheck("go-fixer")},
	}
	tasks := eco.Tasks(EcosystemConfig{VersionControl: true, Fix: true})
	if got := features(tasks); !reflect.DeepEqual(got, []string{FeatureFixer, FeatureVersionControl}) {
		t.Fatalf("Select features = %v, want fixer first then version_control", got)
	}
}

func Test_Ecosystem_SelectSkipsEmptySlots(t *testing.T) {
	eco := &Ecosystem{VersionControl: vcStub{NewBaseCheck("git")}}
	// Quality is selected but its slot is empty, so it must not produce a task.
	tasks := eco.Tasks(EcosystemConfig{VersionControl: true, Quality: true})
	if got := features(tasks); !reflect.DeepEqual(got, []string{FeatureVersionControl}) {
		t.Fatalf("Select features = %v, want only version_control", got)
	}
}

func Test_Ecosystem_SelectDeselected(t *testing.T) {
	eco := &Ecosystem{VersionControl: vcStub{NewBaseCheck("git")}}
	// the slot is filled but not selected, so nothing runs.
	if tasks := eco.Tasks(EcosystemConfig{}); len(tasks) != 0 {
		t.Fatalf("Select with empty selection = %v, want none", features(tasks))
	}
}
