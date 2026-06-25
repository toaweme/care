# mend

[![Quality](https://github.com/toaweme/mend/actions/workflows/tests.yml/badge.svg)](https://github.com/toaweme/mend/actions/workflows/tests.yml)
<a href="https://code.toawe.me/toaweme/mend/health">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://code.toawe.me/toaweme/mend/badge-dark.svg">
        <source media="(prefers-color-scheme: light)" srcset="https://code.toawe.me/toaweme/mend/badge.svg">
        <img alt="mend health" src="https://code.toawe.me/toaweme/mend/badge.svg">
    </picture>
</a>
[![GitHub Tag](https://img.shields.io/github/v/tag/toaweme/mend?label=Tag&color=green)](https://github.com/toaweme/mend/releases)
[![License](https://img.shields.io/badge/License-MIT-blue)](/LICENSE)

## Code and repo health

`mend` runs every quality, security, dependency, and test check for a repository.
It's a shortcut for developers working in multi-repo, cross-language ecosystem scenarios.

Switching from language to language, maintaining good standards across everything we touch is hard.
Maintaining multiple Go or TS, Python, Rust or PHP, or whatever projects is difficult.

Mend CLI is a guide and a helper to conform to each ecosystem's best practices.

```shell
mend
```

```shell
github.com/toaweme/mend  │  C 78/100 needs-attention  │  7 passed, 2 failed, 1 skipped  │  3.2s  │  6 tools
  main · a4d4a5a · 18 commits · dirty +235 -0 · touched 3m ago
✓ build            compiles
✓ dependencies     tidy, no replace directives
✓ docs             84% documented (309/366, 57 undocumented)
✓ runtime          declared 1.25.0 · code 1.22 · deps 1.25.0
✓ secrets          0 secrets
✓ tests            161 tests, 38.5% coverage, 7 untested
✓ vulnerabilities  0 vulnerabilities (+12 in go toolchain)
○ benchmarks       skipped: not applicable
✗ lint             1 issue
  cmd/mend/output/report_json.go:138:12  G306: Expect WriteFile permissions to be 0600 or less (gosec)
✗ version control  1 uncommitted (+235 -0)
  untracked  README.md  +235 -0  3m ago
```

One command, every check, one grade. Run it before you push, in CI, or on a timer behind a dashboard.

## Install

```sh
go install github.com/toaweme/mend/cmd/mend@latest
```

`mend` shells out to a handful of tools (`golangci-lint`, `govulncheck`, `betterleaks`, plus `go`/`gofmt` from your
toolchain). With `auto_install: true` (the default) it provisions any missing binary the moment a check needs it, via
`brew` or `go install`; pin or disable individual tools in config.

## Commands

A command is a verb; modes are flags (no flag-per-capability). Bare `mend` is `mend status` with everything on.

### `mend status` or just `mend`

Run the selected checks against the current repo and render the result.

- `--json`/`-j` emits the report as JSON to stdout
- `--output`/`-o <file>` writes the JSON report to a file instead.
- `--amend`/`-a` is a fast one-shot refresh of just the working-tree state, merged into the `--output` file
  (~36x faster than a full `mend status` run) for an external watcher, cron, or dashboard to poll. 

### `mend get`

Sync a canonical config file into the current repo, from a bundled preset or any remote source. It decouples
*which file goes where* from *where the bytes come from*.

```sh
mend get lint                       # write the canonical .golangci.yml (module prefix expanded)
mend get lint -f owner/repo         # sync .golangci.yml from a repo, verbatim
mend get --from owner/repo/path/x.yml --out config/x.yml   # pull any file
mend get lint --force               # overwrite an existing, governed config
```

Sources resolve in this order:
- **local file** (`./`, `~`, `file://`, or any existing path) 
- **bundled template** (a bare name matching an embedded mend config)
- **remote** (a real `github.com`/`raw.githubusercontent.com`/gist URL, or the`owner/repo[/path]` shorthand). 
 
Local and embedded sources are zero-network, a remote fetch is an explicit.

Set Github repo token via `--token`/`-t`, `env:GITHUB_TOKEN`

## Checklist

Each check runs only where it applies and skips itself otherwise (no benchmarks in a repo with none, nothing Go in a
non-Go repo).

| Check            | What it does                                                                                       |
|------------------|---------------------------------------------------------------------------------------------------|
| Version control  | Uncommitted files as a worklog: per-file line delta + relative age, ordered most-recently-touched  |
| Build            | `go build ./...`, compiler diagnostics parsed and located; any error fails                         |
| Quality          | Golangci-lint when a `.golangci.*` governs the repo, else a `go vet` + `gofmt -l` fallback         |
| Dependencies     | `go mod tidy` delta + replace directives + `go mod verify`; the runtime floor the graph forces     |
| Runtime          | Compares the declared Go version against what the code needs and what deps force (informational)    |
| Docs             | Exported-symbol doc-comment coverage via `go/ast`; warns below a configurable threshold             |
| Tests            | One `go test ./... -json` per profile: per-package/file/function coverage, untested pkgs, slowest   |
| Benchmark        | `go test -bench` (skipped instantly when the repo has no `func Benchmark`)                          |
| Secrets          | Betterleaks over the working tree and (optionally) git history                                      |
| Vulnerabilities  | Govulncheck, called-only findings, categorized `deps`/`code`/`runtime` so toolchain CVEs don't fail |

Every check rolls up into one grade. Each result is weighted, then critical failures cap the score no matter how
green everything else is: a committed secret caps you at F, a reachable vulnerability at C. The result is a single
score out of 100, an `A+..F` letter, and a plain `healthy / needs-attention / failing` verdict. Weights and caps are
yours to retune.

## Why use mend?

- **One score for a whole repo.** Build, lint, deps, runtime, docs, tests, benchmarks, secrets, and vulnerabilities,
  all in one command and one grade. No remembering which tool checks what.
- **Output you can build on.** `--json` emits the full report with the numbers that matter (coverage, vulns, secrets,
  issues, tests) lifted to the top. Ingest it into [codeviewer](https://github.com/toaweme/codeviewer), a dashboard,
  a status badge, or any other tooling, the format is stable and meant to be consumed.
- **Fast refresh for live tracking.** `--amend` re-checks just the working tree and reuses the last heavy run, about
  36x faster than a full pass, so a watcher or status bar can poll it cheaply.
- **No noise from things you can't fix.** Vulnerability findings are filtered to code you actually call, and a CVE
  that lives in the Go toolchain is shown but never drags your grade down.
- **Tells you the real Go version you need.** mend reads your code to work out the minimum Go version it actually
  requires, so you know whether your `go.mod` directive can come down.
- **Drop in best-practice configs.** `mend get` writes canonical linter, release, taskfile, CI, and license files
  into a repo, bundled and offline by default, or synced from a URL when you ask.
- **Configure once, or not at all.** Sensible defaults out of the box; layer `~/.mend/mend.yml` and `./mend.yml` to
  pin tools, turn off checks, or retune the grade. Everything optional.
- **Built to grow beyond Go.** The check engine is language-agnostic; Go is the first ecosystem, with more to come.

> With [Go Report Card](https://goreportcard.com) winding down, coincidentally mend works as a local alternative that runs on your
> own machine.

## Hosted code and health reports

Mend's --json output can be ingested by any 3rd party tooling including our <a href="https://code.toawe.me">code viewer</a>, which also hosts our badges and cards.

Public availability soon.

<p align="center">
  <a href="https://code.toawe.me/toaweme/mend/health"><picture><source media="(prefers-color-scheme: dark)" srcset="https://code.toawe.me/toaweme/mend/card.svg"><source media="(prefers-color-scheme: light)" srcset="https://code.toawe.me/toaweme/mend/card-light.svg"><img alt="mend health" src="https://code.toawe.me/toaweme/mend/card-light.svg" width="48%"></picture></a>
  <a href="https://code.toawe.me/toaweme/mend/code"><picture><source media="(prefers-color-scheme: dark)" srcset="https://code.toawe.me/toaweme/mend/code-card.svg"><source media="(prefers-color-scheme: light)" srcset="https://code.toawe.me/toaweme/mend/code-card-light.svg"><img alt="mend code" src="https://code.toawe.me/toaweme/mend/code-card-light.svg" width="48%"></picture></a>
</p>

---

Made with ❤️ in Lithuania 🇱🇹.