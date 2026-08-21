package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBrowserCommandPassesURLWithoutAShell(t *testing.T) {
	target := "https://epismo.ai/oauth/authorize?response_type=code&client_id=epismo-cli&state=abc"
	for _, goos := range []string{"darwin", "windows", "linux"} {
		command := browserCommand(goos, target)
		// cmd.exe treats the URL's unquoted `&` as a command separator, which
		// truncates the authorization URL at the first parameter.
		if strings.EqualFold(filepath.Base(command.Path), "cmd") || strings.EqualFold(filepath.Base(command.Path), "cmd.exe") {
			t.Errorf("%s: browser command routes the URL through cmd.exe: %v", goos, command.Args)
		}
		found := false
		for _, argument := range command.Args {
			if argument == target {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: URL not passed as a single intact argument: %v", goos, command.Args)
		}
	}
}

// A parallel invocation can win the refresh race and revoke this process's
// refresh token; the loser must pick up the credentials the winner wrote.
func TestRefreshRaceAdoptsCredentialsWrittenByAnotherProcess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid refresh token"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	expired := storedCredentials{AccessToken: "stale", RefreshToken: "revoked", ExpiresAt: "2000-01-01T00:00:00Z", ClientID: cliClientID, UserID: "user-1"}
	if err := writeCredentials(expired); err != nil {
		t.Fatal(err)
	}
	// Land after the final 800ms delay, in the window the old loop slept
	// through without ever re-reading.
	winner := make(chan struct{})
	go func() {
		defer close(winner)
		time.Sleep(900 * time.Millisecond)
		fresh := expired
		fresh.AccessToken = "refreshed-by-winner"
		fresh.RefreshToken = "rotated"
		fresh.ExpiresAt = "2999-01-01T00:00:00Z"
		if err := writeCredentials(fresh); err != nil {
			t.Errorf("winner write failed: %v", err)
		}
	}()
	defer func() { <-winner }()

	a := &app{version: "test", stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, client: newAPIClient("test")}
	auth, err := a.resolveAuthentication()
	if err != nil {
		t.Fatalf("resolveAuthentication = %v", err)
	}
	if auth.AccessToken != "refreshed-by-winner" {
		t.Fatalf("access token = %q, want the token written by the winning process", auth.AccessToken)
	}
}

func TestCommandSurface(t *testing.T) {
	expected := strings.Fields(`
		login logout whoami update completion doctor examples docs
		workspace/list workspace/current workspace/use workspace/clear workspace/create workspace/checkout workspace/update workspace/member/list workspace/member/upsert workspace/member/delete
		team/list team/create team/update team/member/list team/member/add team/member/delete
		credit/balance credit/checkout token/create token/list token/revoke
		playbook/init playbook/search playbook/list playbook/resource/list playbook/create playbook/get playbook/version/list playbook/version/get playbook/version/archive playbook/version/publish playbook/draft/get playbook/draft/save playbook/draft/discard playbook/draft/publish playbook/acl playbook/archive playbook/star playbook/unstar playbook/starred playbook/share playbook/alias/set playbook/alias/list playbook/alias/delete
		case/start case/get case/list case/assign case/acl case/update case/close case/reopen
		case/task/create case/task/list case/record/append case/record/list
		task/list task/get task/assign task/update task/close task/reopen
		playbook/suggestion/create playbook/suggestion/list
		suggestion/get suggestion/list suggestion/update suggestion/resolve
	`)
	actual := make([]string, 0, len(buildCommands()))
	for _, command := range buildCommands() {
		actual = append(actual, strings.ReplaceAll(command.Path, " ", "/"))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("command surface mismatch\nactual:   %v\nexpected: %v", actual, expected)
	}
}

func TestPlaybookSearchRequest(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestPath = request.URL.RequestURI()
		if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization = %q", got)
		}
		if got := request.Header.Get("X-Epismo-Source"); got != "cli" {
			t.Errorf("source = %q", got)
		}
		_, _ = io.WriteString(w, `{"playbooks":[{"createdAt":"2026-01-01T00:00:00Z","owner":{"accountId":"account-1"}}]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"playbook", "search", "--query", "pb:demo", "--category", "learning", "--lang", "ja,fr", "--page-size", "20"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if requestPath != "/v1/playbooks?category=learning&pageSize=20&preferredLangs=ja&preferredLangs=fr&query=pb%3Ademo" {
		t.Fatalf("request path = %q", requestPath)
	}
	if !strings.Contains(stdout.String(), `"created_at"`) || !strings.Contains(stdout.String(), `"account_id"`) || strings.Contains(stdout.String(), `"createdAt"`) || strings.Contains(stdout.String(), `"accountId"`) {
		t.Fatalf("stdout does not use snake_case recursively: %s", stdout.String())
	}
}

func TestPlaybookResourceCommands(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestPath = request.URL.RequestURI()
		_, _ = io.WriteString(w, `{"resources":[]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"playbook", "resource", "list", "--kind", "cli", "--page-size", "20"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if requestPath != "/v1/playbook-resources?kind=cli&pageSize=20" {
		t.Fatalf("request path = %q", requestPath)
	}
}

func TestChildCreateCommandsUseOwningResourcePaths(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	commands := [][]string{
		{"case", "task", "create", "case-1", "--input", `{"kind":"work","title":"Investigate"}`},
		{"case", "record", "append", "case-1", "--input", `{"kind":"note"}`},
		{"playbook", "suggestion", "create", "playbook-1", "--input", `{"baseVersionId":"version-1","title":"Improve","content":"Details"}`},
		{"playbook", "alias", "set", "playbook-1", "release", "--owner-id", "account-1"},
	}
	for _, args := range commands {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exitCode != 0 {
			t.Fatalf("args = %v, exit = %d, stderr = %s", args, exitCode, stderr.String())
		}
	}

	want := []string{
		"/v1/cases/case-1/tasks",
		"/v1/cases/case-1/records",
		"/v1/playbooks/playbook-1/suggestions",
		"/v1/aliases",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestAliasCommandsUseAliasAPI(t *testing.T) {
	type observedRequest struct {
		method string
		uri    string
		body   map[string]any
	}
	var requests []observedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		observed := observedRequest{method: request.Method, uri: request.URL.RequestURI()}
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&observed.body)
		}
		requests = append(requests, observed)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	commands := [][]string{
		{"playbook", "alias", "set", "playbook-1", "release", "--owner-id", "account-1", "--idempotency-key", "key-1"},
		{"playbook", "alias", "list", "playbook-1", "--owner-id", "account-1"},
		{"playbook", "alias", "delete", "release", "--owner-id", "account-1", "--idempotency-key", "key-2"},
	}
	for _, args := range commands {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exitCode != 0 {
			t.Fatalf("args = %v, exit = %d, stderr = %s", args, exitCode, stderr.String())
		}
	}

	if len(requests) != 3 {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].method != http.MethodPut || requests[0].uri != "/v1/aliases" || requests[0].body["ownerId"] != "account-1" || requests[0].body["playbookId"] != "playbook-1" || requests[0].body["alias"] != "release" || requests[0].body["idempotencyKey"] != "key-1" {
		t.Errorf("set request = %#v", requests[0])
	}
	if requests[1].method != http.MethodGet || requests[1].uri != "/v1/aliases?ownerId=account-1&playbookId=playbook-1" {
		t.Errorf("get request = %#v", requests[1])
	}
	if requests[2].method != http.MethodDelete || requests[2].uri != "/v1/aliases/release" || requests[2].body["ownerId"] != "account-1" || requests[2].body["idempotencyKey"] != "key-2" {
		t.Errorf("delete request = %#v", requests[2])
	}
}

func TestPersonalAndParentScopedListsSetImplicitFilters(t *testing.T) {
	var requestURIs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestURIs = append(requestURIs, request.URL.RequestURI())
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	commands := [][]string{
		{"task", "list"},
		{"suggestion", "list"},
		{"case", "task", "list", "case-1"},
		{"case", "record", "list", "case-1"},
		{"playbook", "suggestion", "list", "playbook-1"},
	}
	for _, args := range commands {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exitCode != 0 {
			t.Fatalf("args = %v, exit = %d, stderr = %s", args, exitCode, stderr.String())
		}
	}

	want := []string{
		"/v1/tasks?assignedTo=me",
		"/v1/suggestions?authorId=me",
		"/v1/cases/case-1/tasks",
		"/v1/cases/case-1/records?order=desc",
		"/v1/playbooks/playbook-1/suggestions",
	}
	if !reflect.DeepEqual(requestURIs, want) {
		t.Fatalf("request URIs = %#v, want %#v", requestURIs, want)
	}
}

func TestMutationMergesInputAndExplicitFlags(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != "/v1/cases/case-1/close" {
			t.Errorf("path = %q", request.URL.RequestURI())
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"closed":true}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"case", "close", "case-1", "--input", `{"outcome":"cancelled","expectedLockVersion":2,"extra":"kept"}`, "--outcome", "completed", "--lock-version", "3"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if body["outcome"] != "completed" || body["extra"] != "kept" || body["expectedLockVersion"] != float64(3) {
		t.Fatalf("body = %#v", body)
	}
	if key, _ := body["idempotencyKey"].(string); key == "" {
		t.Fatalf("idempotency key missing: %#v", body)
	}
}

func TestUnsupportedOperationsDoNotAdvertiseIdempotency(t *testing.T) {
	commands := []*command{
		workspaceCreateCommand(),
		teamCreateCommand(),
		creditCheckoutCommand(),
		tokenCreateCommand(),
	}
	for _, cmd := range commands {
		if cmd.Mutation {
			t.Errorf("%s unexpectedly marked as an idempotent mutation", cmd.Path)
		}
		for _, option := range baseOptions(cmd) {
			if option.Field == "idempotencyKey" {
				t.Errorf("%s unexpectedly exposes %s", cmd.Path, option.Name)
			}
		}
	}
}

func TestTokenCreateReadsWorkspaceFromInput(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != "/v1/cli/tokens" {
			t.Errorf("path = %q", request.URL.RequestURI())
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"token", "create", "--input", `{"workspaceId":"workspace-from-input"}`}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if !reflect.DeepEqual(body, map[string]any{"workspaceId": "workspace-from-input"}) {
		t.Fatalf("body = %#v", body)
	}
}

func TestTeamMemberMutationSupportsWorkspaceScopedToken(t *testing.T) {
	var requestURIs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestURIs = append(requestURIs, request.URL.RequestURI())
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "workspace-scoped-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	for _, args := range [][]string{
		{"team", "member", "add", "user-1", "--team-id", "team-1"},
		{"team", "member", "delete", "user-1", "--team-id", "team-1", "--workspace-id", "workspace-1"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exitCode != 0 {
			t.Fatalf("args = %v, exit = %d, stderr = %s", args, exitCode, stderr.String())
		}
	}
	want := []string{
		"/v1/teams/team-1/members",
		"/v1/teams/team-1/members?userIds=user-1&workspaceId=workspace-1",
	}
	if !reflect.DeepEqual(requestURIs, want) {
		t.Fatalf("request URIs = %#v, want %#v", requestURIs, want)
	}
}

func TestRequiredFieldsCanComeFromInputFile(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"case":{}}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	inputPath := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(inputPath, []byte(`{"expected_lock_version":2,"outcome":"completed"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"case", "close", "case-1", "--input", "@" + inputPath}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if body["expectedLockVersion"] != float64(2) || body["outcome"] != "completed" {
		t.Fatalf("body = %#v", body)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"case", "close", "case-1", "--input", `{"outcome":"completed"}`}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 1 || !strings.Contains(stderr.String(), `"code":"MISSING_OPTION_VALUE"`) {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
}

func TestDraftSavePreservesBaseRevisionFromInput(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"draft":{}}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"playbook", "draft", "save", "playbook-1", "--input", `{"baseRevision":7,"definition":{"schemaVersion":1,"title":"Draft"}}`}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if body["baseRevision"] != float64(7) {
		t.Fatalf("body = %#v", body)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"playbook", "draft", "save", "playbook-1", "--input", `{"definition":{"schemaVersion":1,"title":"Draft"}}`}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("default exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if body["baseRevision"] != float64(0) {
		t.Fatalf("default body = %#v", body)
	}
}

func TestCaseRecordListPreservesOrderFromInput(t *testing.T) {
	var requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestURI = request.URL.RequestURI()
		_, _ = io.WriteString(w, `{"records":[]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"case", "record", "list", "case-1", "--input", `{"order":"asc"}`}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if requestURI != "/v1/cases/case-1/records?order=asc" {
		t.Fatalf("request URI = %q", requestURI)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"case", "record", "list", "case-1"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("default exit = %d, stderr = %s", exitCode, stderr.String())
	}
	if requestURI != "/v1/cases/case-1/records?order=desc" {
		t.Fatalf("default request URI = %q", requestURI)
	}
}

func TestWorkspaceCreateIsNotScoped(t *testing.T) {
	var requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestURI = request.URL.RequestURI()
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	configDirectory := t.TempDir()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", configDirectory)
	t.Setenv("EPISMO_TOKEN", "")
	if err := writeConfig(cliConfig{DefaultWorkspace: &savedWorkspace{ID: "workspace-1", Handle: "team"}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(credentialsPath(), storedCredentials{AccessToken: "token", RefreshToken: "refresh", ExpiresAt: "2999-01-01T00:00:00Z", ClientID: cliClientID, UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"workspace", "create", "--handle", "new-team"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if requestURI != "/v1/workspaces" {
		t.Fatalf("request URI = %q", requestURI)
	}
}

func TestWorkspaceCreateKeepsCheckoutFailureDomainSpecific(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/workspaces":
			_, _ = io.WriteString(w, `{"workspace":{"id":"workspace-1","handle":"team"}}`)
		case "/v1/workspaces/workspace-1/checkout":
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"checkout_unavailable"}`)
		default:
			t.Errorf("unexpected path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	t.Setenv("EPISMO_TOKEN", "test-token")

	var stdout, stderr bytes.Buffer
	exit := Main([]string{"workspace", "create", "--handle", "team"}, "test", strings.NewReader(""), &stdout, &stderr)
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("stdout is not JSON: %s: %v", stdout.String(), err)
	}
	checkoutError, _ := response["checkout_error"].(map[string]any)
	_, hasTopLevelHint := response["hint"]
	_, hasPartialStatus := response["partial_success"]
	if exit != 0 || !strings.HasPrefix(stringField(checkoutError, "hint"), "Retry with") || hasTopLevelHint || hasPartialStatus {
		t.Fatalf("exit = %d, stdout = %s, stderr = %s", exit, stdout.String(), stderr.String())
	}
}

func TestWorkspaceSelectionLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/workspaces" {
			t.Errorf("unexpected path = %q", request.URL.Path)
		}
		_, _ = io.WriteString(w, `{"workspaces":[{"id":"workspace-1","handle":"team","accountId":"account-1","role":"admin"}]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	if err := writeCredentials(storedCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: "2999-01-01T00:00:00Z", ClientID: cliClientID, UserID: "user"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"workspace", "use", "workspace-1"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("use exit = %d, stderr = %s", exit, stderr.String())
	}
	config, err := readConfig()
	if err != nil || config.DefaultWorkspace == nil || config.DefaultWorkspace.ID != "workspace-1" || config.DefaultWorkspace.Handle != "team" || config.DefaultWorkspace.AccountID != "account-1" || config.DefaultWorkspace.Role != "admin" {
		t.Fatalf("config after use = %#v, err = %v", config, err)
	}

	stdout.Reset()
	stderr.Reset()
	if exit := Main([]string{"workspace", "current"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), `"id": "workspace-1"`) || !strings.Contains(stdout.String(), `"source": "local_config"`) {
		t.Fatalf("current exit = %d, stdout = %s, stderr = %s", exit, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exit := Main([]string{"workspace", "clear"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), `"cleared": true`) || !strings.Contains(stdout.String(), `"previous_workspace"`) || !strings.Contains(stdout.String(), `"account_id": "account-1"`) {
		t.Fatalf("clear exit = %d, stdout = %s, stderr = %s", exit, stdout.String(), stderr.String())
	}
	config, err = readConfig()
	if err != nil || config.DefaultWorkspace != nil {
		t.Fatalf("config after clear = %#v, err = %v", config, err)
	}
}

func TestWorkspaceUseRejectsInaccessibleWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"workspaces":[]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"workspace", "use", "missing"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 1 || !strings.Contains(stderr.String(), `"code":"WORKSPACE_NOT_FOUND"`) {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestHTTPErrorIsStructured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":"slow down","details":[{"path":["quantity"],"message":"too small"}]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"credit", "balance"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 1 || !strings.Contains(stderr.String(), `"code":"RATE_LIMITED"`) || !strings.Contains(stderr.String(), `"retry_after":"10"`) || !strings.Contains(stderr.String(), `"api_details"`) || !strings.Contains(stderr.String(), `"quantity"`) {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestEnvironmentTokenIgnoresMalformedConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth/userinfo":
			_, _ = io.WriteString(w, `{"sub":"user-1","email":"user@example.com"}`)
		case "/v1/workspaces":
			_, _ = io.WriteString(w, `{"workspaces":[]}`)
		case "/v1/credits":
			_, _ = io.WriteString(w, `{"balance":0,"shortfall":0}`)
		default:
			t.Errorf("unexpected path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	configDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDirectory, "config"), []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "environment-token")
	t.Setenv("EPISMO_CONFIG_DIR", configDirectory)

	for _, args := range [][]string{{"workspace", "list"}, {"whoami"}, {"credit", "balance"}} {
		var stdout, stderr bytes.Buffer
		if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("args = %v, exit = %d, stderr = %s", args, exit, stderr.String())
		}
	}
}

func TestPaymentRequiredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, `{"error":"Insufficient credits."}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"credit", "balance"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 1 || !strings.Contains(stderr.String(), `"code":"PAYMENT_REQUIRED"`) {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
}

func TestConfigPermissionsAndCompatibility(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("EPISMO_CONFIG_DIR", directory)
	legacy := []byte(`{"defaultWorkspace":{"id":" ws-1 ","handle":" team ","accountId":" account-1 ","role":"admin"},"lastLoginEmail":" User@Example.com "}`)
	if err := os.WriteFile(filepath.Join(directory, "config"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := readConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.DefaultWorkspace.ID != "ws-1" || config.DefaultWorkspace.Handle != "team" || config.LastLoginEmail != "User@Example.com" {
		t.Fatalf("config = %#v", config)
	}
	if err := writeCredentials(storedCredentials{AccessToken: "a", RefreshToken: "r", ExpiresAt: "2999-01-01T00:00:00Z", ClientID: cliClientID, UserID: "u"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(credentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	// Windows reports synthesized Unix permission bits; access is governed by
	// the ACL inherited from the user's profile directory instead.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials permissions = %o", info.Mode().Perm())
	}
}

func TestParseJSONRejectsTrailingData(t *testing.T) {
	for _, input := range []string{`{"title":"x"} trailing`, `{"title":"x"} {"title":"y"}`} {
		if _, err := parseJSON([]byte(input), "--input"); err == nil {
			t.Fatalf("parseJSON(%q) accepted trailing data", input)
		}
	}
}
