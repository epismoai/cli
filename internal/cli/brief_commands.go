package cli

import "net/http"

func briefCommands() []*command {
	caseEndpoint := func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }
	briefEndpoint := func(i invocation) string { return caseEndpoint(i) + "/brief" }
	set := apiOperation("case brief set", "set the Brief of a case, including one that is closed; an archived case cannot be changed. An update operation costing 2 credits to the acting user's active workspace, or personal account when no workspace is selected", []string{"case-id"}, http.MethodPut, briefEndpoint, requestBody, true, requiredOption(str("--content", "content", "One sentence, up to 280 characters (must be non-empty)")))
	generate := apiOperation("case brief generate", "generate and save the Brief from case evidence, including for a closed case; an archived case cannot be changed. Runs synchronously and charges token-priced credits to the case billing account. Retry the same idempotency key to receive the prior Brief without another model call", []string{"case-id"}, http.MethodPost, briefEndpoint, requestBody, true)
	deletion := apiOperation("case brief delete", "delete the Brief of a case, including one that is closed; an archived case cannot be changed. Costs 2 credits to the acting user's active workspace, or personal account when no workspace is selected", []string{"case-id"}, http.MethodDelete, briefEndpoint, requestBody, true)
	return []*command{set, deletion, generate}
}
