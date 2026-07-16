# Changelog

All notable changes to this project are documented here, newest first.

Entries are generated from [Conventional Commits](https://www.conventionalcommits.org)
and grouped by change type. This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.9.4] - 2026-07-16

### Features

- New demo videos by [@iberflow](https://github.com/iberflow) in [3fb7a25](https://github.com/toaweme/care/commit/3fb7a25633921a25bb47fe38a1c87ca31faf924e).

### Fixes

- Name the branch in the changelog compare link and contributor list by [@iberflow](https://github.com/iberflow) in [884d2f0](https://github.com/toaweme/care/commit/884d2f0e1702761f47e29dc4dc5f7a5e63618aea).
- Changelog author handles resolve on an unmerged branch by [@iberflow](https://github.com/iberflow) in [aecd168](https://github.com/toaweme/care/commit/aecd168e76a532e0c6a8ccf388a845971a4d1677).
- Include unmerged branch commits in changelog ranges by [@iberflow](https://github.com/iberflow) in [5ecfb46](https://github.com/toaweme/care/commit/5ecfb46d5f38ade286a215a5bc5860335992f026).
- Update pinned care action to v0.9.3 in templates/quality.yml, drop version input by [@iberflow](https://github.com/iberflow) in [5d4e1d3](https://github.com/toaweme/care/commit/5d4e1d30158258bbaf5cd04da9d24db5a9c094fc).

### Documentation

- Changelog for v0.9.3 by [@iberflow](https://github.com/iberflow) in [710c6d4](https://github.com/toaweme/care/commit/710c6d4bb74e4ea7780ac1298eeb30015b075047).

### Tests

- Cover branch, detached HEAD, and shallow-clone changelog paths by [@iberflow](https://github.com/iberflow) in [5084851](https://github.com/toaweme/care/commit/5084851594af458c1cd16913e06821a32222d850).

### CI & Build

- Publish code.json via codereport action by [@iberflow](https://github.com/iberflow) in [#3](https://github.com/toaweme/care/pull/3).
- Write CHANGELOG.md in prepare-release so releases never miss it by [@iberflow](https://github.com/iberflow) in [43b985a](https://github.com/toaweme/care/commit/43b985a272e90bbf1fa371421ae80c688a3303a0).

## [0.9.3] - 2026-07-07

### Fixes

- Refresh care get template to v0.9.2 commit pin with inline version comments by [@iberflow](https://github.com/iberflow) in [c9b9680](https://github.com/toaweme/care/commit/c9b96800f2d1bcacb0e8ff989cbeac6247811956).
- Restore (?m) so DCO matches trailers on multi-line messages by [@iberflow](https://github.com/iberflow) in [0e5fadd](https://github.com/toaweme/care/commit/0e5fadd48f86ba8870c2f42846e40aa4ff5b86fc).

### Documentation

- Add Contributing section to README by [@iberflow](https://github.com/iberflow) in [7a1a267](https://github.com/toaweme/care/commit/7a1a26751929b79080865473a3d4bfd7a6c0ecbd).

### CI & Build

- Gate release on action.yml version default, add prepare-release script by [@iberflow](https://github.com/iberflow) in [68ed794](https://github.com/toaweme/care/commit/68ed794e159feb098c03c7d1152b805f81d731cb).
- Move action version to inline comment so dependabot maintains it by [@iberflow](https://github.com/iberflow) in [3962a87](https://github.com/toaweme/care/commit/3962a870a33c620c30e38f92563f3f36f6258c9c).
- Refresh pinned action version comments to match dependabot bumps by [@iberflow](https://github.com/iberflow) in [4c18435](https://github.com/toaweme/care/commit/4c184352757e0f05d4a3063bdd5aea07ff85e4ba).
- Bootstrap-labels workflow by [@iberflow](https://github.com/iberflow) in [b822a6b](https://github.com/toaweme/care/commit/b822a6bc9880f18cbc52c999a648e2519ca94d30).

### Chores & Other

- Release v0.9.3 by [@iberflow](https://github.com/iberflow) in [8c58390](https://github.com/toaweme/care/commit/8c5839056d7aeea9ffb5fb513bed9f9b9797e0d8).
- Bump the actions-minor group with 2 updates by [@dependabot[bot]](https://github.com/dependabot[bot]) in [cd40bc7](https://github.com/toaweme/care/commit/cd40bc7ca2d09c005e1a6b9babe3aa74b7422675).
- Add community governance (issue-first, DCO, templates) by [@iberflow](https://github.com/iberflow) in [aa9b7b4](https://github.com/toaweme/care/commit/aa9b7b4fab646b33f7111b380f82a3396fc695ca).

## [0.9.2] - 2026-07-03

### Features

- Publish Windows builds via Scoop, installable with `scoop install toaweme/care` by [@iberflow](https://github.com/iberflow) in [ce9f563](https://github.com/toaweme/care/commit/ce9f5636b8324bea05a81e4f768ab4399d57dd75).

### Chores & Other

- Switch Homebrew distribution from formula to cask; `brew install toaweme/tap/care` is unchanged on macOS by [@iberflow](https://github.com/iberflow) in [a73dfbf](https://github.com/toaweme/care/commit/a73dfbf934cf513c4b4491581538dec6f43f8392).

## [0.9.0] - 2026-07-02

### Features

- Rework care get with repeatable -r token=value replacements, drop embedded templates and the get lint preset by [@iberflow](https://github.com/iberflow) in [d4d4b78](https://github.com/toaweme/care/commit/d4d4b7851bf0e40acaa9de5eb2bd2929a53e6fd2).

## [0.8.2] - 2026-07-01

### Documentation

- Update CHANGELOG by [@iberflow](https://github.com/iberflow) in [ca5b6e8](https://github.com/toaweme/care/commit/ca5b6e898643cb681929674bc452b1a87ec74024).
- Pin action.yml default and README examples to v0.8.1 by [@iberflow](https://github.com/iberflow) in [7422959](https://github.com/toaweme/care/commit/7422959baad120259dbfd1b21c23dcd1905ba98d).

### Chores & Other

- Bump cli to v0.3.3 by [@iberflow](https://github.com/iberflow) in [c80e9af](https://github.com/toaweme/care/commit/c80e9af44a6877c58111f9b5abf63ebe08de9f35).
- Bump toaweme deps to latest releases by [@iberflow](https://github.com/iberflow) in [75ee6b2](https://github.com/toaweme/care/commit/75ee6b2482ce2504357af2f8808f970ca5265410).

## [0.8.1] - 2026-07-01

### Fixes

- Don't dock score for a clean, upstreamless checkout by [@iberflow](https://github.com/iberflow) in [30e3eb0](https://github.com/toaweme/care/commit/30e3eb0c9ffa99520e527f4245ec8bee9fc06bcd).

### Documentation

- Pin binary install example to v0.8.0 by [@iberflow](https://github.com/iberflow) in [484239f](https://github.com/toaweme/care/commit/484239f716f588bd0ac7e509abc7d501c4777bcf).

## [0.8.0] - 2026-07-01

### Features

- Support install-only input to skip everything but install by [@iberflow](https://github.com/iberflow) in [40630f7](https://github.com/toaweme/care/commit/40630f73a1373417bc7104dee2f1e5927b0a5715).
- Add timers to stdout by [@iberflow](https://github.com/iberflow) in [c650393](https://github.com/toaweme/care/commit/c65039363b4d399d0e138e52d9d78b89828f9d4c).

### Documentation

- Bump to 0.8.0 by [@iberflow](https://github.com/iberflow) in [ac8ff03](https://github.com/toaweme/care/commit/ac8ff03573d6690cad1bf3e81877f63ef68241a6).

### Refactors

- Rename default report file to care.json, drop redundant output input by [@iberflow](https://github.com/iberflow) in [2beb1c9](https://github.com/toaweme/care/commit/2beb1c964be6ca327b8d8443a6a0493715b5fb2b).

## [0.7.1] - 2026-07-01

### Fixes

- Don't fail the version-control check on a clean tree with no upstream by [@iberflow](https://github.com/iberflow) in [2fea55c](https://github.com/toaweme/care/commit/2fea55ccf4ba657a42495710fefe2e7d72e196d8).

### Refactors

- Replace stale templates/tests.yml with the canonical quality.yml by [@iberflow](https://github.com/iberflow) in [30db7d6](https://github.com/toaweme/care/commit/30db7d692e47e40bc36e9da9b817395f6e4d45ce).

## [0.7.0] - 2026-07-01

### Features

- Configurable timeout on report publish curl calls by [@iberflow](https://github.com/iberflow) in [26d7bbd](https://github.com/toaweme/care/commit/26d7bbde878be718556993243aa19f5846e112a3).

## [0.6.0] - 2026-06-30

### Features

- Per-check grading policy with score breakdown by [@iberflow](https://github.com/iberflow) in [fd554b7](https://github.com/toaweme/care/commit/fd554b74b2d53203ce8ee11e2dd82f447b54dd8a).

### Documentation

- Update README and action inputs by [@iberflow](https://github.com/iberflow) in [b5f7eae](https://github.com/toaweme/care/commit/b5f7eae05d19319e93a34eb371d681048145254d).
- Bump README to 0.5.0 by [@iberflow](https://github.com/iberflow) in [f526d71](https://github.com/toaweme/care/commit/f526d7127e3875fd132f4ed7414b422a0c4decdc).

## [0.5.0] - 2026-06-29

### Refactors

- Rename --pretty flag to --stdout by [@iberflow](https://github.com/iberflow) in [ca06f3f](https://github.com/toaweme/care/commit/ca06f3f4eceaab45bc71b894e5a9e774d9c99198).

## [0.4.0] - 2026-06-29

### Documentation

- Update readme and changelog by [@iberflow](https://github.com/iberflow) in [996a365](https://github.com/toaweme/care/commit/996a3650112dc2c99a2657359cbf438a6db70467).

### Refactors

- Simplify the github action to care status by [@iberflow](https://github.com/iberflow) in [663eabc](https://github.com/toaweme/care/commit/663eabc3e697124d470c4081e595fb153566c5bf).

### CI & Build

- Show installed care tools by [@iberflow](https://github.com/iberflow) in [c441aa3](https://github.com/toaweme/care/commit/c441aa3c9eeb37401a60d2ef3531e1b39fb36ae9).

### Chores & Other

- Bump go to 1.26.4 by [@iberflow](https://github.com/iberflow) in [edb302e](https://github.com/toaweme/care/commit/edb302e11df73ecb66c6736f034fe16e540c4f40).

## [0.3.1] - 2026-06-28

### Features

- Surface tool-failure errors in status output by [@iberflow](https://github.com/iberflow) in [1195e62](https://github.com/toaweme/care/commit/1195e628dfe678957f633198720af2102a9a46e7).

### Documentation

- Update README by [@iberflow](https://github.com/iberflow) in [1ff123d](https://github.com/toaweme/care/commit/1ff123d45ba86e8225d8d1643c93c0d4250ace79).

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

[0.9.4]: https://github.com/toaweme/care/compare/v0.9.3...v0.9.4
[0.9.3]: https://github.com/toaweme/care/compare/v0.9.2...v0.9.3
[0.9.2]: https://github.com/toaweme/care/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/toaweme/care/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/toaweme/care/compare/v0.8.2...v0.9.0
[0.8.2]: https://github.com/toaweme/care/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/toaweme/care/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/toaweme/care/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/toaweme/care/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/toaweme/care/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/toaweme/care/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/toaweme/care/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/toaweme/care/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/toaweme/care/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/toaweme/care/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/toaweme/care/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/toaweme/care/releases/tag/v0.1.0
