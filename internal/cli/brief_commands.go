package cli

import "net/http"

func briefCommands() []*command {
	caseEndpoint := func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }
	briefEndpoint := func(i invocation) string { return caseEndpoint(i) + "/brief" }
	set := apiOperation("case brief set", "set or clear the Brief of an open Case; costs 1 credit to the caller's billing account", []string{"case-id"}, http.MethodPut, briefEndpoint, requestBody, true, requiredOption(str("--content", "content", "One sentence, up to 280 characters (empty clears it)")))
	refresh := apiOperation("case brief refresh", "refresh and save the Brief from current case evidence; runs synchronously and charges token-priced credits to the case billing account. Retry the same idempotency key to receive the prior Brief without another model call", []string{"case-id"}, http.MethodPost, briefEndpoint, requestBody, true)
	return []*command{set, refresh}
}
