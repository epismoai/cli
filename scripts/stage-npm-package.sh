#!/bin/sh
set -eu

source_dir="${1:-dist}"
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
platforms_file="$script_dir/../npm/platforms.txt"

if [ ! -d "$source_dir" ]; then
  echo "npm staging source directory does not exist: $source_dir" >&2
  exit 1
fi
if [ -L npm/vendor ]; then
  echo "refusing to stage npm binaries through a symbolic link" >&2
  exit 1
fi

checksums_file="$source_dir/checksums.txt"
if [ ! -f "$checksums_file" ] || [ -L "$checksums_file" ]; then
  echo "checksums file is missing or is a symbolic link: $checksums_file" >&2
  exit 1
fi

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

stage_binary() {
  target="$1"
  source_name="$2"
  binary_name="$3"
  source_path="$source_dir/$source_name"
  target_dir="npm/vendor/$target"
  if [ ! -f "$source_path" ] || [ -L "$source_path" ]; then
    echo "required npm binary is missing or is a symbolic link: $source_path" >&2
    exit 1
  fi
  expected_checksum=$(awk -v name="$source_name" '$2 == name { print $1; found = 1 } END { if (!found) exit 1 }' "$checksums_file") || {
    echo "no checksum entry found for $source_name in $checksums_file" >&2
    exit 1
  }
  actual_checksum=$(sha256_of "$source_path")
  if [ "$actual_checksum" != "$expected_checksum" ]; then
    echo "checksum verification failed for $source_name: expected $expected_checksum, got $actual_checksum" >&2
    exit 1
  fi
  mkdir -p "$target_dir"
  install -m 0755 "$source_path" "$target_dir/$binary_name"
  cmp "$source_path" "$target_dir/$binary_name"
}

if [ ! -f "$platforms_file" ]; then
  echo "npm platform manifest is missing: $platforms_file" >&2
  exit 1
fi

while read -r target source_name binary_name; do
  [ -z "$target" ] && continue
  stage_binary "$target" "$source_name" "$binary_name"
done < "$platforms_file"

echo "Staged all Epismo platform binaries in npm/vendor"
