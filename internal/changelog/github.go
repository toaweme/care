package changelog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	thttp "github.com/toaweme/http"
)

// GitHub is the GitHub implementation of GitHost. It enriches a tag range with
// author handles via one compare API call, forms the web compare link, and
// detects first-time contributors. Every call degrades to an error the engine
// catches (falling back to the git-log path) rather than panicking.
//
// It is host-aware: github.com uses api.github.com, and a GitHub Enterprise host
// uses that host's /api/v3 base, so a self-hosted instance works out of the box.
type GitHub struct {
	client thttp.Client
	host   string
	owner  string
	repo   string
	token  string
}

var _ GitHost = (*GitHub)(nil)

// NewGitHub builds a GitHub host for a remote. token may be empty for public
// repos; a token raises the rate limit and reaches private repos.
func NewGitHub(client thttp.Client, remote Remote, token string) *GitHub {
	host := remote.Host
	if host == "" {
		host = "github.com"
	}
	return &GitHub{client: client, host: host, owner: remote.Owner, repo: remote.Repo, token: token}
}

// apiBase is the REST API root: api.github.com for github.com, else the
// Enterprise host's /api/v3 mount.
func (g *GitHub) apiBase() string {
	if g.host == "github.com" {
		return "https://api.github.com"
	}
	return "https://" + g.host + "/api/v3"
}

// CompareCommits returns the commits in (from, to] with their GitHub author
// handles, via a single compare call. For a first release (from empty) it falls
// back to listing commits reachable from to.
func (g *GitHub) CompareCommits(ctx context.Context, from, to string) ([]Commit, error) {
	if from == "" {
		return g.listCommits(ctx, to)
	}
	var payload struct {
		Commits []ghCommit `json:"commits"`
	}
	url := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", g.apiBase(), g.owner, g.repo, from, to)
	if err := g.getJSON(ctx, url, &payload); err != nil {
		return nil, err
	}
	return toCommits(payload.Commits), nil
}

// CompareURL returns the web compare link, or "" when there is no prior tag to
// diff against (the first release has nothing to compare to).
func (g *GitHub) CompareURL(from, to string) string {
	if from == "" || to == "" {
		return ""
	}
	return fmt.Sprintf("https://%s/%s/%s/compare/%s...%s", g.host, g.owner, g.repo, from, to)
}

// webBase is the repository's web root, https://host/owner/repo, the prefix for
// commit and pull-request links.
func (g *GitHub) webBase() string {
	return fmt.Sprintf("https://%s/%s/%s", g.host, g.owner, g.repo)
}

// CommitURL returns the web link to a commit by hash, or "" when the hash is
// empty.
func (g *GitHub) CommitURL(hash string) string {
	if hash == "" {
		return ""
	}
	return g.webBase() + "/commit/" + hash
}

// PRURL returns the web link to a pull request by number, or "" when the number
// is empty.
func (g *GitHub) PRURL(number string) string {
	if number == "" {
		return ""
	}
	return g.webBase() + "/pull/" + number
}

// UserURL returns the web link to a user's profile by handle, or "" when the
// handle is empty. GitHub profiles live at the host root, not under owner/repo.
func (g *GitHub) UserURL(handle string) string {
	if handle == "" {
		return ""
	}
	return "https://" + g.host + "/" + handle
}

// NewContributors returns the handles whose first commit to the repo falls in
// (from, to]. For each author in the range it asks whether they committed up to
// (and including) from; those with none are first-timers. A first release (from
// empty) reports no new-contributor extra, matching gh's behavior of omitting it
// when there is no prior baseline.
func (g *GitHub) NewContributors(ctx context.Context, from, to string) ([]string, error) {
	if from == "" {
		return nil, nil
	}
	commits, err := g.CompareCommits(ctx, from, to)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var newcomers []string
	for _, c := range commits {
		if c.Handle == "" || seen[c.Handle] {
			continue
		}
		seen[c.Handle] = true
		prior, err := g.hasPriorCommit(ctx, c.Handle, from)
		if err != nil {
			return nil, err
		}
		if !prior {
			newcomers = append(newcomers, c.Handle)
		}
	}
	return newcomers, nil
}

func (g *GitHub) listCommits(ctx context.Context, ref string) ([]Commit, error) {
	var payload []ghCommit
	url := fmt.Sprintf("%s/repos/%s/%s/commits?sha=%s&per_page=100", g.apiBase(), g.owner, g.repo, ref)
	if err := g.getJSON(ctx, url, &payload); err != nil {
		return nil, err
	}
	return toCommits(payload), nil
}

// hasPriorCommit reports whether handle authored any commit reachable from the
// from ref (i.e. before this range), which disqualifies them as a first-time
// contributor.
func (g *GitHub) hasPriorCommit(ctx context.Context, handle, from string) (bool, error) {
	var payload []ghCommit
	url := fmt.Sprintf("%s/repos/%s/%s/commits?sha=%s&author=%s&per_page=1", g.apiBase(), g.owner, g.repo, from, handle)
	if err := g.getJSON(ctx, url, &payload); err != nil {
		return false, err
	}
	return len(payload) > 0, nil
}

func (g *GitHub) getJSON(ctx context.Context, url string, out any) error {
	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	if g.token != "" {
		headers["Authorization"] = "Bearer " + g.token
	}
	resp, err := g.client.Get(ctx, thttp.GetRequest{Request: thttp.Request{Path: url, Headers: headers}})
	if err != nil {
		return fmt.Errorf("failed to call GitHub API %q: %w", url, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %q returned status %d", url, resp.StatusCode)
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("failed to decode GitHub API response from %q: %w", url, err)
	}
	return nil
}

// ghCommit is the subset of the GitHub commit payload we read.
type ghCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

func toCommits(in []ghCommit) []Commit {
	commits := make([]Commit, 0, len(in))
	for _, c := range in {
		subject := c.Commit.Message
		if i := strings.IndexByte(subject, '\n'); i >= 0 {
			subject = subject[:i]
		}
		commit := Commit{
			Hash:    c.SHA,
			Subject: strings.TrimSpace(subject),
			Author:  c.Commit.Author.Name,
			Handle:  c.Author.Login,
		}
		Parse(&commit)
		commits = append(commits, commit)
	}
	// the compare API returns oldest-first; the git backend and renderer expect
	// newest-first, so reverse to match.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits
}
