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
