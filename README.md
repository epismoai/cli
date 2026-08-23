# Epismo CLI

Native command-line interface for [Epismo](https://epismo.ai). It is written in Go and returns JSON, making it useful in terminals, scripts, and agent workflows.

## Install

macOS and Linux:

```sh
curl -fsSL https://epismo.ai/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://epismo.ai/install.ps1 | iex
```

Or install with Go or npm:

```sh
go install github.com/epismoai/cli/cmd/epismo@latest
npm install -g epismo
```

Prebuilt archives, standalone executables, and `checksums.txt` are available from [GitHub Releases](https://github.com/epismoai/cli/releases).

## Updating

The CLI checks GitHub Releases for a newer version at most once every 24 hours in interactive terminals. When an update is available, run:

```sh
epismo update
```

The command identifies how the active executable was installed and returns the appropriate installation command; it never modifies the executable itself. Default shell installations receive the short command shown above. A custom installation directory is included only when needed. Go and global Node installations receive a manager-specific command when the package manager can be identified. Yarn's global command is returned only for Yarn Classic because modern Yarn does not support that workflow. Set `EPISMO_UPDATE_CHECK=0` to disable automatic checks.

The npm package contains a dependency-free JavaScript launcher and all supported native binaries. It uses no lifecycle installation scripts and performs no additional downloads during installation. The launcher selects the binary for the current platform and passes its package-manager context to the native CLI. Shell and PowerShell installers write an installation receipt next to the executable; Go installations are identified from embedded build metadata. If the method cannot be determined safely, `epismo update` links to the update instructions without modifying the executable.

## Quick start

```sh
epismo login
epismo workspace list
epismo workspace use acme # optional: save a default workspace by ID or handle
epismo --workspace acme playbook search onboarding
epismo playbook resource list --kind cli
epismo playbook init --title Onboarding > playbook.json
epismo playbook create --definition @playbook.json
```

Run `epismo --help` for command groups, or append `--help` to any group or command for its options.

Use `epismo examples` for common workflows, `epismo doctor` to inspect local setup, and `epismo completion zsh` (or `bash`, `fish`, `powershell`) to install shell completion.
The source man page is available at [`docs/epismo.1`](docs/epismo.1).

## Output and input

Successful commands write one JSON document to stdout. Progress events, warnings, and errors are written to stderr as newline-delimited JSON (one compact JSON object per line). This keeps output easy to use from scripts and agents while allowing interactive commands to report progress.

CLI JSON field names and enum values use `snake_case`. Machine-readable warning and error codes use `SCREAMING_SNAKE_CASE`. Input supplied through `--input` accepts both `snake_case` and `camelCase` fields at every nesting level. Do not provide both spellings of the same field in one object. OAuth protocol fields retain their standard wire names such as `access_token` and `grant_type`.

Interactive prompts such as the email-code input prompt are plain terminal text. All machine-readable diagnostic records remain one-line JSON objects.

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "...",
    "retryable": false
  }
}
```

Progress and warnings use a common event envelope:

```json
{"event":{"level":"info","code":"BROWSER_WAITING","message":"Waiting for authorization in your browser...","details":{"timeout_seconds":300}}}
```

Commands that accept a request body support inline JSON, a file, or stdin:

```sh
epismo playbook create --input @playbook.json
epismo case record append CASE_ID --input - < record.json
```

Explicit flags override fields supplied through `--input`. Mutations create an idempotency key automatically unless you provide `--idempotency-key` or `idempotency_key` in the input.

## Everyday terminal use

The default output is JSON for scripts and agents. Choose a human-friendly output format when working interactively:

```sh
epismo workspace list --output table
epismo credit checkout --quantity 500 --output value --field checkout_url
epismo task list --all --output jsonl
epismo playbook search --query onboarding --jq '.playbooks[] | .id'
```

Global options may appear before or after the command:

```sh
epismo --workspace acme playbook list
epismo -w acme task list --all
EPISMO_WORKSPACE=acme epismo case list
```

Workspace references accept an exact ID or unique handle. The effective workspace is chosen in this order: `--workspace`, `EPISMO_WORKSPACE`, a workspace-scoped token, then the saved default workspace. A scoped token cannot grant access outside its scope.

`--dry-run` previews any command that changes remote or local state without sending a request, opening an authorization or checkout flow, or changing local configuration. This includes creates, updates, publishes, draft saves, assignments, records, stars, aliases, membership changes, archive/delete/revoke/close operations, and login/logout or workspace-selection changes. Read-only commands reject `--dry-run` instead of silently ignoring it. In an interactive terminal, especially impactful operations still ask for confirmation during a real run; pass `--yes` to skip that prompt in scripts that allocate a TTY.

`--input` also works on list/search commands for agent workflows; there it supplies query parameters rather than a request body.

## Authentication and configuration

`epismo login` opens a browser-based OAuth login. With `--email`, it automatically uses your organization SSO when available, otherwise it prompts for an email code.

For CI or other non-interactive use, create a workspace-scoped token and pass it with `EPISMO_TOKEN`:

```sh
epismo token create --workspace-id WORKSPACE_ID
EPISMO_TOKEN=... epismo playbook search
```

Configuration is stored in `~/.epismo` (compatible with the original npm CLI). Set `EPISMO_CONFIG_DIR` to use a different directory. For local API development, set `APP_ENV=dev`, or override `EPISMO_API_URL` and `EPISMO_WEB_URL`.

## API contract

[`contracts/openapi.json`](contracts/openapi.json) is a generated snapshot of Epismo's public OpenAPI 3.1 contract. Refresh it after an API deployment:

```sh
sh scripts/sync-openapi.sh
```

Do not edit the snapshot manually. Its `info.version` represents API compatibility (for example, `1.0.0` for `/v1`), not the CLI release version. Contract tests ensure remote commands and query options remain compatible with the API.

The OpenAPI snapshot describes the server wire format, which uses `camelCase` for Epismo fields. The CLI translates those fields to and from its public `snake_case` representation at the process boundary.

## Develop

The CLI runtime uses only the Go standard library.

```sh
go test -race ./...
go vet ./...
go build ./cmd/epismo
```

Releases are built for macOS, Linux, and Windows from semantic-version tags such as `v1.3.1`. The tag is the release version source; the development `package.json` version is intentionally not updated.

## License

[Apache License 2.0](LICENSE).
