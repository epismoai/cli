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

Browse public cases without an account:

```sh
epismo case popular
epismo case get CASE_ID
```

Public playbooks work the same way:

```sh
epismo playbook list
epismo playbook get PLAYBOOK_ID
```

Log in for search, private data, and writes:

```sh
epismo login
epismo workspace list
epismo workspace use acme # optional: save a default workspace by ID or handle
epismo workspace member invite teammate@example.com
epismo workspace invitation revoke INVITATION_ID
epismo case start --title "Customer Onboarding"
epismo --workspace acme playbook search onboarding
epismo playbook resource list --kind cli
epismo playbook create --definition '{"title":"Onboarding","steps":[]}'
```

`epismo workspace checkout <workspace-id>` starts a hosted subscription checkout. If the workspace has a cancellation scheduled for the end of its current billing period, it resumes that subscription instead of creating a second one.

## Cases and handoffs

Start a case, append timeline records, connect dependent workflows, and explore the DAG:

```sh
epismo case start --title "Customer Onboarding"
epismo case record append CASE_ID --kind output --content "Completed setup"
epismo case record update RECORD_ID --content "Revised setup notes"
epismo case record delete RECORD_ID
epismo case handoff create FIRST_CASE_ID --to-case-id SECOND_CASE_ID
epismo case handoff remove CASE_ID HANDOFF_ID
epismo case handoff graph get CASE_ID --scope connected
epismo case record list CASE_ID --scope ancestors
epismo case get CASE_ID
epismo case brief generate CASE_ID
epismo case brief delete CASE_ID
epismo case brief set CASE_ID --content "Waiting for launch approval."
```

A Brief is one current sentence for a case. Closing the case does not remove it, and an archived case cannot be changed. Case reads include it when present. Read it through `case get`, set it with `case brief set`, delete it with `case brief delete`, or generate it from current case evidence with `case brief generate`. Generation runs synchronously and charges token-priced credits to the case billing account; set and delete are update operations costing 2 credits each to the acting user's active workspace, or personal account when no workspace is selected. Reviews do not update the Brief. Set requires non-empty content.

## Slack sources

Connect your Slack account with `/epismo connect` after a workspace Owner or Admin installs the Slack app. Select that workspace in the CLI before linking a thread you can read:

```sh
epismo source link CASE_ID --url 'https://acme.slack.com/archives/C123/p1234567890123456'
epismo case get CASE_ID
epismo source refresh CASE_ID SOURCE_ID
epismo source snapshot get CASE_ID REVISION
epismo source unlink CASE_ID SOURCE_ID
```

Sources are separate from timeline records. `case get` includes saved sources for work collaborators; inspect their status, `checked_at`, and snapshot `captured_at` before relying on the content. Reads use saved captures and never fetch Slack. Fetching runs asynchronously; no partial capture is published. When `snapshot_omitted` is true, retrieve the saved revision with `source snapshot get`.

Link and unlink cost 2 credits each; linking includes the first completed capture, even when updates queue before it starts. Each subsequent completed manual or automatic capture costs 1 credit per open, non-archived case sharing the source, plus each closed case whose explicit refresh request it fulfills, billed to each case’s billing account. Queuing, failures, pagination, and retries are free; requests coalesced into one capture incur one charge per participating case. Use the same idempotency key for an uncertain retry. Unlink removes that case’s capture history; disconnecting or losing Slack authorization removes affected captures. Text already copied into records, reviews, or a Brief remains.

Linked content is readable by all case work collaborators, including those without access to the original Slack thread. Public readers cannot see sources. Review the case audience before linking or expanding sharing. The Web app displays, links, refreshes, and unlinks sources. In an open case you can edit, choose **Add source** under **External sources** and paste a Slack thread URL. The picker offers **Integration settings** when authorization is needed or no source connections are available. New links can also come from Slack's **Link thread to Epismo** shortcut or CLI/API/MCP.

## Playbook access

Use `visibility` and explicit editors instead of a raw ACL:

```sh
epismo playbook access get PLAYBOOK_ID
epismo playbook access set PLAYBOOK_ID --visibility public --editors USER_ID,TEAM_ID
epismo playbook owner transfer PLAYBOOK_ID --owner-id WORKSPACE_OR_USER_ID
```

`public` permits published reads only. The Playbook owner and workspace owners/admins retain implicit edit access and need no explicit `--editors` grant. Every workspace member can read published workspace-owned Playbooks; other members need an explicit user or team editor grant to edit them. Editors can change visibility and collaborators while retaining at least one team or another user as an explicit editor. Only owner managers can remove all additional sharing, archive a Playbook, or archive a historical version. Any member may create a Playbook owned by their workspace, or move a personally owned private Playbook into it with `playbook owner transfer`.

Case editors can change public/private visibility and collaborators while retaining at least one team or another user as an explicit editor. Only the current Case assignee can remove all additional sharing or archive the Case. Access changes must preserve every task assignee's edit access. The dedicated `playbook access get` remains owner-manager-only, and `case access get` remains current-assignee-only; editors can inspect collaborators with the normal `playbook get` or `case get` before replacing sharing.

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
epismo case record append CASE_ID --input - < record.json
epismo playbook create --input @playbook.json
```

Explicit flags override fields supplied through `--input`. Mutations create an idempotency key automatically unless you provide `--idempotency-key` or `idempotency_key` in the input.

## Everyday terminal use

The default output is JSON for scripts and agents. Choose a human-friendly output format when working interactively:

```sh
epismo workspace list --output table
epismo case list --output jsonl
epismo credit checkout --quantity 500 --output value --field checkout_url
epismo playbook search --query onboarding --jq '.playbooks[] | .id'
```

Global options may appear before or after the command:

```sh
EPISMO_WORKSPACE=acme epismo case list
epismo --workspace acme playbook list
epismo -w acme task list --all
```

Workspace references accept an exact ID or unique handle. The effective workspace is chosen in this order: `--workspace`, `EPISMO_WORKSPACE`, a workspace-scoped token, then the saved default workspace. A scoped token cannot grant access outside its scope.

`--dry-run` previews any command that changes remote or local state without sending a request, opening an authorization or checkout flow, or changing local configuration. This includes creates, updates, publishes, draft saves, assignments, records, handoffs, aliases, membership changes, archive/delete/revoke/close operations, and login/logout or workspace-selection changes. Read-only commands reject `--dry-run` instead of silently ignoring it. In an interactive terminal, especially impactful operations still ask for confirmation during a real run; pass `--yes` to skip that prompt in scripts that allocate a TTY.

`--input` also works on list/search commands for agent workflows; there it supplies query parameters rather than a request body.

## Authentication and configuration

`epismo login` opens a browser-based OAuth login. With `--email`, it automatically uses your organization SSO when available, otherwise it prompts for an email code.

`epismo case get`, `epismo case popular`, `epismo case record list`, `epismo case handoff graph get`, `epismo playbook list`, and UUID-based `epismo playbook get` work before login for public cases and public playbooks. `epismo case get` accepts a case code (e.g. `PANDA-317`) or UUID. Public case reads expose the current title, input, records, and readable handoffs; tasks, assignment, and collaborator identities remain restricted to work collaborators. `case get` includes the latest ten records; pass its `records_next_cursor` to `case record list --cursor` with `--scope self` for older public records. Use `case popular --playbook-id <id>` to filter discovery. The CLI creates a stable random `anonymousId` in its config for analytics and fair-use rate limiting; it is not an authentication credential. Search, aliases, private data, live case work, and writes still require login.

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
