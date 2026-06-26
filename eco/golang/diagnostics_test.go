package golang

import (
	"reflect"
	"testing"
)

func Test_ParseDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []diagnostic
	}{
		{
			name: "go build error with continuation lines",
			in: "# github.com/toaweme/care\n" +
				"./broken.go:3:21: not enough return values\n" +
				"\thave ()\n" +
				"\twant (int)\n",
			want: []diagnostic{{File: "./broken.go", Line: 3, Col: 21, Message: "not enough return values"}},
		},
		{
			name: "go vet diagnostic",
			in:   "withtest/bad.go:5:26: fmt.Printf format %d has arg \"string\" of wrong type string\n",
			want: []diagnostic{{File: "withtest/bad.go", Line: 5, Col: 26, Message: `fmt.Printf format %d has arg "string" of wrong type string`}},
		},
		{
			name: "diagnostic without column",
			in:   "main.go:10: undefined: Foo\n",
			want: []diagnostic{{File: "main.go", Line: 10, Message: "undefined: Foo"}},
		},
		{
			name: "no diagnostics",
			in:   "",
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseDiagnostics([]byte(c.in))
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("parseDiagnostics()\n got = %+v\nwant = %+v", got, c.want)
			}
		})
	}
}
