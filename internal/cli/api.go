package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
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
		code := "NETWORK_ERROR"
		message := err.Error()
		if strings.Contains(message, "context deadline exceeded") {
			code, message = "REQUEST_TIMEOUT", "Request timed out."
		}
		return nil, &Error{Code: code, Message: message, Retryable: true, ExitCode: 1, Cause: err}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	if len(data) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			result = map[string]any{"error": string(data)}
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := fmt.Sprintf("HTTP %d", response.StatusCode)
		for _, key := range []string{"error", "error_description", "message"} {
			if value, ok := result[key].(string); ok && value != "" {
				message = value
				break
			}
		}
		details := map[string]any{"status": response.StatusCode, "pathname": path}
		if apiDetails, ok := result["details"]; ok {
			details["apiDetails"] = apiDetails
		}
		if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
			details["retryAfter"] = retryAfter
		}
		return nil, &Error{Code: httpErrorCode(response.StatusCode), Message: message, Retryable: response.StatusCode >= 500 || response.StatusCode == 429, Details: details, ExitCode: 1}
	}
	return result, nil
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
