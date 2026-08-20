package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type apiClient struct {
	baseURL string
	version string
	http    *http.Client
}

func newAPIClient(version string) *apiClient {
	return &apiClient{baseURL: apiURL(), version: version, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *apiClient) request(method, path, token string, body any) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, &Error{Code: "REQUEST_ENCODING_FAILED", Message: "Failed to encode the API request.", ExitCode: 1, Cause: err}
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, c.baseURL+path, reader)
	if err != nil {
		return nil, &Error{Code: "REQUEST_BUILD_FAILED", Message: "Failed to create the API request.", Hint: "Check EPISMO_API_URL and try again.", ExitCode: 1, Cause: err}
	}
	req.Header.Set("User-Agent", "epismo/"+c.version)
	req.Header.Set("X-Epismo-Source", "cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(req)
	if err != nil {
		code, message := "NETWORK_ERROR", "Network request failed."
		if errors.Is(err, context.DeadlineExceeded) {
			code, message = "REQUEST_TIMEOUT", "Request timed out."
		}
		return nil, &Error{Code: code, Message: message, Retryable: true, ExitCode: 1, Cause: err}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &Error{Code: "RESPONSE_READ_FAILED", Message: "Failed to read the API response.", Retryable: true, Details: map[string]any{"status": response.StatusCode, "pathname": path}, ExitCode: 1, Cause: err}
	}
	result := map[string]any{}
	var decodeErr error
	if len(bytes.TrimSpace(data)) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			decodeErr = err
		} else if err := decoder.Decode(&struct{}{}); err != io.EOF {
			decodeErr = fmt.Errorf("multiple JSON values")
		} else {
			var ok bool
			result, ok = decoded.(map[string]any)
			if !ok {
				decodeErr = fmt.Errorf("JSON object expected")
			}
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code, message, apiRetryable, apiError := apiErrorFields(result, response.StatusCode)
		details := map[string]any{"status": response.StatusCode, "pathname": path}
		if apiDetails, ok := result["details"]; ok {
			details["apiDetails"] = apiDetails
		}
		if apiError != nil {
			details["apiError"] = apiError
		}
		if decodeErr != nil {
			details["invalidResponse"] = true
		}
		if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
			details["retryAfter"] = retryAfter
		}
		return nil, &Error{Code: code, Message: message, Retryable: apiRetryable || retryableHTTPStatus(response.StatusCode), Details: details, ExitCode: 1}
	}
	if decodeErr != nil {
		return nil, &Error{Code: "INVALID_API_RESPONSE", Message: "API returned an invalid JSON response.", Retryable: true, Details: map[string]any{"status": response.StatusCode, "pathname": path}, ExitCode: 1, Cause: decodeErr}
	}
	return result, nil
}

func apiErrorFields(result map[string]any, status int) (string, string, bool, any) {
	code := httpErrorCode(status)
	message := fmt.Sprintf("HTTP %d", status)
	retryable := false
	var rawError any

	if value, ok := result["error"]; ok {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				message = typed
				if isMachineCode(typed) {
					code = strings.ToUpper(snakeCase(typed))
				}
			}
		case map[string]any:
			rawError = typed
			if value := stringField(typed, "code"); value != "" {
				code = strings.ToUpper(snakeCase(value))
			}
			if value := stringField(typed, "message"); value != "" {
				message = value
			}
			if value, ok := typed["retryable"].(bool); ok {
				retryable = value
			}
		}
	}
	if value := stringField(result, "code"); value != "" {
		code = strings.ToUpper(snakeCase(value))
	}
	if value, ok := result["retryable"].(bool); ok {
		retryable = value
	}
	for _, key := range []string{"error_description", "message"} {
		if value, ok := result[key].(string); ok && strings.TrimSpace(value) != "" {
			message = value
			break
		}
	}
	return code, message, retryable, rawError
}

func isMachineCode(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, current := range trimmed {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) && current != '_' && current != '-' {
			return false
		}
	}
	return true
}

func retryableHTTPStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooEarly || status == http.StatusTooManyRequests || status >= 500
}

func httpErrorCode(status int) string {
	switch status {
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 402:
		return "PAYMENT_REQUIRED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 409:
		return "CONFLICT"
	case 422:
		return "UNPROCESSABLE_ENTITY"
	case 429:
		return "RATE_LIMITED"
	default:
		if status >= 500 {
			return "SERVER_ERROR"
		}
		return "HTTP_ERROR"
	}
}

func withWorkspace(path, workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "workspaceId=" + url.QueryEscape(strings.TrimSpace(workspaceID))
}
