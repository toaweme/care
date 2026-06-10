package gomod

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_ReplaceDirectives(t *testing.T) {
	tests := []struct {
		name  string
		gomod string
		want  []string
	}{
		{
			name:  "no replaces",
			gomod: "module example.com/x\n\ngo 1.22\n",
			want:  []string{},
		},
		{
			name:  "single replace",
			gomod: "module example.com/x\n\ngo 1.22\n\nreplace example.com/a => ../a\n",
			want:  []string{"example.com/a"},
		},
		{
			name:  "block of replaces",
			gomod: "module example.com/x\n\ngo 1.22\n\nreplace (\n\texample.com/a => ../a\n\texample.com/b => ../b\n)\n",
			want:  []string{"example.com/a", "example.com/b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(tt.gomod), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := ReplaceDirectives(dir)
			if err != nil {
				t.Fatalf("ReplaceDirectives: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}
