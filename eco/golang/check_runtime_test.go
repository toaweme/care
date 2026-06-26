package golang

import (
	"testing"

	"github.com/toaweme/care/eco/golang/gomod"
)

func Test_toolchainNote(t *testing.T) {
	tests := []struct {
		name     string
		d        gomod.Directives
		wantNote bool
	}{
		{"none", gomod.Directives{GoVersion: "1.25.0"}, false},
		{"redundant same minor", gomod.Directives{GoVersion: "1.25.0", Toolchain: "go1.25.4"}, true},
		{"raises floor", gomod.Directives{GoVersion: "1.25.0", Toolchain: "go1.26.0"}, true},
		{"older toolchain", gomod.Directives{GoVersion: "1.25.0", Toolchain: "go1.24.0"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolchainNote(tt.d) != ""; got != tt.wantNote {
				t.Errorf("toolchainNote(%+v) note=%v, want %v", tt.d, got, tt.wantNote)
			}
		})
	}
}

func Test_maxGoVer(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"", "", ""},
		{"1.22", "", "1.22"},
		{"", "1.25.0", "1.25.0"},
		{"1.22", "1.25.0", "1.25.0"},
		{"1.25.0", "1.25.0", "1.25.0"},
		{"1.26", "1.25.0", "1.26"},
	}
	for _, tt := range tests {
		if got := maxGoVer(tt.a, tt.b); got != tt.want {
			t.Errorf("maxGoVer(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func Test_runtimeDeps_sortedByVersionDesc(t *testing.T) {
	in := []gomod.DepGo{
		{Module: "z/low", Version: "v1", Go: "1.21"},
		{Module: "a/high", Version: "v2", Go: "1.25.0"},
		{Module: "b/high", Version: "v3", Go: "1.25.0"},
		{Module: "mid", Version: "v4", Go: "1.23"},
	}
	got := runtimeDeps(in)
	want := []string{"a/high", "b/high", "mid", "z/low"} // 1.25.0 (a,b), 1.23, 1.21
	for i, w := range want {
		if got[i].Module != w {
			t.Fatalf("position %d = %q, want %q (order: %+v)", i, got[i].Module, w, got)
		}
	}
}
