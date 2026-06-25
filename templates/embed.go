// Package templates ships the canonical toaweme config files (golangci, taskfile,
// gitignore, GitHub quality + release workflows, goreleaser, licenses) as an
// embedded filesystem so the `mend setup` subcommands can write them into target
// repos without any network dependency. Workflow templates live under
// `.github/workflows/`; the repo kind is encoded in the filename (`*.library.yml`
// / `*.binary.yml`). The setup file matrix maps each source to its destination
// path (e.g. `release.binary.yml` -> `.github/workflows/release.yml`).
package templates

import "embed"

// FS is the embedded filesystem of canonical toaweme config and workflow templates.
//
//go:embed .golangci.yml gitignore .github/workflows licenses
var FS embed.FS
