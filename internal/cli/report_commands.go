package cli

import "net/http"

func reportCommands() []*command {
	caseEndpoint := func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }
	reportEndpoint := func(i invocation) string { return caseEndpoint(i) + "/report" }
	actionsEndpoint := func(i invocation) string { return reportEndpoint(i) + "/actions" }

	get := apiOperation("report case get", "read the latest saved report and job state, or a selected report with --report-id; superseded reports are read-only. Requires work reading access and never calls AI", []string{"case-id"}, http.MethodGet, reportEndpoint, requestQuery, false,
		str("--report-id", "reportId", "saved report UUID; omit for the latest report"))
	list := apiOperation("report case list", "list saved report dates, verdicts and summaries, newest first; current work access and sharing audience apply. Read a full report with report case get --report-id", []string{"case-id"}, http.MethodGet, func(i invocation) string { return reportEndpoint(i) + "/history" }, requestQuery, false, pagingOptions()...)
	generate := apiOperation("report case generate", "queue an asynchronous case evaluation; requires case editing access. Returns immediately with HTTP 202 Accepted. Poll report case get for status and the committed report", []string{"case-id"}, http.MethodPost, reportEndpoint, requestBody, true)
	act := apiOperation("report case act", "confirm a latest-report proposal unchanged to create a task, a case with a handoff, a suggestion, or a playbook with its first version; or dismiss with feedback. Review the full proposed contents before confirming; retries use the proposal identity. Superseded unhandled proposals return a conflict; refresh the latest report before acting", []string{"case-id"}, http.MethodPost, actionsEndpoint, requestBody, false,
		requiredOption(str("--report-id", "reportId", "saved report UUID owning the proposal")),
		requiredOption(str("--proposal-id", "proposalId", "proposal UUID within the report")),
		requiredOption(choice("--action", "action", "action to take", "create", "dismiss")),
		str("--feedback", "feedback", "optional dismissal feedback retained for future evaluations"))

	personal := apiOperation("report personal generate", "generate a personal report of assigned open cases, assigned open tasks and managed open suggestions; returns HTTP 202 with a job; poll report personal get; uses credits. Data-owner AI policies apply. Retries with the same idempotency key return the saved report", nil, http.MethodPost, func(invocation) string { return "/v1/reports/personal" }, requestBody, true)
	personalGet := apiOperation("report personal get", "read the latest personal report and job in this billing home, or select --report-id or --job-id", nil, http.MethodGet, func(invocation) string { return "/v1/reports/personal" }, requestQuery, false,
		str("--report-id", "reportId", "saved report UUID"), str("--job-id", "jobId", "generation job UUID"))
	personalList := apiOperation("report personal list", "list saved personal report dates, verdicts and summaries, newest first; read a full report with report personal get --report-id", nil, http.MethodGet, func(invocation) string { return "/v1/reports/personal/history" }, requestQuery, false, pagingOptions()...)

	personalAct := apiOperation("report personal act", "confirm a saved proposal to create work, or dismiss it; review the proposed content before confirming", []string{"report-id"}, http.MethodPost, func(i invocation) string { return "/v1/reports/" + escaped(i.positional(0)) + "/actions" }, requestBody, false,
		requiredOption(str("--proposal-id", "proposalId", "proposal UUID within the report")), requiredOption(choice("--action", "action", "action to take", "create", "dismiss")), str("--feedback", "feedback", "optional dismissal feedback"))
	return []*command{
		get, commandAlias(get, "case report get"),
		list, commandAlias(list, "case report list"),
		generate, commandAlias(generate, "case report generate"),
		act, commandAlias(act, "case report act"),
		personal, personalGet, personalList, personalAct,
	}
}
