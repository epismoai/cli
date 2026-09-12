package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllPaginationAndJQProjection(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("cursor") == "" {
			_, _ = io.WriteString(w, `{"playbooks":[{"id":"one"}],"nextCursor":"two"}`)
			return
		}
		_, _ = io.WriteString(w, `{"playbooks":[{"id":"two"}]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list", "--all", "--jq", ".playbooks[] | .id"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if calls != 2 || !strings.Contains(stdout.String(), `"one"`) || !strings.Contains(stdout.String(), `"two"`) {
		t.Fatalf("calls=%d stdout=%s", calls, stdout.String())
	}
}

func TestPublicPlaybookListWorksBeforeLoginWithStableAnonymousID(t *testing.T) {
	var anonymousIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Errorf("anonymous list sent authorization = %q", authorization)
		}
		anonymousIDs = append(anonymousIDs, request.Header.Get("X-Epismo-Anonymous-Id"))
		_, _ = io.WriteString(w, `{"playbooks":[]}`)
	}))
	defer server.Close()
	configDirectory := t.TempDir()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", configDirectory)
	t.Setenv("EPISMO_TOKEN", "")

	for range 2 {
		var stdout, stderr bytes.Buffer
		if exit := Main([]string{"playbook", "list"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
		}
	}
	if len(anonymousIDs) != 2 || anonymousIDs[0] == "" || anonymousIDs[0] != anonymousIDs[1] {
		t.Fatalf("anonymous IDs = %v", anonymousIDs)
	}
	config, err := readConfig()
	if err != nil || config.AnonymousID != anonymousIDs[0] {
		t.Fatalf("config = %#v, err = %v", config, err)
	}
}

func TestEpismoTokenWorksWhenConfigDirectoryIsUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("cannot restrict permissions when running as root")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	configDirectory := filepath.Join(parent, "config")

	var gotAuthorization, gotAnonymousID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		gotAuthorization = request.Header.Get("Authorization")
		gotAnonymousID = request.Header.Get("X-Epismo-Anonymous-Id")
		_, _ = io.WriteString(w, `{"playbooks":[]}`)
	}))
	defer server.Close()

	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", configDirectory)
	t.Setenv("EPISMO_TOKEN", "test-token")

	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if gotAuthorization != "Bearer test-token" {
		t.Fatalf("authorization = %q", gotAuthorization)
	}
	if !uuidPattern.MatchString(strings.ToLower(gotAnonymousID)) {
		t.Fatalf("anonymous id = %q", gotAnonymousID)
	}
}

func TestPublicPlaybookListFallsBackToAnonymousWithCorruptCredentials(t *testing.T) {
	configDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDirectory, "credentials"), []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		gotAuthorization = request.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"playbooks":[]}`)
	}))
	defer server.Close()

	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", configDirectory)
	t.Setenv("EPISMO_TOKEN", "")

	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if gotAuthorization != "" {
		t.Fatalf("authorization = %q, want anonymous fallback (no auth header)", gotAuthorization)
	}
}

func TestAllPaginationMergesResourceBacklinks(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("cursor") == "" {
			_, _ = io.WriteString(w, `{"playbooks":[{"id":"one"}],"resourceBacklinks":[{"playbookId":"one","mentions":[{"stepIndex":0,"resourcePosition":0}]}],"nextCursor":"two"}`)
			return
		}
		_, _ = io.WriteString(w, `{"playbooks":[{"id":"two"}],"resourceBacklinks":[{"playbookId":"two","mentions":[{"stepIndex":1,"resourcePosition":0}]}]}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list", "--all", "--resource-kind", "cli", "--resource-ref", "github:epismoai/cli"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if calls != 2 || !strings.Contains(stdout.String(), `"playbook_id": "one"`) || !strings.Contains(stdout.String(), `"playbook_id": "two"`) {
		t.Fatalf("calls=%d stdout=%s", calls, stdout.String())
	}
}

func TestAllPaginationControlsMayComeFromInput(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Has("all") || r.URL.Query().Has("limit") {
			t.Errorf("client-only pagination controls leaked into query: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("pageSize") != "1" {
			t.Errorf("pageSize = %q, want 1", r.URL.Query().Get("pageSize"))
		}
		if r.URL.Query().Get("cursor") == "" {
			_, _ = io.WriteString(w, `{"playbooks":[{"id":"one"}],"nextCursor":"two"}`)
			return
		}
		_, _ = io.WriteString(w, `{"playbooks":[{"id":"two"}],"nextCursor":"three"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list", "--input", `{"all":true,"limit":2,"page_size":1}`}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if items, _ := result["playbooks"].([]any); len(items) != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestAllPaginationStopsWhenFirstPageReachesLimit(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"playbooks":[{"id":"one"}],"nextCursor":"two"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list", "--all", "--limit", "1"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestAllPaginationUsesRemainingLimitForLaterPages(t *testing.T) {
	var pageSizes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageSizes = append(pageSizes, r.URL.Query().Get("pageSize"))
		if r.URL.Query().Get("cursor") == "" {
			_, _ = io.WriteString(w, `{"playbooks":[{"id":"one"},{"id":"two"},{"id":"three"}],"nextCursor":"second"}`)
			return
		}
		_, _ = io.WriteString(w, `{"playbooks":[{"id":"four"},{"id":"five"}],"nextCursor":"third"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if exit := Main([]string{"playbook", "list", "--all", "--page-size", "3", "--limit", "5"}, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if len(pageSizes) != 2 || pageSizes[0] != "3" || pageSizes[1] != "2" {
		t.Fatalf("page sizes = %v", pageSizes)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result["playbooks"].([]any)) != 5 || result["next_cursor"] != "third" {
		t.Fatalf("result = %#v", result)
	}
}
