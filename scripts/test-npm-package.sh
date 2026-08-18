#!/bin/sh
set -eu

expected_version="${1:-}"
if [ -z "$expected_version" ]; then
  echo "expected CLI version is required" >&2
  exit 1
fi

temp_base="${TMPDIR:-/tmp}"
case "$temp_base" in
  /*) ;;
  *) temp_base="/tmp" ;;
esac
temp_base="${temp_base%/}"
test_dir=$(mktemp -d "$temp_base/epismo-npm-test.XXXXXX")
case "$test_dir" in
  "$temp_base"/epismo-npm-test.*) ;;
  *) echo "refusing unsafe temporary directory: $test_dir" >&2; exit 1 ;;
esac
cleanup() { rm -rf -- "$test_dir"; }
trap cleanup EXIT INT TERM

package_file=$(npm pack --silent --pack-destination "$test_dir")
case "$package_file" in
  */*|*\\*|"") echo "unexpected npm package filename: $package_file" >&2; exit 1 ;;
  *.tgz) ;;
  *) echo "unexpected npm package filename: $package_file" >&2; exit 1 ;;
esac
if [ ! -f "$test_dir/$package_file" ] || [ -L "$test_dir/$package_file" ]; then
  echo "npm pack did not create the expected tarball" >&2
  exit 1
fi

npm install --global --ignore-scripts --prefix "$test_dir/prefix" "$test_dir/$package_file" >/dev/null
actual_version=$("$test_dir/prefix/bin/epismo" --version)
if [ "$actual_version" != "$expected_version" ]; then
  echo "npm package started version $actual_version, expected $expected_version" >&2
  exit 1
fi

echo "Verified npm package with lifecycle scripts disabled"
