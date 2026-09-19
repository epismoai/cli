package cli

import (
	"fmt"
	"net/http"
	"strings"
)

func playbookCommands() []*command {
	page := pagingOptions()
	init := &command{Path: "playbook init", Summary: "print a minimal playbook definition template", Options: []optionSpec{str("--title", "title", "initial playbook title"), choice("--category", "category", "initial category", "productivity", "programming", "design", "sales", "marketing", "operations", "learning")}, Examples: []string{"epismo playbook init --title Onboarding > playbook.json", "epismo playbook create --definition @playbook.json"}, Run: func(_ *app, inv invocation) (any, error) {
		definition := map[string]any{"title": inv.text("title"), "steps": []any{}}
		if definition["title"] == "" {
			definition["title"] = "Untitled Playbook"
		}
		if category := inv.text("category"); category != "" {
			definition["category"] = category
		}
		return definition, nil
	}}
	search := apiOperation("playbook search", "search readable playbooks, including pb: alias references", nil, http.MethodGet, staticEndpoint("/v1/playbooks"), requestQuery, false, append(page, str("--query", "query", "full-text query or pb: alias reference"), choice("--category", "category", "playbook category", "productivity", "programming", "design", "sales", "marketing", "operations", "learning"), csv("--lang", "preferredLangs", "comma-separated two-letter content languages in priority order"))...)
	list := publicApiOperation("playbook list", "list readable playbooks, most recently updated first; public playbooks work without login", nil, http.MethodGet, staticEndpoint("/v1/playbooks"), requestQuery, append(page, choice("--resource-kind", "resourceKind", "resource kind", "skill", "mcp", "cli", "api", "plugin", "graph", "document", "agent", "custom"), str("--resource-ref", "resourceRef", "normalized or provider-specific resource reference"))...)
	resourceList := apiOperation("playbook resource list", "list resource references used by readable playbooks", nil, http.MethodGet, staticEndpoint("/v1/playbook-resources"), requestQuery, false, choice("--kind", "kind", "resource kind", "skill", "mcp", "cli", "api", "plugin", "graph", "document", "agent", "custom"), integer("--page-size", "pageSize", "results per page (1-200)"))
	create := apiOperation("playbook create", "create a playbook and its first version", nil, http.MethodPost, staticEndpoint("/v1/playbooks"), requestBody, true, str("--owner-id", "ownerId", "User or Workspace account that should own the playbook"), defaultOption(choice("--visibility", "visibility", "published visibility", "private", "public"), "private"), csv("--editors", "editors", "comma-separated editor Account or Team IDs"), objectSource("--definition", "definition", "playbook definition JSON object"))
	create.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		if _, ok := payload["ownerId"]; !ok {
			accountID := ctx.Auth.AccountID
			if accountID == "" {
				user, err := a.userInfo(ctx.Auth.AccessToken)
				if err != nil {
					return nil, err
				}
				accountID = stringField(user, "account_id")
			}
			if accountID == "" {
				return nil, &Error{Code: "ACCOUNT_ID_UNAVAILABLE", Message: "The authenticated user does not have a personal Account ID.", Hint: "Pass --owner-id explicitly, or run `epismo login` again.", ExitCode: 1}
			}
			payload["ownerId"] = accountID
		}
		if _, supplied := payload["access"]; !supplied {
			access := map[string]any{"visibility": payload["visibility"]}
			if editors, ok := payload["editors"]; ok {
				access["editors"] = editors
			}
			payload["access"] = access
		}
		delete(payload, "visibility")
		delete(payload, "editors")
		return a.client.request(http.MethodPost, withWorkspace("/v1/playbooks", ctx.WorkspaceID), ctx.Auth.AccessToken, payload)
	}
	get := &command{Path: "playbook get", Summary: "get a playbook and its latest immutable version", Args: []string{"playbook-id-or-ref"}, Run: func(a *app, inv invocation) (any, error) {
		ctx, err := a.publicReadContext()
		if err != nil {
			return nil, err
		}
		id, err := resolvePlaybookID(a, ctx, inv.positional(0))
		if err != nil {
			return nil, err
		}
		return a.client.request(http.MethodGet, withWorkspace("/v1/playbooks/"+escaped(id), ctx.WorkspaceID), ctx.Auth.AccessToken, nil)
	}}
	versionList := apiOperation("playbook version list", "list versions", []string{"playbook-id"}, http.MethodGet, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions" }, requestQuery, false, page...)
	versionGet := apiOperation("playbook version get", "get one immutable version", []string{"playbook-id", "version-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions/" + escaped(i.positional(1))
	}, requestNone, false)
	versionArchive := apiOperation("playbook version archive", "archive a non-latest version", []string{"playbook-id", "version-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions/" + escaped(i.positional(1))
	}, requestBody, true)
	versionPublish := apiOperation("playbook version publish", "publish a version", []string{"playbook-id"}, http.MethodPost, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions" }, requestBody, true, str("--base-version-id", "baseVersionId", "current latest version ID"), objectSource("--definition", "definition", "new playbook definition JSON object"))
	draftGet := apiOperation("playbook draft get", "get the current draft", []string{"playbook-id"}, http.MethodGet, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft" }, requestNone, false)
	draftSave := apiOperation("playbook draft save", "create or overwrite the draft", []string{"playbook-id"}, http.MethodPut, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft" }, requestBody, false, defaultOption(integer("--base-revision", "baseRevision", "revision last read (0 for a new draft)"), 0), objectSource("--definition", "definition", "playbook definition JSON object"))
	draftDiscard := apiOperation("playbook draft discard", "discard the draft", []string{"playbook-id"}, http.MethodDelete, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft" }, requestBody, true)
	draftPublish := apiOperation("playbook draft publish", "publish the draft as a new version", []string{"playbook-id"}, http.MethodPost, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft/publish" }, requestBody, true, requiredOption(integer("--expected-draft-revision", "expectedDraftRevision", "draft revision last read and reviewed")))
	accessGet := apiOperation("playbook access get", "get public/private visibility and explicit editors", []string{"playbook-id"}, http.MethodGet, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/access" }, requestNone, false)
	accessSet := apiOperation("playbook access set", "set public/private visibility and explicit editors", []string{"playbook-id"}, http.MethodPut, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/access" }, requestBody, true, requiredOption(choice("--visibility", "visibility", "published visibility", "private", "public")), csv("--editors", "editors", "comma-separated editor Account or Team IDs"))
	owner := apiOperation("playbook owner", "change which account owns a playbook", []string{"playbook-id"}, http.MethodPatch, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/owner" }, requestBody, true, requiredOption(str("--owner-id", "ownerId", "User or Workspace account that should own the playbook")))
	archive := apiOperation("playbook archive", "archive a playbook", []string{"playbook-id"}, http.MethodDelete, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) }, requestBody, true)
	share := apiOperation("playbook share", "create a share link", []string{"playbook-id"}, http.MethodPost, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/share" }, requestBody, true)
	return []*command{init, search, list, resourceList, create, get, versionList, versionGet, versionArchive, versionPublish, draftGet, draftSave, draftDiscard, draftPublish, accessGet, accessSet, owner, archive, share}
}

func aliasCommands() []*command {
	set := &command{Path: "playbook alias set", Summary: "set or rename a playbook alias in the active namespace", Args: []string{"playbook-id", "alias"}, Options: []optionSpec{str("--owner-id", "ownerId", "alias owner Account ID")}, Input: &inputSpec{}, Safety: commandSafety{DryRun: true, IdempotencyKey: true}, Run: func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		payload["playbookId"] = inv.positional(0)
		payload["alias"] = inv.positional(1)
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		if err := addAliasOwner(a, ctx, payload); err != nil {
			return nil, err
		}
		return a.client.request(http.MethodPut, withWorkspace("/v1/aliases", ctx.WorkspaceID), ctx.Auth.AccessToken, payload)
	}}
	list := &command{Path: "playbook alias list", Summary: "list aliases for a playbook", Args: []string{"playbook-id"}, Options: []optionSpec{str("--owner-id", "ownerId", "filter by alias owner Account ID")}, Input: &inputSpec{}, Run: func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		payload["playbookId"] = inv.positional(0)
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		return a.client.request(http.MethodGet, withWorkspace(queryString("/v1/aliases", payload), ctx.WorkspaceID), ctx.Auth.AccessToken, nil)
	}}
	remove := &command{Path: "playbook alias delete", Summary: "delete an alias from the active namespace", Args: []string{"alias"}, Options: []optionSpec{str("--owner-id", "ownerId", "alias owner Account ID")}, Input: &inputSpec{}, Safety: commandSafety{DryRun: true, IdempotencyKey: true}, Run: func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		if err := addAliasOwner(a, ctx, payload); err != nil {
			return nil, err
		}
		return a.client.request(http.MethodDelete, withWorkspace("/v1/aliases/"+escaped(inv.positional(0)), ctx.WorkspaceID), ctx.Auth.AccessToken, payload)
	}}
	return []*command{set, list, remove}
}

func addAliasOwner(a *app, ctx executionContext, payload map[string]any) error {
	if stringField(payload, "ownerId") != "" {
		return nil
	}
	ownerID := ctx.Auth.AccountID
	if ownerID == "" {
		user, err := a.userInfo(ctx.Auth.AccessToken)
		if err != nil {
			return err
		}
		ownerID = stringField(user, "account_id")
	}
	if ownerID == "" {
		return &Error{Code: "ACCOUNT_ID_UNAVAILABLE", Message: "The authenticated user does not have a personal Account ID.", Hint: "Pass --owner-id explicitly, or run `epismo login` again.", ExitCode: 1}
	}
	payload["ownerId"] = ownerID
	return nil
}

func caseCommands() []*command {
	start := apiOperation("case start", "start a research, implementation, planning, or review effort; omit --version-id without a playbook. Reuse an existing case for the same goal, then save context with case record append", nil, http.MethodPost, staticEndpoint("/v1/cases"), requestBody, true, str("--version-id", "versionId", "playbook version ID"), str("--title", "title", "case title"), object("--case-input", "input", "case input JSON object"), csv("--acl", "acl", "comma-separated ACL principals"), boolOption("--auto-review", "autoReview", "when true, appending an OUTPUT record enqueues an Epismo AI review charged to this case's billing account captured at start"))
	get := publicApiOperation("case get", "read saved case context before continuing; pass the UUID from its URL. Check goals, constraints, decisions, artifacts, and next steps. Public cases return published content without login; this does not restore an agent session", []string{"case-id"}, http.MethodGet, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }, requestNone)
	list := apiOperation("case list", "find collaborator-accessible cases to resume, most recently updated first; match the intended goal before case get, and ask if ambiguous", nil, http.MethodGet, staticEndpoint("/v1/cases"), requestQuery, false, append(pagingOptions(), str("--assigned-to", "assignedTo", "only cases assigned to this account"), choice("--status", "status", "case status", "open", "closed"))...)
	popular := publicApiOperation("case popular", "rank public cases for discovery", nil, http.MethodGet, staticEndpoint("/v1/cases/popular"), requestQuery, append(pagingOptions(), str("--playbook-id", "playbookId", "only cases started from this playbook"), csv("--lang", "preferredLangs", "preferred ISO 639-1 languages, in order"))...)
	accessGet := apiOperation("case access get", "get a case's public/private setting and work collaborators", []string{"case-id"}, http.MethodGet, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/access" }, requestNone, false)
	accessSet := lockedMutation("case access set", "set public/private visibility and work collaborators", "case-id", http.MethodPut, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/access" }, requiredOption(choice("--visibility", "visibility", "case visibility", "private", "public")), csv("--editors", "editors", "comma-separated work collaborator principals"))
	share := apiOperation("case share", "create a share link that redirects to the case page; case ACLs still apply", []string{"case-id"}, http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/share" }, requestBody, true)
	assign := lockedMutation("case assign", "change which account is responsible for a case", "case-id", http.MethodPatch, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/assignee" }, requiredOption(str("--assigned-to", "assignedTo", "account responsible for the case")))
	acl := lockedMutation("case acl", "replace a case ACL", "case-id", http.MethodPatch, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/acl" }, requiredOption(csv("--acl", "acl", "comma-separated ACL principals")))
	update := lockedMutation("case update", "update editable case fields; any work collaborator can retitle or change autoReview", "case-id", http.MethodPatch, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }, str("--title", "title", "case title"), boolOption("--auto-review", "autoReview", "when set, turn OUTPUT reviews on or off; charges still use the case billing account captured at start"))
	review := apiOperation("case review", "queue an Epismo AI review of this open case's shared records and tasks and return immediately. Any case editor may request it. Optional --prompt adds caller guidance after the fixed review rules. Poll case record list with kinds=review and origins=system; replay the same idempotency key after completion to receive the REVIEW record. Distinct from a human- or agent-authored review record and from an APPROVAL task. Credits are charged to the case billing account captured at start", []string{"case-id"}, http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/review" }, requestBody, true, str("--prompt", "prompt", "optional caller guidance appended to the Epismo AI review instructions"))
	overview := apiOperation("case overview", "ask Epismo AI for a one-shot situational overview of this case (status, progress, blockers, next focus). Any case editor may request it. Runs synchronously and returns the brief in the response. Replay the same idempotency key to receive the prior brief without regenerating. Distinct from case review. Credits are charged to the case billing account captured at start", []string{"case-id"}, http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/overview" }, requestBody, true)
	close := lockedMutation("case close", "close a case", "case-id", http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/close" }, choice("--outcome", "outcome", "case outcome", "completed", "cancelled", "abandoned"), array("--records", "records", "JSON array of final records"))
	reopen := lockedMutation("case reopen", "reopen a closed case", "case-id", http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/reopen" })
	return []*command{start, get, list, popular, accessGet, accessSet, share, assign, acl, update, review, overview, close, reopen}
}

func caseHandoffCommands() []*command {
	handoff := apiOperation("case handoff", "connect two distinct efforts so one case supplies context to another; switching agents on the same effort only needs case get", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/handoffs"
	}, requestBody, true,
		requiredOptionUnless(str("--to-case-id", "toCaseId", "case receiving work from case-id"), "--from-case-id"),
		str("--from-case-id", "_fromCaseId", "case handing work off to case-id; mutually exclusive with --to-case-id"),
	)
	handoff.Prepare = prepareCaseHandoff
	graph := publicApiOperation("case handoff graph", "get a case handoff graph, or only the directly connected cases", []string{"case-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/handoff-graph"
	}, requestQuery, choice("--scope", "scope", "handoff scope", "self", "ancestors", "descendants", "neighbors", "connected"))
	candidates := apiOperation("case handoff candidate list", "list open cases this case may still be connected to", []string{"case-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/handoff-candidates"
	}, requestQuery, false, append(pagingOptions(), choice("--direction", "direction", "outgoing hands this case off; incoming receives a handoff", "outgoing", "incoming"))...)
	return []*command{handoff, graph, candidates}
}

func prepareCaseHandoff(inv invocation) (invocation, error) {
	hasTo := inv.Present["toCaseId"]
	hasFrom := inv.Present["_fromCaseId"]
	if hasTo && hasFrom {
		return invocation{}, &Error{
			Code:     "INVALID_ARGUMENT",
			Message:  "Options --to-case-id and --from-case-id are mutually exclusive.",
			Hint:     "Specify exactly one handoff direction.",
			ExitCode: 1,
		}
	}
	if !hasFrom {
		return inv, nil
	}

	// The API is anchored at the source case. For the reverse CLI form, move the
	// explicit source into the path and the positional case into the request body.
	values := make(map[string]any, len(inv.Values))
	for field, value := range inv.Values {
		values[field] = value
	}
	present := make(map[string]bool, len(inv.Present))
	for field, value := range inv.Present {
		present[field] = value
	}
	positionals := append([]string(nil), inv.Positionals...)
	positionals[0] = inv.text("_fromCaseId")
	values["toCaseId"] = inv.positional(0)
	present["toCaseId"] = true
	delete(values, "_fromCaseId")
	delete(present, "_fromCaseId")

	inv.Positionals = positionals
	inv.Values = values
	inv.Present = present
	return inv, nil
}

func caseTaskCommands() []*command {
	create := apiOperation("case task create", "create a work or approval task in a case", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/tasks"
	}, requestBody, true, choice("--kind", "kind", "task kind", "work", "approval"), str("--title", "title", "task title"), str("--instructions", "instructions", "task instructions"), str("--source-step-id", "sourceStepId", "four-character step ID"), str("--assigned-to", "assignedTo", "task assignee"), str("--subject-record-id", "subjectRecordId", "record this APPROVAL task is checking"))
	listOptions := append(pagingOptions(), choice("--status", "status", "task status", "open", "closed"))
	list := apiOperation("case task list", "list tasks in a case", []string{"case-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/tasks"
	}, requestQuery, false, listOptions...)
	return []*command{create, list}
}

func caseRecordCommands() []*command {
	appendRecord := apiOperation("case record append", "save decisions, findings, feedback, or continuation context in an open case. kind is closed: note (commentary, a decision, or a handoff), output (a durable deliverable; appending one can trigger an Epismo AI review), or review (a verdict; data.verdict must be pass, changes_requested, or insufficient). activity is server-only. Use case review for a billed Epismo AI verdict (origin=system). Include goal, constraints, agreed decisions versus proposals, open questions, artifact references, and next step as relevant. Do not save secrets or raw transcripts", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/records"
	}, requestBody, true, str("--task-id", "taskId", "task this record belongs to"), str("--source-step-id", "sourceStepId", "four-character step ID"), choice("--kind", "kind", "record kind", "note", "output", "review"), str("--content", "content", "record content"), object("--data", "data", "record data JSON object"), choice("--origin", "origin", "record origin", "user", "agent"), str("--client-name", "clientName", "client name"), str("--client-version", "clientVersion", "client version"))
	listOptions := append(pagingOptions(), choice("--scope", "scope", "handoff scope", "self", "ancestors", "descendants", "neighbors", "connected"), str("--task-id", "taskId", "task ID"), str("--created-by", "createdBy", "creator Account ID or me"), csv("--kinds", "kinds", "comma-separated record kinds (note, output, review, activity)"), csv("--origins", "origins", "comma-separated origins"), csv("--acl", "acl", "ACL principals"), defaultOption(choice("--order", "order", "sort order", "asc", "desc"), "desc"))
	list := publicApiOperation("case record list", "read saved decisions and progress in a case; follow pagination for additional context and distinguish current decisions from superseded proposals", []string{"case-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/records"
	}, requestQuery, listOptions...)
	update := apiOperation("case record update", "edit a record you created. Kind, content, and structured data may change; task and provenance stay fixed", []string{"record-id"}, http.MethodPatch, func(i invocation) string {
		return "/v1/records/" + escaped(i.positional(0))
	}, requestBody, true, choice("--kind", "kind", "record kind", "note", "output", "review"), str("--content", "content", "record content"), object("--data", "data", "record data JSON object"))
	deleteRecord := apiOperation("case record delete", "redact a record you created. The id remains as a tombstone for references", []string{"record-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/records/" + escaped(i.positional(0))
	}, requestBody, true)
	return []*command{appendRecord, list, update, deleteRecord}
}

func lockedMutation(path, summary, arg, method string, endpoint func(invocation) string, options ...optionSpec) *command {
	options = append(options, requiredOption(integer("--lock-version", "expectedLockVersion", "expected lock version")))
	return apiOperation(path, summary, []string{arg}, method, endpoint, requestBody, true, options...)
}

func taskCommands() []*command {
	create := apiOperation("task create", "create a work or approval task in a case", nil, http.MethodPost, func(i invocation) string {
		caseID := strings.TrimSpace(i.text("caseId"))
		return "/v1/cases/" + escaped(caseID) + "/tasks"
	}, requestBody, true, str("--case-id", "caseId", "case ID"), choice("--kind", "kind", "task kind", "work", "approval"), str("--title", "title", "task title"), str("--instructions", "instructions", "task instructions"), str("--source-step-id", "sourceStepId", "four-character step ID"), str("--assigned-to", "assignedTo", "task assignee"), str("--subject-record-id", "subjectRecordId", "record this APPROVAL task is checking"))
	create.Prepare = requireField("caseId", "Pass --case-id <case-id> or use `epismo case task create <case-id>`.")
	listOptions := append(pagingOptions(), str("--case-id", "caseId", "case ID"), str("--assigned-to", "assignedTo", "assignee Account ID or me"), choice("--status", "status", "task status", "open", "closed"))
	list := &command{Path: "task list", Summary: "list your assigned tasks across cases", Options: listOptions, Input: &inputSpec{Help: "query-parameters JSON object, @file, or - for stdin"}, Run: func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(stringField(payload, "caseId")) == "" && strings.TrimSpace(stringField(payload, "assignedTo")) == "" {
			payload["assignedTo"] = "me"
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		return pagedRequest(a, http.MethodGet, "/v1/tasks", ctx, payload)
	}}
	get := apiOperation("task get", "get a task", []string{"task-id"}, http.MethodGet, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) }, requestNone, false)
	update := lockedMutation("task update", "update editable task fields, including the assignee", "task-id", http.MethodPatch, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) }, str("--title", "title", "task title"), str("--instructions", "instructions", "task instructions"), str("--assigned-to", "assignedTo", "account responsible for the task; empty string unassigns"))
	setStatus := lockedMutation("task set status", "close a task with an outcome, or reopen a closed one", "task-id", http.MethodPost, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) + "/status" }, requiredOption(choice("--status", "status", "task status", "open", "closed")), str("--outcome", "outcome", "required when closing"), array("--records", "records", "JSON array of records to append with the transition"))
	return []*command{create, list, get, update, setStatus}
}

func recordCommands() []*command {
	appendRecord := apiOperation("record append", "append a record to a case", nil, http.MethodPost, func(i invocation) string {
		caseID := strings.TrimSpace(i.text("caseId"))
		return "/v1/cases/" + escaped(caseID) + "/records"
	}, requestBody, true, str("--case-id", "caseId", "case ID"), str("--task-id", "taskId", "task this record belongs to"), str("--source-step-id", "sourceStepId", "four-character step ID"), choice("--kind", "kind", "record kind", "note", "output", "review"), str("--content", "content", "record content"), object("--data", "data", "record data JSON object"), choice("--origin", "origin", "record origin", "user", "agent"), str("--client-name", "clientName", "client name"), str("--client-version", "clientVersion", "client version"))
	appendRecord.Prepare = requireField("caseId", "Pass --case-id <case-id> or use `epismo case record append <case-id>`.")
	listOptions := append(pagingOptions(), str("--case-id", "caseId", "case ID"), choice("--scope", "scope", "handoff scope", "self", "ancestors", "descendants", "neighbors", "connected"), str("--task-id", "taskId", "task ID"), str("--created-by", "createdBy", "creator Account ID or me"), csv("--kinds", "kinds", "comma-separated record kinds (note, output, review, activity)"), csv("--origins", "origins", "comma-separated origins"), csv("--acl", "acl", "ACL principals"), defaultOption(choice("--order", "order", "sort order", "asc", "desc"), "desc"))
	list := apiOperation("record list", "list records in a case", nil, http.MethodGet, func(i invocation) string {
		caseID := strings.TrimSpace(i.text("caseId"))
		return "/v1/cases/" + escaped(caseID) + "/records"
	}, requestQuery, false, listOptions...)
	list.Prepare = requireField("caseId", "Pass --case-id <case-id> or use `epismo case record list <case-id>`.")
	update := apiOperation("record update", "edit a record you created. Kind, content, and structured data may change; task and provenance stay fixed", []string{"record-id"}, http.MethodPatch, func(i invocation) string {
		return "/v1/records/" + escaped(i.positional(0))
	}, requestBody, true, choice("--kind", "kind", "record kind", "note", "output", "review"), str("--content", "content", "record content"), object("--data", "data", "record data JSON object"))
	deleteRecord := apiOperation("record delete", "redact a record you created. The id remains as a tombstone for references", []string{"record-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/records/" + escaped(i.positional(0))
	}, requestBody, true)
	return []*command{appendRecord, list, update, deleteRecord}
}

// requireField reports an error naming the given hint when field is empty.
// The leading-positional-to-option conversion for these convenience commands
// already happens in normalizeConvenienceArgs before parsing, so by the time
// a command's Prepare hook runs, the option is either already populated or
// genuinely missing.
func requireField(field, hint string) func(invocation) (invocation, error) {
	return func(inv invocation) (invocation, error) {
		if strings.TrimSpace(inv.text(field)) == "" {
			return invocation{}, &Error{
				Code:     "INVALID_INPUT",
				Message:  fmt.Sprintf(`"%s" is required.`, field),
				Hint:     hint,
				ExitCode: 1,
			}
		}
		return inv, nil
	}
}

func suggestionCommands() []*command {
	create := apiOperation("suggestion create", "propose a change to a playbook against an immutable base version", nil, http.MethodPost, func(i invocation) string {
		playbookID := strings.TrimSpace(i.text("playbookId"))
		return "/v1/playbooks/" + escaped(playbookID) + "/suggestions"
	}, requestBody, true, str("--playbook-id", "playbookId", "playbook ID"), str("--base-version-id", "baseVersionId", "base version ID"), str("--target-step-id", "targetStepId", "target step ID"), str("--title", "title", "suggestion title"), str("--content", "content", "suggestion content"))
	create.Prepare = requireField("playbookId", "Pass --playbook-id <playbook-id> or use `epismo playbook suggestion create <playbook-id>`.")
	get := apiOperation("suggestion get", "get a suggestion and its resolution state", []string{"suggestion-id"}, http.MethodGet, func(i invocation) string { return "/v1/suggestions/" + escaped(i.positional(0)) }, requestNone, false)
	listOptions := append(pagingOptions(), str("--playbook-id", "playbookId", "playbook ID"), str("--author-id", "authorId", "author Account ID or me"), choice("--view", "view", "suggestion view (inbox or sent)", "inbox", "sent"), csv("--statuses", "statuses", "comma-separated statuses"))
	list := apiOperation("suggestion list", "list suggestions across playbooks", nil, http.MethodGet, staticEndpoint("/v1/suggestions"), requestQuery, false, listOptions...)
	listRun := list.Run
	list.Run = func(a *app, inv invocation) (any, error) {
		if !inv.Present["playbookId"] && !inv.Present["authorId"] && !inv.Present["view"] {
			inv.Values["authorId"], inv.Present["authorId"] = "me", true
		}
		return listRun(a, inv)
	}
	update := apiOperation("suggestion update", "edit your own open suggestion", []string{"suggestion-id"}, http.MethodPatch, func(i invocation) string { return "/v1/suggestions/" + escaped(i.positional(0)) }, requestBody, true, str("--title", "title", "suggestion title"), str("--content", "content", "suggestion content"))
	resolve := apiOperation("suggestion resolve", "apply, decline, archive, or reopen a suggestion", []string{"suggestion-id"}, http.MethodPost, func(i invocation) string { return "/v1/suggestions/" + escaped(i.positional(0)) + "/resolve" }, requestBody, true, choice("--status", "status", "resolution status", "open", "applied", "declined", "archived"), str("--result-version-id", "resultVersionId", "version published from it"))
	return []*command{create, get, list, update, resolve}
}

func playbookSuggestionCommands() []*command {
	create := apiOperation("playbook suggestion create", "propose a change to a playbook against an immutable base version", []string{"playbook-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/suggestions"
	}, requestBody, true, str("--base-version-id", "baseVersionId", "base version ID"), str("--target-step-id", "targetStepId", "target step ID"), str("--title", "title", "suggestion title"), str("--content", "content", "suggestion content"))
	listOptions := append(pagingOptions(), str("--author-id", "authorId", "author Account ID or me"), choice("--view", "view", "suggestion view (inbox or sent)", "inbox", "sent"), csv("--statuses", "statuses", "comma-separated statuses"))
	list := apiOperation("playbook suggestion list", "list suggestions for a playbook", []string{"playbook-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/suggestions"
	}, requestQuery, false, listOptions...)
	return []*command{create, list}
}
