package sync

const providerEmbed = "embed"

// EmbedFunc reads an embedded template by name, returning an error when the name
// is not embedded. It lets the embed provider stay decoupled from the templates
// package.
type EmbedFunc func(name string) ([]byte, error)

// embedProvider resolves a bare name that matches a bundled template. It runs
// after the local filesystem (an on-disk file of the same name wins) and before
// the remote providers (the name shadows a remote shorthand).
type embedProvider struct{ read EmbedFunc }

var _ Provider = embedProvider{}

func (embedProvider) Name() string { return providerEmbed }

func (p embedProvider) Resolve(spec string) (Source, bool, error) {
	if p.read == nil {
		return Source{}, false, nil
	}
	data, err := p.read(spec)
	if err != nil {
		// not an embedded template; let the remote providers try.
		return Source{}, false, nil //nolint:nilerr // a read miss means "not mine", signaled by ok=false, not an error
	}
	return Source{Provider: providerEmbed, kind: kindEmbed, name: spec, data: data}, true, nil
}
