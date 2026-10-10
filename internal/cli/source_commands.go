package cli

import "net/http"

func caseSourceCommands() []*command {
	link := apiOperation("case source link", "link a Slack thread or Google Drive file to an open case (2 credits); requires your connected provider account in the active space. Google Drive supports personal and workspace spaces; connect and select Docs, Sheets, or Slides in Web first. Linking the same resource to this case reuses its source and contributes your grant. Captures are shared with case work collaborators, even without access to the original resource; public readers cannot read them", []string{"case-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources"
	}, requestBody, true, requiredOption(str("--url", "url", "Slack thread permalink or selected Google Docs, Sheets, or Slides URL")))
	unlink := apiOperation("case source unlink", "unlink a source from this case for all collaborators, removing its content and contributed grants without disconnecting their provider accounts (2 credits)", []string{"case-id", "source-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1))
	}, requestBody, true)
	refresh := apiOperation("case source refresh", "queue case source refresh; each completed capture costs 1 credit to this case billing account, including a closed case whose explicit request it fulfills. Queuing, failures, pagination, and retries are free; coalesced requests incur one charge for this case. Inspect case get for status, checked_at, and the saved snapshot. Reading never fetches from the provider", []string{"case-id", "source-id"}, http.MethodPost, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1)) + "/refresh"
	}, requestBody, true)
	get := apiOperation("case source get", "get a linked source with its latest complete saved content and status; requires case work access and does not fetch from the provider", []string{"case-id", "source-id"}, http.MethodGet, func(i invocation) string {
		return "/v1/cases/" + escaped(i.positional(0)) + "/sources/" + escaped(i.positional(1))
	}, requestNone, false)
	return []*command{link, unlink, refresh, get}
}
