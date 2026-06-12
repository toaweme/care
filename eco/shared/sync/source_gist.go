package sync

import "strings"

const (
	providerGist = "gist"
	rawGistHost  = "gist.githubusercontent.com"
)

// gistProvider resolves a gist URL to a raw, fetchable URL.
type gistProvider struct{}

var _ Provider = gistProvider{}

func (gistProvider) Name() string { return providerGist }

func (gistProvider) Resolve(spec string) (Source, bool, error) {
	if !strings.Contains(spec, "gist.github.com/") && !strings.Contains(spec, rawGistHost+"/") {
		return Source{}, false, nil
	}
	u := ensureScheme(spec)
	// a gist.github.com web URL points at the gist page; its raw counterpart on
	// gist.githubusercontent.com serves the file. A gist.githubusercontent.com URL
	// is already raw - fetch it verbatim.
	if strings.Contains(u, "gist.github.com/") {
		u = strings.Replace(u, "gist.github.com/", rawGistHost+"/", 1)
		if !strings.Contains(u, "/raw") {
			u = strings.TrimRight(u, "/") + "/raw"
		}
	}
	return Source{Provider: providerGist, url: u}, true, nil
}
