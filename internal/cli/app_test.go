package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceHandleGlobalOptionResolvesBeforeRequest(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if r.URL.Path == "/v1/workspaces" {
			_, _ = io.WriteString(w, `{"workspaces":[{"id":"workspace-1","handle":"acme"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"playbooks":[]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"--workspace", "acme", "playbook", "search", "onboarding"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if len(paths) != 2 || paths[1] != "/v1/playbooks?query=onboarding&workspaceId=workspace-1" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestSearchConvenienceArgumentPreservesOptionValues(t *testing.T) {
	cmd := playbookCommands()[1]
	for _, args := range [][]string{
		{"--category", "learning"},
		{"--lang", "ja,fr", "onboarding"},
		{"--category=learning", "-q", "onboarding"},
	} {
		normalized := normalizeConvenienceArgs(cmd, args)
		if _, err := parseInvocation(cmd, normalized, strings.NewReader("")); err != nil {
			t.Fatalf("args = %v, normalized = %v, err = %v", args, normalized, err)
		}
	}
}

func TestDryRunDoesNotCallAPIForAnyMutationClass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { t.Fatal("API should not be called") }))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	tests := [][]string{
		{"playbook", "archive", "playbook-1", "--dry-run"},                                    // dangerous API mutation
		{"playbook", "create", "--definition", `{"title":"Preview","steps":[]}`, "--dry-run"}, // non-dangerous idempotent API mutation
		{"playbook", "draft", "save", "playbook-1", "--definition", `{"title":"Draft","steps":[]}`, "--dry-run"},
		{"team", "create", "--name", "Preview", "--dry-run"},   // API mutation without idempotency support
		{"workspace", "use", "acme", "--dry-run"},              // local mutation that normally resolves remote state
		{"login", "--email", "owner@example.com", "--dry-run"}, // auth and local credential mutation
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, exit, stderr.String())
		}
		if !strings.Contains(stdout.String(), `"dry_run": true`) {
			t.Fatalf("args=%v stdout=%s", args, stdout.String())
		}
	}
}

func TestDryRunRejectsReadOnlyCommands(t *testing.T) {
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	for _, args := range [][]string{
		{"playbook", "list", "--dry-run"},
		{"playbook", "list", "--dry-run", "--help"},
		{"playbook", "list", "--dry-run", "--schema"},
		{"docs", "playbook", "create", "--dry-run"},
	} {
		var stdout, stderr bytes.Buffer
		exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr)
		if exit != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"DRY_RUN_NOT_SUPPORTED"`) {
			t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", args, exit, stdout.String(), stderr.String())
		}
	}
}

func TestEveryCommandHasExpectedDryRunSupport(t *testing.T) {
	want := map[string]bool{
		"login": true, "logout": true,
		"workspace use": true, "workspace clear": true, "workspace create": true, "workspace checkout": true, "workspace update": true,
		"workspace member upsert": true, "workspace member delete": true,
		"team create": true, "team update": true, "team member add": true, "team member delete": true,
		"credit checkout": true, "token create": true, "token revoke": true,
		"playbook create": true, "playbook version archive": true, "playbook version publish": true,
		"playbook draft save": true, "playbook draft discard": true, "playbook draft publish": true,
		"playbook acl": true, "playbook archive": true, "playbook star": true, "playbook unstar": true, "playbook share": true,
		"playbook alias set": true, "playbook alias delete": true,
		"case start": true, "case assign": true, "case acl": true, "case update": true, "case close": true, "case reopen": true,
		"case task create": true, "case record append": true,
		"task assign": true, "task update": true, "task close": true, "task reopen": true,
		"playbook suggestion create": true, "suggestion update": true, "suggestion resolve": true,
	}
	for _, cmd := range buildCommands() {
		if cmd.Safety.DryRun != want[cmd.Path] {
			t.Errorf("%s Safety.DryRun=%v, want %v", cmd.Path, cmd.Safety.DryRun, want[cmd.Path])
		}
		delete(want, cmd.Path)
	}
	if len(want) != 0 {
		t.Fatalf("unknown expected commands: %#v", want)
	}
}

func TestGlobalBooleanOptionsAcceptExplicitValues(t *testing.T) {
	remaining, options, err := extractGlobalOptions([]string{"--yes=false", "--dry-run=true", "--quiet=false", "--schema=false", "playbook", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Yes || !options.DryRun || options.Quiet || options.Schema {
		t.Fatalf("options = %#v", options)
	}
	if got := strings.Join(remaining, " "); got != "playbook list" {
		t.Fatalf("remaining = %q", got)
	}
	if _, _, err := extractGlobalOptions([]string{"--yes=maybe", "playbook", "list"}); errorCode(err) != "INVALID_ARGUMENT" {
		t.Fatalf("error = %v", err)
	}
}

func TestGlobalOptionValidationUsesDiagnosticFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"--diagnostic-format", "human", "--output", "xml", "playbook", "list"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 1 {
		t.Fatalf("exit = %d", exit)
	}
	if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "error: Invalid --output value.") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDryRunValidatesPayload(t *testing.T) {
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"task", "close", "task-1", "--lock-version", "1", "--dry-run"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 1 || !strings.Contains(stderr.String(), `"code":"MISSING_OPTION_VALUE"`) || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
}

func TestCompletionAndDocs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"completion", "bash"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "complete -F") {
		t.Fatalf("completion exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := Main([]string{"docs", "playbook", "search"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "Usage: epismo playbook search") {
		t.Fatalf("docs exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := Main([]string{"docs", "playbook", "search", "--help"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "Usage: epismo playbook search") {
		t.Fatalf("docs command help exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := Main([]string{"docs", "--help"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 || !strings.Contains(stdout.String(), "Usage: epismo docs") || stderr.Len() != 0 {
		t.Fatalf("docs help exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := Main([]string{"docs", "does-not-exist"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"UNKNOWN_COMMAND"`) {
		t.Fatalf("invalid docs exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
}

func TestSchemaGlobalOptionMayPrecedeCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"--schema", "playbook", "list"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"command": "playbook list"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestHumanDiagnosticFormatDoesNotRequireTTY(t *testing.T) {
	var stderr bytes.Buffer
	a := &app{stderr: &stderr, options: globalOptions{DiagnosticFormat: "human"}}
	a.event("warning", "TEST_WARNING", "Readable warning.", nil)
	if stderr.String() != "Readable warning.\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHumanDiagnosticsIncludeDetails(t *testing.T) {
	var stderr bytes.Buffer
	a := &app{stderr: &stderr, options: globalOptions{DiagnosticFormat: "human"}}
	a.event("info", "BROWSER_FALLBACK_URL", "Open this URL if the browser did not open.", map[string]any{"url": "https://example.test/authorize"})
	if !strings.Contains(stderr.String(), "url: https://example.test/authorize") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
