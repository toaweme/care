package tests

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func Test_ParseTestJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []PackageResult
	}{
		{
			name: "passing with coverage and per-test cases",
			input: lines(
				`{"Action":"run","Package":"example.com/a","Test":"Test_One"}`,
				`{"Action":"pass","Package":"example.com/a","Test":"Test_One","Elapsed":0.01}`,
				`{"Action":"output","Package":"example.com/a","Output":"coverage: 84.2% of statements\n"}`,
				`{"Action":"pass","Package":"example.com/a","Elapsed":0.312}`,
			),
			want: []PackageResult{{
				Name: "example.com/a", Coverage: 84.2, Passed: true, DurationMs: 312,
				Tests: []TestCase{{Name: "Test_One", Action: "pass", ElapsedMs: 10}},
			}},
		},
		{
			name: "failing package records failed test",
			input: lines(
				`{"Action":"fail","Package":"example.com/b","Test":"Test_Bad","Elapsed":0}`,
				`{"Action":"fail","Package":"example.com/b","Elapsed":0.045}`,
			),
			want: []PackageResult{{
				Name: "example.com/b", Passed: false, DurationMs: 45,
				Tests: []TestCase{{Name: "Test_Bad", Action: "fail"}},
			}},
		},
		{
			name: "failing test captures its output, package build error captured",
			input: lines(
				`{"Action":"output","Package":"example.com/c","Test":"Test_Bad","Output":"=== RUN   Test_Bad\n"}`,
				`{"Action":"output","Package":"example.com/c","Test":"Test_Bad","Output":"    foo_test.go:9: got 1, want 2\n"}`,
				`{"Action":"fail","Package":"example.com/c","Test":"Test_Bad","Elapsed":0.01}`,
				`{"Action":"output","Package":"example.com/c","Output":"# example.com/c [build failed]\n"}`,
				`{"Action":"fail","Package":"example.com/c","Elapsed":0.02}`,
			),
			want: []PackageResult{{
				Name: "example.com/c", Passed: false, DurationMs: 20,
				Output: "# example.com/c [build failed]",
				Tests: []TestCase{{
					Name: "Test_Bad", Action: "fail", ElapsedMs: 10,
					Output: "    foo_test.go:9: got 1, want 2",
				}},
			}},
		},
		{
			name:  "no test files is skipped but passing",
			input: lines(`{"Action":"skip","Package":"example.com/templates","Elapsed":0}`),
			want: []PackageResult{{
				Name: "example.com/templates", Passed: true, Skipped: true,
			}},
		},
		{
			name:  "empty output",
			input: "",
			want:  []PackageResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTestJSON([]byte(tt.input))
			if err != nil {
				t.Fatalf("parseTestJSON: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseTestJSON()\n got = %+v\nwant = %+v", got, tt.want)
			}
		})
	}
}

func Test_ComputeTotal(t *testing.T) {
	tests := []struct {
		name     string
		packages []PackageResult
		want     float64
	}{
		{name: "empty", packages: nil, want: 0},
		{
			name:     "averages percentages",
			packages: []PackageResult{{Coverage: 80, Passed: true}, {Coverage: 60, Passed: true}},
			want:     70,
		},
		{
			name:     "skips skipped packages",
			packages: []PackageResult{{Coverage: 80, Passed: true}, {Skipped: true, Passed: true}},
			want:     80,
		},
		{
			name: "weights by statements when counts are known",
			packages: []PackageResult{
				{Statements: 100, Covered: 90, Passed: true}, // big package, well covered
				{Statements: 10, Covered: 0, Passed: true},   // tiny package, uncovered
			},
			want: 81.818, // 90/110, not the 45 an unweighted mean would give
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeTotal(tt.packages); got < tt.want-0.01 || got > tt.want+0.01 {
				t.Fatalf("computeTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ParseCoverProfile(t *testing.T) {
	profile := lines(
		"mode: set",
		"example.com/a/foo.go:7.34,9.2 2 1",
		"example.com/a/foo.go:12.2,14.3 1 0",
		"example.com/a/bar.go:3.10,5.4 3 1",
		"example.com/b/baz.go:1.1,2.2 4 0",
	)
	dir := t.TempDir()
	path := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(path, []byte(profile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	got, err := parseCoverProfile(path)
	if err != nil {
		t.Fatalf("parseCoverProfile: %v", err)
	}

	a := got["example.com/a"]
	if a == nil || a.statements != 6 || a.covered != 5 {
		t.Fatalf("package a = %+v, want statements=6 covered=5", a)
	}
	if len(a.files) != 2 {
		t.Fatalf("package a files = %d, want 2", len(a.files))
	}
	if a.files[0].Path != "example.com/a/bar.go" || a.files[0].Covered != 3 {
		t.Fatalf("package a first file = %+v, want bar.go covered=3", a.files[0])
	}
	b := got["example.com/b"]
	if b == nil || b.statements != 4 || b.covered != 0 {
		t.Fatalf("package b = %+v, want statements=4 covered=0", b)
	}

	pkgs := []PackageResult{{Name: "example.com/a", Passed: true}, {Name: "example.com/b", Passed: true}}
	applyCoverage(pkgs, got)
	if pkgs[0].Statements != 6 || pkgs[0].Covered != 5 {
		t.Fatalf("applyCoverage a = %+v, want statements=6 covered=5", pkgs[0])
	}
	if got := pkgs[0].Coverage; got < 83.33-0.01 || got > 83.33+0.01 {
		t.Fatalf("applyCoverage a coverage = %v, want ~83.33", got)
	}
}

func Test_ParseCoverProfile_Uncovered(t *testing.T) {
	profile := lines(
		"mode: set",
		"example.com/a/foo.go:7.34,9.2 2 1",   // covered
		"example.com/a/foo.go:12.2,14.3 1 0",  // uncovered, lines 12-14
		"example.com/a/foo.go:20.1,20.10 1 0", // uncovered, line 20
	)
	dir := t.TempDir()
	path := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(path, []byte(profile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	got, err := parseCoverProfile(path)
	if err != nil {
		t.Fatalf("parseCoverProfile: %v", err)
	}
	files := got["example.com/a"].files
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	want := []LineRange{{Start: 12, End: 14}, {Start: 20, End: 20}}
	if !reflect.DeepEqual(files[0].Uncovered, want) {
		t.Fatalf("uncovered = %+v, want %+v", files[0].Uncovered, want)
	}
}

func Test_BlockLines(t *testing.T) {
	cases := []struct {
		in     string
		want   LineRange
		wantOK bool
	}{
		{"12.2,14.3", LineRange{12, 14}, true},
		{"7.1,7.40", LineRange{7, 7}, true},
		{"nonsense", LineRange{}, false},
		{"12", LineRange{}, false},
	}
	for _, c := range cases {
		got, ok := blockLines(c.in)
		if ok != c.wantOK || got != c.want {
			t.Errorf("blockLines(%q) = %+v,%v want %+v,%v", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func Test_ParseTestJSON_NoTestFiles(t *testing.T) {
	// without coverage, a package with no tests reports "[no test files]" then skips.
	in := lines(
		`{"Action":"output","Package":"example.com/x","Output":"?   \texample.com/x\t[no test files]\n"}`,
		`{"Action":"skip","Package":"example.com/x","Elapsed":0}`,
	)
	got, err := parseTestJSON([]byte(in))
	if err != nil {
		t.Fatalf("parseTestJSON: %v", err)
	}
	if len(got) != 1 || !got[0].NoTestFiles || !got[0].Skipped {
		t.Fatalf("got = %+v, want one NoTestFiles+Skipped package", got)
	}
}

func lines(ls ...string) string {
	var b []byte
	for _, l := range ls {
		b = append(b, l...)
		b = append(b, '\n')
	}
	return string(b)
}
