#!/bin/sh
set -eu

source_url=${1:-https://api.epismo.ai/openapi.json}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(dirname -- "$script_dir")
destination="$repo_dir/contracts/openapi.json"
download=$(mktemp "$repo_dir/contracts/openapi.download.XXXXXX")
temporary=$(mktemp "$repo_dir/contracts/openapi.formatted.XXXXXX")

cleanup() {
	rm -f -- "$download" "$temporary"
}
trap cleanup EXIT INT TERM

curl -fsS --retry 3 --output "$download" "$source_url"
go run "$script_dir/format-openapi.go" "$download" "$temporary"
sh "$script_dir/check-openapi.sh" "$temporary"
mv -- "$temporary" "$destination"
rm -f -- "$download"
trap - EXIT INT TERM

echo "Updated contracts/openapi.json from $source_url"
