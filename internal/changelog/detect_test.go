package changelog

import "testing"

func Test_ParseRemote(t *testing.T) {
	tests := []struct {
		url       string
		wantOK    bool
		wantHost  string
		wantOwner string
		wantRepo  string
	}{
		{"https://github.com/toaweme/care.git", true, "github.com", "toaweme", "care"},
		{"https://github.com/toaweme/care", true, "github.com", "toaweme", "care"},
		{"git@github.com:toaweme/care.git", true, "github.com", "toaweme", "care"},
		{"ssh://git@gitlab.com/group/proj.git", true, "gitlab.com", "group", "proj"},
		{"not a remote", false, "", "", ""},
	}
	for _, tt := range tests {
		got, ok := ParseRemote(tt.url)
		if ok != tt.wantOK {
			t.Errorf("ParseRemote(%q) ok = %v, want %v", tt.url, ok, tt.wantOK)
			continue
		}
		if ok && (got.Host != tt.wantHost || got.Owner != tt.wantOwner || got.Repo != tt.wantRepo) {
			t.Errorf("ParseRemote(%q) = %+v, want host=%s owner=%s repo=%s", tt.url, got, tt.wantHost, tt.wantOwner, tt.wantRepo)
		}
	}
}

func Test_firstEnv(t *testing.T) {
	t.Setenv("CARE_TEST_A", "")
	t.Setenv("CARE_TEST_B", "second")
	if got := firstEnv("CARE_TEST_A", "CARE_TEST_B"); got != "second" {
		t.Errorf("firstEnv = %q, want second (skips empty)", got)
	}
	if got := firstEnv("CARE_TEST_MISSING"); got != "" {
		t.Errorf("firstEnv = %q, want empty", got)
	}
}

// clearCIEnv neutralizes every platform CI signal so host-matching is exercised
// in isolation, regardless of which CI the suite itself runs under (e.g. GitHub
// Actions sets GITHUB_ACTIONS, which would otherwise identify an unknown host).
func clearCIEnv(t *testing.T) {
	t.Helper()
	for _, env := range []string{"GITHUB_ACTIONS", "GITLAB_CI", "GITEA_ACTIONS", "FORGEJO_ACTIONS", "BITBUCKET_BUILD_NUMBER"} {
		t.Setenv(env, "")
	}
}

func Test_DetectGitHost_UnknownHostDegrades(t *testing.T) {
	clearCIEnv(t)
	if f := DetectGitHost(t.Context(), "", "https://example.invalid/team/repo.git", ""); f != nil {
		t.Errorf("unknown host should degrade to nil host, got %T", f)
	}
}

func Test_DetectGitHost_RecognizedButUnimplementedDegrades(t *testing.T) {
	// gitlab is in the registry but has no GitHost yet, so it degrades to git-log.
	if f := DetectGitHost(t.Context(), "", "https://gitlab.com/group/proj.git", ""); f != nil {
		t.Errorf("unimplemented platform should degrade to nil host, got %T", f)
	}
}

func Test_DetectGitHost_EnterpriseHostMatches(t *testing.T) {
	f := DetectGitHost(t.Context(), "", "https://github.mycorp.com/team/app.git", "tok")
	gh, ok := f.(*GitHub)
	if !ok {
		t.Fatalf("github enterprise host should map to *GitHub, got %T", f)
	}
	if gh.apiBase() != "https://github.mycorp.com/api/v3" {
		t.Errorf("enterprise apiBase = %q, want the /api/v3 mount", gh.apiBase())
	}
	if got := gh.CompareURL("v1", "v2"); got != "https://github.mycorp.com/team/app/compare/v1...v2" {
		t.Errorf("enterprise compare URL = %q", got)
	}
}

func Test_detectPlatform(t *testing.T) {
	clearCIEnv(t)
	tests := []struct {
		host string
		want string
	}{
		{"github.com", "github"},
		{"gitlab.com", "gitlab"},
		{"codeberg.org", "gitea"},
		{"bitbucket.org", "bitbucket"},
		{"github.enterprise.io", "github"},
		{"totally.unknown.host", ""},
	}
	for _, tt := range tests {
		p := detectPlatform(tt.host)
		got := ""
		if p != nil {
			got = p.name
		}
		if got != tt.want {
			t.Errorf("detectPlatform(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func Test_detectPlatform_SelfHostedViaCISignal(t *testing.T) {
	clearCIEnv(t)
	t.Setenv("FORGEJO_ACTIONS", "true")
	// a private host matching no known SaaS domain is identified by its CI signal.
	if p := detectPlatform("git.internal.corp"); p == nil || p.name != "gitea" {
		t.Errorf("self-hosted CI signal should detect gitea, got %v", p)
	}
}

func Test_platform_resolveToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("CI_JOB_TOKEN", "ci-job")
	p := detectPlatform("gitlab.com")
	if got := p.resolveToken(""); got != "ci-job" {
		t.Errorf("resolveToken native = %q, want ci-job (falls through GITLAB_TOKEN to CI_JOB_TOKEN)", got)
	}
	if got := p.resolveToken("explicit"); got != "explicit" {
		t.Errorf("resolveToken explicit = %q, want explicit", got)
	}
}

func Test_DetectGitHost_GitHubNativeToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghtok")
	f := DetectGitHost(t.Context(), "", "https://github.com/toaweme/care.git", "")
	gh, ok := f.(*GitHub)
	if !ok {
		t.Fatalf("expected *GitHub, got %T", f)
	}
	if gh.token != "ghtok" {
		t.Errorf("token = %q, want ghtok (native env fallback)", gh.token)
	}
}

func Test_DetectGitHost_ExplicitTokenWins(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghtok")
	f := DetectGitHost(t.Context(), "", "https://github.com/toaweme/care.git", "explicit")
	gh := f.(*GitHub)
	if gh.token != "explicit" {
		t.Errorf("token = %q, want explicit (override beats env)", gh.token)
	}
}
