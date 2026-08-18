#!/bin/sh
set -eu

output_dir="dist"
if [ "${1:-dist}" != "$output_dir" ]; then
  echo "release output directory is fixed at dist" >&2
  exit 1
fi
version="${RELEASE_VERSION:-}"
if [ -z "$version" ]; then
  echo "RELEASE_VERSION is required (for example, RELEASE_VERSION=1.3.1)" >&2
  exit 1
fi
if ! printf '%s\n' "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
  echo "RELEASE_VERSION must be a semantic version without a v prefix" >&2
  exit 1
fi
if [ -L "$output_dir" ]; then
  echo "refusing to use a symbolic-link dist directory" >&2
  exit 1
fi
mkdir -p "$output_dir"
output_dir_abs=$(cd "$output_dir" && pwd)
rm -f "$output_dir"/epismo_darwin_amd64.tar.gz \
  "$output_dir"/epismo_darwin_arm64.tar.gz \
  "$output_dir"/epismo_linux_amd64.tar.gz \
  "$output_dir"/epismo_linux_arm64.tar.gz \
  "$output_dir"/epismo_windows_amd64.zip \
  "$output_dir"/epismo_windows_arm64.zip \
  "$output_dir"/epismo_darwin_amd64 \
  "$output_dir"/epismo_darwin_arm64 \
  "$output_dir"/epismo_linux_amd64 \
  "$output_dir"/epismo_linux_arm64 \
  "$output_dir"/epismo_windows_amd64.exe \
  "$output_dir"/epismo_windows_arm64.exe \
  "$output_dir"/epismo.rb \
  "$output_dir"/checksums.txt

build_archive() {
  target_os="$1"
  target_arch="$2"
  extension="$3"
  binary="epismo"
  if [ "$target_os" = "windows" ]; then binary="epismo.exe"; fi
  work_dir=$(mktemp -d "${TMPDIR:-/tmp}/epismo-release.XXXXXX")
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build \
    -trimpath -ldflags="-s -w -X main.version=$version" \
    -o "$work_dir/$binary" ./cmd/epismo
  if [ "$target_os" = "windows" ]; then
    cp "$work_dir/$binary" "$output_dir_abs/epismo_${target_os}_${target_arch}.exe"
  else
    cp "$work_dir/$binary" "$output_dir_abs/epismo_${target_os}_${target_arch}"
    chmod 0755 "$output_dir_abs/epismo_${target_os}_${target_arch}"
  fi
  archive="$output_dir_abs/epismo_${target_os}_${target_arch}.$extension"
  if [ "$extension" = "zip" ]; then
    (cd "$work_dir" && zip -q "$archive" "$binary")
  else
    tar -C "$work_dir" -czf "$archive" "$binary"
  fi
  rm -rf "$work_dir"
}

build_archive darwin amd64 tar.gz
build_archive darwin arm64 tar.gz
build_archive linux amd64 tar.gz
build_archive linux arm64 tar.gz
build_archive windows amd64 zip
build_archive windows arm64 zip

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$output_dir" && sha256sum epismo_* > checksums.txt)
else
  (cd "$output_dir" && shasum -a 256 epismo_* > checksums.txt)
fi
