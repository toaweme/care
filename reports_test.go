package mend

import (
	"strings"
	"testing"
)

func Test_RuntimeReport_Summary(t *testing.T) {
	tests := []struct {
		name string
		r    RuntimeReport
		want []string // substrings the summary must contain
	}{
		{
			name: "minimal",
			r:    RuntimeReport{Declared: "1.25.0", Minimum: "1.25.0", CodeMin: "1.22", DepMin: "1.25.0", DepModule: "golang.org/x/mod", CacheComplete: true},
			want: []string{"1.25.0, minimal", "code 1.22", "deps 1.25.0 (golang.org/x/mod)"},
		},
		{
			name: "reducible",
			r:    RuntimeReport{Declared: "1.26.0", Minimum: "1.25.0", Reducible: true, CodeMin: "1.22", DepMin: "1.25.0", DepModule: "golang.org/x/mod", CacheComplete: true},
			want: []string{"can drop to 1.25.0", "code 1.22", "deps 1.25.0"},
		},
		{
			name: "partial cache is not called minimal",
			r:    RuntimeReport{Declared: "1.25.0", Minimum: "1.25.0", CodeMin: "1.22", DepMin: "1.25.0", DepModule: "x/mod", CacheComplete: false},
			want: []string{"[partial cache]"},
		},
		{
			name: "maximum bound shown when set",
			r:    RuntimeReport{Declared: "3.11", Minimum: "3.9", Maximum: "3.12", CacheComplete: true},
			want: []string{"max 3.12"},
		},
		{
			name: "toolchain note",
			r:    RuntimeReport{Declared: "1.25.0", Minimum: "1.25.0", CacheComplete: true, Toolchain: "go1.26.0", ToolchainNote: "redundant; remove with `go get toolchain@none`"},
			want: []string{"toolchain go1.26.0 redundant"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Summary(0)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("summary %q missing %q", got, w)
				}
			}
		})
	}
}

func Test_RuntimeReport_Rows_depsOnlyAtVV(t *testing.T) {
	r := RuntimeReport{Deps: []RuntimeDep{{Module: "x/mod", Version: "v0.37.0", Min: "1.25.0"}}}
	if rows := r.Rows(0); rows != nil {
		t.Errorf("v0 rows = %v, want nil", rows)
	}
	if rows := r.Rows(1); rows != nil {
		t.Errorf("v1 rows = %v, want nil", rows)
	}
	rows := r.Rows(2)
	if len(rows) != 1 || rows[0][0] != "x/mod" || rows[0][2] != "1.25.0" {
		t.Errorf("vv rows = %v, want one x/mod row", rows)
	}
}
