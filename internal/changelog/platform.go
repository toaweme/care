package changelog

import (
	"os"
	"strings"

	thttp "github.com/toaweme/http"
)

// platform maps a git host (and its CI) to the data care needs to enrich notes:
// the native token env vars to read when the caller passes none, the CI env
// signals that identify the platform on a self-hosted host, a host matcher, and
// the GitHost factory. A platform with a nil build is recognized (so its token
// still resolves) but has no GitHost yet, so the engine degrades to the git-log
// path until one is added.
type platform struct {
	name     string
	tokenEnv []string
	ciEnv    []string
	matches  func(host string) bool
	build    func(client thttp.Client, remote Remote, token string) GitHost
}

// platforms is the ordered registry of known git hosting platforms. Order
// matters only for the host-match fallback; CI-signal detection is exact. Adding
// GitLab/Gitea support is dropping a build func into the matching entry, no
// caller changes.
var platforms = []platform{
	{
		name:     "github",
		tokenEnv: []string{"GITHUB_TOKEN", "GH_TOKEN"},
		ciEnv:    []string{"GITHUB_ACTIONS"},
		matches:  func(host string) bool { return host == "github.com" || strings.Contains(host, "github") },
		build:    func(c thttp.Client, r Remote, token string) GitHost { return NewGitHub(c, r, token) },
	},
	{
		name:     "gitlab",
		tokenEnv: []string{"GITLAB_TOKEN", "CI_JOB_TOKEN"},
		ciEnv:    []string{"GITLAB_CI"},
		matches:  func(host string) bool { return host == "gitlab.com" || strings.Contains(host, "gitlab") },
		build:    nil,
	},
	{
		name:     "gitea",
		tokenEnv: []string{"GITEA_TOKEN", "FORGEJO_TOKEN"},
		ciEnv:    []string{"GITEA_ACTIONS", "FORGEJO_ACTIONS"},
		matches: func(host string) bool {
			return strings.Contains(host, "gitea") || strings.Contains(host, "forgejo") || host == "codeberg.org"
		},
		build: nil,
	},
	{
		name:     "bitbucket",
		tokenEnv: []string{"BITBUCKET_TOKEN", "BITBUCKET_ACCESS_TOKEN"},
		ciEnv:    []string{"BITBUCKET_BUILD_NUMBER"},
		matches:  func(host string) bool { return strings.Contains(host, "bitbucket") },
		build:    nil,
	},
}

// detectPlatform identifies a remote's platform. The remote host is the source
// of truth, so a known host wins first; only when the host matches nothing (a
// self-hosted instance on a private domain) do CI env signals identify it. This
// avoids a cross-CI false positive, e.g. running on GitHub Actions against a
// Bitbucket remote. Returns nil when no platform is recognized.
func detectPlatform(host string) *platform {
	for i := range platforms {
		if platforms[i].matches(host) {
			return &platforms[i]
		}
	}
	for i := range platforms {
		for _, env := range platforms[i].ciEnv {
			if os.Getenv(env) != "" {
				return &platforms[i]
			}
		}
	}
	return nil
}

// resolveToken returns the explicit token when set, otherwise the first non-empty
// native token env var for the platform. The explicit value always wins.
func (p *platform) resolveToken(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return firstEnv(p.tokenEnv...)
}

// firstEnv returns the value of the first set, non-empty environment variable
// from names, or "" when none are set.
func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}
