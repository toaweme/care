package changelog

import (
	"context"
	"testing"
)

func Test_Semver(t *testing.T) {
	if got := Semver("v1.2.3"); got != "1.2.3" {
		t.Errorf("Semver = %q, want 1.2.3", got)
	}
	if got := Semver("0.1.0"); got != "0.1.0" {
		t.Errorf("Semver = %q, want 0.1.0", got)
	}
}

func Test_Git_PreviousTag(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "fix: two")
	tag(t, dir, "v0.2.0")

	git := NewGit(dir)
	prev, err := git.PreviousTag(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "v0.1.0" {
		t.Errorf("PreviousTag(v0.2.0) = %q, want v0.1.0", prev)
	}

	// the first tag has no predecessor: empty, not an error.
	first, err := git.PreviousTag(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if first != "" {
		t.Errorf("PreviousTag(v0.1.0) = %q, want empty", first)
	}
}

func Test_Git_LatestTag(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "feat: one")
	tag(t, dir, "v0.1.0")
	commit(t, dir, "feat: two")
	tag(t, dir, "v0.10.0")

	latest, err := NewGit(dir).LatestTag(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// version sort, not lexical: v0.10.0 outranks v0.1.0.
	if latest != "v0.10.0" {
		t.Errorf("LatestTag = %q, want v0.10.0", latest)
	}
}
