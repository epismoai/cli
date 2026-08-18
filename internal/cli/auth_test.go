package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPKCEAndAuthorizationURL(t *testing.T) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(digest[:]); challenge != want {
		t.Fatalf("challenge = %q, want %q", challenge, want)
	}
	if _, err := base64.RawURLEncoding.DecodeString(verifier); err != nil {
		t.Fatalf("verifier is not unpadded base64url: %v", err)
	}

	t.Setenv("EPISMO_WEB_URL", "https://web.example.test/")
	target, err := url.Parse(authorizationURL("http://127.0.0.1:1234/callback", challenge, "state-1", " User@Example.com "))
	if err != nil {
		t.Fatal(err)
	}
	query := target.Query()
	for key, want := range map[string]string{
		"response_type":         "code",
		"client_id":             cliClientID,
		"redirect_uri":          "http://127.0.0.1:1234/callback",
		"scope":                 "read write offline_access",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"state":                 "state-1",
		"login_hint":            " User@Example.com ",
	} {
		if got := query.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestOAuthCallbackServer(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		wantCode      string
		wantWorkspace string
		wantErrorCode string
	}{
		{name: "success", query: "state=expected&code=code-1&workspace_id=workspace-1", wantCode: "code-1", wantWorkspace: "workspace-1"},
		{name: "invalid state", query: "state=wrong&code=code-1", wantErrorCode: "INVALID_OAUTH_STATE"},
		{name: "authorization denied", query: "state=expected&error=access_denied", wantErrorCode: "AUTHORIZATION_DENIED"},
		{name: "missing code", query: "state=expected", wantErrorCode: "AUTHORIZATION_CODE_MISSING"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			redirectURI, resultChannel, stop, err := startCallbackServer("expected")
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			response, err := http.Get(redirectURI + "?" + test.query)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			select {
			case result := <-resultChannel:
				if result.code != test.wantCode || result.workspaceID != test.wantWorkspace {
					t.Errorf("result = %#v", result)
				}
				if test.wantErrorCode == "" {
					if result.err != nil {
						t.Fatalf("unexpected error: %v", result.err)
					}
				} else if errorCode(result.err) != test.wantErrorCode {
					t.Fatalf("error = %v, want code %s", result.err, test.wantErrorCode)
				}
			case <-time.After(time.Second):
				t.Fatal("callback result timed out")
			}
		})
	}
}

func TestOAuthCallbackIgnoresOtherPathsAndRejectsDuplicate(t *testing.T) {
	redirectURI, resultChannel, stop, err := startCallbackServer("expected")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	base := strings.TrimSuffix(redirectURI, "/callback")
	response, err := http.Get(base + "/not-callback")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("non-callback status = %d", response.StatusCode)
	}
	response, err = http.Get(redirectURI + "?state=expected&code=first")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	result := <-resultChannel
	if result.code != "first" || result.err != nil {
		t.Fatalf("first result = %#v", result)
	}
	response, err = http.Get(redirectURI + "?state=expected&code=second")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status = %d", response.StatusCode)
	}
}

func TestTokenRefreshPersistsRotatedCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth/token" || request.Method != http.MethodPost {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["grant_type"] != "refresh_token" || body["refresh_token"] != "old-refresh" || body["client_id"] != cliClientID {
			t.Errorf("body = %#v", body)
		}
		_, _ = io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_TOKEN", "")
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	if err := writeCredentials(storedCredentials{AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: "2000-01-01T00:00:00Z", ClientID: cliClientID, UserID: "user-1", AccountID: "account-1"}); err != nil {
		t.Fatal(err)
	}
	a := &app{client: newAPIClient("test"), stderr: &bytes.Buffer{}}
	auth, err := a.resolveAuthentication()
	if err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken != "new-access" || auth.AccountID != "account-1" || shouldRefresh(&storedCredentials{ExpiresAt: auth.ExpiresAt}) {
		t.Fatalf("auth = %#v", auth)
	}
	stored, err := readCredentials()
	if err != nil || stored == nil || stored.RefreshToken != "new-refresh" || stored.AccessToken != "new-access" {
		t.Fatalf("stored = %#v, err = %v", stored, err)
	}
}

func TestTokenRefreshFailureClassification(t *testing.T) {
	originalDelays := refreshRetryDelays
	refreshRetryDelays = nil
	defer func() { refreshRetryDelays = originalDelays }()
	for _, test := range []struct {
		name          string
		status        int
		wantRetryable bool
		wantMessage   string
	}{
		{name: "expired", status: http.StatusBadRequest, wantMessage: "Session expired"},
		{name: "server failure", status: http.StatusServiceUnavailable, wantRetryable: true, wantMessage: "network or server error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, `{"error":"refresh failed"}`)
			}))
			defer server.Close()
			t.Setenv("EPISMO_API_URL", server.URL)
			t.Setenv("EPISMO_TOKEN", "")
			t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
			if err := writeCredentials(storedCredentials{AccessToken: "old", RefreshToken: "refresh", ExpiresAt: "2000-01-01T00:00:00Z", ClientID: cliClientID, UserID: "user"}); err != nil {
				t.Fatal(err)
			}
			_, err := (&app{client: newAPIClient("test")}).resolveAuthentication()
			var cliErr *Error
			if !errors.As(err, &cliErr) || cliErr.Code != "TOKEN_REFRESH_FAILED" || cliErr.Retryable != test.wantRetryable || !strings.Contains(cliErr.Message, test.wantMessage) {
				t.Fatalf("error = %#v", cliErr)
			}
		})
	}
}

func TestOTPLoginExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/otp-tokens":
			_, _ = io.WriteString(w, `{"otpId":"otp-1"}`)
		case "/oauth/token":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["otp"] != "123456" || body["otp_id"] != "otp-1" || body["grant_type"] != "otp" {
				t.Errorf("body = %#v", body)
			}
			_, _ = io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)
		case "/oauth/userinfo":
			if request.Header.Get("Authorization") != "Bearer access" {
				t.Errorf("authorization = %q", request.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, `{"sub":"user-1","account_id":"account-1","email":"Verified@Example.com"}`)
		default:
			t.Errorf("unexpected path = %q", request.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	var stderr bytes.Buffer
	a := &app{client: newAPIClient("test"), stdin: strings.NewReader(" 123456\n"), stderr: &stderr, isTTY: true}
	credentials, email, err := a.loginOTP("user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if credentials.AccessToken != "access" || credentials.RefreshToken != "refresh" || credentials.UserID != "user-1" || credentials.AccountID != "account-1" || email != "verified@example.com" {
		t.Fatalf("credentials = %#v, email = %q", credentials, email)
	}
}

func TestInteractiveLoginRequirements(t *testing.T) {
	a := &app{isTTY: false}
	if _, _, err := a.loginOTP("user@example.com"); errorCode(err) != "NON_INTERACTIVE_INPUT_REQUIRED" {
		t.Fatalf("OTP error = %v", err)
	}
	if _, _, _, err := a.loginBrowser(""); errorCode(err) != "NON_INTERACTIVE_BROWSER_LOGIN" {
		t.Fatalf("browser error = %v", err)
	}
}

func TestLogoutRevokesAndClearsCredentials(t *testing.T) {
	var authorization string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	if err := writeCredentials(storedCredentials{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: "2999-01-01T00:00:00Z", ClientID: cliClientID, UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	result, err := (&app{client: newAPIClient("test")}).logout()
	if err != nil {
		t.Fatal(err)
	}
	if result["hadCredentials"] != true || authorization != "Bearer access" || body["token"] != "refresh" || body["client_id"] != cliClientID {
		t.Fatalf("result = %#v, authorization = %q, body = %#v", result, authorization, body)
	}
	if credentials, err := readCredentials(); err != nil || credentials != nil {
		t.Fatalf("credentials after logout = %#v, err = %v", credentials, err)
	}
}

func TestAPIRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	client := &apiClient{baseURL: server.URL, version: "test", http: &http.Client{Timeout: time.Millisecond}}
	_, err := client.request(http.MethodGet, "/slow", "", nil)
	var cliErr *Error
	if !errors.As(err, &cliErr) || cliErr.Code != "REQUEST_TIMEOUT" || !cliErr.Retryable {
		t.Fatalf("error = %#v", cliErr)
	}
}

func TestHTTPErrorCodeMapping(t *testing.T) {
	for status, want := range map[int]string{
		400: "BAD_REQUEST",
		401: "UNAUTHORIZED",
		402: "PAYMENT_REQUIRED",
		403: "FORBIDDEN",
		404: "NOT_FOUND",
		409: "CONFLICT",
		422: "UNPROCESSABLE_ENTITY",
		429: "RATE_LIMITED",
		500: "SERVER_ERROR",
		418: "HTTP_ERROR",
	} {
		if got := httpErrorCode(status); got != want {
			t.Errorf("status %d = %q, want %q", status, got, want)
		}
	}
}

func errorCode(err error) string {
	var cliErr *Error
	if errors.As(err, &cliErr) {
		return cliErr.Code
	}
	return ""
}
