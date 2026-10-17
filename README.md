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

See the [CLI guide](https://epismo.ai/en/docs/cli) for setup and workflow documentation.

## Ownership and resource lifecycle

Playbooks and Cases belong to a personal or workspace account, which is also
responsible for their usage charges. Omit `--owner-id` on create, copy, or case
start to use the active workspace, or your personal account when none is selected.
Assignees and maintainers identify responsibility and do not grant access.
Workspace members need explicit sharing unless they exercise Owner/Admin rights.
`playbook assign` changes `assignedTo`, the shared responsibility field on
Playbooks, Cases, and Tasks. The Playbook UI calls this person the Maintainer.
Case ownership transfer requires ownership and the latest lock version; it preserves sharing
and assignment but clears the saved Brief, including DNA and priority.

```sh
epismo playbook list --archived
epismo playbook restore PLAYBOOK_ID
epismo playbook assign PLAYBOOK_ID --assigned-to USER_ID
epismo case owner transfer CASE_ID --owner-id ACCOUNT_ID --lock-version 2
epismo case list --archived
epismo case archive CASE_ID --lock-version 3
epismo case restore CASE_ID --lock-version 4
epismo case delete CASE_ID --lock-version 5
```

Archive and restore require editing access and preserve work. Permanent Case
deletion requires ownership, an already archived Case, and its current lock
version. It cannot be undone. Get the current lock version from the appropriate
active or archived listing.

Before removing a workspace member, preview the responsibility/access handover:

```sh
epismo workspace member handover WORKSPACE_ID --departing-user-id OLD_USER_ID --successor-user-id NEW_USER_ID
```

Use `--preview=false` to execute and `--transfer-open-approvals` to include open
approval tasks. This explicitly grants the successor editing access; normal
reassignment alone does not. Remove membership separately with
`workspace member delete --workspace-id WORKSPACE_ID OLD_USER_ID`, then run the
handover again with `--preview=false --finalize` to clean up intervening changes.
`--finalize` requires membership to have already been removed; it does not remove
the member.
The operation defaults to preview and supports the normal idempotency key and
local `--dry-run` options. Completed approvals and authorship remain history.

Initial handover preserves direct grants. Workspace member removal clears the
User's direct grants on items owned by that Workspace, including completed and
archived items. Finalization repeats cleanup for late grants. Personal and
other-Workspace shares remain unchanged.
Team and Workspace management authority follow current membership and the
participation Workspace's SSO. Non-member direct shares use normal Epismo
authentication. Resource lists
use explicit access and supported publication, with no ownership exception.

These operations require an API deployment that supports the bundled OpenAPI
contract. Required workspace SSO also applies to CLI and automation credentials;
sign in again when current SSO evidence is missing or expires.

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
epismo case brief set CASE_ID --content "Waiting for launch approval." --details '{"needs":[{"text":"Launch approval","recordIds":[]}]}' --priority high
epismo case brief set CASE_ID --priority urgent
epismo case brief set CASE_ID --priority none
epismo case list --status open --priority high
epismo case list --priority none
```

A Brief contains a summary sentence of up to 280 characters and optional DNA details: `decisions`, `needs`, and `actions`. Each category has up to three `{text, recordIds}` items, each with at most three supporting Record IDs from the same case. Details summarize evidence; they do not approve work or assign tasks. Closing the case does not remove it, and an archived case cannot be changed. Case reads include it when present; reading never generates it. Use `case brief set` with any of `--content`, `--details`, and `--priority`. Omitted fields are preserved; pass `--details '{}'` to clear DNA. Priority-only Briefs may have an empty summary. `case brief delete` removes the summary and details and clears Case priority, including when only priority is set. Generation runs synchronously, extracts supported DNA, and optionally assesses priority; receives all saved Brief values as prior context and may revise or remove them; a null DNA or priority result clears the corresponding field. It charges token-priced credits to the case billing account; set and delete cost 2 credits each to the acting user's active workspace, or personal account when no workspace is selected. Reviews do not update the Brief.

Brief priority is `urgent`, `high`, `normal`, or `low`. Use `case brief set --priority` to edit it without changing the summary or DNA, including after close. Priority is read from `case.brief.priority`. Pass `--priority none` to clear it on writes or to select unset Cases on `case list`; omit the option to preserve it on writes or include all priorities on lists. The CLI converts `none` to the API's empty-string representation, including when passed through `--input`. Responses use enum numbers: `0` unset, `1` urgent, `2` high, `3` normal, `4` low. Generated priority needs concrete evidence; an absent urgency signal does not imply `normal`.

## Case sources

Link external evidence to a case and read its saved content with `case get`.
Slack threads and Discord messages/threads require a connected provider account
in the selected workspace.
For Google Docs, Sheets, or Slides, connect Google Drive and select the file in
Epismo Web first, then link its URL.

```sh
epismo case source link CASE_ID --url SOURCE_URL
epismo case get CASE_ID
```

Run `epismo case source --help` for get, refresh, and unlink commands, and use
command-level help for access requirements and credit costs.

Setup: [Slack](https://epismo.ai/en/docs/integrations/slack) · [Discord](https://epismo.ai/en/docs/integrations/discord) · [Google Drive](https://epismo.ai/en/docs/integrations/google-drive).

## Access and sharing

Resources use a nested `access` object with `visibility` (`private` or `public`) and UUID-to-role grants (`viewer` or `editor`):

```sh
# Set resource access (replaces the access configuration)
epismo playbook access set PLAYBOOK_ID --access '{"visibility":"public","grants":{"11111111-1111-4111-8111-111111111111":"editor"}}'
epismo case access set CASE_ID --access '{"visibility":"private","grants":{"11111111-1111-4111-8111-111111111111":"editor"}}'
epismo playbook owner transfer PLAYBOOK_ID --owner-id WORKSPACE_OR_USER_ID

# Create an independent copy of a published playbook version
epismo playbook copy PLAYBOOK_ID --source-version-id VERSION_ID --owner-id ACCOUNT_ID
epismo playbook copies list PLAYBOOK_ID --all

# Connect workspaces with shared teams
epismo -w sender team invite TEAM_ID --emails customer@example.com
epismo -w sender team invitation list TEAM_ID
epismo team invitation get TOKEN
epismo -w recipient team invitation accept TOKEN
epismo -w recipient team disconnect TEAM_ID
```

`--access` is accepted on `playbook create`, `case start`, `playbook copy`, and access setters. Viewers can read published Playbooks and private Cases, Tasks, Records, and linked sources; editors can update content and drafts. Playbook and Case updates must retain at least one explicit User or Team editor, including when an owner changes sharing. Creation adds the creator as an editor. Editors can archive resources; assignment alone grants no rights. Member removal is rejected if it would remove the final editor from any owned item, including completed and archived items; add a replacement editor first.

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
{
  "event": {
    "level": "info",
    "code": "BROWSER_WAITING",
    "message": "Waiting for authorization in your browser...",
    "details": { "timeout_seconds": 300 }
  }
}
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

`epismo case get`, `epismo case popular`, `epismo case record list`, `epismo case handoff graph get`, `epismo playbook list`, and UUID-based `epismo playbook get` work before login for public cases and public playbooks. `epismo case get` accepts a case code (e.g. `PANDA-317`) or UUID. Public Case reads expose titles, inputs, Tasks, Records, Briefs, readable handoffs, and public author/assignee account references. Private grants, linked sources, and private configuration remain omitted. `case get` includes the latest ten records; pass its `records_next_cursor` to `case record list --cursor` with `--scope self` for older public records. Use `case popular --playbook-id <id>` to filter discovery. The CLI creates a stable random `anonymousId` in its config for analytics and fair-use rate limiting; it is not an authentication credential. Search, aliases, private data, live case work, and writes still require login.

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

Discovery filters never grant access. Playbook browsing/resources and Task lists accept an optional `ownerId` filter for the resource or parent Case. Suggestion lists use `playbookOwnerId` for their current parent; Suggestions have no independent owner. Use `--input` to pass fields without a dedicated CLI flag. Web Workspace lists apply these filters automatically; API/CLI workspace connection alone does not restrict discovery ownership.
