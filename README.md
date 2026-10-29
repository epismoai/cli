# Epismo CLI

Use [Epismo](https://epismo.ai) from your terminal, scripts, and AI agents. Save and resume shared work with cases, and use playbooks for reusable guidance.

## Install

macOS and Linux:

```sh
curl -fsSL https://epismo.ai/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://epismo.ai/install.ps1 | iex
```

Or install with npm or Go:

```sh
npm install -g epismo
go install github.com/epismoai/cli/cmd/epismo@latest
```

Binaries and checksums are available from [GitHub Releases](https://github.com/epismoai/cli/releases). Run `epismo update` for update instructions for your installation.

## Quick start

```sh
epismo login
epismo workspace use acme # optional: select your workspace
epismo case start --title "Customer onboarding"
epismo case get CASE_ID
```

Public cases and playbooks can be browsed before login. Private work and writes require authentication.

## Find what you need

```sh
epismo --help
epismo case --help
epismo examples
```

Append `--help` to any command for its arguments and options.

- [CLI guide](https://epismo.ai/en/docs/cli): setup and everyday workflows.
- [Command reference](https://epismo.ai/en/docs/cli/reference): available commands and global options.
- [Cases guide](https://epismo.ai/en/docs/cli/cases): records, tasks, handoffs, reports, and sharing.

## Scripts and agents

Commands return JSON on stdout; errors and progress go to stderr. Output fields use `snake_case`. Use `--output table` for interactive reading, or `--request @file.json` and `--request -` for JSON input from a file or stdin. `--request` supplies all request parameters; `case start --input` and `case update --input` supply only the case input values. On update, `--input` replaces the complete object. Attach one playbook version to an open case without a playbook using `case update CASE_ID --version-id VERSION_ID --lock-version N`, optionally with `--input`; the resulting input must satisfy that version’s schema.

For automation, supply a workspace-scoped token through `EPISMO_TOKEN`. Set `EPISMO_WORKSPACE` or pass `--workspace` to select a workspace. Run `epismo doctor` to inspect your setup.

## Develop

The CLI is written in Go and its runtime uses only the standard library.

```sh
go test -race ./...
go vet ./...
go build ./cmd/epismo
```

The generated API snapshot lives in [`contracts/openapi.json`](contracts/openapi.json). Refresh it after an API deployment with `sh scripts/sync-openapi.sh`; do not edit it manually. Releases are built from semantic-version tags such as `v1.3.1`.

## License

[Apache License 2.0](LICENSE).
