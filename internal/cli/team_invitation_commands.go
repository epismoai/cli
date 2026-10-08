package cli

import "net/http"

func sharedTeamCommands() []*command {
	invite := apiOperation("team invite", "invite email recipients to connect their chosen workspace to a team", []string{"team-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/teams/" + escaped(i.positional(0)) + "/invitations"
	}, requestBody, false, requiredOption(csv("--emails", "emails", "comma-separated recipient emails")))
	list := apiOperation("team invitation list", "list pending team invitations", []string{"team-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/teams/" + escaped(i.positional(0)) + "/invitations"
	}, requestNone, false)
	revoke := apiOperation("team invitation revoke", "revoke a pending team invitation", []string{"team-id", "invitation-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/teams/" + escaped(i.positional(0)) + "/invitations/" + escaped(i.positional(1))
	}, requestNone, false)
	get := apiOperationUnscoped("team invitation get", "resolve an email-bound team invitation", []string{"token"}, http.MethodGet, func(i invocation) string {
		return "/v1/invitations/" + escaped(i.positional(0))
	}, requestNone, false)
	accept := apiOperation("team invitation accept", "connect the selected workspace and join the invited team", []string{"token"}, http.MethodPost, func(i invocation) string {
		return "/v1/invitations/" + escaped(i.positional(0)) + "/accept"
	}, requestNone, false)
	disconnect := apiOperation("team disconnect", "disconnect the selected workspace and remove its team participants", []string{"team-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/teams/" + escaped(i.positional(0)) + "/connection"
	}, requestNone, false)
	return []*command{invite, list, revoke, get, accept, disconnect}
}
