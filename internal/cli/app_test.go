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

func TestDangerousDryRunDoesNotCallAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { t.Fatal("API should not be called") }))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "archive", "playbook-1", "--dry-run"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"dry_run": true`) {
		t.Fatalf("stdout = %s", stdout.String())
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

func TestDangerousDryRunValidatesPayload(t *testing.T) {
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
