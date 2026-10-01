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

func TestCaseBriefMutationsAndCaseRead(t *testing.T) {
	type request struct {
		method, path string
		body         map[string]any
	}
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		if r.Body != nil {
			data, _ := io.ReadAll(r.Body)
			if len(data) > 0 {
				_ = json.Unmarshal(data, &body)
			}
		}
		requests = append(requests, request{r.Method, r.URL.Path, body})
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"case":{"id":"case-1","brief":{"updatedAt":"2026-09-30T00:00:00Z","content":"Waiting"}},"tasks":[],"records":[]}`)
		} else if r.Method == http.MethodDelete {
			_, _ = io.WriteString(w, `{}`)
		} else {
			_, _ = io.WriteString(w, `{"brief":{"updatedAt":"2026-09-30T00:00:00Z","content":"Waiting"}}`)
		}
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	for _, args := range [][]string{
		{"case", "get", "case-1"},
		{"case", "brief", "set", "case-1", "--content", "Waiting", "--idempotency-key", "33333333-3333-4333-8333-333333333333"},
		{"case", "brief", "delete", "case-1", "--idempotency-key", "44444444-4444-4444-8444-444444444444"},
		{"case", "brief", "generate", "case-1", "--idempotency-key", "55555555-5555-4555-8555-555555555555"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, exit, stderr.String())
		}
		if args[2] != "delete" && !strings.Contains(stdout.String(), `"content": "Waiting"`) {
			t.Fatalf("Brief missing from output: %s", stdout.String())
		}
	}
	if len(requests) != 4 {
		t.Fatalf("requests = %+v", requests)
	}
	if requests[0].method != http.MethodGet || requests[0].path != "/v1/cases/case-1" {
		t.Fatalf("Case read = %+v", requests[0])
	}
	for i, request := range requests[1:] {
		method := http.MethodPut
		if i == 1 {
			method = http.MethodDelete
		}
		if i == 2 {
			method = http.MethodPost
		}
		if request.path != "/v1/cases/case-1/brief" || request.method != method {
			t.Fatalf("mutation = %+v", request)
		}
	}
	if requests[1].body["content"] != "Waiting" || requests[1].body["idempotencyKey"] != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("set body = %+v", requests[1].body)
	}
	if _, exists := requests[2].body["content"]; exists {
		t.Fatalf("delete must not send content: %+v", requests[2].body)
	}
	if requests[2].body["idempotencyKey"] != "44444444-4444-4444-8444-444444444444" {
		t.Fatalf("delete body = %+v", requests[2].body)
	}
	if requests[3].body["idempotencyKey"] != "55555555-5555-4555-8555-555555555555" {
		t.Fatalf("generate body = %+v", requests[3].body)
	}
	if _, exists := requests[3].body["content"]; exists {
		t.Fatalf("generate must not send manually authored content: %+v", requests[3].body)
	}
}
