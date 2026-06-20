package sync

import "strings"

// kind is how a resolved source's bytes are obtained: read from disk, taken from
// the embedded templates, or fetched from a remote URL.
type kind int

const (
	kindRemote kind = iota // zero value: a github/gist URL fetched over http
	kindLocal              // a filesystem path read with os.ReadFile
	kindEmbed              // an embedded template, bytes already in hand
)

// Source is a resolved sync source. It is provider-agnostic - the host-specific
// work happens in a Provider, so adding gitlab/gitea/bitbucket later needs no
// change here or in the fetcher.
type Source struct {
	// Provider names the resolver that produced this source ("local", "embed",
	// "github", "gist"), for display and future per-provider auth.
	Provider string
	// Raw is the original spec, kept for error messages.
	Raw string

	kind kind
	// url is the raw URL to GET for a remote source; empty when a shorthand named a
	// repo but no file, until WithFile completes it.
	url string
	// fillPrefix completes a file-less remote repo: url = fillPrefix + filename.
	fillPrefix string
	// path is the filesystem path for a local source.
	path string
	// name and data carry an embedded template's identity and bytes.
	name string
	data []byte
}

// URL is the raw URL the fetcher GETs (remote sources only).
func (s Source) URL() string { return s.url }

// HasFile reports whether the source points at concrete content. Local and embed
// sources always do; a bare remote repo does not until WithFile fills the name.
func (s Source) HasFile() bool {
	if s.kind == kindRemote {
		return s.url != ""
	}
	return true
}

// WithFile completes a file-less remote repo by appending the filename to its
// repo root. It is a no-op for any source that already points at a file.
func (s Source) WithFile(name string) Source {
	if s.kind == kindRemote && s.url == "" && s.fillPrefix != "" {
		s.url = s.fillPrefix + name
	}
	return s
}

// String renders the source for output.
func (s Source) String() string {
	switch s.kind {
	case kindLocal:
		return s.path
	case kindEmbed:
		return s.name + " (embedded)"
	case kindRemote:
		switch {
		case s.url != "":
			return s.url
		case s.fillPrefix != "":
			return s.fillPrefix + "<file>"
		}
	}
	return s.Raw
}

// Provider resolves a source spec into a Source. The engine runs the providers in
// order - local filesystem, embedded templates, then the remote host families -
// and a provider returns false to mean "not mine, try the next one".
type Provider interface {
	Name() string
	Resolve(spec string) (Source, bool, error)
}

func stripScheme(s string) string {
	if i := strings.Index(s, "://"); i >= 0 {
		return s[i+3:]
	}
	return s
}

func ensureScheme(s string) string {
	if strings.Contains(s, "://") {
		return s
	}
	return "https://" + s
}

func splitClean(s string) []string {
	raw := strings.Split(strings.Trim(s, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
