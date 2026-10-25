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

Commands return JSON on stdout; errors and progress go to stderr. Output fields use `snake_case`. Use `--output table` for interactive reading, or `--input @file.json` and `--input -` for JSON input from a file or stdin.

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

## Reports

### Personal reports

Use `epismo report personal generate` to assess your assigned open cases,
assigned open tasks in open cases, and managed open suggestions. Only work
owned by the active workspace, or your account in personal space, is included.
Generation returns a job; poll `epismo report personal get --job-id JOB_ID`.
Use `report personal list` for saved history and `report personal get --report-id
REPORT_ID` for a full saved result. Only the generating user can read a personal
report in its original billing home. Saved history has no 14-day cutoff;
personal and team generation use current work without prior reports or proposal
handling history. Review existing work before confirming a new recommendation.

### Team reports

Team review is available from the CLI, API and MCP without a Web UI. Current
members can generate and read shared reports in the active workspace:

```sh
epismo report team generate <team-id>
epismo report team get <team-id> --job-id <job-id>
epismo report team list <team-id>
epismo report team get <team-id> --report-id <report-id>
```

Generation returns a job and uses workspace credits; reuse `--idempotency-key`
when retrying. The assessment includes open Cases explicitly shared with this
Team and owned by the workspace, plus their Tasks and recent Records and open Suggestions on Team-shared Playbooks. Private
work and other owners are excluded. Saved text remains readable by current Team
members after source changes; evidence links require live access. Generation returns up to three proposals, without prior report or handling
history as model input. Use `epismo report team act <report-id> --proposal-id
<proposal-id> --action create|dismiss` to handle a proposal explicitly. Handling
is shared across Team members; creation requires live destination access.
Creation, charging and shared handling commit together so another member's
retry cannot duplicate work. New
Cases and Playbooks retain creator-private access, without copying source sharing.

### Report health

Case, Personal and Team reports share `health`: `healthy`, `at_risk`,
`critical` or `unknown`, with a `summary` describing the current situation and main evidence-based reason (at most 280 characters). This diagnoses recorded
progress and blockers, not approval or whether tasks remain. Legacy verdicts map
`pass` to `healthy`, `changes_requested` to `at_risk`, and `insufficient`,
`need_info` or empty verdicts to `unknown`. This renames the saved assessment;
generate a new report for a diagnosis against the current health criteria. Historical reports retain their IDs and
saved contents.
