package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoveryFiltersReachTheAPI(t *testing.T) {
	for _, tc := range []struct {
		args                 []string
		path                 string
		extraKey, extraValue string
	}{
		{[]string{"case", "list", "--sharing-scope", "anyone", "--playbook-id", "guide", "--status", "open"}, "/v1/cases", "playbookId", "guide"},
		{[]string{"playbook", "list", "--sharing-scope", "shared", "--assigned-to", "maintainer"}, "/v1/playbooks", "assignedTo", "maintainer"},
		{[]string{"playbook", "search", "--query", "incident", "--sharing-scope", "shared", "--assigned-to", "maintainer"}, "/v1/playbooks", "assignedTo", "maintainer"},
		{[]string{"playbook", "resource", "list", "--kind", "cli", "--sharing-scope", "shared", "--assigned-to", "maintainer"}, "/v1/playbook-resources", "assignedTo", "maintainer"},
	} {
		t.Run(strings.Join(tc.args[:2], " "), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != tc.path || r.URL.Query().Get(tc.extraKey) != tc.extraValue {
					t.Errorf("request = %s", r.URL.RequestURI())
				}
				wantScope := "shared"
				if tc.path == "/v1/cases" {
					wantScope = "anyone"
					if r.URL.Query().Get("status") != "open" {
						t.Errorf("status missing: %s", r.URL.RequestURI())
					}
				}
				if r.URL.Query().Get("sharingScope") != wantScope || r.URL.Query().Has("includePublic") {
					t.Errorf("discovery query = %s", r.URL.RawQuery)
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_WORKSPACE", "")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var out, stderr bytes.Buffer
			if code := Main(tc.args, "test", strings.NewReader(""), &out, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			if calls != 1 {
				t.Fatalf("requests = %d", calls)
			}
		})
	}
}
