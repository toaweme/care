package golang

import (
	"reflect"
	"testing"
)

func Test_ParseGofmtList(t *testing.T) {
	out := "internal/foo.go\nvendor/dep/x.go\ncmd/main.go\npkg/testdata/golden.go\n\n"
	got := parseGofmtList([]byte(out))
	want := []string{"internal/foo.go", "cmd/main.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseGofmtList() = %v, want %v (vendored/testdata dropped)", got, want)
	}
}
