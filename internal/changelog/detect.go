package changelog

import (
	"context"
	"regexp"

	thttp "github.com/toaweme/http"
)

// remoteRe parses the owner/repo and host out of the common git remote URL
// shapes: https://host/owner/repo(.git) and git@host:owner/repo(.git).
var remoteRe = regexp.MustCompile(`(?:https?://|git@|ssh://git@)([^/:]+)[/:]([^/]+)/(.+?)(?:\.git)?$`)

// ParseRemote extracts the host and owner/repo from a git remote URL. ok is false
// when the URL matches no known shape, signaling the caller to degrade.
func ParseRemote(url string) (Remote, bool) {
	m := remoteRe.FindStringSubmatch(url)
	if m == nil {
		return Remote{}, false
	}
	return Remote{Host: m[1], Owner: m[2], Repo: m[3]}, true
}

// DetectGitHost builds the GitHost for a repository's origin remote, or nil when no
// platform is recognized (or the recognized one has no GitHost yet), so the engine
// uses the host-neutral git-log path. The platform registry maps hosts to their
// host factory; GitLab/Gitea drop in there without touching callers. remoteURL
// overrides detection (an explicit --remote flag); it is treated as a remote URL.
//
// token is the caller's platform-agnostic override (the --token flag). When it is
// empty, the detected platform's native token env var is used, so the caller
// never has to know which one applies.
func DetectGitHost(ctx context.Context, dir, remoteURL, token string) GitHost {
	url := remoteURL
	if url == "" {
		url, _ = NewGit(dir).run(ctx, "remote", "get-url", "origin")
	}
	remote, ok := ParseRemote(url)
	if !ok {
		return nil
	}
	p := detectPlatform(remote.Host)
	if p == nil || p.build == nil {
		return nil
	}
	// an empty base URL lets the client GET the fully-qualified API URLs verbatim.
	client := thttp.NewClient(thttp.Config{UserAgent: "toaweme/care"})
	return p.build(client, remote, p.resolveToken(token))
}
