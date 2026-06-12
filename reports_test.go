package mend

import (
	"strings"
	"testing"
)

func Test_Bound_String(t *testing.T) {
	tests := []struct {
		b    Bound
		want string
	}{
		{Bound{}, ""},
		{Bound{Min: "1.25"}, "1.25"},
		{Bound{Max: "22"}, "<=22"},
		{Bound{Min: "18", Max: "22"}, "18..22"},
	}
	for _, tt := range tests {
		if got := tt.b.String(); got != tt.want {
			t.Errorf("Bound%+v.String() = %q, want %q", tt.b, got, tt.want)
		}
	}
}

func Test_RuntimeReport_Summary(t *testing.T) {
	tests := []struct {
		name string
		r    RuntimeReport
		want []string // substrings the summary must contain
		deny []string // substrings it must NOT contain
	}{
		{
			name: "minimal go - every number labeled",
			r:    RuntimeReport{Version: VersionRange{Declared: Bound{Min: "1.25.0"}, Required: Bound{Min: "1.22"}}, Minimum: "1.25.0", DepFloor: "1.25.0"},
			want: []string{"go.mod 1.25.0", "code 1.22", "deps 1.25.0"},
			deny: []string{"minimal", "(min "}, // already minimal: no marker
		},
		{
			name: "reducible marks go.mod",
			r:    RuntimeReport{Version: VersionRange{Declared: Bound{Min: "1.26.0"}, Required: Bound{Min: "1.22"}}, Minimum: "1.25.0", DepFloor: "1.25.0", Reducible: true},
			want: []string{"go.mod 1.26.0 (min 1.25.0)", "code 1.22", "deps 1.25.0"},
		},
		{
			name: "node range with module",
			r:    RuntimeReport{Version: VersionRange{Declared: Bound{Min: "18.17", Max: "23"}, Required: Bound{Min: "20"}}, Module: "esm"},
			want: []string{"go.mod 18.17..23", "code 20", "esm"},
		},
		{
			name: "toolchain note",
			r:    RuntimeReport{Version: VersionRange{Declared: Bound{Min: "1.25.0"}}, Minimum: "1.25.0", Toolchain: "go1.26.0", ToolchainNote: "redundant"},
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
			for _, d := range tt.deny {
				if strings.Contains(got, d) {
					t.Errorf("summary %q should not contain %q", got, d)
				}
			}
		})
	}
}

func Test_RuntimeReport_Rows_fromV(t *testing.T) {
	r := RuntimeReport{Version: VersionRange{Declared: Bound{Min: "1.25.0"}, Required: Bound{Min: "1.22"}}, RequiredReason: "go/types.Alias"}
	if rows := r.Rows(0); rows != nil {
		t.Errorf("v0 rows = %v, want nil", rows)
	}
	rows := r.Rows(1)
	if len(rows) < 2 || rows[0][0] != "go.mod" || rows[1][0] != "code" {
		t.Fatalf("v1 rows = %v, want go.mod + code breakdown", rows)
	}
}

func Test_DepsReport_floorAndTable(t *testing.T) {
	r := DepsReport{
		RuntimeFloor:   "1.25.0",
		RuntimeFloorBy: "golang.org/x/mod",
		Deps:           []RuntimeDep{{Module: "golang.org/x/mod", Version: "v0.37.0", Min: "1.25.0"}},
	}
	// floor is context: hidden at v0, shown from -v.
	if s := r.Summary(0); strings.Contains(s, "deps require") {
		t.Errorf("v0 summary should not show the floor: %q", s)
	}
	if s := r.Summary(1); !strings.Contains(s, "deps require 1.25.0 (golang.org/x/mod)") {
		t.Errorf("v1 summary missing floor: %q", s)
	}
	// the per-dep table is exhaustive detail: only at -vv.
	if rows := r.Rows(1); len(rows) != 0 {
		t.Errorf("v1 rows = %v, want none (no issues, table is -vv)", rows)
	}
	if rows := r.Rows(2); len(rows) != 1 || rows[0][0] != "golang.org/x/mod" {
		t.Errorf("vv rows = %v, want the dep table", rows)
	}
}
