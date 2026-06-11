package minver

import (
	"os"
	"path/filepath"
	"testing"
)

// writeModule writes a throwaway module declaring `go <goVer>` with main.go
// holding body, and returns its directory.
func writeModule(t *testing.T, goVer, body string) string {
	t.Helper()
	dir := t.TempDir()
	mod := "module example.test\n\ngo " + goVer + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(body), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return dir
}

func Test_Scanner_ScanDir(t *testing.T) {
	hist, err := LoadHistory()
	if err != nil {
		t.Skipf("api history unavailable: %v", err)
	}

	tests := []struct {
		name  string
		goVer string
		body  string
		want  int
	}{
		{
			name:  "plain code needs nothing new",
			goVer: "1.18",
			body:  "package main\n\nfunc main() {\n\ts := []int{1, 2}\n\tfor _, v := range s {\n\t\t_ = v\n\t}\n}\n",
			want:  0,
		},
		{
			name:  "range over integer",
			goVer: "1.22",
			body:  "package main\n\nfunc main() {\n\tfor i := range 10 {\n\t\t_ = i\n\t}\n}\n",
			want:  22,
		},
		{
			name:  "range over function",
			goVer: "1.23",
			body:  "package main\n\nfunc seq(yield func(int) bool) {\n\t_ = yield(1)\n}\n\nfunc main() {\n\tfor v := range seq {\n\t\t_ = v\n\t}\n}\n",
			want:  23,
		},
		{
			name:  "min builtin",
			goVer: "1.21",
			body:  "package main\n\nfunc main() {\n\t_ = min(1, 2)\n}\n",
			want:  21,
		},
		{
			name:  "generic function",
			goVer: "1.18",
			body:  "package main\n\nfunc id[T any](v T) T { return v }\n\nfunc main() { _ = id(1) }\n",
			want:  18,
		},
		{
			name:  "stdlib slices.Sort is 1.21",
			goVer: "1.21",
			body:  "package main\n\nimport \"slices\"\n\nfunc main() {\n\ts := []int{3, 1, 2}\n\tslices.Sort(s)\n}\n",
			want:  21,
		},
		{
			name:  "generic type alias",
			goVer: "1.24",
			body:  "package main\n\ntype Pair[T any] struct{ a, b T }\ntype Alias[T any] = Pair[T]\n\nfunc main() { _ = Alias[int]{} }\n",
			want:  24,
		},
	}

	s := NewScanner(hist)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeModule(t, tt.goVer, tt.body)
			res, err := s.ScanDir(t.Context(), dir)
			if err != nil {
				t.Fatalf("ScanDir: %v", err)
			}
			if res.Min != tt.want {
				t.Fatalf("Min = %d, want %d (reasons: %+v)", res.Min, tt.want, res.Reasons)
			}
		})
	}
}
