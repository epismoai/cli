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

func TestCaseUpdatePreservesPlaybookSelectionIntent(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		present     bool
		wantVersion any
	}{
		{"replace with flag", []string{"--version-id", "new-version", "--lock-version", "7"}, true, "new-version"},
		{"remove with snake case JSON", []string{"--request", `{"version_id":null,"expected_lock_version":7}`}, true, nil},
		{"remove with camel case JSON", []string{"--request", `{"versionId":null,"expectedLockVersion":7}`}, true, nil},
		{"preserve omitted selection", []string{"--title", "Renamed", "--lock-version", "7"}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPatch || r.URL.Path != "/v1/cases/case-id" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				version, present := body["versionId"]
				if present != tc.present || version != tc.wantVersion {
					t.Errorf("versionId = %#v (present=%v), want %#v (present=%v)", version, present, tc.wantVersion, tc.present)
				}
				if body["expectedLockVersion"] != float64(7) || body["idempotencyKey"] == nil {
					t.Errorf("missing mutation safeguards: %#v", body)
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_WORKSPACE", "")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var stdout, stderr bytes.Buffer
			args := append([]string{"case", "update", "case-id"}, tc.args...)
			if code := Main(args, "test", strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr.String())
			}
			if calls != 1 {
				t.Errorf("requests = %d, want 1", calls)
			}
		})
	}
}
