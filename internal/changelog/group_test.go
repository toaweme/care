package changelog

import "testing"

func Test_Grouper_Group(t *testing.T) {
	commits := []Commit{
		{Subject: "feat: a"},
		{Subject: "fix: b"},
		{Subject: "docs: c"},
		{Subject: "ref: d"},
		{Subject: "refactor: e"},
		{Subject: "test: f"},
		{Subject: "ci: g"},
		{Subject: "build: h"},
		{Subject: "chore: i"},
		{Subject: "random message"},
	}
	sections := NewGrouper(DefaultGroups).Group(commits)

	wantOrder := []string{"Features", "Fixes", "Documentation", "Refactors", "Tests", "CI & Build", "Chores & Other"}
	if len(sections) != len(wantOrder) {
		t.Fatalf("got %d sections, want %d: %+v", len(sections), len(wantOrder), sections)
	}
	for i, title := range wantOrder {
		if sections[i].Title != title {
			t.Errorf("section[%d] = %q, want %q", i, sections[i].Title, title)
		}
	}
	// Refactors must hold both ref and refactor; Chores must hold chore + the
	// non-conventional message (nothing is dropped).
	if got := len(sections[3].Commits); got != 2 {
		t.Errorf("Refactors holds %d commits, want 2", got)
	}
	if got := len(sections[6].Commits); got != 2 {
		t.Errorf("Chores & Other holds %d commits, want 2", got)
	}
}

func Test_Grouper_DropsEmptySections(t *testing.T) {
	sections := NewGrouper(DefaultGroups).Group([]Commit{{Subject: "feat: only"}})
	if len(sections) != 1 || sections[0].Title != "Features" {
		t.Fatalf("expected only a Features section, got %+v", sections)
	}
}
