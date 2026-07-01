// Package templates ships the canonical toaweme config files (golangci,
// gitignore, the GitHub quality workflow) as an embedded filesystem so
// `care get` can write them into target repos without any network dependency.
// `.github/workflows/quality.yml` is the org's single CI gate: a Go compat
// matrix plus the `toaweme/care` action, pinned to the release this binary
// shipped with.
package templates

import "embed"

// FS is the embedded filesystem of canonical toaweme config and workflow templates.
//
//go:embed .golangci.yml gitignore .github/workflows licenses
var FS embed.FS
