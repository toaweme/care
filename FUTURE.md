# Future

Design notes for things not built yet. Each section is self-contained.

## Report card badge (goreportcard replacement)

goreportcard.com is being sunset. Many popular Go repos depend on it. mend can
replace it, but the goal is **near-zero infra cost for us and no server-side tool
execution** - the thing that made goreportcard expensive and killed it.

### Core insight

The grade is a deterministic function of the repo at a commit, and mend already
computes it client-side (local or CI). So we never run tools on our infra. The
only open question is *where the grade value lives* so a badge renderer can read
it. Four options, three are bad:

- Render at read time -> that is goreportcard, requires us to run tools. Dead.
- Store on our server -> reintroduces storage, submit endpoint, auth. Avoid.
- Commit `badge.json` to the repo -> spams commit history, needs `contents:
  write`, can trigger CI loops. Widely disliked. Avoid.
- Store in the user's own GitHub-hosted storage (NOT main branch) -> the good
  one. This is what real projects do.

Storing the value in the user's GitHub storage also gives anti-forgery for free:
only people who can write that storage (push to the repo / own the gist) can
change the badge. No signing keys, no Logto, no OIDC needed. A stranger cannot
forge your badge; you can inflate your own, but that is unenforceable for free
anyway (and stays *verifiable*: `mend verify` re-runs at the commit and compares,
since the report is deterministic).

### Default flow: Gist (de-facto standard)

This is what most dynamic/coverage badges already do (e.g.
`Schneegans/dynamic-badges-action`). No commit to the repo at all.

1. mend runs in CI, computes the grade client-side.
2. mend emits a shields endpoint JSON:
   `{schemaVersion:1, label:"mend", message:"A+", color:"brightgreen"}`.
3. CI writes that JSON to a Gist (one-time setup: a gist id + a PAT secret).
4. README badge points shields.io at the gist raw URL:
   `img.shields.io/endpoint?url=<gist raw url>`.

shields renders the SVG, GitHub hosts the JSON, the user's CI minutes do the
compute. We host nothing and run nothing.

### Alternative flow: GitHub Pages (no PAT, no commit)

For users who do not want a PAT. The modern Pages flow
(`actions/upload-pages-artifact` + `actions/deploy-pages`) deploys from a
workflow artifact - it does **not** commit to a branch and uses the built-in
`GITHUB_TOKEN`. mend writes `badge.json`, the workflow publishes it, shields
reads `owner.github.io/repo/badge.json`.

### Optional later: branded URL

If we want `badge.mend.dev/gh/owner/repo` for parity/marketing, add a
**stateless** Cloudflare Worker that read-through-fetches the user's gist/Pages
JSON and returns it (or renders SVG), edge-cached. No storage, no auth, no
signing. Still effectively $0. Ship independently.

### shields.io is optional, not a dependency

shields.io is free and open-source (no API key for endpoint badges), but it is a
third-party we do not control - same structural risk that killed goreportcard.
So treat it as a convenience, not lock-in:

- Default to shields endpoint rendering.
- We can self-host shields (single Node container) if it degrades.
- Rendering our own SVG is ~20 lines (templated SVG + text-width math), so we can
  drop shields entirely with zero change to where the data lives.

GitHub's camo proxy caches badge images, so end users rarely feel shields
latency regardless.

### Link badges to the self-hosted webui

We are adding a **self-hosted webui** to view mend's reports and code
analytics/reading. This composes cleanly with the badge:

- The badge image (grade) stays in the user's GitHub storage (gist/Pages) as
  above - cheap, always available, no infra from us.
- The badge *links* to the user's self-hosted mend webui, where the full report,
  history, and analytics live: `[![mend](badge.svg)](https://mend.myco.com/owner/repo)`.
- Everything (compute, badge data, report hosting) lives on the user's infra.
  Our infra stays at zero. The webui is software we ship, not a service we run.
- Keeps the trust story intact: the report behind the badge is viewable and
  re-runnable on the owner's own installation; `mend verify` still reproduces it
  independently.

### mend surface

- `mend badge` - compute and emit `badge.json` (shields endpoint schema).
- `--target gist|pages|file` flag for where it lands.
- `mend badge migrate` - swap a goreportcard README line for the mend one.
- CI snippet in docs for both gist and Pages targets.
- `mend verify gh/owner/repo` - re-run at the badge's commit and compare
  (trust-by-reproduction, no infra).

### Explicitly rejected

- Logto / OIDC / any auth on our side - GitHub write access is the auth.
- Backend-held signing key + embedded public key - no forgery vector once the
  data lives in the user's repo storage, so signing buys nothing.
- KV/D1 storage + submit endpoint - not needed; data lives in user storage.
