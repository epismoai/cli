#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 OPENAPI_FILE" >&2
	exit 2
fi

contract_file=$1
if [ ! -f "$contract_file" ]; then
	echo "OpenAPI contract not found: $contract_file" >&2
	exit 1
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(dirname -- "$script_dir")
case "$contract_file" in
	/*) ;;
	*) contract_file=$(CDPATH= cd -- "$(dirname -- "$contract_file")" && pwd)/$(basename -- "$contract_file") ;;
esac

(
	cd "$repo_dir"
	EPISMO_OPENAPI_CONTRACT="$contract_file" go test ./internal/cli -run '^(TestOpenAPIContractMetadata|TestEveryRemoteCommandUsesDocumentedOperation|TestQueryCommandOptionsMatchOpenAPI)$'
)
