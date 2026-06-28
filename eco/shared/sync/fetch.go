// Package sync resolves file sources (local, embedded, remote) and writes them
// into the working tree, honoring an existing-file/force rule.
package sync

import (
	"context"
	"fmt"

	"github.com/toaweme/http"
)

// Fetcher resolves a Source's bytes from its raw URL. It is deliberately
// host-agnostic, doing a GET with an optional bearer credential, so the same
// code serves github, gists, and any future provider that resolves to a
// fetchable URL. Builtin sources have no URL and are resolved by the engine,
// not here.
type Fetcher interface {
	Fetch(ctx context.Context, src Source) ([]byte, error)
}

type httpFetcher struct {
	client http.Client
	token  string
}

var _ Fetcher = (*httpFetcher)(nil)

// NewFetcher builds a URL fetcher over an injected http client (its config is the
// caller's concern). The token, when set, is attached as a bearer credential so
// private sources resolve; public sources need none.
func NewFetcher(client http.Client, token string) Fetcher {
	return &httpFetcher{client: client, token: token}
}

func (f *httpFetcher) Fetch(ctx context.Context, src Source) ([]byte, error) {
	url := src.URL()
	if url == "" {
		return nil, fmt.Errorf("source %s names no file to fetch", src)
	}

	headers := map[string]string{}
	if f.token != "" {
		headers["Authorization"] = "Bearer " + f.token
	}

	resp, err := f.client.Get(ctx, http.GetRequest{Request: http.Request{Path: url, Headers: headers}})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", src, err)
	}
	if err := checkStatus(resp.StatusCode, src); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func checkStatus(code int, src Source) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == 404:
		return fmt.Errorf("source %s not found (private source without a token, or wrong path/ref)", src)
	case code == 401 || code == 403:
		return fmt.Errorf("access to %s denied (pass --token or set GITHUB_TOKEN): http %d", src, code)
	default:
		return fmt.Errorf("failed to fetch %s: http %d", src, code)
	}
}
