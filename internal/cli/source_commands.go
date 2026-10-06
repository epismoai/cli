package cli

import "net/http"

func sourceCommands() []*command {
	link := apiOperation("source link", "link a Slack thread to an open case (2 credits); requires your connected Slack account in the active workspace. Linking the same thread to this case reuses its source and contributes your grant. Captures are shared with case work collaborators, even without Slack access; public readers cannot read them", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources"
	}, requestBody, true, requiredOption(str("--url", "url", "Slack thread permalink")))
	unlink := apiOperation("source unlink", "unlink a source from this case for all collaborators, removing its content and contributed grants without disconnecting their Slack accounts (2 credits)", []string{"case-id", "source-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1))
	}, requestBody, true)
	refresh := apiOperation("source refresh", "queue source refresh; each completed capture costs 1 credit to this case billing account, including a closed case whose explicit request it fulfills. Queuing, failures, pagination, and retries are free; coalesced requests incur one charge for this case. Inspect case get for status, checked_at, and the saved snapshot. Reading never fetches from Slack", []string{"case-id", "source-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1)) + "/refresh"
	}, requestBody, true)
	get := apiOperation("source get", "get a linked source with its latest complete saved content and status; requires case work access and does not fetch from Slack", []string{"case-id", "source-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1))
	}, requestNone, false)
	return []*command{link, unlink, refresh, get}
}
