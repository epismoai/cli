package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func str(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindString}
}
func integer(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindInteger}
}
func object(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindObject}
}
func objectSource(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindObjectSource}
}
func array(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindArray}
}
func csv(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindCSV}
}
func choice(name, field, help string, choices ...string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindString, Choices: choices}
}
func requiredOption(option optionSpec) optionSpec           { option.Required = true; return option }
func defaultOption(option optionSpec, value any) optionSpec { option.Default = value; return option }

func pagingOptions() []optionSpec {
	return []optionSpec{integer("--page-size", "pageSize", "results per page (1-100)"), str("--cursor", "cursor", "cursor returned by the previous page")}
}

func (i invocation) payload(a *app) (map[string]any, error) {
	base := map[string]any{}
	if raw := i.text("_input"); raw != "" {
		var err error
		base, err = readObjectSource(raw, "--input", a.stdin)
		if err != nil {
			return nil, err
		}
	}
	overrides := map[string]any{}
	for field, value := range i.Values {
		if field == "_input" {
			continue
		}
		if i.Present[field] {
			overrides[field] = value
		}
	}
	result := mergeDefined(base, overrides)
	for _, option := range baseOptions(i.Command) {
		if option.Field == "_input" {
			continue
		}
		if _, exists := result[option.Field]; !exists && option.Default != nil {
			result[option.Field] = option.Default
		}
	}
	for _, option := range baseOptions(i.Command) {
		value, exists := result[option.Field]
		if option.Required && (!exists || value == nil) {
			return nil, required(option.Name)
		}
	}
	if i.Command.Mutation {
		if key, _ := result["idempotencyKey"].(string); strings.TrimSpace(key) == "" {
			result["idempotencyKey"] = newUUID()
		}
	}
	return result, nil
}

func newUUID() string {
	data := make([]byte, 16)
	_, _ = rand.Read(data)
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(data)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:])
}

func queryString(path string, payload map[string]any) string {
	query := url.Values{}
	for key, value := range payload {
		if value == nil || value == "" {
			continue
		}
		switch typed := value.(type) {
		case []string:
			for _, item := range typed {
				query.Add(key, item)
			}
		case []any:
			for _, item := range typed {
				query.Add(key, fmt.Sprint(item))
			}
		case map[string]any:
			encoded, _ := json.Marshal(typed)
			query.Set(key, string(encoded))
		default:
			query.Set(key, fmt.Sprint(value))
		}
	}
	if encoded := query.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

type requestMode int

const (
	requestNone requestMode = iota
	requestQuery
	requestBody
)

func apiOperation(path, summary string, args []string, method string, endpoint func(invocation) string, mode requestMode, mutation bool, options ...optionSpec) *command {
	return apiOperationScoped(path, summary, args, method, endpoint, mode, mutation, true, options...)
}

func apiOperationUnscoped(path, summary string, args []string, method string, endpoint func(invocation) string, mode requestMode, mutation bool, options ...optionSpec) *command {
	return apiOperationScoped(path, summary, args, method, endpoint, mode, mutation, false, options...)
}

func apiOperationScoped(path, summary string, args []string, method string, endpoint func(invocation) string, mode requestMode, mutation, scoped bool, options ...optionSpec) *command {
	cmd := &command{Path: path, Summary: summary, Args: args, Options: options, Input: mode != requestNone, Mutation: mutation}
	cmd.Run = func(a *app, inv invocation) (any, error) {
		payload, err := inv.payload(a)
		if err != nil {
			return nil, err
		}
		ctx, err := a.context()
		if err != nil {
			return nil, err
		}
		requestPath := endpoint(inv)
		var body any
		if mode == requestQuery {
			requestPath = queryString(requestPath, payload)
		} else if mode == requestBody {
			body = payload
		}
		if scoped {
			requestPath = withWorkspace(requestPath, ctx.WorkspaceID)
		}
		return a.client.request(method, requestPath, ctx.Auth.AccessToken, body)
	}
	return cmd
}

func staticEndpoint(path string) func(invocation) string {
	return func(invocation) string { return path }
}

func escaped(value string) string { return url.PathEscape(strings.TrimSpace(value)) }

func resolvePlaybookID(a *app, ctx executionContext, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "pb:") && !strings.HasPrefix(lower, "playbook:") {
		return trimmed, nil
	}
	body := trimmed[strings.Index(trimmed, ":")+1:]
	parts := strings.Split(body, "/")
	payload := map[string]any{}
	if len(parts) == 1 && parts[0] != "" {
		payload["alias"] = parts[0]
	} else if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		payload["handle"], payload["alias"] = parts[0], parts[1]
	} else {
		return "", &Error{Code: "INVALID_INPUT", Message: "Invalid Playbook alias reference: " + trimmed, Hint: "Use pb:<alias> in your own namespace, or pb:<handle>/<alias> for another owner's.", ExitCode: 1}
	}
	response, err := a.client.request(http.MethodGet, withWorkspace(queryString("/v1/aliases/resolve", payload), ctx.WorkspaceID), ctx.Auth.AccessToken, nil)
	if err != nil {
		return "", err
	}
	id := stringField(response, "playbookId")
	if id == "" {
		return "", &Error{Code: "ALIAS_NOT_FOUND", Message: "Alias reference did not resolve to a Playbook: " + trimmed, Hint: "Check the Playbook and its aliases with `epismo playbook alias list <playbook-id>`.", ExitCode: 1}
	}
	return id, nil
}
