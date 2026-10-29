package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCaseUpdateInputAndPlaybookRequests(t *testing.T) {
	requestFile := filepath.Join(t.TempDir(), "attachment.json")
	if err := os.WriteFile(requestFile, []byte(`{"version_id":"version-1","input":{"customer":"Acme"},"expected_lock_version":7,"idempotency_key":"retry-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want map[string]any
	}{
		{"title preserves input", []string{"--title", "Updated"}, map[string]any{"title": "Updated"}},
		{"replace input", []string{"--input", `{"customer":"Acme"}`}, map[string]any{"input": map[string]any{"customer": "Acme"}}},
		{"clear input", []string{"--input", `{}`}, map[string]any{"input": map[string]any{}}},
		{"attach preserving input", []string{"--version-id", "version-1"}, map[string]any{"versionId": "version-1"}},
		{"attach replacing input", []string{"--version-id", "version-1", "--input", `{"customer":"Acme"}`}, map[string]any{"versionId": "version-1", "input": map[string]any{"customer": "Acme"}}},
		{"request file", []string{"--request", "@" + requestFile}, map[string]any{"versionId": "version-1", "input": map[string]any{"customer": "Acme"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch || r.URL.Path != "/v1/cases/case-1" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			args := append([]string{"case", "update", "case-1", "--lock-version", "7", "--idempotency-key", "retry-key"}, tc.args...)
			var stdout, stderr bytes.Buffer
			if exit := Main(args, "test", strings.NewReader(""), &stdout, &stderr); exit != 0 {
				t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
			}
			want := map[string]any{"expectedLockVersion": float64(7), "idempotencyKey": "retry-key"}
			for field, value := range tc.want {
				want[field] = value
			}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("request body = %#v, want %#v", body, want)
			}
		})
	}
}

func TestCaseUpdateRejectsNonObjectInput(t *testing.T) {
	for _, input := range []string{`null`, `[]`, `"text"`, `{`} {
		t.Run(input, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid input reached the API")
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "test-token")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			var stdout, stderr bytes.Buffer
			if exit := Main([]string{"case", "update", "case-1", "--lock-version", "7", "--input", input}, "test", strings.NewReader(""), &stdout, &stderr); exit == 0 {
				t.Fatal("invalid input succeeded")
			}
		})
	}
}
