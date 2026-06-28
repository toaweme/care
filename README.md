# care

[![Quality](https://github.com/toaweme/care/actions/workflows/quality.yml/badge.svg)](https://github.com/toaweme/care/actions/workflows/quality.yml)
<a href="https://code.toawe.me/toaweme/care/health">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://code.toawe.me/toaweme/care/badge-dark.svg">
        <source media="(prefers-color-scheme: light)" srcset="https://code.toawe.me/toaweme/care/badge.svg">
        <img alt="care health" src="https://code.toawe.me/toaweme/care/badge.svg">
    </picture>
</a>
[![GitHub Tag](https://img.shields.io/github/v/tag/toaweme/care?label=Tag&color=green)](https://github.com/toaweme/care/releases)
[![License](https://img.shields.io/badge/License-MIT-blue)](/LICENSE)

## Code and repo health

`care` runs every quality, security, dependency, and test check for a repository.
It's a shortcut for developers working in multi-repo, cross-language ecosystem scenarios.

Switching from language to language, maintaining good standards across everything we touch is hard.

Care CLI is a guide and a helper to conform to each ecosystem's best practices. Golang for now.

```shell
care
```

```shell
github.com/toaweme/care  │  C 78/100 needs-attention  │  7 passed, 2 failed, 1 skipped  │  3.2s  │  6 tools
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
  cmd/care/output/report_json.go:138:12  G306: Expect WriteFile permissions to be 0600 or less (gosec)
✗ version control  1 uncommitted (+235 -0)
  untracked  README.md  +235 -0  3m ago
```

One command, every check, one grade. Run it before you push, in CI, or on a timer behind a dashboard.

## Install

```sh
# go
go install github.com/toaweme/care/cmd/care@latest

# homebrew
brew install toaweme/tap/care

# binary
wget -qO- https://github.com/toaweme/care/releases/download/v{v}/care_{v}_linux_x64.tar.gz | tar xz
```

Every release also lists the exact archive for each OS/arch on the
[releases page](https://github.com/toaweme/care/releases).

`care` shells out to a handful of tools (`golangci-lint`, `govulncheck`, `betterleaks`, plus `go`/`gofmt` from your
toolchain). With `auto_install: true` (the default) it provisions any missing binary the moment a check needs it, via
`brew` or `go install`; pin or disable individual tools in config.

## Commands

A command is a verb; modes are flags (no flag-per-capability). Bare `care` is `care status` with everything on.

### `care status` or just `care`

Run the selected checks against the current repo and render the result.

- `--json`/`-j` emits the report as JSON to stdout
- `--output`/`-o <file>` writes the JSON report to a file instead.
- `--pretty`/`-p` outputs to stdout. Useful in CI where we need both JSON file and the logs.
- `--amend`/`-a` is a fast one-shot refresh of just the working-tree state, merged into the `--output` file
  (~36x faster than a full `care status` run) for an external watcher, cron, or dashboard to poll. 

### `care get`

Sync a canonical config file into the current repo, from a bundled preset or any remote source. It decouples
*which file goes where* from *where the bytes come from*.

```sh
care get lint                       # write the canonical .golangci.yml (module prefix expanded)
care get lint -f owner/repo         # sync .golangci.yml from a repo, verbatim
care get --from owner/repo/path/x.yml --out config/x.yml   # pull any file
care get lint --force               # overwrite an existing, governed config
```

Sources resolve in this order:
- **local file** (`./`, `~`, `file://`, or any existing path) 
- **bundled template** (a bare name matching an embedded care config)
- **remote** (a real `github.com`/`raw.githubusercontent.com`/gist URL, or the`owner/repo[/path]` shorthand). 
 
Local and embedded sources are zero-network, a remote fetch is an explicit.

Set Github repo token via `--token`/`-t`, `env:GITHUB_TOKEN`

### `care changelog`

Derive release notes from conventional commits, the org's single source for a release body (it replaces
goreleaser's own changelog). The positional tag is the range end; `--since` sets the start.

```sh
care changelog                         # notes for the latest tag (or HEAD), since the previous tag
care changelog v1.2.0                  # notes ending at v1.2.0, since the tag before it
care changelog v1.2.0 --since v1.0.0   # explicit range
care changelog --full                  # from the first commit, ignoring tags
care changelog --write                 # maintain CHANGELOG.md in place instead of printing
```

- Prints to stdout by default (redirect to capture). `--write` maintains the Keep a Changelog file at
  `--file`/`-f` (default `./CHANGELOG.md`).
- For the natural range, a matching `## [version]` section already in `--file` is used verbatim, so hand-written
  prose reaches the release. An explicit `--since`/`--full` always re-derives from git.
- `--plain` drops commit/PR links and author attribution. Git-host extras (Full Changelog link, contributors)
  need a token via `--token`/`-t` or `GITHUB_TOKEN`/`GH_TOKEN`.

## GitHub Actions

Use the bundled action to run `care` in a repo's CI. Pin it to an exact tag:
the tag is the single source of truth, so it installs the matching `care` binary
and verifies its cosign signature and SHA-256 before running.

The action does one thing: install `care` and run `care status`. Three optional
inputs modify it - `output` writes a JSON report, `publish-url` publishes it, and
`strict` fails the job on failing checks. With none of them it just runs status and
reports to the log:

```yaml
jobs:
  quality:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@<sha>
      - uses: toaweme/care@v0.4.0   # installs care v0.4.0, runs `care status`
```

Publishing needs `id-token: write` (a GitHub OIDC token is minted with the URL's
origin as audience). Point `publish-url` at your own ingestion
engine. `strict: true` fails the step when a check fails. Omit it to report without
failing the job:

```yaml
jobs:
  quality:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write # only for publishing
    steps:
      - uses: actions/checkout@<sha>
      - uses: toaweme/care@v0.4.0
        with:
          strict: true                                  # fail the job, but keep the report
          output: report.care.json                      # write the JSON report (<name>.care.json)
          publish-url: https://ci.example.com/care      # POST the report here; omit to keep it local
```

`care` is left on `PATH`, so any other care command is just your own step, e.g.
`run: care get lint` to sync the lint config before the check.

Inputs (none are required):

| Input | Purpose | Default |
| --- | --- | --- |
| `version` | Override the binary version, only when pinning the action to a SHA or branch | Latest |
| `output` | Report file path, care's own `--output` (use a `<name>.care.json` name). A failing check still writes it rather than failing the step | - |
| `strict` | `true` fails the step when a check fails, after any report is published. `false` reports without failing | `false` |
| `publish-url` | Full URL to POST the report to; empty keeps it local. Needs `id-token: write` | - |
| `verify` | Cosign signature check | `true` |
| `dir` | Directory care runs in (care's `--cwd`), for a module in a subdirectory with its own `go.mod`. The report still lands in the workspace root | `.` |

The publish endpoint receives the report JSON as the POST body with an
`Authorization: Bearer <OIDC token>` header (token audience is the URL's origin).
A self-hosted codeviewer exposes the same path on its own host:
`https://<your-host>/ingest?kind=care`.

The report stays in the workspace even when checks fail, so a failure report is
readable without re-running care. Only the download temp dir is cleaned up. The
`report-path` output exposes the report location for later steps, so you can
upload it as an artifact yourself if you want one.

Reports stay local unless you set `publish-url` and grant `id-token: write`, in
which case the report is POSTed there (care's hosted dashboard, or your own
endpoint). With `publish-url` set but no `id-token: write`, publishing is skipped
with a warning.

Pin to an exact tag and bump it deliberately when you adopt a new release.

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

## Why use care?

- **One score for a whole repo.** Build, lint, deps, runtime, docs, tests, benchmarks, secrets, and vulnerabilities,
  all in one command and one grade. No remembering which tool checks what.
- **Output you can build on.** `--json` emits the full report with the numbers that matter (coverage, vulns, secrets,
  issues, tests) lifted to the top. Ingest it into [codeviewer](https://github.com/toaweme/codeviewer), a dashboard,
  a status badge, or any other tooling, the format is stable and meant to be consumed.
- **Fast refresh for live tracking.** `--amend` re-checks just the working tree and reuses the last heavy run, about
  36x faster than a full pass, so a watcher or status bar can poll it cheaply.
- **No noise from things you can't fix.** Vulnerability findings are filtered to code you actually call, and a CVE
  that lives in the Go toolchain is shown but never drags your grade down.
- **Tells you the real Go version you need.** care reads your code to work out the minimum Go version it actually
  requires, so you know whether your `go.mod` directive can come down.
- **Drop in best-practice configs.** `care get` writes canonical linter, release, taskfile, CI, and license files
  into a repo, bundled and offline by default, or synced from a URL when you ask.
- **Configure once, or not at all.** Sensible defaults out of the box; layer `~/.care/care.yml` and `./care.yml` to
  pin tools, turn off checks, or retune the grade. Everything optional.
- **Built to grow beyond Go.** The check engine is language-agnostic; Go is the first ecosystem, with more to come.

> With [Go Report Card](https://goreportcard.com) winding down, coincidentally care works as a local alternative that runs on your
> own machine.

## Hosted code and health reports

Care's --json output can be ingested by any 3rd party tooling including our <a href="https://code.toawe.me">code viewer</a>, which also hosts our badges and cards.

Public availability soon.

<p align="center">
  <a href="https://code.toawe.me/toaweme/care/health"><picture><source media="(prefers-color-scheme: dark)" srcset="https://code.toawe.me/toaweme/care/card.svg"><source media="(prefers-color-scheme: light)" srcset="https://code.toawe.me/toaweme/care/card-light.svg"><img alt="care health" src="https://code.toawe.me/toaweme/care/card-light.svg" width="48%"></picture></a>
  <a href="https://code.toawe.me/toaweme/care/code"><picture><source media="(prefers-color-scheme: dark)" srcset="https://code.toawe.me/toaweme/care/code-card.svg"><source media="(prefers-color-scheme: light)" srcset="https://code.toawe.me/toaweme/care/code-card-light.svg"><img alt="care code" src="https://code.toawe.me/toaweme/care/code-card-light.svg" width="48%"></picture></a>
</p>

---

Made with ❤️ in Lithuania 🇱🇹.