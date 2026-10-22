package cli

import "net/http"

func reviewCommands() []*command {
	caseEndpoint := func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }
	reviewEndpoint := func(i invocation) string { return caseEndpoint(i) + "/review" }
	actionsEndpoint := func(i invocation) string { return reviewEndpoint(i) + "/actions" }

	get := apiOperation("case review get", "read the latest saved review and job state, or a selected report with --report-id; superseded reports are read-only. Requires work reading access and never calls AI", []string{"case-id"}, http.MethodGet, reviewEndpoint, requestQuery, false,
		str("--report-id", "reportId", "saved report UUID; omit for the latest review"))
	list := apiOperation("case review list", "list saved review dates, verdicts and summaries, newest first; current work access and sharing audience apply. Read a full report with case review get --report-id", []string{"case-id"}, http.MethodGet, func(i invocation) string { return reviewEndpoint(i) + "/history" }, requestQuery, false, pagingOptions()...)
	generate := apiOperation("case review generate", "queue an asynchronous case evaluation; requires case editing access. Returns immediately with HTTP 202 Accepted. Poll case review get for status and the committed report", []string{"case-id"}, http.MethodPost, reviewEndpoint, requestBody, true)
	act := apiOperation("case review act", "confirm a latest-report proposal unchanged to create a task, a case with a handoff, a suggestion, or a playbook with its first version; or dismiss with feedback. Review the full proposed contents before confirming; retries use the proposal identity. Superseded unhandled proposals return a conflict; refresh the latest review before acting", []string{"case-id"}, http.MethodPost, actionsEndpoint, requestBody, false,
		requiredOption(str("--report-id", "reportId", "saved report UUID owning the proposal")),
		requiredOption(str("--proposal-id", "proposalId", "proposal UUID within the report")),
		requiredOption(choice("--action", "action", "action to take", "create", "dismiss")),
		str("--feedback", "feedback", "optional dismissal feedback retained for future evaluations"))

	return []*command{get, list, generate, act}
}
