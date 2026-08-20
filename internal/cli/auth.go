package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

const (
	cliClientID          = "epismo-cli"
	refreshThreshold     = 5 * time.Minute
	authorizationTimeout = 5 * time.Minute
)

// refreshRetryDelays paces the wait for a parallel invocation to finish a
// concurrent token refresh. Credentials are re-read after each delay elapses.
var refreshRetryDelays = []time.Duration{
	100 * time.Millisecond,
	200 * time.Millisecond,
	400 * time.Millisecond,
	800 * time.Millisecond,
}

func randomBase64URL(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func generatePKCE() (string, string, error) {
	verifier, err := randomBase64URL(32)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// browserCommand builds the platform command that opens target. It takes goos
// explicitly so the argument list can be asserted from any host.
func browserCommand(goos, target string) *exec.Cmd {
	switch goos {
	case "darwin":
		return exec.Command("open", target)
	case "windows":
		// Not `cmd /c start`: Go's Windows argument escaping quotes only args
		// containing spaces, tabs, or quotes, so the `&` separators in the
		// authorization URL reach cmd.exe unquoted and are parsed as command
		// separators. rundll32 receives the URL without a shell in between.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return exec.Command("xdg-open", target)
	}
}

func openBrowser(target string) error {
	command := browserCommand(runtime.GOOS, target)
	if err := command.Start(); err != nil {
		return &Error{Code: "BROWSER_OPEN_FAILED", Message: err.Error(), Hint: "Open this URL manually in your browser: " + target, ExitCode: 1, Cause: err}
	}
	go func() { _ = command.Wait() }()
	return nil
}

type callbackResult struct {
	code        string
	workspaceID string
	err         error
}

func startCallbackServer(expectedState string) (string, <-chan callbackResult, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, &Error{Code: "OAUTH_CALLBACK_FAILED", Message: "Failed to bind a local callback port for browser login.", ExitCode: 1, Cause: err}
	}
	redirectURI := "http://" + listener.Addr().String() + "/callback"
	result := make(chan callbackResult, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	var sent atomic.Bool
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/callback" {
			http.NotFound(w, request)
			return
		}
		if !sent.CompareAndSwap(false, true) {
			http.Error(w, "Authorization already completed.", http.StatusConflict)
			return
		}
		receivedState := request.URL.Query().Get("state")
		if len(receivedState) != len(expectedState) || subtle.ConstantTimeCompare([]byte(receivedState), []byte(expectedState)) != 1 {
			_, _ = fmt.Fprint(w, "<html><body><p>Authorization failed. Please close this tab and try again.</p></body></html>")
			result <- callbackResult{err: &Error{Code: "INVALID_OAUTH_STATE", Message: "OAuth callback state did not match the login request.", Retryable: true, Hint: "Run `epismo login` in an interactive terminal, or use a pre-issued EPISMO_TOKEN.", ExitCode: 1}}
			return
		}
		if oauthError := request.URL.Query().Get("error"); oauthError != "" {
			_, _ = fmt.Fprint(w, "<html><body><p>Authorization denied. You may close this tab.</p></body></html>")
			result <- callbackResult{err: &Error{Code: "AUTHORIZATION_DENIED", Message: "Authorization denied: " + oauthError, ExitCode: 1}}
			return
		}
		code := request.URL.Query().Get("code")
		if code == "" {
			_, _ = fmt.Fprint(w, "<html><body><p>Authorization failed. Please close this tab and try again.</p></body></html>")
			result <- callbackResult{err: &Error{Code: "AUTHORIZATION_CODE_MISSING", Message: "No authorization code received from the browser callback.", Retryable: true, ExitCode: 1}}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, "<html><body><p>Authorization complete. Return to your terminal to continue.</p></body></html>")
		result <- callbackResult{code: code, workspaceID: request.URL.Query().Get("workspace_id")}
	})
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			select {
			case result <- callbackResult{err: &Error{Code: "OAUTH_CALLBACK_FAILED", Message: err.Error(), Retryable: true, ExitCode: 1}}:
			default:
			}
		}
	}()
	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
	return redirectURI, result, stop, nil
}

func authorizationURL(redirectURI, challenge, state, email string) string {
	target, _ := url.Parse(webURL() + "/oauth/authorize")
	query := target.Query()
	query.Set("response_type", "code")
	query.Set("client_id", cliClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", "read write offline_access")
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	if email != "" {
		query.Set("login_hint", email)
	}
	target.RawQuery = query.Encode()
	return target.String()
}

func (a *app) token(body map[string]any) (map[string]any, error) {
	return a.client.request(http.MethodPost, "/oauth/token", "", body)
}

func (a *app) userInfo(token string) (map[string]any, error) {
	return a.client.request(http.MethodGet, "/oauth/userinfo", token, nil)
}

func (a *app) workspaces(token string) ([]map[string]any, error) {
	response, err := a.client.request(http.MethodGet, "/v1/workspaces", token, nil)
	if err != nil {
		return nil, err
	}
	items, _ := response["workspaces"].([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			result = append(result, object)
		}
	}
	return result, nil
}

func stringField(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return strings.TrimSpace(value)
}

func intField(object map[string]any, key string) int {
	switch value := object[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case fmt.Stringer:
		var parsed int
		_, _ = fmt.Sscan(value.String(), &parsed)
		return parsed
	default:
		return 0
	}
}

func verifiedCredentials(tokenResponse, user map[string]any) (storedCredentials, string) {
	expiresAt := time.Now().Add(time.Duration(intField(tokenResponse, "expires_in")) * time.Second).UTC().Format(time.RFC3339Nano)
	return storedCredentials{
		AccessToken:  stringField(tokenResponse, "access_token"),
		RefreshToken: stringField(tokenResponse, "refresh_token"),
		ExpiresAt:    expiresAt,
		ClientID:     cliClientID,
		UserID:       stringField(user, "sub"),
		AccountID:    stringField(user, "account_id"),
	}, normalizeEmail(stringField(user, "email"))
}

func shouldRefresh(credentials *storedCredentials) bool {
	expiresAt, err := time.Parse(time.RFC3339Nano, credentials.ExpiresAt)
	return err != nil || time.Until(expiresAt) < refreshThreshold
}

func (a *app) login(email string) (map[string]any, error) {
	config, err := readConfig()
	if err != nil {
		return nil, err
	}
	requestedEmail := normalizeEmail(email)
	existing, err := readCredentials()
	if err != nil {
		return nil, err
	}
	if existing != nil && !shouldRefresh(existing) && (requestedEmail == "" || requestedEmail == normalizeEmail(config.LastLoginEmail)) {
		return loginResult(*existing, config.LastLoginEmail, config, true), nil
	}

	var credentials storedCredentials
	var verifiedEmail, workspaceID string
	if requestedEmail != "" {
		method, err := a.client.request(http.MethodPost, "/v1/login-options", "", map[string]any{"email": requestedEmail})
		if err != nil {
			return nil, err
		}
		loginMethod := stringField(method, "method")
		if loginMethod != "otp" && loginMethod != "sso" {
			return nil, &Error{Code: "LOGIN_METHOD_INVALID", Message: "The server returned an unsupported login method.", Hint: "Update the Epismo CLI and try again.", ExitCode: 1}
		}
		if loginMethod == "otp" {
			credentials, verifiedEmail, err = a.loginOTP(requestedEmail)
			if err != nil {
				return nil, err
			}
		} else {
			credentials, verifiedEmail, workspaceID, err = a.loginBrowser(requestedEmail)
			if err != nil {
				return nil, err
			}
		}
	} else {
		credentials, verifiedEmail, workspaceID, err = a.loginBrowser("")
		if err != nil {
			return nil, err
		}
	}
	if verifiedEmail != "" {
		config.LastLoginEmail = verifiedEmail
	}
	if workspaceID != "" && workspaceID != credentials.UserID {
		items, listErr := a.workspaces(credentials.AccessToken)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range items {
			if stringField(item, "id") == workspaceID {
				config.DefaultWorkspace = workspaceFromMap(item)
				break
			}
		}
	}
	if err := writeCredentials(credentials); err != nil {
		return nil, err
	}
	if err := writeConfig(config); err != nil {
		return nil, err
	}
	return loginResult(credentials, config.LastLoginEmail, config, false), nil
}

func (a *app) loginOTP(email string) (storedCredentials, string, error) {
	if !a.isTTY {
		return storedCredentials{}, "", &Error{Code: "NON_INTERACTIVE_INPUT_REQUIRED", Message: "Email-code login requires an interactive terminal.", Hint: "Run the command in a terminal, or use a pre-issued EPISMO_TOKEN.", ExitCode: 1}
	}
	issued, err := a.client.request(http.MethodPost, "/v1/otp-tokens", "", map[string]any{"email": email})
	if err != nil {
		return storedCredentials{}, "", err
	}
	otpID := stringField(issued, "otpId")
	if otpID == "" {
		return storedCredentials{}, "", &Error{Code: "OTP_ID_MISSING", Message: "Failed to obtain an OTP id from the server.", Retryable: true, ExitCode: 1}
	}
	a.event("info", "OTP_SENT", "A sign-in code was sent.", map[string]any{"email": email})
	_, _ = fmt.Fprint(a.stderr, "Code: ")
	code, _ := bufio.NewReader(a.stdin).ReadString('\n')
	code = strings.TrimSpace(code)
	if code == "" {
		return storedCredentials{}, "", &Error{Code: "OTP_CODE_REQUIRED", Message: "A sign-in code is required.", Hint: "Run `epismo login --email " + email + "` to request a new code.", ExitCode: 1}
	}
	token, err := a.token(map[string]any{"grant_type": "otp", "otp_id": otpID, "otp": code, "client_id": cliClientID})
	if err != nil {
		return storedCredentials{}, "", err
	}
	user, err := a.userInfo(stringField(token, "access_token"))
	if err != nil {
		return storedCredentials{}, "", err
	}
	credentials, verifiedEmail := verifiedCredentials(token, user)
	return credentials, verifiedEmail, nil
}

func (a *app) loginBrowser(email string) (storedCredentials, string, string, error) {
	if !a.isTTY {
		return storedCredentials{}, "", "", &Error{Code: "NON_INTERACTIVE_BROWSER_LOGIN", Message: "Browser login requires an interactive terminal.", Hint: "Run `epismo login` in an interactive terminal, or use a pre-issued EPISMO_TOKEN.", ExitCode: 1}
	}
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return storedCredentials{}, "", "", err
	}
	state, err := randomBase64URL(32)
	if err != nil {
		return storedCredentials{}, "", "", err
	}
	redirectURI, callback, stop, err := startCallbackServer(state)
	if err != nil {
		return storedCredentials{}, "", "", err
	}
	defer stop()
	target := authorizationURL(redirectURI, challenge, state, email)
	a.event("info", "BROWSER_OPENING", "Opening browser for authorization...", nil)
	if err := openBrowser(target); err != nil {
		a.event("warning", "BROWSER_OPEN_FAILED", "Could not open the browser automatically.", nil)
	}
	a.event("info", "BROWSER_FALLBACK_URL", "Open this URL if the browser did not open.", map[string]any{"url": target})
	a.event("info", "BROWSER_WAITING", "Waiting for authorization in your browser (times out in 5 minutes)...", map[string]any{"timeout_seconds": 300})
	var result callbackResult
	select {
	case result = <-callback:
	case <-time.After(authorizationTimeout):
		return storedCredentials{}, "", "", &Error{Code: "AUTHORIZATION_TIMED_OUT", Message: "Authorization timed out after 5 minutes.", Retryable: true, Hint: "Run `epismo login` in an interactive terminal, or use a pre-issued EPISMO_TOKEN.", ExitCode: 1}
	}
	if result.err != nil {
		return storedCredentials{}, "", "", result.err
	}
	a.event("info", "BROWSER_AUTHORIZED", "Authorization received. Completing sign-in...", nil)
	token, err := a.token(map[string]any{"grant_type": "authorization_code", "code": result.code, "client_id": cliClientID, "redirect_uri": redirectURI, "code_verifier": verifier})
	if err != nil {
		return storedCredentials{}, "", "", err
	}
	user, err := a.userInfo(stringField(token, "access_token"))
	if err != nil {
		return storedCredentials{}, "", "", err
	}
	credentials, verifiedEmail := verifiedCredentials(token, user)
	return credentials, verifiedEmail, result.workspaceID, nil
}

func loginResult(credentials storedCredentials, email string, config cliConfig, already bool) map[string]any {
	result := map[string]any{"loggedIn": !already, "userId": credentials.UserID, "expiresAt": credentials.ExpiresAt}
	if already {
		delete(result, "loggedIn")
		result["alreadyLoggedIn"] = true
		result["nextActions"] = []any{"Run `epismo logout` first to switch accounts, or pass --email <other> to sign in as someone else."}
	} else if config.DefaultWorkspace == nil {
		result["nextActions"] = []any{"Run `epismo workspace use <workspace-id>` to set a default workspace."}
	}
	if email != "" {
		result["lastLoginEmail"] = email
	}
	return result
}

func (a *app) resolveAuthentication() (authContext, error) {
	if token := strings.TrimSpace(os.Getenv("EPISMO_TOKEN")); token != "" {
		return authContext{AccessToken: token}, nil
	}
	credentials, err := readCredentials()
	if err != nil {
		return authContext{}, err
	}
	if credentials == nil {
		return authContext{}, &Error{Code: "NOT_AUTHENTICATED", Message: "Not authenticated.", Hint: "Set EPISMO_TOKEN or run `epismo login`.", ExitCode: 1}
	}
	if shouldRefresh(credentials) {
		original := credentials.AccessToken
		token, refreshErr := a.token(map[string]any{"grant_type": "refresh_token", "refresh_token": credentials.RefreshToken, "client_id": credentials.ClientID})
		if refreshErr == nil {
			credentials.AccessToken = stringField(token, "access_token")
			credentials.RefreshToken = stringField(token, "refresh_token")
			credentials.ExpiresAt = time.Now().Add(time.Duration(intField(token, "expires_in")) * time.Second).UTC().Format(time.RFC3339Nano)
			if err := writeCredentials(*credentials); err != nil {
				return authContext{}, err
			}
		} else {
			// A parallel invocation may have won the refresh race: the server
			// revokes the old refresh token on rotation, so the loser's request
			// fails while valid credentials land on disk moments later. Re-read
			// after every delay, including the last one.
			for attempt := 0; ; attempt++ {
				latest, _ := readCredentials()
				if latest != nil && latest.UserID == credentials.UserID && latest.AccessToken != original && !shouldRefresh(latest) {
					credentials = latest
					refreshErr = nil
					break
				}
				if attempt >= len(refreshRetryDelays) {
					break
				}
				time.Sleep(refreshRetryDelays[attempt])
			}
			if refreshErr != nil {
				normalized := normalizeError(refreshErr)
				message := "Session expired. Please log in again."
				hint := "Set EPISMO_TOKEN or run `epismo login`."
				if normalized.Retryable {
					message = "Token refresh failed due to a network or server error."
					hint = "Check your network connection and try again, or run `epismo login`."
				}
				return authContext{}, &Error{Code: "TOKEN_REFRESH_FAILED", Message: message, Retryable: normalized.Retryable, Hint: hint, ExitCode: 1}
			}
		}
	}
	return authContext{UserID: credentials.UserID, AccessToken: credentials.AccessToken, ExpiresAt: credentials.ExpiresAt, AccountID: credentials.AccountID}, nil
}

func workspaceFromMap(item map[string]any) *savedWorkspace {
	return &savedWorkspace{ID: stringField(item, "id"), Handle: stringField(item, "handle"), AccountID: stringField(item, "accountId"), Role: stringField(item, "role")}
}

func (a *app) logout() (map[string]any, error) {
	credentials, err := readCredentials()
	if err != nil {
		return nil, err
	}
	hadCredentials := credentials != nil && credentials.AccessToken != ""
	remoteRevocation := "not_needed"
	var revocationError map[string]any
	if credentials != nil && credentials.RefreshToken != "" {
		remoteRevocation = "succeeded"
		if _, revokeErr := a.client.request(http.MethodPost, "/oauth/revoke", credentials.AccessToken, map[string]any{"token": credentials.RefreshToken, "client_id": credentials.ClientID}); revokeErr != nil {
			remoteRevocation = "failed"
			normalized := normalizeError(revokeErr)
			revocationError = errorPayload(normalized)
			a.event("warning", "TOKEN_REVOCATION_FAILED", "Local credentials will be cleared, but remote token revocation failed.", map[string]any{"error": revocationError})
		}
	}
	if err := clearCredentials(); err != nil {
		return nil, err
	}
	result := map[string]any{"cleared": true, "hadCredentials": hadCredentials, "remoteRevocation": remoteRevocation}
	if revocationError != nil {
		result["revocationError"] = revocationError
	}
	return result, nil
}
