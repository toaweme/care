package install

import (
	"context"
	"fmt"
	"testing"
)

type mockRunner struct {
	lookPath   map[string]bool
	runCalls   []runCall
	runOutputs map[string][]byte
	runErr     error
}

type runCall struct {
	name string
	args []string
}

func (m *mockRunner) LookPath(name string) (string, error) {
	if m.lookPath[name] {
		return "/usr/bin/" + name, nil
	}
	return "", fmt.Errorf("not found: %s", name)
}

func (m *mockRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	m.runCalls = append(m.runCalls, runCall{name: name, args: args})
	if m.runOutputs != nil {
		if out, ok := m.runOutputs[name]; ok {
			return out, m.runErr
		}
	}
	return nil, m.runErr
}

func Test_Brew_Available(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		lookPath map[string]bool
		want     bool
	}{
		{name: "darwin with brew", goos: "darwin", lookPath: map[string]bool{"brew": true}, want: true},
		{name: "linux with brew", goos: "linux", lookPath: map[string]bool{"brew": true}, want: true},
		{name: "windows", goos: "windows", lookPath: map[string]bool{"brew": true}, want: false},
		{name: "no brew", goos: "darwin", lookPath: map[string]bool{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Brew(WithRunner(&mockRunner{lookPath: tt.lookPath}), WithGOOS(tt.goos))
			if got := b.Available(); got != tt.want {
				t.Fatalf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Brew_Install(t *testing.T) {
	runner := &mockRunner{lookPath: map[string]bool{"brew": true}}
	b := Brew(WithRunner(runner), WithGOOS("darwin"))

	if err := b.Install(context.Background(), Tool{Bin: "mytool", Brew: "mytool-brew"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(runner.runCalls) != 1 {
		t.Fatalf("got %d run calls, want 1", len(runner.runCalls))
	}
	call := runner.runCalls[0]
	if call.name != "brew" || call.args[0] != "install" || call.args[1] != "mytool-brew" {
		t.Fatalf("unexpected call: %s %v", call.name, call.args)
	}
}

func Test_Brew_Install_NoFormula(t *testing.T) {
	b := Brew(WithRunner(&mockRunner{lookPath: map[string]bool{"brew": true}}), WithGOOS("darwin"))
	err := b.Install(context.Background(), Tool{Bin: "mytool"})
	if err == nil || !contains(err.Error(), "no brew formula configured") {
		t.Fatalf("want no-formula error, got %v", err)
	}
}

func Test_Go_Available(t *testing.T) {
	tests := []struct {
		name     string
		lookPath map[string]bool
		want     bool
	}{
		{name: "go on PATH", lookPath: map[string]bool{"go": true}, want: true},
		{name: "go missing", lookPath: map[string]bool{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := Go(WithRunner(&mockRunner{lookPath: tt.lookPath}))
			if got := g.Available(); got != tt.want {
				t.Fatalf("Available() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Go_Install(t *testing.T) {
	runner := &mockRunner{lookPath: map[string]bool{"go": true}}
	g := Go(WithRunner(runner))

	if err := g.Install(context.Background(), Tool{Bin: "mytool", GoPath: "example.com/mytool"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(runner.runCalls) != 1 {
		t.Fatalf("got %d run calls, want 1", len(runner.runCalls))
	}
	call := runner.runCalls[0]
	if call.name != "go" || call.args[0] != "install" || call.args[1] != "example.com/mytool@latest" {
		t.Fatalf("unexpected call: %s %v", call.name, call.args)
	}
}

func Test_Go_Install_PinnedVersion(t *testing.T) {
	runner := &mockRunner{lookPath: map[string]bool{"go": true}}
	g := Go(WithRunner(runner))

	if err := g.Install(context.Background(), Tool{Bin: "mytool", GoPath: "example.com/mytool", Version: "v1.2.3"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got := runner.runCalls[0].args[1]; got != "example.com/mytool@v1.2.3" {
		t.Fatalf("install arg = %q, want pinned version", got)
	}
}

func Test_Go_Install_NoImportPath(t *testing.T) {
	g := Go(WithRunner(&mockRunner{lookPath: map[string]bool{"go": true}}))
	err := g.Install(context.Background(), Tool{Bin: "mytool"})
	if err == nil || !contains(err.Error(), "no import path configured") {
		t.Fatalf("want no-import-path error, got %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
