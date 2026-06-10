// Package templates ships the canonical toaweme config files (golangci, taskfile,
// gitignore, GitHub workflows, licenses) as an embedded filesystem so the `mend
// setup` subcommands can write them into target repos without any network
// dependency.
package templates

import "embed"

//go:embed .golangci.yml gitignore taskfile.library.yml taskfile.binary.yml taskfile.run.go.yml taskfile.test.go.yml .github/workflows licenses
var FS embed.FS
