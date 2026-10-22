# OpenAPI contract

`openapi.json` is a generated snapshot of the public API contract used to
validate the CLI's HTTP operations without requiring network access. The
deployed contract at `https://api.epismo.ai/openapi.json` is the source of truth;
do not edit this snapshot manually. Refresh it after the API is deployed:

```sh
sh scripts/sync-openapi.sh
```

The contract's `info.version` is independent of the CLI release version. CLI
tests verify the documented operation coverage, query parameters, and structured
HTTP 402 response.

The release workflow separately verifies that the deployed contract still
contains every operation and query parameter required by the CLI. Additional
API operations do not block a release.
