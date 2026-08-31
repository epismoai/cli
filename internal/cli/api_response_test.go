package cli

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIErrorPreservesStructuredServerError(t *testing.T) {
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error":{"code":"workspaceHandleTaken","message":"Handle is already in use.","retryable":false,"conflictingWorkspaceId":"workspace-1"}}`)
	}))
	defer server.Close()
	client := newAPIClient("test")
	client.baseURL = server.URL

	_, err := client.request(http.MethodPost, "/v1/workspaces", "token", map[string]any{"handle": "taken"})
	var cliErr *Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("error = %v", err)
	}
	if cliErr.Code != "WORKSPACE_HANDLE_TAKEN" || cliErr.Message != "Handle is already in use." || cliErr.Retryable {
		t.Fatalf("error = %#v", cliErr)
	}
	apiError, _ := cliErr.Details["apiError"].(map[string]any)
	if stringField(apiError, "conflictingWorkspaceId") != "workspace-1" {
		t.Fatalf("api error details = %#v", apiError)
	}
}

func TestInvalidSuccessfulAPIResponseFails(t *testing.T) {
	for name, response := range map[string]string{
		"malformed": `not json`,
		"array":     `[]`,
		"multiple":  `{} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()
			client := newAPIClient("test")
			client.baseURL = server.URL

			_, err := client.request(http.MethodGet, "/test", "", nil)
			var cliErr *Error
			if !errors.As(err, &cliErr) || cliErr.Code != "INVALID_API_RESPONSE" || !cliErr.Retryable {
				t.Fatalf("error = %#v", cliErr)
			}
		})
	}
}

func TestRetryableHTTPStatuses(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests, http.StatusBadGateway} {
		if !retryableHTTPStatus(status) {
			t.Errorf("status %d should be retryable", status)
		}
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusConflict} {
		if retryableHTTPStatus(status) {
			t.Errorf("status %d should not be retryable", status)
		}
	}
}
