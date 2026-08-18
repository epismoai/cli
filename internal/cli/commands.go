package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func buildCommands() []*command {
	commands := []*command{
		loginCommand(), logoutCommand(), whoamiCommand(), updateCommand(),
		workspaceListCommand(), workspaceCurrentCommand(), workspaceUseCommand(), workspaceClearCommand(), workspaceCreateCommand(), workspaceCheckoutCommand(), workspaceUpdateCommand(), workspaceMemberListCommand(), workspaceMemberUpsertCommand(), workspaceMemberDeleteCommand(),
		teamListCommand(), teamCreateCommand(), teamUpdateCommand(), teamMemberListCommand(), teamMemberAddCommand(), teamMemberDeleteCommand(),
		creditBalanceCommand(), creditCheckoutCommand(), tokenCreateCommand(), tokenListCommand(), tokenRevokeCommand(),
	}
	commands = append(commands, playbookCommands()...)
	commands = append(commands, aliasCommands()...)
	commands = append(commands, caseCommands()...)
	commands = append(commands, caseTaskCommands()...)
	commands = append(commands, caseRecordCommands()...)
	commands = append(commands, taskCommands()...)
	commands = append(commands, playbookSuggestionCommands()...)
	commands = append(commands, suggestionCommands()...)
	return commands
}

func loginCommand() *command {
	return &command{Path: "login", Summary: "log in through a browser, or use email with automatic SSO discovery", Input: true, Options: []optionSpec{str("--email", "email", "use company SSO when configured, otherwise enter a code here")}, Run: func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		return a.login(stringField(payload, "email"))
	}}
}

func logoutCommand() *command {
	return &command{Path: "logout", Summary: "revoke credentials and clear ~/.epismo/credentials", Run: func(a *app, _ invocation) (any, error) { return a.logout() }}
}

func whoamiCommand() *command {
	return &command{Path: "whoami", Summary: "show the currently authenticated user", Run: func(a *app, _ invocation) (any, error) {
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		user, err := a.userInfo(ctx.Auth.AccessToken)
		if err != nil {
			return nil, err
		}
		workspaces, err := a.workspaces(ctx.Auth.AccessToken)
		if err != nil {
			return nil, err
		}
		config := cliConfig{}
		if strings.TrimSpace(os.Getenv("EPISMO_TOKEN")) == "" {
			config, err = readConfig()
			if err != nil {
				return nil, err
			}
		}
		defaultID := any(nil)
		if config.DefaultWorkspace != nil {
			defaultID = config.DefaultWorkspace.ID
		}
		effective := any(nil)
		if ctx.WorkspaceID != "" {
			effective = ctx.WorkspaceID
		}
		expires := any(nil)
		if ctx.Auth.ExpiresAt != "" {
			expires = ctx.Auth.ExpiresAt
		}
		handle := any(nil)
		if value := stringField(user, "handle"); value != "" {
			handle = value
		}
		accountID := any(nil)
		if value := stringField(user, "account_id"); value != "" {
			accountID = value
		}
		return map[string]any{"auth": map[string]any{"mode": "oauth", "expiresAt": expires}, "user": map[string]any{"id": stringField(user, "sub"), "email": user["email"], "username": user["username"], "handle": handle, "accountId": accountID}, "defaultWorkspaceId": defaultID, "defaultWorkspaceApplied": strings.TrimSpace(os.Getenv("EPISMO_TOKEN")) == "", "effectiveWorkspaceId": effective, "workspaces": mapsToAny(workspaces)}, nil
	}}
}

func mapsToAny(items []map[string]any) []any {
	result := make([]any, len(items))
	for i := range items {
		result[i] = items[i]
	}
	return result
}

func workspaceListCommand() *command {
	return &command{Path: "workspace list", Summary: "list accessible workspaces", Run: func(a *app, _ invocation) (any, error) {
		auth, err := a.resolveAuthentication()
		if err != nil {
			return nil, err
		}
		items, err := a.workspaces(auth.AccessToken)
		if err != nil {
			return nil, err
		}
		config := cliConfig{}
		if strings.TrimSpace(os.Getenv("EPISMO_TOKEN")) == "" {
			config, err = readConfig()
			if err != nil {
				return nil, err
			}
		}
		defaultID := ""
		if config.DefaultWorkspace != nil {
			defaultID = config.DefaultWorkspace.ID
		}
		for _, item := range items {
			item["isDefault"] = stringField(item, "id") == defaultID
		}
		return map[string]any{"workspaces": mapsToAny(items)}, nil
	}}
}

func workspaceCurrentCommand() *command {
	return &command{Path: "workspace current", Summary: "show the saved default workspace", Run: func(_ *app, _ invocation) (any, error) {
		config, err := readConfig()
		if err != nil {
			return nil, err
		}
		if config.DefaultWorkspace == nil {
			return map[string]any{"currentWorkspace": nil}, nil
		}
		encoded := map[string]any{"id": config.DefaultWorkspace.ID, "isDefault": true, "source": "local-config"}
		if config.DefaultWorkspace.Handle != "" {
			encoded["handle"] = config.DefaultWorkspace.Handle
		}
		if config.DefaultWorkspace.AccountID != "" {
			encoded["accountId"] = config.DefaultWorkspace.AccountID
		}
		if config.DefaultWorkspace.Role != "" {
			encoded["role"] = config.DefaultWorkspace.Role
		}
		return map[string]any{"currentWorkspace": encoded}, nil
	}}
}

func workspaceUseCommand() *command {
	return &command{Path: "workspace use", Summary: "set the default workspace", Args: []string{"workspace-id"}, Run: func(a *app, inv invocation) (any, error) {
		auth, err := a.resolveAuthentication()
		if err != nil {
			return nil, err
		}
		items, err := a.workspaces(auth.AccessToken)
		if err != nil {
			return nil, err
		}
		requested := strings.TrimSpace(inv.positional(0))
		var matched map[string]any
		for _, item := range items {
			if stringField(item, "id") == requested {
				matched = item
				break
			}
		}
		if matched == nil {
			return nil, &Error{Code: "WORKSPACE_NOT_FOUND", Message: "Workspace not found or not accessible: " + requested, Hint: "Run `epismo workspace list` to see accessible workspaces.", ExitCode: 1}
		}
		config, err := readConfig()
		if err != nil {
			return nil, err
		}
		config.DefaultWorkspace = workspaceFromMap(matched)
		if err := writeConfig(config); err != nil {
			return nil, err
		}
		matched["isDefault"] = true
		return map[string]any{"workspace": matched}, nil
	}}
}

func workspaceClearCommand() *command {
	return &command{Path: "workspace clear", Summary: "clear the saved workspace (reverts to personal space)", Run: func(_ *app, _ invocation) (any, error) {
		config, err := readConfig()
		if err != nil {
			return nil, err
		}
		previous := any(nil)
		if config.DefaultWorkspace != nil {
			previous = config.DefaultWorkspace
		}
		config.DefaultWorkspace = nil
		if err := writeConfig(config); err != nil {
			return nil, err
		}
		return map[string]any{"cleared": true, "previousWorkspace": previous}, nil
	}}
}

func workspaceCreateCommand() *command {
	cmd := apiOperationUnscoped("workspace create", "create a new workspace", nil, http.MethodPost, staticEndpoint("/v1/workspaces"), requestBody, false, str("--handle", "handle", "workspace handle (URL-safe slug)"))
	oldRun := cmd.Run
	cmd.Run = func(a *app, inv invocation) (any, error) {
		created, err := oldRun(a, inv)
		if err != nil {
			return nil, err
		}
		object := created.(map[string]any)
		workspace, _ := object["workspace"].(map[string]any)
		id := stringField(workspace, "id")
		if id == "" {
			return object, nil
		}
		ctx, contextErr := a.context()
		if contextErr != nil {
			normalized := normalizeError(contextErr)
			object["checkoutError"] = map[string]any{"code": normalized.Code, "message": normalized.Message, "retryable": normalized.Retryable}
			object["hint"] = fmt.Sprintf("Workspace creation succeeded. Retry checkout with `epismo workspace checkout %s`.", id)
			return object, nil
		}
		checkout, checkoutErr := a.client.request(http.MethodPost, "/v1/workspaces/"+escaped(id)+"/checkout", ctx.Auth.AccessToken, nil)
		if checkoutErr != nil {
			normalized := normalizeError(checkoutErr)
			object["checkoutError"] = map[string]any{"code": normalized.Code, "message": normalized.Message, "retryable": normalized.Retryable}
			object["hint"] = fmt.Sprintf("Workspace creation succeeded. Retry checkout with `epismo workspace checkout %s`.", id)
			return object, nil
		}
		for key, value := range checkout {
			object[key] = value
		}
		return object, nil
	}
	return cmd
}

func workspaceCheckoutCommand() *command {
	return apiOperationUnscoped("workspace checkout", "get the billing URL for a workspace", []string{"workspace-id"}, http.MethodPost, func(i invocation) string { return "/v1/workspaces/" + escaped(i.positional(0)) + "/checkout" }, requestNone, false)
}
func workspaceUpdateCommand() *command {
	return apiOperationUnscoped("workspace update", "update a workspace you own", []string{"workspace-id"}, http.MethodPatch, func(i invocation) string { return "/v1/workspaces/" + escaped(i.positional(0)) }, requestBody, false, str("--handle", "handle", "updated workspace handle"))
}

func workspaceIDOption() optionSpec {
	return str("--workspace-id", "workspaceId", "workspace id (defaults to the saved workspace or token scope when supported)")
}

func workspaceMemberListCommand() *command {
	return &command{Path: "workspace member list", Summary: "list members in a workspace", Options: []optionSpec{workspaceIDOption()}, Run: func(a *app, inv invocation) (any, error) {
		return selectedWorkspaceRequest(a, inv.text("workspaceId"), http.MethodGet, "/members", nil)
	}}
}

func selectedWorkspaceRequest(a *app, workspaceID, method, suffix string, body any) (map[string]any, error) {
	ctx, err := a.context()
	if err != nil {
		return nil, err
	}
	if workspaceID == "" {
		workspaceID = ctx.WorkspaceID
	}
	if workspaceID == "" {
		return nil, &Error{Code: "INVALID_INPUT", Message: "No workspace selected.", Hint: "Run `epismo workspace use <workspace-id>` first, or pass --workspace-id.", ExitCode: 1}
	}
	return a.client.request(method, "/v1/workspaces/"+escaped(workspaceID)+suffix, ctx.Auth.AccessToken, body)
}

func workspaceMemberUpsertCommand() *command {
	return &command{Path: "workspace member upsert", Summary: "add a workspace member or update the member role", Args: []string{"user-ids"}, Options: []optionSpec{workspaceIDOption(), requiredOption(choice("--role", "role", "owner | admin | member", "owner", "admin", "member"))}, Run: func(a *app, inv invocation) (any, error) {
		users, err := stringArray(inv.positional(0), "<user-ids>")
		if err != nil {
			return nil, err
		}
		return selectedWorkspaceRequest(a, inv.text("workspaceId"), http.MethodPut, "/members", map[string]any{"userIds": users, "role": inv.text("role")})
	}}
}
func workspaceMemberDeleteCommand() *command {
	return &command{Path: "workspace member delete", Summary: "remove a workspace member", Args: []string{"user-ids"}, Options: []optionSpec{workspaceIDOption()}, Run: func(a *app, inv invocation) (any, error) {
		users, err := stringArray(inv.positional(0), "<user-ids>")
		if err != nil {
			return nil, err
		}
		return selectedWorkspaceRequest(a, inv.text("workspaceId"), http.MethodDelete, "/members?userIds="+url.QueryEscape(strings.Join(users, ",")), nil)
	}}
}

func teamListCommand() *command {
	return apiOperation("team list", "list teams visible in the selected workspace", nil, http.MethodGet, staticEndpoint("/v1/teams"), requestNone, false)
}
func teamCreateCommand() *command {
	return apiOperation("team create", "create a team in the selected workspace", nil, http.MethodPost, staticEndpoint("/v1/teams"), requestBody, false, str("--name", "name", "team name"), str("--description", "description", "team description"))
}
func teamUpdateCommand() *command {
	return apiOperation("team update", "update a team you can access", []string{"team-id"}, http.MethodPatch, func(i invocation) string { return "/v1/teams/" + escaped(i.positional(0)) }, requestBody, false, str("--name", "name", "updated team name"), str("--description", "description", "updated team description"))
}

func teamMemberListCommand() *command {
	cmd := apiOperation("team member list", "list members across one or more teams", nil, http.MethodPost, staticEndpoint("/v1/teams/members/list"), requestBody, false, csv("--team-ids", "teamIds", "JSON array or comma-separated team ids"))
	old := cmd.Run
	cmd.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ids, err := stringArray(payload["teamIds"], "teamIds")
		if err != nil || len(ids) == 0 {
			return nil, &Error{Code: "INVALID_INPUT", Message: `"teamIds" is required.`, Hint: "Pass --team-ids <id1,id2>.", ExitCode: 1}
		}
		_ = old
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		return a.client.request(http.MethodPost, withWorkspace("/v1/teams/members/list", ctx.WorkspaceID), ctx.Auth.AccessToken, map[string]any{"teamIds": ids})
	}
	return cmd
}

func teamMemberAddCommand() *command    { return teamMemberMutation("add", http.MethodPut) }
func teamMemberDeleteCommand() *command { return teamMemberMutation("delete", http.MethodDelete) }
func teamMemberMutation(name, method string) *command {
	return &command{Path: "team member " + name, Summary: name + " member(s) in a team", Args: []string{"user-ids"}, Options: []optionSpec{requiredOption(str("--team-id", "teamId", "team id")), workspaceIDOption()}, Run: func(a *app, inv invocation) (any, error) {
		users, err := stringArray(inv.positional(0), "<user-ids>")
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		workspaceID := strings.TrimSpace(inv.text("workspaceId"))
		if workspaceID == "" {
			workspaceID = ctx.WorkspaceID
		}
		if workspaceID == "" && strings.TrimSpace(os.Getenv("EPISMO_TOKEN")) == "" {
			return nil, &Error{Code: "INVALID_INPUT", Message: "No workspace selected.", Hint: "Run `epismo workspace use <workspace-id>` first, or pass --workspace-id.", ExitCode: 1}
		}
		path := "/v1/teams/" + escaped(inv.text("teamId")) + "/members"
		var body any
		if method == http.MethodPut {
			body = map[string]any{"userIds": users}
		} else {
			path += "?userIds=" + url.QueryEscape(strings.Join(users, ","))
		}
		return a.client.request(method, withWorkspace(path, workspaceID), ctx.Auth.AccessToken, body)
	}}
}

func creditBalanceCommand() *command {
	return apiOperation("credit balance", "show current credit balance", nil, http.MethodGet, staticEndpoint("/v1/credits"), requestNone, false)
}
func creditCheckoutCommand() *command {
	cmd := apiOperation("credit checkout", "start a credit purchase and get a checkout URL", nil, http.MethodPost, staticEndpoint("/v1/credits"), requestBody, false, integer("--quantity", "quantity", "Number of credits to add"))
	cmd.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		quantity, ok := exactInteger(payload["quantity"])
		if !ok || quantity <= 0 {
			return nil, &Error{Code: "INVALID_INPUT", Message: `"quantity" is required and must be a positive integer.`, Hint: "Pass --quantity 500 or use --input @file.json.", ExitCode: 1}
		}
		if quantity < 500 {
			return nil, &Error{Code: "INVALID_INPUT", Message: "Minimum total purchase is 500 credits.", ExitCode: 1}
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		return a.client.request(http.MethodPost, withWorkspace("/v1/credits", ctx.WorkspaceID), ctx.Auth.AccessToken, map[string]any{"quantity": quantity})
	}
	return cmd
}
func tokenCreateCommand() *command {
	cmd := apiOperation("token create", "issue a workspace-scoped CLI token for CI/CD pipelines", nil, http.MethodPost, staticEndpoint("/v1/cli/tokens"), requestBody, false, str("--workspace-id", "workspaceId", "workspace to scope the token to"))
	cmd.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		workspaceID := strings.TrimSpace(stringField(payload, "workspaceId"))
		if workspaceID == "" {
			workspaceID = ctx.WorkspaceID
		}
		body := map[string]any{}
		if workspaceID != "" {
			body["workspaceId"] = workspaceID
		}
		return a.client.request(http.MethodPost, "/v1/cli/tokens", ctx.Auth.AccessToken, body)
	}
	return cmd
}

func tokenListCommand() *command {
	cmd := apiOperationUnscoped("token list", "list your issued CLI tokens", nil, http.MethodGet, staticEndpoint("/v1/cli/tokens"), requestQuery, false, str("--workspace-id", "workspaceId", "workspace the tokens were scoped to"))
	cmd.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		workspaceID := strings.TrimSpace(stringField(payload, "workspaceId"))
		if workspaceID == "" {
			workspaceID = ctx.WorkspaceID
		}
		return a.client.request(http.MethodGet, withWorkspace("/v1/cli/tokens", workspaceID), ctx.Auth.AccessToken, nil)
	}
	return cmd
}

func tokenRevokeCommand() *command {
	cmd := apiOperationUnscoped("token revoke", "revoke a CLI token by ID", []string{"token-id"}, http.MethodDelete, func(i invocation) string { return "/v1/cli/tokens/" + escaped(i.positional(0)) }, requestQuery, true, str("--workspace-id", "workspaceId", "workspace the token was scoped to"))
	cmd.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		workspaceID := strings.TrimSpace(stringField(payload, "workspaceId"))
		if workspaceID == "" {
			workspaceID = ctx.WorkspaceID
		}
		return a.client.request(http.MethodDelete, withWorkspace("/v1/cli/tokens/"+escaped(inv.positional(0)), workspaceID), ctx.Auth.AccessToken, nil)
	}
	return cmd
}
