# Changelog

All notable changes to this project are documented here, newest first.

Entries are generated from [Conventional Commits](https://www.conventionalcommits.org)
and grouped by change type. This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-06-28

### Features

- Add reference-link footer and unreleased section to changelog by [@iberflow](https://github.com/iberflow) in [4394f76](https://github.com/toaweme/care/commit/4394f760fe7e16c530f1cc3a1601f6851ba0d9c9).

### Fixes

- Stop goreleaser dropping care's release notes by [@iberflow](https://github.com/iberflow) in [1d08f31](https://github.com/toaweme/care/commit/1d08f3186cd84c09a532b1a5a216db7ca68f0958).

### Documentation

- Add v0.3.0 release notes by [@iberflow](https://github.com/iberflow) in [bff4e90](https://github.com/toaweme/care/commit/bff4e90bf5138f72d108e791e3c4ec5e708f621a).

### Refactors

- Tidy up changelog package by [@iberflow](https://github.com/iberflow) in [8af6e51](https://github.com/toaweme/care/commit/8af6e5177539963636184db2a7a4bfa73ec98f40).
- Rewrite doc comments in stdlib style by [@iberflow](https://github.com/iberflow) in [39df098](https://github.com/toaweme/care/commit/39df098670f580859adc4791a9191615af2c9ace).

### CI & Build

- Log release notes and guard against empty release body by [@iberflow](https://github.com/iberflow) in [dd73b65](https://github.com/toaweme/care/commit/dd73b65248cf001a61f277e58522bf0871ca5e47).

## [0.2.0] - 2026-06-28

### Features

- Add changelog command for conventional-commit release notes by [@iberflow](https://github.com/iberflow) in [746c3da](https://github.com/toaweme/care/commit/746c3daaab295d1939b6ea5e1dd68b294aa8747c).
- Install tools from verified release downloads by [@iberflow](https://github.com/iberflow) in [13da7fa](https://github.com/toaweme/care/commit/13da7fa3b186237f9b8e17e91234e4bb549457ee).

### Documentation

- Add v0.2.0 release notes by [@iberflow](https://github.com/iberflow) in [7c3fb96](https://github.com/toaweme/care/commit/7c3fb96605fe631052ea0a728eab31cede69a6ac).

### Chores & Other

- Bump deps by [@iberflow](https://github.com/iberflow) in [52da119](https://github.com/toaweme/care/commit/52da119e1ecdbe22927df0f09c7a4c91f6553056).

## [0.1.0] - 2026-06-26

### Features

- Core check model and parallel runner by [@iberflow](https://github.com/iberflow) in [141da96](https://github.com/toaweme/care/commit/141da960fb8f7ed6b8332f57e976800ef942f927).
- Language-agnostic features (version control, secrets) by [@iberflow](https://github.com/iberflow) in [4637c6b](https://github.com/toaweme/care/commit/4637c6b3b94d06961ef42c68c1e4797e4bb15ace).
- Go ecosystem checks by [@iberflow](https://github.com/iberflow) in [e47311a](https://github.com/toaweme/care/commit/e47311a52a450fa75c9c86125588f1dbe51b2b3b).
- Devops helpers and health rating by [@iberflow](https://github.com/iberflow) in [e299d44](https://github.com/toaweme/care/commit/e299d4429550c94a324237a88e1bf36f44b2b802).
- Bundled project templates by [@iberflow](https://github.com/iberflow) in [0c74ca4](https://github.com/toaweme/care/commit/0c74ca4c6f9a39dd06e459e2097d091a179bfcf6).
- Mend status command and output rendering by [@iberflow](https://github.com/iberflow) in [2b2dc75](https://github.com/toaweme/care/commit/2b2dc755be84051bb166e5e803cc394751e9bf51).
- Capture MB/s and custom ReportMetric benchmark columns + updated golangci config by [@iberflow](https://github.com/iberflow) in [d6748c9](https://github.com/toaweme/care/commit/d6748c9f06a8b6fc88eec2fbab1635a85438711a).
- Runtime checks by [@iberflow](https://github.com/iberflow) in [5575523](https://github.com/toaweme/care/commit/55755235692e1766621bf522d1c2ecf8b524ac4d).
- Mend setup syncs files from local, embedded, and remote sources + cli bump by [@iberflow](https://github.com/iberflow) in [8a9dd84](https://github.com/toaweme/care/commit/8a9dd846d1cdfdc3debc0b21351ae9169ca1b937).
- Categorize vulnerabilities by origin so toolchain vulns don't by [@iberflow](https://github.com/iberflow) in [0437d32](https://github.com/toaweme/care/commit/0437d32b08d63b4b54a875b74c2ee9b08bd76e0b).
- Track touched_at + uncommitted line counts in repo status by [@iberflow](https://github.com/iberflow) in [e43a6d8](https://github.com/toaweme/care/commit/e43a6d8d4363ee48633b269f3fe006a0aa62c5ea).
- Use -a, --amend and -o, --output with status command to quick update the --json output for live repo stats by [@iberflow](https://github.com/iberflow) in [220c9af](https://github.com/toaweme/care/commit/220c9afe72e3091f9d52abfd6cff081806a0a679).
- Readme and license by [@iberflow](https://github.com/iberflow) in [4dba620](https://github.com/toaweme/care/commit/4dba62012dedf94dbab4706f138dea72b28b7628).
- Track git tag and full SHA in report, CI-aware ref resolution, --pretty flag by [@iberflow](https://github.com/iberflow) in [97847aa](https://github.com/toaweme/care/commit/97847aaa79cc6cc23887439f52fa5738e3fdbb60).
- Add codeviewer publish workflow template by [@iberflow](https://github.com/iberflow) in [ac79f50](https://github.com/toaweme/care/commit/ac79f506d304abcab8e71f347919cabbb12e356f).
- Add goreleaser release pipeline by [@iberflow](https://github.com/iberflow) in [6f708c0](https://github.com/toaweme/care/commit/6f708c08fbb254f18ab96261ea116537479ba5d9).
- Install instructions in readme and releases by [@iberflow](https://github.com/iberflow) in [5cb4618](https://github.com/toaweme/care/commit/5cb461801c86746e413d58cd172455296a164eef).
- Github action workflow by [@iberflow](https://github.com/iberflow) in [8ddaaac](https://github.com/toaweme/care/commit/8ddaaac9949ff48a856489521ba2e3839193d94f).

### Fixes

- Golangci output by [@iberflow](https://github.com/iberflow) in [2cf8001](https://github.com/toaweme/care/commit/2cf80014e1b5eb9f7c48bbd26bd2728533c01d4e).
- Linter issues by [@iberflow](https://github.com/iberflow) in [ce770d0](https://github.com/toaweme/care/commit/ce770d059cd966f1398d64b1469906babf9f1eb9).
- Stdout filepath output by [@iberflow](https://github.com/iberflow) in [9032a6c](https://github.com/toaweme/care/commit/9032a6cca07e24f685723e15b781380aa5881035).
- Ci workflow by [@iberflow](https://github.com/iberflow) in [ba2572b](https://github.com/toaweme/care/commit/ba2572b4804f5f14d551216059ffd9ccc91e2e70).
- Readme by [@iberflow](https://github.com/iberflow) in [89e57cf](https://github.com/toaweme/care/commit/89e57cfe0de2a12b99c4a9d7a19180ba6f11720f).
- **Secrets:** Skip gitignored files in working-tree scan by [@iberflow](https://github.com/iberflow) in [d07ebf1](https://github.com/toaweme/care/commit/d07ebf1e49da03a37be7aa4d57f13e4defd81c82).
- **Output:** Use readable mid-gray for dim text instead of palette 8 by [@iberflow](https://github.com/iberflow) in [324078b](https://github.com/toaweme/care/commit/324078b5fdbda0112fd69754e8288af22a34fac9).
- Resolve golangci-lint issues by [@iberflow](https://github.com/iberflow) in [c7fbc4f](https://github.com/toaweme/care/commit/c7fbc4fe2fc6eb65246e4914a79d8b3781ab27ef).

### Refactors

- Status command flags by [@iberflow](https://github.com/iberflow) in [8482654](https://github.com/toaweme/care/commit/8482654a9ddfcfade5de2c732291c3acc0b24c13).
- Rename project to have better odds of being friends with mend.io by [@iberflow](https://github.com/iberflow) in [e86f530](https://github.com/toaweme/care/commit/e86f53069671ef134c778bb1b5040ea33bc897f4).

### CI & Build

- Exclude v0.0.0 base tag from triggering a release by [@iberflow](https://github.com/iberflow) in [b2dfd02](https://github.com/toaweme/care/commit/b2dfd025c23a600ddde209f2c3931e22ecb93928).
- Drop v0.0.0 base-tag exclusion now that the seed tag is removed by [@iberflow](https://github.com/iberflow) in [b7ee587](https://github.com/toaweme/care/commit/b7ee587d0bbe7f2f30b91c1e19424c056226357c).
- Speed up lint, dedupe tag runs by [@iberflow](https://github.com/iberflow) in [6a6ab74](https://github.com/toaweme/care/commit/6a6ab74718f175f3262b56d69f680a1178945483).

### Chores & Other

- Simplify things and tune outputs + initial ci stuff by [@iberflow](https://github.com/iberflow) in [bd730e2](https://github.com/toaweme/care/commit/bd730e27dba244a306a6b9bbb498a693484cdca8).
- Tidy up by [@iberflow](https://github.com/iberflow) in [c6d9cd1](https://github.com/toaweme/care/commit/c6d9cd1cd8ce573716e0f8a16a60f52ee68ae6f0).
- Bump deps by [@iberflow](https://github.com/iberflow) in [39dff27](https://github.com/toaweme/care/commit/39dff27c3d24f35cf44ad61edcbc64d798602dc8).
- Tidy up by [@iberflow](https://github.com/iberflow) in [a4d4a5a](https://github.com/toaweme/care/commit/a4d4a5a000d0a8b4d7786cf5118d538ee098b87e).
- Fix readme typo by [@iberflow](https://github.com/iberflow) in [863b412](https://github.com/toaweme/care/commit/863b412ec1b28b5f51da75308645c4aac03e9660).
- Cleanup by [@iberflow](https://github.com/iberflow) in [6ec5fe1](https://github.com/toaweme/care/commit/6ec5fe15d39a9332cfeb4ce780362c5ccc63ec0a).
- Pin workflow actions to SHAs and add dependabot by [@iberflow](https://github.com/iberflow) in [6819313](https://github.com/toaweme/care/commit/6819313a5ebadf45edfbd146609908fc771cbb55).
- Pin goreleaser to v2 line, write formula to Formula/ by [@iberflow](https://github.com/iberflow) in [bc77248](https://github.com/toaweme/care/commit/bc77248405c1ee8e2e9404e3d382329255ead506).
- Changelog shows @username only and keeps all commit types by [@iberflow](https://github.com/iberflow) in [7ba0f03](https://github.com/toaweme/care/commit/7ba0f0315693562ea38feef4cdedfb5be3109981).

[0.3.0]: https://github.com/toaweme/care/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/toaweme/care/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/toaweme/care/releases/tag/v0.1.0
