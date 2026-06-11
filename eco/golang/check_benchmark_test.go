package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/toaweme/mend"
)

func Test_ParseBenchOutput(t *testing.T) {
	out := []byte(`goos: darwin
goarch: arm64
pkg: github.com/toaweme/mend/eco/golang
BenchmarkPlain-10       	 1000000	      1043 ns/op	     256 B/op	       4 allocs/op
BenchmarkThroughput-8   	  500000	      2100 ns/op	   512.34 MB/s	     128 B/op	       2 allocs/op
BenchmarkCustom-10      	  250000	      4200 ns/op	         3 items/op
PASS
ok  	github.com/toaweme/mend/eco/golang	3.210s
`)

	got := parseBenchOutput(out)
	if len(got) != 3 {
		t.Fatalf("expected 3 benchmarks, got %d", len(got))
	}

	// the -GOMAXPROCS suffix is stripped, package is carried from the pkg: header.
	for _, b := range got {
		if b.Package != "github.com/toaweme/mend/eco/golang" {
			t.Fatalf("benchmark %q has wrong package %q", b.Name, b.Package)
		}
	}

	plain := got[0]
	if plain.Name != "BenchmarkPlain" || plain.Runs != 1000000 || plain.NsPerOp != 1043 || plain.BytesPerOp != 256 || plain.AllocsPerOp != 4 {
		t.Fatalf("standard columns parsed wrong: %+v", plain)
	}
	if len(plain.Extra) != 0 {
		t.Fatalf("plain benchmark should have no extra metrics, got %+v", plain.Extra)
	}

	// SetBytes throughput rides the same line between ns/op and B/op: kept as Extra,
	// standard columns still parsed.
	tp := got[1]
	if tp.NsPerOp != 2100 || tp.BytesPerOp != 128 || tp.AllocsPerOp != 2 {
		t.Fatalf("throughput standard columns parsed wrong: %+v", tp)
	}
	if want := []mend.BenchMetric{{Unit: "MB/s", Value: 512.34}}; !sameMetrics(tp.Extra, want) {
		t.Fatalf("expected MB/s captured, got %+v", tp.Extra)
	}

	// a ReportMetric custom unit with no standard mem columns.
	custom := got[2]
	if want := []mend.BenchMetric{{Unit: "items/op", Value: 3}}; !sameMetrics(custom.Extra, want) {
		t.Fatalf("expected custom items/op captured, got %+v", custom.Extra)
	}
}

func sameMetrics(a, b []mend.BenchMetric) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Test_hasBenchmarks(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{
			name:  "no test files",
			files: map[string]string{"main.go": "package x\n"},
			want:  false,
		},
		{
			name:  "test file without benchmark",
			files: map[string]string{"x_test.go": "package x\n\nfunc Test_It(t *testing.T) {}\n"},
			want:  false,
		},
		{
			name:  "benchmark in a nested package",
			files: map[string]string{"sub/b_test.go": "package sub\n\nimport \"testing\"\n\nfunc BenchmarkFoo(b *testing.B) {}\n"},
			want:  true,
		},
		{
			name:  "benchmark mention only in a comment does not count",
			files: map[string]string{"x_test.go": "package x\n\n// func Benchmark would go here\nfunc Test_It(t *testing.T) {}\n"},
			want:  false,
		},
		{
			name:  "benchmark under testdata is ignored",
			files: map[string]string{"testdata/b_test.go": "package x\n\nfunc BenchmarkFoo(b *testing.B) {}\n"},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for rel, body := range tt.files {
				p := filepath.Join(dir, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := hasBenchmarks(dir); got != tt.want {
				t.Errorf("hasBenchmarks = %v, want %v", got, tt.want)
			}
		})
	}
}
