package cli

import "net/http"

func sourceCommands() []*command {
	link := apiOperation("source link", "link a Slack thread to an open case (2 credits); requires your connected Slack account in the active workspace. Captures are shared with case work collaborators, even without Slack access; public readers cannot read them", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources"
	}, requestBody, true, requiredOption(str("--url", "url", "Slack thread permalink")))
	unlink := apiOperation("source unlink", "unlink a source from this case (2 credits)", []string{"case-id", "source-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1))
	}, requestBody, true)
	refresh := apiOperation("source refresh", "queue source refresh; each completed capture costs 1 credit per open, non-archived case sharing the source and per closed case whose explicit request it fulfills, billed to each case account. Queuing, failures, pagination, and retries are free; coalesced requests incur one charge per participating case. Inspect case get for status, checked_at, and the saved snapshot. Reading never fetches from Slack", []string{"case-id", "source-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1)) + "/refresh"
	}, requestBody, true)
	get := apiOperation("source get", "get a linked source with its latest complete saved content and status; requires case work access and does not fetch from Slack", []string{"case-id", "source-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1))
	}, requestNone, false)
	return []*command{link, unlink, refresh, get}
}
