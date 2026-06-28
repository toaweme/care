package changelog

import "testing"

func Test_Parse_Conventional(t *testing.T) {
	tests := []struct {
		name      string
		subject   string
		wantType  string
		wantScope string
		wantDesc  string
		wantPR    string
		breaking  bool
	}{
		{"feat", "feat: add thing", "feat", "", "Add thing", "", false},
		{"feat scope", "feat(api): add thing", "feat", "api", "Add thing", "", false},
		{"breaking bang", "feat!: drop old api", "feat", "", "Drop old api", "", true},
		{"breaking scope bang", "fix(core)!: change", "fix", "core", "Change", "", true},
		{"ref", "ref: rename project", "ref", "", "Rename project", "", false},
		{"non conventional", "just a message", "", "", "Just a message", "", false},
		{"breaking body marker", "chore: cleanup BREAKING CHANGE", "chore", "", "Cleanup BREAKING CHANGE", "", true},
		{"pr suffix", "feat: add thing (#42)", "feat", "", "Add thing", "42", false},
		{"pr suffix non conventional", "merged something (#7)", "", "", "Merged something", "7", false},
		{"already capitalized", "fix: Broken call", "fix", "", "Broken call", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Commit{Subject: tt.subject}
			Parse(&c)
			if c.Type != tt.wantType {
				t.Errorf("type = %q, want %q", c.Type, tt.wantType)
			}
			if c.Scope != tt.wantScope {
				t.Errorf("scope = %q, want %q", c.Scope, tt.wantScope)
			}
			if c.Desc != tt.wantDesc {
				t.Errorf("desc = %q, want %q", c.Desc, tt.wantDesc)
			}
			if c.PR != tt.wantPR {
				t.Errorf("pr = %q, want %q", c.PR, tt.wantPR)
			}
			if c.Breaking != tt.breaking {
				t.Errorf("breaking = %v, want %v", c.Breaking, tt.breaking)
			}
		})
	}
}

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
