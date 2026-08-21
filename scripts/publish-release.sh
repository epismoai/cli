#!/bin/sh
set -eu

usage() {
  echo "usage: $0 <version>" >&2
  echo "example: $0 1.8.1" >&2
  exit 2
}

version="${1:-}"
if [ "$#" -ne 1 ] || [ -z "$version" ]; then
  usage
fi

version=${version#v}
if ! printf '%s\n' "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
  echo "version must be a semantic version such as 1.8.1" >&2
  exit 2
fi

tag="v$version"

git switch main
git pull --ff-only origin main

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  echo "tag $tag already exists locally" >&2
  exit 1
fi
if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
  echo "tag $tag already exists on origin" >&2
  exit 1
fi

if gh release view "$tag" >/dev/null 2>&1; then
  echo "GitHub release $tag already exists" >&2
  exit 1
fi

git tag -a "$tag" -m "Epismo CLI $tag"
git push origin "$tag"
gh release create "$tag" \
  --verify-tag \
  --title "Epismo CLI $tag" \
  --generate-notes
