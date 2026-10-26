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

func TestOwnershipAndLifecycleRequests(t *testing.T) {
	contract := readOpenAPIContract(t)
	components, _ := contract["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	handover, _ := schemas["HandoverWorkspaceMemberRequest"].(map[string]any)
	properties, _ := handover["properties"].(map[string]any)
	required, _ := handover["required"].([]any)
	if len(properties) == 0 || len(required) == 0 {
		t.Fatal("handover request schema is missing properties or required fields")
	}

	for _, tc := range []struct {
		name, method, path string
		args               []string
		field              string
		value              any
	}{
		{"create default home", "POST", "/v1/playbooks", []string{"playbook", "create", "--definition", `{"title":"Guide"}`}, "ownerId", nil},
		{"copy default home", "POST", "/v1/playbooks/source/copies", []string{"playbook", "copy", "source", "--source-version-id", "version"}, "ownerId", nil},
		{"case explicit home", "POST", "/v1/cases", []string{"case", "start", "--owner-id", "home", "--title", "Work"}, "ownerId", "home"},
		{"playbook assignment", "PATCH", "/v1/playbooks/guide/assignee", []string{"playbook", "assign", "guide", "--assigned-to", "user"}, "assignedTo", "user"},
		{"case ownership", "PATCH", "/v1/cases/case/owner", []string{"case", "owner", "transfer", "case", "--owner-id", "home", "--lock-version", "7"}, "expectedLockVersion", float64(7)},

		{"playbook restore", "POST", "/v1/playbooks/guide/restore", []string{"playbook", "restore", "guide"}, "", nil},
		{"case archive", "POST", "/v1/cases/case/archive", []string{"case", "archive", "case", "--lock-version", "3", "--yes"}, "expectedLockVersion", float64(3)},
		{"case restore", "POST", "/v1/cases/case/restore", []string{"case", "restore", "case", "--lock-version", "4"}, "expectedLockVersion", float64(4)},
		{"case delete", "DELETE", "/v1/cases/case", []string{"case", "delete", "case", "--lock-version", "5", "--yes"}, "expectedLockVersion", float64(5)},
		{"handover preview", "POST", "/v1/workspaces/home/member-handover", []string{"workspace", "member", "handover", "home", "--member-id", "old", "--successor-id", "new"}, "preview", true},
		{"handover transfer approvals", "POST", "/v1/workspaces/home/member-handover", []string{"workspace", "member", "handover", "home", "--member-id", "old", "--successor-id", "new", "--preview=false", "--transfer-approvals"}, "transferApprovals", true},
		{"handover finalize", "POST", "/v1/workspaces/home/member-handover", []string{"workspace", "member", "handover", "home", "--member-id", "old", "--successor-id", "new", "--preview=false", "--finalize"}, "preview", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if tc.field != "" && body[tc.field] != tc.value {
					t.Errorf("body[%s] = %#v, want %#v", tc.field, body[tc.field], tc.value)
				}
				if tc.name == "create default home" || tc.name == "copy default home" || tc.name == "case explicit home" {
					access, _ := body["access"].(map[string]any)
					grants, ok := access["grants"].(map[string]any)
					if access["visibility"] != "restricted" || !ok || len(grants) != 0 {
						t.Errorf("default access = %#v, want restricted visibility and empty grants", access)
					}
				}
				if tc.name == "case delete" {
					if _, exists := body["confirmationCode"]; exists {
						t.Error("case deletion sent a removed confirmation-code field")
					}
				}
				if strings.HasPrefix(tc.name, "handover") {
					if body["memberId"] != "old" || body["successorId"] != "new" {
						t.Errorf("handover participants = %#v", body)
					}
					for field := range body {
						if _, ok := properties[field]; !ok {
							t.Errorf("handover sent field %q absent from the API contract", field)
						}
					}
					for _, field := range required {
						if _, ok := body[field.(string)]; !ok {
							t.Errorf("handover omitted required API field %q", field)
						}
					}
				}
				if tc.name == "case ownership" && body["ownerId"] != "home" {
					t.Errorf("ownerId = %#v", body["ownerId"])
				}
				if tc.name == "handover finalize" && body["finalize"] != true {
					t.Errorf("finalize = %#v", body["finalize"])
				}
				if body["idempotencyKey"] == nil {
					t.Error("missing idempotency key")
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var out, stderr bytes.Buffer
			if code := Main(tc.args, "test", strings.NewReader(""), &out, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			if calls != 1 {
				t.Fatalf("requests = %d; omitted ownership must not trigger userinfo lookup", calls)
			}
		})
	}
}

func TestCreationPreservesSelectedWorkspaceWithoutChoosingPersonalOwner(t *testing.T) {
	for _, args := range [][]string{
		{"playbook", "create", "--definition", `{"title":"Guide"}`},
		{"playbook", "copy", "source", "--source-version-id", "version"},
		{"case", "start", "--title", "Work"},
	} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/v1/workspaces" {
					_, _ = io.WriteString(w, `{"workspaces":[{"id":"workspace-id","handle":"selected"}]}`)
					return
				}
				if r.Method != "POST" {
					t.Errorf("unexpected lookup: %s %s", r.Method, r.URL)
					return
				}
				writes++
				if r.URL.Query().Get("workspaceId") != "workspace-id" {
					t.Errorf("workspace query = %s", r.URL.RawQuery)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if _, exists := body["ownerId"]; exists {
					t.Errorf("default ownership was overridden: %#v", body)
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			t.Setenv("EPISMO_WORKSPACE", "selected")
			var out, stderr bytes.Buffer
			if code := Main(args, "test", strings.NewReader(""), &out, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			if writes != 1 {
				t.Fatalf("writes = %d", writes)
			}
		})
	}
}

func TestArchivedListsSendFilters(t *testing.T) {
	for _, resource := range []string{"case", "playbook"} {
		t.Run(resource, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/v1/"+resource+"s" || r.URL.Query().Get("archived") != "true" {
					t.Errorf("request = %s %s", r.Method, r.URL)
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var out, stderr bytes.Buffer
			if code := Main([]string{resource, "list", "--archived"}, "test", strings.NewReader(""), &out, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			if calls != 1 {
				t.Fatalf("requests = %d", calls)
			}
		})
	}
}
