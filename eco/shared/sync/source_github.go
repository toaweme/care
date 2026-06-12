package sync

import (
	"fmt"
	"strings"
)

const (
	providerGitHub = "github"
	rawGitHubHost  = "raw.githubusercontent.com"
	rawHostBase    = "https://" + rawGitHubHost + "/"
)

// githubProvider resolves github file URLs (blob/raw/raw.githubusercontent) and
// the host-less owner/repo[/path] shorthand to a raw, fetchable URL. It is the
// catch-all, so it runs last in the chain.
type githubProvider struct{}

var _ Provider = githubProvider{}

func (githubProvider) Name() string { return providerGitHub }

func (githubProvider) Resolve(spec string) (Source, bool, error) {
	body := stripScheme(spec)
	switch {
	case strings.HasPrefix(body, rawGitHubHost+"/"):
		return githubFromRaw(strings.TrimPrefix(body, rawGitHubHost+"/"))
	case strings.HasPrefix(body, "github.com/"):
		return githubFromWeb(strings.TrimPrefix(body, "github.com/"))
	default:
		// a scheme-qualified foreign host is not ours; let it fall through to a
		// clean "unrecognized" rather than mis-parsing it as a shorthand.
		if strings.Contains(spec, "://") {
			return Source{}, false, nil
		}
		return githubShorthand(body)
	}
}

// githubFromRaw resolves an already-raw URL tail (O/R/<ref>/<path>).
func githubFromRaw(rest string) (Source, bool, error) {
	parts := splitClean(rest)
	if len(parts) < 2 {
		return Source{}, false, fmt.Errorf("github raw url must name at least owner/repo")
	}
	if len(parts) == 2 {
		return githubBareRepo(parts[0], parts[1]), true, nil
	}
	return Source{Provider: providerGitHub, url: rawHostBase + strings.Join(parts, "/")}, true, nil
}

// githubFromWeb resolves a github.com web URL tail after the host.
func githubFromWeb(rest string) (Source, bool, error) {
	parts := splitClean(rest)
	if len(parts) < 2 {
		return Source{}, false, fmt.Errorf("github url must name at least owner/repo")
	}
	owner, repo := parts[0], parts[1]
	if len(parts) == 2 {
		return githubBareRepo(owner, repo), true, nil
	}
	// /blob/<ref>/<path> (web view) and /raw/<ref>/<path> (raw redirect) both map to
	// raw.githubusercontent.com/O/R/<ref>/<path>; the ref+path tail passes through
	// verbatim, so a multi-segment ref like refs/heads/main needs no special casing.
	switch parts[2] {
	case "blob", "raw":
		if len(parts) < 4 {
			return Source{}, false, fmt.Errorf("github url %q names no file path", rest)
		}
		return Source{Provider: providerGitHub, url: rawURL(owner, repo, strings.Join(parts[3:], "/"))}, true, nil
	default:
		return Source{}, false, fmt.Errorf("unsupported github url %q (expected a /blob/ or /raw/ file url)", rest)
	}
}

// githubShorthand resolves the host-less owner/repo[/path] form against the
// default branch (HEAD). A first segment with a dot is a host, not an owner, so it
// is declined for a future provider to claim.
func githubShorthand(body string) (Source, bool, error) {
	parts := splitClean(body)
	if len(parts) < 2 {
		return Source{}, false, fmt.Errorf("source %q must name at least owner/repo", body)
	}
	if strings.Contains(parts[0], ".") {
		return Source{}, false, nil
	}
	owner, repo := parts[0], parts[1]
	if len(parts) == 2 {
		return githubBareRepo(owner, repo), true, nil
	}
	return Source{Provider: providerGitHub, url: rawURL(owner, repo, "HEAD/"+strings.Join(parts[2:], "/"))}, true, nil
}

func githubBareRepo(owner, repo string) Source {
	return Source{Provider: providerGitHub, fillPrefix: rawURL(owner, repo, "HEAD/")}
}

func rawURL(owner, repo, tail string) string {
	return rawHostBase + owner + "/" + repo + "/" + tail
}
