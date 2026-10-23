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

func TestCaseReportRequests(t *testing.T) {
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
		{"report", "case", "get", "case-1"},
		{"report", "case", "generate", "case-1", "--idempotency-key", "11111111-1111-4111-8111-111111111111"},
		{"report", "case", "act", "case-1", "--report-id", "report-1", "--proposal-id", "proposal-1", "--action", "create"},
		{"report", "case", "act", "case-1", "--report-id", "report-1", "--proposal-id", "proposal-1", "--action", "dismiss", "--feedback", "Not needed"},
		{"case", "report", "get", "case-1"},
		{"case", "report", "generate", "case-1", "--idempotency-key", "11111111-1111-4111-8111-111111111111"},
		{"case", "report", "act", "case-1", "--report-id", "report-1", "--proposal-id", "proposal-1", "--action", "create"},
		{"case", "report", "act", "case-1", "--report-id", "report-1", "--proposal-id", "proposal-1", "--action", "dismiss", "--feedback", "Not needed"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
			t.Fatalf("args=%v exit=%d stderr=%s", args, exit, stderr.String())
		}
	}

	if len(requests) != 8 {
		t.Fatalf("requests = %+v", requests)
	}

	for offset := 0; offset <= 4; offset += 4 {
		// 1. report case get
		if requests[offset+0].method != http.MethodGet || requests[offset+0].path != "/v1/cases/case-1/report" {
			t.Fatalf("Review get = %+v", requests[offset+0])
		}

		// 2. report case generate
		if requests[offset+1].method != http.MethodPost || requests[offset+1].path != "/v1/cases/case-1/report" {
			t.Fatalf("Review generate = %+v", requests[offset+1])
		}
		if requests[offset+1].body["idempotencyKey"] != "11111111-1111-4111-8111-111111111111" {
			t.Fatalf("generate idempotencyKey = %+v", requests[offset+1].body)
		}

		// 3. report case act create
		if requests[offset+2].method != http.MethodPost || requests[offset+2].path != "/v1/cases/case-1/report/actions" {
			t.Fatalf("Review act create = %+v", requests[offset+2])
		}
		if requests[offset+2].body["reportId"] != "report-1" || requests[offset+2].body["proposalId"] != "proposal-1" || requests[offset+2].body["action"] != "create" {
			t.Fatalf("act create body = %+v", requests[offset+2].body)
		}

		for _, index := range []int{offset + 2, offset + 3} {
			for key := range requests[index].body {
				if key != "reportId" && key != "proposalId" && key != "action" && key != "feedback" {
					t.Fatalf("unexpected action field %q: %+v", key, requests[index].body)
				}
			}
		}

		// 4. report case act dismiss
		if requests[offset+3].method != http.MethodPost || requests[offset+3].path != "/v1/cases/case-1/report/actions" {
			t.Fatalf("Review act dismiss = %+v", requests[offset+3])
		}
		if requests[offset+3].body["action"] != "dismiss" || requests[offset+3].body["feedback"] != "Not needed" {
			t.Fatalf("act dismiss body = %+v", requests[offset+3].body)
		}
	}
}

func TestCaseReportHistoryRequests(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		path      string
		query     map[string]string
		response  string
		resultKey string
	}{
		{"history page", []string{"report", "case", "list", "case-1", "--page-size", "2", "--cursor", "older+/=?"}, "/v1/cases/case-1/report/history", map[string]string{"pageSize": "2", "cursor": "older+/=?"}, `{"reports":[{"id":"old","summary":"Past review"}],"nextCursor":"next"}`, "reports"},
		{"selected review", []string{"report", "case", "get", "case-1", "--report-id", "old-report"}, "/v1/cases/case-1/report", map[string]string{"reportId": "old-report"}, `{"report":{"id":"old-report","content":"Full historical review"},"historical":true}`, "historical"},
		{"case report history alias", []string{"case", "report", "list", "case-1", "--page-size", "2", "--cursor", "older+/=?"}, "/v1/cases/case-1/report/history", map[string]string{"pageSize": "2", "cursor": "older+/=?"}, `{"reports":[{"id":"old","summary":"Past review"}],"nextCursor":"next"}`, "reports"},
		{"case report get alias", []string{"case", "report", "get", "case-1", "--report-id", "old-report"}, "/v1/cases/case-1/report", map[string]string{"reportId": "old-report"}, `{"report":{"id":"old-report","content":"Full historical review"},"historical":true}`, "historical"},
		{"case detail includes latest review", []string{"case", "get", "case-1"}, "/v1/cases/case-1", map[string]string{}, `{"case":{"id":"case-1"},"report":{"id":"latest","content":"Full latest review"},"reportJob":{"id":"job","status":"complete"}}`, "report"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != tt.path {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing authorization")
				}
				if len(r.URL.Query()) != len(tt.query) {
					t.Errorf("query = %v", r.URL.Query())
				}
				for key, want := range tt.query {
					if r.URL.Query().Get(key) != want {
						t.Errorf("query %s = %q, want %q", key, r.URL.Query().Get(key), want)
					}
				}
				body, _ := io.ReadAll(r.Body)
				if len(body) != 0 {
					t.Errorf("GET body = %q", body)
				}
				_, _ = io.WriteString(w, tt.response)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var stdout, stderr bytes.Buffer
			if exit := Main(tt.args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
				t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
			}
			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result[tt.resultKey] == nil {
				t.Fatalf("missing %s in %s", tt.resultKey, stdout.String())
			}
			if tt.resultKey == "historical" && result["historical"] != true {
				t.Fatalf("historical marker lost: %s", stdout.String())
			}
			if calls != 1 {
				t.Fatalf("calls=%d; reads must not automatically fetch history or generate reviews", calls)
			}
		})
	}
}

func TestPersonalReportRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/reports/personal" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["idempotencyKey"] != "11111111-1111-4111-8111-111111111111" {
			t.Errorf("body=%v", body)
		}
		if _, ok := body["accessPrincipals"]; ok {
			t.Errorf("unexpected selection: %v", body)
		}
		_, _ = io.WriteString(w, `{"job":{"id":"job-1","status":"queued"},"historical":false}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"report", "personal", "generate", "--idempotency-key", "11111111-1111-4111-8111-111111111111"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	job, _ := result["job"].(map[string]any)
	if job["id"] != "job-1" || job["status"] != "queued" || result["historical"] != false || result["report"] != nil {
		t.Fatalf("result=%s", stdout.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestPersonalReportGetRequests(t *testing.T) {
	for _, test := range []struct {
		name, flag, field, id string
	}{
		{name: "latest"},
		{name: "saved report", flag: "--report-id", field: "reportId", id: "report-1"},
		{name: "generation job", flag: "--job-id", field: "jobId", id: "job-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/v1/reports/personal" {
					t.Errorf("request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing authentication")
				}
				if test.flag == "" {
					if len(r.URL.Query()) != 0 {
						t.Errorf("unexpected query: %s", r.URL.RawQuery)
					}
				} else if len(r.URL.Query()) != 1 || r.URL.Query().Get(test.field) != test.id {
					t.Errorf("query: %s", r.URL.RawQuery)
				}
				_, _ = io.WriteString(w, `{"report":{"id":"report-1","verdict":"","summary":"Confirm approval"},"job":{"id":"job-1","status":"complete"},"historical":false}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			args := []string{"report", "personal", "get"}
			if test.flag != "" {
				args = append(args, test.flag, test.id)
			}
			var stdout, stderr bytes.Buffer
			if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
				t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
			}
			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			report, _ := result["report"].(map[string]any)
			job, _ := result["job"].(map[string]any)
			if report["id"] != "report-1" || report["verdict"] != "" || job["status"] != "complete" || result["historical"] != false {
				t.Fatalf("result=%s", stdout.String())
			}
			if calls != 1 {
				t.Fatalf("calls=%d; reads must not trigger generation or extra history requests", calls)
			}
		})
	}
}

func TestPersonalReportHistoryRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/reports/personal/history" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "10" || r.URL.Query().Get("cursor") != "cursor-1" {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"reports":[{"id":"report-1","summary":"Personal summary"}],"nextCursor":"cursor-2"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"report", "personal", "list", "--page-size", "10", "--cursor", "cursor-1"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	reports, _ := result["reports"].([]any)
	if len(reports) != 1 || result["next_cursor"] != "cursor-2" {
		t.Fatalf("result=%s", stdout.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestPersonalReportActRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/reports/report-1/actions" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["proposalId"] != "proposal-1" || body["action"] != "create" {
			t.Errorf("body=%v", body)
		}
		_, _ = io.WriteString(w, `{"report":{"id":"report-1","summary":"Personal summary"}}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "test-token")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := Main([]string{"report", "personal", "act", "report-1", "--proposal-id", "proposal-1", "--action", "create"}, "test", strings.NewReader(""), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}
