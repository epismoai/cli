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

func TestCaseHandoffSupportsBothDirections(t *testing.T) {
	type observedRequest struct {
		path string
		body map[string]any
	}
	var requests []observedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		observed := observedRequest{path: request.URL.Path}
		if err := json.NewDecoder(request.Body).Decode(&observed.body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, observed)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	commands := [][]string{
		{"case", "handoff", "source-case", "--to-case-id", "target-case"},
		{"case", "handoff", "target-case", "--from-case-id", "source-case"},
	}
	for _, args := range commands {
		var stdout, stderr bytes.Buffer
		if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, exit, stderr.String())
		}
	}

	if len(requests) != 2 {
		t.Fatalf("requests = %#v", requests)
	}
	for _, request := range requests {
		if request.path != "/v1/cases/source-case/handoffs" || request.body["toCaseId"] != "target-case" {
			t.Errorf("request = %#v", request)
		}
		if _, leaked := request.body["_fromCaseId"]; leaked {
			t.Errorf("client-only option leaked into request: %#v", request.body)
		}
	}
}

func TestCaseHandoffRejectsBothDirections(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"case", "handoff", "case-1", "--to-case-id", "case-2", "--from-case-id", "case-3", "--dry-run"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"INVALID_ARGUMENT"`) {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
}

func TestCaseHandoffReverseDirectionDryRunUsesNormalizedRequest(t *testing.T) {
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"case", "handoff", "target-case", "--from-case-id", "source-case", "--idempotency-key", "key-1", "--dry-run"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"arguments": [`) || !strings.Contains(stdout.String(), `"source-case"`) || !strings.Contains(stdout.String(), `"to_case_id": "target-case"`) {
		t.Fatalf("stdout=%s", stdout.String())
	}
}
