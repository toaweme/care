#!/usr/bin/env bash
# prepare a care release commit. The one thing that MUST live in the tagged
# commit is action.yml's version input default: a SHA-pinned caller reads it
# from the tag's commit, so if it is stale the action installs the wrong
# release. This script bumps it to match the new tag, commits, and creates the
# annotated tag. Pushing (which triggers the release workflow) is left to you.
#
# usage: scripts/prepare-release.sh vX.Y.Z
set -euo pipefail

tag="${1:-}"
case "$tag" in
  v[0-9]*) ;;
  *) echo "usage: $(basename "$0") vX.Y.Z" >&2; exit 2 ;;
esac

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if [ -n "$(git status --porcelain)" ]; then
  echo "working tree is not clean; commit or stash first" >&2
  exit 1
fi

# the version input is the only default that holds a vX.Y.Z string, so this
# targets it without touching the other input defaults (e.g. strict: "false").
current=$(grep -oE 'default: "v[0-9][^"]*"' action.yml | head -1 | sed -E 's/.*"(v[^"]*)".*/\1/')
if [ -z "$current" ]; then
  echo "could not find a version default in action.yml" >&2
  exit 1
fi

if [ "$current" = "$tag" ]; then
  echo "action.yml already defaults to $tag"
else
  # -i.bak keeps this portable across BSD (macOS) and GNU sed.
  sed -i.bak -E "s/(default: \")v[0-9][^\"]*(\")/\\1$tag\\2/" action.yml
  rm -f action.yml.bak
  echo "bumped action.yml version default: $current -> $tag"
fi

git add action.yml
git commit -m "chore: release $tag"
git tag -a "$tag" -m "$tag"

cat <<MSG

prepared $tag. review the commit, then push to trigger the release:
  git push origin main && git push origin $tag
MSG
