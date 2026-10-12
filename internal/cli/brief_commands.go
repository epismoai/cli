package cli

import "net/http"

func briefCommands() []*command {
	caseEndpoint := func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }
	briefEndpoint := func(i invocation) string { return caseEndpoint(i) + "/brief" }
	set := apiOperation("case brief set", "edit a Brief summary, DNA details, or priority, including after close; omitted fields are preserved. Pass --details {} to clear DNA or --priority none to clear priority. Priority-only and DNA-only writes are supported. An archived case cannot be changed. Costs 2 credits to the acting user's active workspace, or personal account when no workspace is selected", []string{"case-id"}, http.MethodPut, briefEndpoint, requestBody, true,
		str("--content", "content", "Optional summary sentence, up to 280 characters; omission preserves it"),
		object("--details", "details", "DNA JSON object: decisions, needs, actions; up to three {text, recordIds} items each"),
		choice("--priority", "priority", "Brief priority; none clears it, omission preserves it", "none", "urgent", "high", "normal", "low"))
	generate := apiOperation("case brief generate", "generate and save a summary, supported DNA details, and an optional Brief priority assessment, including after close; existing summary, DNA, priority, and update time are supplied to the model; null DNA or priority clears that field. Details do not grant approval or assign tasks. An archived case cannot be changed. Runs synchronously and charges token-priced credits to the case billing account. Retry the same idempotency key to receive the prior Brief without another model call", []string{"case-id"}, http.MethodPost, briefEndpoint, requestBody, true)
	deletion := apiOperation("case brief delete", "delete the Brief summary and DNA and clear its priority, including when only priority is set and after close; an archived case cannot be changed. Costs 2 credits to the acting user's active workspace, or personal account when no workspace is selected", []string{"case-id"}, http.MethodDelete, briefEndpoint, requestBody, true)
	return []*command{set, deletion, generate}
}
