package minver

import "testing"

func Test_apiFileMinor(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		want  int
		wantK bool
	}{
		{"baseline", "go1.txt", 0, true},
		{"minor", "go1.21.txt", 21, true},
		{"two digit", "go1.26.txt", 26, true},
		{"except", "except.txt", 0, false},
		{"readme", "README", 0, false},
		{"not txt", "go1.21", 0, false},
		{"garbage", "go1.x.txt", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := apiFileMinor(tt.file)
			if ok != tt.wantK || got != tt.want {
				t.Fatalf("apiFileMinor(%q) = (%d, %t), want (%d, %t)", tt.file, got, ok, tt.want, tt.wantK)
			}
		})
	}
}

func Test_History_recordLine(t *testing.T) {
	h := &History{ids: map[string]map[string]int{}, members: map[string]map[string]map[string]int{}}
	// later versions must not overwrite the earliest introduction.
	h.recordLine("slices", "func Sort[$0 interface{...}]($0)", 21)
	h.recordLine("slices", "func Sort[$0 interface{...}]($0)", 24)
	h.recordLine("archive/tar", "method (*Writer) AddFS(fs.FS) error #58000", 22)
	h.recordLine("net/http", "type Request struct, Body io.ReadCloser", 0)
	h.recordLine("io", "type Reader interface, Read([]uint8) (int, error)", 0)
	h.recordLine("cmp", "type Ordered interface {}", 21)
	h.recordLine("time", "const Layout untyped string", 20)

	tests := []struct {
		name           string
		pkg, typ, sym  string
		wantVer        int
		wantOK, member bool
	}{
		{name: "func earliest wins", pkg: "slices", sym: "Sort", wantVer: 21, wantOK: true},
		{name: "method", pkg: "archive/tar", typ: "Writer", sym: "AddFS", wantVer: 22, wantOK: true, member: true},
		{name: "struct field", pkg: "net/http", typ: "Request", sym: "Body", wantVer: 0, wantOK: true, member: true},
		{name: "iface method", pkg: "io", typ: "Reader", sym: "Read", wantVer: 0, wantOK: true, member: true},
		{name: "type id", pkg: "cmp", sym: "Ordered", wantVer: 21, wantOK: true},
		{name: "const id", pkg: "time", sym: "Layout", wantVer: 20, wantOK: true},
		{name: "missing", pkg: "slices", sym: "Nope", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int
			var ok bool
			if tt.member {
				got, ok = h.lookupMember(tt.pkg, tt.typ, tt.sym)
			} else {
				got, ok = h.lookup(tt.pkg, tt.sym)
			}
			if ok != tt.wantOK || (ok && got != tt.wantVer) {
				t.Fatalf("lookup = (%d, %t), want (%d, %t)", got, ok, tt.wantVer, tt.wantOK)
			}
		})
	}
}
