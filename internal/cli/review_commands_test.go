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

func TestCaseReviewRequests(t *testing.T) {
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
			_, _ = io.WriteString(w, `{"report":{"id":"report-1","summary":"Evaluation summary"},"job":{"id":"job-1","status":"complete"}}`)
		} else {
			_, _ = io.WriteString(w, `{"job":{"id":"job-1","status":"queued"}}`)
		}
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())

	for _, args := range [][]string{
		{"case", "review", "get", "case-1"},
		{"case", "review", "generate", "case-1", "--idempotency-key", "11111111-1111-4111-8111-111111111111"},
		{"case", "review", "act", "case-1", "--report-id", "report-1", "--proposal-id", "proposal-1", "--action", "create"},
		{"case", "review", "act", "case-1", "--report-id", "report-1", "--proposal-id", "proposal-1", "--action", "dismiss", "--feedback", "Not needed"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, exit, stderr.String())
		}
	}

	if len(requests) != 4 {
		t.Fatalf("requests = %+v", requests)
	}

	// 1. case review get
	if requests[0].method != http.MethodGet || requests[0].path != "/v1/cases/case-1/review" {
		t.Fatalf("Review get = %+v", requests[0])
	}

	// 2. case review generate
	if requests[1].method != http.MethodPost || requests[1].path != "/v1/cases/case-1/review" {
		t.Fatalf("Review generate = %+v", requests[1])
	}
	if requests[1].body["idempotencyKey"] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("generate idempotencyKey = %+v", requests[1].body)
	}

	// 3. case review act create
	if requests[2].method != http.MethodPost || requests[2].path != "/v1/cases/case-1/review/actions" {
		t.Fatalf("Review act create = %+v", requests[2])
	}
	if requests[2].body["reportId"] != "report-1" || requests[2].body["proposalId"] != "proposal-1" || requests[2].body["action"] != "create" {
		t.Fatalf("act create body = %+v", requests[2].body)
	}

	for _, index := range []int{2, 3} {
		for key := range requests[index].body {
			if key != "reportId" && key != "proposalId" && key != "action" && key != "feedback" {
				t.Fatalf("unexpected action field %q: %+v", key, requests[index].body)
			}
		}
	}

	// 4. case review act dismiss
	if requests[3].method != http.MethodPost || requests[3].path != "/v1/cases/case-1/review/actions" {
		t.Fatalf("Review act dismiss = %+v", requests[3])
	}
	if requests[3].body["action"] != "dismiss" || requests[3].body["feedback"] != "Not needed" {
		t.Fatalf("act dismiss body = %+v", requests[3].body)
	}
}
