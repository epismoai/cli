package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestSharingCommandsSendNestedAccessAndKeepGrantUUIDs(t *testing.T) {
	const principal = "abcdefab-cdef-4abc-8def-abcdefabcdef"
	const settings = `{"visibility":"public","grants":{"abcdefab-cdef-4abc-8def-abcdefabcdef":"viewer"}}`
	for _, tc := range []struct {
		name         string
		args         []string
		method, path string
	}{
		{"case start", []string{"case", "start", "--title", "Shared work", "--access", settings}, "POST", "/v1/cases"},
		{"case access", []string{"case", "access", "set", "case-id", "--access", settings, "--lock-version", "7", "--yes"}, "PUT", "/v1/cases/case-id/access"},
		{"playbook create", []string{"playbook", "create", "--owner-id", "owner-id", "--definition", `{"title":"Shared guide","steps":[]}`, "--access", settings}, "POST", "/v1/playbooks"},
		{"playbook copy", []string{"playbook", "copy", "source-id", "--source-version-id", "version-id", "--owner-id", "owner-id", "--access", settings}, "POST", "/v1/playbooks/source-id/copies"},
		{"input access", []string{"playbook", "access", "set", "playbook-id", "--input", `{"access":{"visibility":"public","grants":{"abcdefab-cdef-4abc-8def-abcdefabcdef":"viewer"}}}`, "--yes"}, "PUT", "/v1/playbooks/playbook-id/access"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				_, _ = io.WriteString(w, `{"access":`+settings+`}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var stdout, stderr bytes.Buffer
			if exit := Main(tc.args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
				t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
			}
			access, _ := body["access"].(map[string]any)
			grants, _ := access["grants"].(map[string]any)
			if access["visibility"] != "public" || !reflect.DeepEqual(grants, map[string]any{principal: "viewer"}) {
				t.Fatalf("access=%#v", access)
			}
			for _, key := range []string{"acl", "visibility", "editors"} {
				if _, ok := body[key]; ok {
					t.Fatalf("legacy field %s in %#v", key, body)
				}
			}
			if !strings.Contains(stdout.String(), principal) {
				t.Fatalf("output changed UUID: %s", stdout.String())
			}
			if tc.name == "playbook copy" && (body["sourceVersionId"] != "version-id" || body["ownerId"] != "owner-id") {
				t.Fatalf("copy body=%#v", body)
			}
			if tc.name == "case access" && body["expectedLockVersion"] != float64(7) {
				t.Fatalf("lock version=%#v", body)
			}
		})
	}
}

func TestSharedTeamCommandsUseSelectedWorkspace(t *testing.T) {
	for _, tc := range []struct {
		args         []string
		method, path string
		scoped       bool
	}{
		{[]string{"team", "invite", "team-id", "--emails", "one@example.com,two@example.com"}, "POST", "/v1/teams/team-id/invitations", true},
		{[]string{"team", "invitation", "list", "team-id"}, "GET", "/v1/teams/team-id/invitations", true},
		{[]string{"team", "invitation", "revoke", "team-id", "invitation-id", "--yes"}, "DELETE", "/v1/teams/team-id/invitations/invitation-id", true},
		{[]string{"team", "invitation", "get", "invitation-token"}, "GET", "/v1/invitations/invitation-token", false},
		{[]string{"team", "invitation", "accept", "invitation-token"}, "POST", "/v1/invitations/invitation-token/accept", true},
		{[]string{"team", "disconnect", "team-id", "--yes"}, "DELETE", "/v1/teams/team-id/connection", true},
	} {
		t.Run(strings.Join(tc.args[:3], " "), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/workspaces" {
					_, _ = io.WriteString(w, `{"workspaces":[{"id":"selected-id","handle":"selected"}]}`)
					return
				}
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("request=%s %s", r.Method, r.URL.Path)
				}
				workspace := r.URL.Query().Get("workspaceId")
				if tc.scoped && workspace != "selected-id" || !tc.scoped && workspace != "" {
					t.Errorf("workspace=%q scoped=%v", workspace, tc.scoped)
				}
				if tc.args[1] == "invite" {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if !reflect.DeepEqual(body["emails"], []any{"one@example.com", "two@example.com"}) {
						t.Errorf("body=%#v", body)
					}
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var stdout, stderr bytes.Buffer
			args := append([]string{"-w", "selected"}, tc.args...)
			if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
				t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
			}
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}
