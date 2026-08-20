package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
