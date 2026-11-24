package cli

import "net/http"

func reviewCommands() []*command {
	remove := apiOperation("review delete", "delete a Case, Personal, or Team review by review ID; existing decisions and created work remain", []string{"review-id"}, http.MethodDelete, func(i invocation) string { return "/v1/reviews/" + escaped(i.positional(0)) }, requestQuery, false)
	remove.Safety.Confirmation = true
	cancel := apiOperation("review cancel", "cancel a queued or running Case, Personal, or Team review generation job by job ID", []string{"job-id"}, http.MethodPost, func(i invocation) string { return "/v1/reviews/jobs/" + escaped(i.positional(0)) + "/cancel" }, requestNone, false)
	caseEndpoint := func(i invocation) string { return "/v1/cases/" + escaped(i.positional(0)) }
	reviewEndpoint := func(i invocation) string { return caseEndpoint(i) + "/reviews" }
	actionsEndpoint := func(i invocation) string { return reviewEndpoint(i) + "/actions" }

	get := apiOperation("review case get", "read the latest saved review and job state, or a selected review with --review-id; superseded reviews are read-only. Requires current Case access and never calls AI", []string{"case-id"}, http.MethodGet, reviewEndpoint, requestQuery, false,
		str("--review-id", "reviewId", "saved review UUID; omit for the latest review"))
	list := apiOperation("review case list", "list review dates, health diagnoses and summaries, newest first; deleted entries retain deleted_at with erased content; current Case access and owning account apply. Read a full review with review case get --review-id", []string{"case-id"}, http.MethodGet, func(i invocation) string { return reviewEndpoint(i) + "/history" }, requestQuery, false, pagingOptions()...)
	generate := apiOperation("review case run", "queue an asynchronous case review; requires case editing access. Returns immediately with HTTP 202 Accepted. Poll review case get for status and the committed review", []string{"case-id"}, http.MethodPost, reviewEndpoint, requestBody, true, str("--criteria", "criteria", "optional natural-language evaluation criteria"))
	act := apiOperation("review case act", "confirm a latest-review proposal unchanged to create a task, a case with a handoff, a suggestion, or a playbook with its first version; or dismiss with feedback. Review the full proposed contents before confirming; retries use the proposal identity. Superseded unhandled proposals return a conflict; refresh the latest review before acting", []string{"case-id"}, http.MethodPost, actionsEndpoint, requestBody, false,
		requiredOption(str("--review-id", "reviewId", "saved review UUID owning the proposal")),
		requiredOption(str("--proposal-id", "proposalId", "proposal UUID within the review")),
		requiredOption(choice("--action", "action", "action to take", "create", "dismiss")),
		str("--feedback", "feedback", "optional dismissal feedback retained for future reviews"))

	personal := apiOperation("review personal run", "queue a personal review of assigned open cases, assigned open tasks in open cases and managed open suggestions in this billing home; returns HTTP 202 with a job; poll review personal get; uses credits. Data-owner AI policies apply. Prior review feedback is considered; previous review contents and proposal handling history are not model input. Reuse the same idempotency key on retries", nil, http.MethodPost, func(invocation) string { return "/v1/me/reviews" }, requestBody, true, str("--criteria", "criteria", "optional natural-language evaluation criteria"))
	personalGet := apiOperation("review personal get", "read the latest personal review and job in this billing home, or select --review-id or --job-id", nil, http.MethodGet, func(invocation) string { return "/v1/me/reviews" }, requestQuery, false,
		str("--review-id", "reviewId", "saved review UUID"), str("--job-id", "jobId", "generation job UUID"))
	personalList := apiOperation("review personal list", "list personal review dates, health diagnoses and summaries, newest first; deleted entries retain deleted_at with erased content; read a full review with review personal get --review-id", nil, http.MethodGet, func(invocation) string { return "/v1/me/reviews/history" }, requestQuery, false, pagingOptions()...)

	personalAct := apiOperation("review personal act", "confirm a saved proposal to create work, or dismiss it; review the proposed content before confirming", []string{"review-id"}, http.MethodPost, func(i invocation) string { return "/v1/reviews/" + escaped(i.positional(0)) + "/actions" }, requestBody, false,
		requiredOption(str("--proposal-id", "proposalId", "proposal UUID within the review")), requiredOption(choice("--action", "action", "action to take", "create", "dismiss")), str("--feedback", "feedback", "optional dismissal feedback"))
	teamEndpoint := func(i invocation) string { return "/v1/teams/" + escaped(i.positional(0)) + "/reviews" }
	teamGenerate := apiOperation("review team run", "queue a review of shared open team work in the active workspace; requires team membership and uses workspace credits. Returns HTTP 202 with a job; poll review team get. Includes open suggestions on team-shared playbooks. Results and proposal handling are shared with current team members. Prior review feedback is considered; previous review contents are not model input", []string{"team-id"}, http.MethodPost, teamEndpoint, requestBody, true, str("--criteria", "criteria", "optional natural-language evaluation criteria"))
	teamGet := apiOperation("review team get", "read the latest team review and job, or select --review-id or --job-id; requires current team membership. Saved text remains readable after source changes", []string{"team-id"}, http.MethodGet, teamEndpoint, requestQuery, false, str("--review-id", "reviewId", "saved review UUID"), str("--job-id", "jobId", "generation job UUID"))
	teamList := apiOperation("review team list", "list team review summaries, newest first; deleted entries retain deleted_at with erased content; requires current team membership in the active workspace", []string{"team-id"}, http.MethodGet, func(i invocation) string { return teamEndpoint(i) + "/history" }, requestQuery, false, pagingOptions()...)
	teamAct := apiOperation("review team act", "confirm a saved team proposal to create work, or dismiss it; requires current team membership and live destination permissions. Handling is shared. New cases and playbooks retain restricted creator access", []string{"review-id"}, http.MethodPost, func(i invocation) string { return "/v1/reviews/" + escaped(i.positional(0)) + "/actions" }, requestBody, false,
		requiredOption(str("--proposal-id", "proposalId", "proposal UUID within the review")), requiredOption(choice("--action", "action", "action to take", "create", "dismiss")), str("--feedback", "feedback", "optional dismissal feedback"))
	feedbackEndpoint := func(i invocation) string { return "/v1/reviews/" + escaped(i.positional(0)) + "/feedback" }
	feedbackCreate := apiOperation("review feedback create", "add feedback to a saved review; reuse --idempotency-key when retrying", []string{"review-id"}, http.MethodPost, feedbackEndpoint, requestBody, true,
		requiredOption(str("--content", "content", "feedback text")),
		str("--verification-id", "verificationId", "optional verification UUID in this review"))
	feedbackList := apiOperation("review feedback list", "list feedback on a saved review", []string{"review-id"}, http.MethodGet, feedbackEndpoint, requestQuery, false)
	feedbackDelete := apiOperation("review feedback delete", "delete a feedback comment from a saved review", []string{"review-id", "feedback-id"}, http.MethodDelete, func(i invocation) string {
		return "/v1/reviews/" + escaped(i.positional(0)) + "/feedback/" + escaped(i.positional(1))
	}, requestNone, false)
	return []*command{
		feedbackCreate, feedbackList, feedbackDelete,
		remove, cancel,
		get, commandAlias(get, "case review get"),
		list, commandAlias(list, "case review list"),
		generate, commandAlias(generate, "case review run"),
		act, commandAlias(act, "case review act"),
		personal, personalGet, personalList, personalAct,
		teamGenerate, teamGet, teamList, teamAct,
	}
}
