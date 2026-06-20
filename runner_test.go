package mend

import (
	"context"
	"reflect"
	"testing"
)

// noteReport is a minimal Report for runner tests: it carries a one-line summary.
type noteReport struct{ note string }

func (r noteReport) Summary(int) string  { return r.note }
func (r noteReport) Rows(int) [][]string { return nil }

// progCheck is a programmable Check[noteReport]: Run returns a fixed status and
// it reports a fixed Applies so a test can make it self-skip a dir.
type progCheck struct {
	BaseCheck
	status   Status
	applies  bool
	runCalls *int
}

func prog(name string, status Status) *progCheck {
	return &progCheck{BaseCheck: NewBaseCheck(name), status: status, applies: true}
}

func (f *progCheck) Applies(string) bool { return f.applies }

func (f *progCheck) Run(context.Context, string, RunOptions) Output[noteReport] {
	if f.runCalls != nil {
		*f.runCalls++
	}
	switch f.status {
	case StatusOK:
		return Pass(noteReport{"ok"})
	case StatusFail:
		return Fail(noteReport{"fail"})
	case StatusSkip:
		return Skip[noteReport]("skipped")
	default:
		return Warn(noteReport{"warn"})
	}
}

func runOutputs(out []Rendered) []Rendered {
	var r []Rendered
	for _, o := range out {
		if o.Phase() == PhaseRun {
			r = append(r, o)
		}
	}
	return r
}

func Test_Runner_DeterministicOrder(t *testing.T) {
	tasks := []Task{
		newTask[noteReport]("c", prog("c", StatusOK)),
		newTask[noteReport]("a", prog("a", StatusFail)),
		newTask[noteReport]("b", prog("b", StatusOK)),
	}
	r := NewRunner(false, nil)

	// repeated runs produce identical feature-sorted order despite concurrent
	// completion.
	for i := range 5 {
		got := runOutputs(r.Run(t.Context(), tasks, "/repo", RunOptions{}))
		var seq []string
		for _, o := range got {
			seq = append(seq, o.Feature())
		}
		want := []string{"a", "b", "c"}
		if !reflect.DeepEqual(seq, want) {
			t.Fatalf("run %d order = %v, want %v", i, seq, want)
		}
	}
}

func Test_Runner_SkipsNotApplicable(t *testing.T) {
	skipFeat := prog("go.only", StatusOK)
	skipFeat.applies = false
	tasks := []Task{
		newTask[noteReport]("agnostic", prog("agnostic", StatusOK)),
		newTask[noteReport]("go_only", skipFeat),
	}
	r := NewRunner(false, nil)
	got := runOutputs(r.Run(t.Context(), tasks, "/repo", RunOptions{}))
	if len(got) != 2 {
		t.Fatalf("got %d outputs, want 2", len(got))
	}
	for _, o := range got {
		if o.Feature() == "go_only" && o.Status() != StatusSkip {
			t.Errorf("not-applicable task = %+v, want skip", o)
		}
	}
}

func Test_Runner_FixerRunsBeforeChecks(t *testing.T) {
	calls := 0
	fix := prog("fixer", StatusOK)
	fix.runCalls = &calls
	tasks := []Task{
		newTask[noteReport](FeatureFixer, fix),
		newTask[noteReport](FeatureTests, prog("tests", StatusOK)),
	}
	r := NewRunner(false, nil)
	got := runOutputs(r.Run(t.Context(), tasks, "/repo", RunOptions{}))
	if calls != 1 {
		t.Fatalf("fixer ran %d times, want 1", calls)
	}
	if len(got) != 2 {
		t.Fatalf("got %d outputs, want 2", len(got))
	}
}
