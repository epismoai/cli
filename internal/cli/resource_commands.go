package cli

import (
	"net/http"
	"strings"
)

func playbookCommands() []*command {
	page := pagingOptions()
	init := &command{Path: "playbook init", Summary: "print a minimal Playbook Definition template", Options: []optionSpec{str("--title", "title", "initial Playbook title"), choice("--category", "category", "initial category", "productivity", "programming", "design", "sales", "marketing", "operations", "learning")}, Examples: []string{"epismo playbook init --title Onboarding > playbook.json", "epismo playbook create --definition @playbook.json"}, Run: func(_ *app, inv invocation) (any, error) {
		definition := map[string]any{"title": inv.text("title"), "steps": []any{}}
		if definition["title"] == "" {
			definition["title"] = "Untitled Playbook"
		}
		if category := inv.text("category"); category != "" {
			definition["category"] = category
		}
		return definition, nil
	}}
	search := apiOperation("playbook search", "search readable Playbooks, including pb: alias references", nil, http.MethodGet, staticEndpoint("/v1/playbooks"), requestQuery, false, append(page, str("--query", "query", "full-text query or pb: alias reference"), choice("--category", "category", "Playbook category", "productivity", "programming", "design", "sales", "marketing", "operations", "learning"), csv("--lang", "preferredLangs", "comma-separated two-letter content languages in priority order"))...)
	list := apiOperation("playbook list", "list readable Playbooks, most recently updated first", nil, http.MethodGet, staticEndpoint("/v1/playbooks"), requestQuery, false, append(page, choice("--resource-kind", "resourceKind", "resource kind", "skill", "mcp", "cli", "api", "plugin", "graph", "document", "agent", "custom"), str("--resource-ref", "resourceRef", "normalized or provider-specific resource reference"))...)
	resourceList := apiOperation("playbook resource list", "list resource references used by readable Playbooks", nil, http.MethodGet, staticEndpoint("/v1/playbook-resources"), requestQuery, false, choice("--kind", "kind", "resource kind", "skill", "mcp", "cli", "api", "plugin", "graph", "document", "agent", "custom"), integer("--page-size", "pageSize", "results per page (1-200)"))
	create := apiOperation("playbook create", "create a Playbook and its first Version", nil, http.MethodPost, staticEndpoint("/v1/playbooks"), requestBody, true, str("--owner-id", "ownerId", "owner Account ID"), defaultOption(choice("--visibility", "visibility", "published visibility", "private", "public"), "private"), csv("--editors", "editors", "comma-separated editor Account or Team IDs"), objectSource("--definition", "definition", "Playbook Definition JSON object"))
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
	get := &command{Path: "playbook get", Summary: "get a Playbook and its latest immutable Version", Args: []string{"playbook-id-or-ref"}, Run: func(a *app, inv invocation) (any, error) {
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		id, err := resolvePlaybookID(a, ctx, inv.positional(0))
		if err != nil {
			return nil, err
		}
		return a.client.request(http.MethodGet, withWorkspace("/v1/playbooks/"+escaped(id), ctx.WorkspaceID), ctx.Auth.AccessToken, nil)
	}}
	versionList := apiOperation("playbook version list", "list Versions", []string{"playbook-id"}, http.MethodGet, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions" }, requestQuery, false, page...)
	versionGet := apiOperation("playbook version get", "get one immutable Version", []string{"playbook-id", "version-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions/" + escaped(i.positional(1))
	}, requestNone, false)
	versionArchive := apiOperation("playbook version archive", "archive a non-latest Version", []string{"playbook-id", "version-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions/" + escaped(i.positional(1))
	}, requestBody, true)
	versionPublish := apiOperation("playbook version publish", "publish a Version", []string{"playbook-id"}, http.MethodPost, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/versions" }, requestBody, true, str("--base-version-id", "baseVersionId", "current latest Version ID"), objectSource("--definition", "definition", "new Playbook Definition JSON object"))
	draftGet := apiOperation("playbook draft get", "get the current Draft", []string{"playbook-id"}, http.MethodGet, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft" }, requestNone, false)
	draftSave := apiOperation("playbook draft save", "create or overwrite the Draft", []string{"playbook-id"}, http.MethodPut, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft" }, requestBody, false, defaultOption(integer("--base-revision", "baseRevision", "revision last read (0 for a new Draft)"), 0), objectSource("--definition", "definition", "Playbook Definition JSON object"))
	draftDiscard := apiOperation("playbook draft discard", "discard the Draft", []string{"playbook-id"}, http.MethodDelete, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft" }, requestBody, true)
	draftPublish := apiOperation("playbook draft publish", "publish the Draft as a new Version", []string{"playbook-id"}, http.MethodPost, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/draft/publish" }, requestBody, true, requiredOption(integer("--expected-draft-revision", "expectedDraftRevision", "Draft revision last read and reviewed")))
	accessGet := apiOperation("playbook access get", "get public/private visibility and explicit editors", []string{"playbook-id"}, http.MethodGet, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/access" }, requestNone, false)
	accessSet := apiOperation("playbook access set", "set public/private visibility and explicit editors", []string{"playbook-id"}, http.MethodPut, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/access" }, requestBody, true, requiredOption(choice("--visibility", "visibility", "published visibility", "private", "public")), csv("--editors", "editors", "comma-separated editor Account or Team IDs"))
	archive := apiOperation("playbook archive", "archive a Playbook", []string{"playbook-id"}, http.MethodDelete, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) }, requestBody, true)
	star := apiOperation("playbook star", "star a Playbook", []string{"playbook-id"}, http.MethodPut, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/star" }, requestBody, true)
	unstar := apiOperation("playbook unstar", "remove a Playbook star", []string{"playbook-id"}, http.MethodDelete, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/star" }, requestBody, true)
	starred := apiOperation("playbook starred", "list your starred Playbooks", nil, http.MethodGet, staticEndpoint("/v1/me/starred-playbooks"), requestQuery, false, page...)
	share := apiOperation("playbook share", "create a share link", []string{"playbook-id"}, http.MethodPost, func(i invocation) string { return "/v1/playbooks/" + escaped(i.positional(0)) + "/share" }, requestBody, true)
	return []*command{init, search, list, resourceList, create, get, versionList, versionGet, versionArchive, versionPublish, draftGet, draftSave, draftDiscard, draftPublish, accessGet, accessSet, archive, star, unstar, starred, share}
}

func aliasCommands() []*command {
	set := &command{Path: "playbook alias set", Summary: "set or rename a Playbook alias in the active namespace", Args: []string{"playbook-id", "alias"}, Options: []optionSpec{str("--owner-id", "ownerId", "alias owner Account ID")}, Input: &inputSpec{}, Safety: commandSafety{DryRun: true, IdempotencyKey: true}, Run: func(a *app, inv invocation) (any, error) {
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
	list := &command{Path: "playbook alias list", Summary: "list aliases for a Playbook", Args: []string{"playbook-id"}, Options: []optionSpec{str("--owner-id", "ownerId", "filter by alias owner Account ID")}, Input: &inputSpec{}, Run: func(a *app, inv invocation) (any, error) {
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
	start := apiOperation("case start", "start a Playbook or ad-hoc Case", nil, http.MethodPost, staticEndpoint("/v1/cases"), requestBody, true, str("--version-id", "versionId", "Playbook Version ID"), str("--title", "title", "Case title"), object("--case-input", "input", "Case input JSON object"), csv("--acl", "acl", "comma-separated ACL principals"))
	get := apiOperation("case get", "get a Case with its status, ACL, and lock version", []string{"case-id"}, http.MethodGet, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }, requestNone, false)
	list := apiOperation("case list", "list readable Cases", nil, http.MethodGet, staticEndpoint("/v1/cases"), requestQuery, false, append(pagingOptions(), str("--assigned-to", "assignedTo", "only Cases assigned to this account"), choice("--status", "status", "Case status", "open", "closed"))...)
	assign := lockedMutation("case assign", "hand overall responsibility for a Case", "case-id", http.MethodPatch, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/assignee" }, requiredOption(str("--assigned-to", "assignedTo", "account responsible for the Case")))
	acl := lockedMutation("case acl", "replace a Case ACL", "case-id", http.MethodPatch, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/acl" }, requiredOption(csv("--acl", "acl", "comma-separated ACL principals")))
	update := lockedMutation("case update", "update editable Case fields", "case-id", http.MethodPatch, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }, str("--title", "title", "Case title"))
	close := lockedMutation("case close", "close a Case", "case-id", http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/close" }, choice("--outcome", "outcome", "Case outcome", "completed", "cancelled", "abandoned"), array("--records", "records", "JSON array of final Records"))
	reopen := lockedMutation("case reopen", "reopen a closed Case", "case-id", http.MethodPost, func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) + "/reopen" })
	return []*command{start, get, list, assign, acl, update, close, reopen}
}

func caseTaskCommands() []*command {
	create := apiOperation("case task create", "create a work or review Task in a Case", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/tasks"
	}, requestBody, true, choice("--kind", "kind", "Task kind", "work", "review"), str("--title", "title", "Task title"), str("--instructions", "instructions", "Task instructions"), str("--source-step-id", "sourceStepId", "four-character Step ID"), str("--assigned-to", "assignedTo", "Task assignee"), str("--subject-record-id", "subjectRecordId", "Record under review"))
	listOptions := append(pagingOptions(), choice("--status", "status", "Task status", "open", "closed"))
	list := apiOperation("case task list", "list Tasks in a Case", []string{"case-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/tasks"
	}, requestQuery, false, listOptions...)
	return []*command{create, list}
}

func caseRecordCommands() []*command {
	appendRecord := apiOperation("case record append", "append an immutable Record to a Case", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/records"
	}, requestBody, true, str("--task-id", "taskId", "Task this Record belongs to"), str("--source-step-id", "sourceStepId", "four-character Step ID"), str("--kind", "kind", "Record kind"), str("--content", "content", "Record content"), object("--data", "data", "Record data JSON object"), choice("--origin", "origin", "Record origin", "user", "agent"), str("--client-name", "clientName", "client name"), str("--client-version", "clientVersion", "client version"))
	listOptions := append(pagingOptions(), str("--task-id", "taskId", "Task ID"), str("--created-by", "createdBy", "creator Account ID or me"), csv("--kinds", "kinds", "comma-separated Record kinds"), csv("--origins", "origins", "comma-separated origins"), csv("--acl", "acl", "ACL principals"), defaultOption(choice("--order", "order", "sort order", "asc", "desc"), "desc"))
	list := apiOperation("case record list", "list Records in a Case", []string{"case-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/records"
	}, requestQuery, false, listOptions...)
	return []*command{appendRecord, list}
}

func lockedMutation(path, summary, arg, method string, endpoint func(invocation) string, options ...optionSpec) *command {
	options = append(options, requiredOption(integer("--lock-version", "expectedLockVersion", "expected lock version")))
	return apiOperation(path, summary, []string{arg}, method, endpoint, requestBody, true, options...)
}

func taskCommands() []*command {
	listOptions := append(pagingOptions(), str("--case-id", "caseId", "Case ID"), str("--assigned-to", "assignedTo", "assignee Account ID or me"), choice("--status", "status", "Task status", "open", "closed"))
	list := &command{Path: "task list", Summary: "list your assigned Tasks across Cases", Options: listOptions, Input: &inputSpec{Help: "query-parameters JSON object, @file, or - for stdin"}, Run: func(a *app, inv invocation) (any, error) {
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
	get := apiOperation("task get", "get a Task", []string{"task-id"}, http.MethodGet, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) }, requestNone, false)
	assign := lockedMutation("task assign", "assign a Task", "task-id", http.MethodPatch, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) + "/assignee" }, str("--assigned-to", "assignedTo", "omit in --input to clear assignment"))
	update := lockedMutation("task update", "update editable Task fields", "task-id", http.MethodPatch, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) }, str("--title", "title", "Task title"), str("--instructions", "instructions", "Task instructions"))
	close := lockedMutation("task close", "close a Task with an outcome", "task-id", http.MethodPost, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) + "/close" }, requiredOption(str("--outcome", "outcome", "free-text result of the Task")), array("--records", "records", "JSON array of final Records"))
	reopen := lockedMutation("task reopen", "reopen a closed Task", "task-id", http.MethodPost, func(i invocation) string { return "/v1/tasks/" + escaped(i.positional(0)) + "/reopen" })
	return []*command{list, get, assign, update, close, reopen}
}

func suggestionCommands() []*command {
	get := apiOperation("suggestion get", "get a Suggestion and its resolution state", []string{"suggestion-id"}, http.MethodGet, func(i invocation) string { return "/v1/suggestions/" + escaped(i.positional(0)) }, requestNone, false)
	listOptions := append(pagingOptions(), str("--playbook-id", "playbookId", "Playbook ID"), str("--author-id", "authorId", "author Account ID or me"), csv("--statuses", "statuses", "comma-separated statuses"))
	list := apiOperation("suggestion list", "list Suggestions you sent", nil, http.MethodGet, staticEndpoint("/v1/suggestions"), requestQuery, false, listOptions...)
	listRun := list.Run
	list.Run = func(a *app, inv invocation) (any, error) {
		if !inv.Present["playbookId"] && !inv.Present["authorId"] {
			inv.Values["authorId"], inv.Present["authorId"] = "me", true
		}
		return listRun(a, inv)
	}
	update := apiOperation("suggestion update", "edit your own open Suggestion", []string{"suggestion-id"}, http.MethodPatch, func(i invocation) string { return "/v1/suggestions/" + escaped(i.positional(0)) }, requestBody, true, str("--title", "title", "Suggestion title"), str("--content", "content", "Suggestion content"))
	resolve := apiOperation("suggestion resolve", "apply, decline, archive, or reopen a Suggestion", []string{"suggestion-id"}, http.MethodPost, func(i invocation) string { return "/v1/suggestions/" + escaped(i.positional(0)) + "/resolve" }, requestBody, true, choice("--status", "status", "resolution status", "open", "applied", "declined", "archived"), str("--result-version-id", "resultVersionId", "Version published from it"))
	return []*command{get, list, update, resolve}
}

func playbookSuggestionCommands() []*command {
	create := apiOperation("playbook suggestion create", "propose a change to a Playbook against an immutable base Version", []string{"playbook-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/suggestions"
	}, requestBody, true, str("--base-version-id", "baseVersionId", "base Version ID"), str("--target-step-id", "targetStepId", "target Step ID"), str("--title", "title", "Suggestion title"), str("--content", "content", "Suggestion content"))
	listOptions := append(pagingOptions(), str("--author-id", "authorId", "author Account ID or me"), csv("--statuses", "statuses", "comma-separated statuses"))
	list := apiOperation("playbook suggestion list", "list Suggestions for a Playbook", []string{"playbook-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/playbooks/" + escaped(i.positional(0)) + "/suggestions"
	}, requestQuery, false, listOptions...)
	return []*command{create, list}
}
