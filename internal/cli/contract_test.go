package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type commandOperation struct {
	method string
	path   string
}

func TestOpenAPIContractMetadata(t *testing.T) {
	contract := readOpenAPIContract(t)
	if version := stringField(contract, "openapi"); version != "3.1.0" {
		t.Fatalf("OpenAPI version = %q, want 3.1.0", version)
	}
	info, _ := contract["info"].(map[string]any)
	if version := stringField(info, "version"); version == "" {
		t.Fatal("OpenAPI info.version is missing")
	}

	paths, _ := contract["paths"].(map[string]any)
	for _, rawPath := range paths {
		path, _ := rawPath.(map[string]any)
		for _, rawOperation := range path {
			operation, _ := rawOperation.(map[string]any)
			responses, _ := operation["responses"].(map[string]any)
			if _, ok := responses["402"]; ok {
				return
			}
		}
	}
	t.Fatal("OpenAPI contract does not document HTTP 402")
}

func TestEveryRemoteCommandUsesDocumentedOperation(t *testing.T) {
	contract := readOpenAPIContract(t)
	operations := map[string]commandOperation{
		"workspace list":             {"get", "/v1/workspaces"},
		"workspace use":              {"get", "/v1/workspaces"},
		"workspace create":           {"post", "/v1/workspaces"},
		"workspace checkout":         {"post", "/v1/workspaces/{workspaceId}/checkout"},
		"workspace update":           {"patch", "/v1/workspaces/{workspaceId}"},
		"workspace member list":      {"get", "/v1/workspaces/{workspaceId}/members"},
		"workspace member upsert":    {"put", "/v1/workspaces/{workspaceId}/members"},
		"workspace member delete":    {"delete", "/v1/workspaces/{workspaceId}/members"},
		"team list":                  {"get", "/v1/teams"},
		"team create":                {"post", "/v1/teams"},
		"team update":                {"patch", "/v1/teams/{teamId}"},
		"team member list":           {"post", "/v1/teams/members/list"},
		"team member add":            {"put", "/v1/teams/{teamId}/members"},
		"team member delete":         {"delete", "/v1/teams/{teamId}/members"},
		"credit balance":             {"get", "/v1/credits"},
		"credit checkout":            {"post", "/v1/credits"},
		"token create":               {"post", "/v1/cli/tokens"},
		"token list":                 {"get", "/v1/cli/tokens"},
		"token revoke":               {"delete", "/v1/cli/tokens/{tokenId}"},
		"playbook search":            {"get", "/v1/playbooks"},
		"playbook list":              {"get", "/v1/playbooks"},
		"playbook resource list":     {"get", "/v1/playbook-resources"},
		"playbook create":            {"post", "/v1/playbooks"},
		"playbook get":               {"get", "/v1/playbooks/{playbookId}"},
		"playbook version list":      {"get", "/v1/playbooks/{playbookId}/versions"},
		"playbook version get":       {"get", "/v1/playbooks/{playbookId}/versions/{versionId}"},
		"playbook version archive":   {"delete", "/v1/playbooks/{playbookId}/versions/{versionId}"},
		"playbook version publish":   {"post", "/v1/playbooks/{playbookId}/versions"},
		"playbook draft get":         {"get", "/v1/playbooks/{playbookId}/draft"},
		"playbook draft save":        {"put", "/v1/playbooks/{playbookId}/draft"},
		"playbook draft discard":     {"delete", "/v1/playbooks/{playbookId}/draft"},
		"playbook draft publish":     {"post", "/v1/playbooks/{playbookId}/draft/publish"},
		"playbook access get":        {"get", "/v1/playbooks/{playbookId}/access"},
		"playbook access set":        {"put", "/v1/playbooks/{playbookId}/access"},
		"playbook archive":           {"delete", "/v1/playbooks/{playbookId}"},
		"playbook star":              {"put", "/v1/playbooks/{playbookId}/star"},
		"playbook unstar":            {"delete", "/v1/playbooks/{playbookId}/star"},
		"playbook starred":           {"get", "/v1/me/starred-playbooks"},
		"playbook share":             {"post", "/v1/playbooks/{playbookId}/share"},
		"playbook alias set":         {"put", "/v1/aliases"},
		"playbook alias list":        {"get", "/v1/aliases"},
		"playbook alias delete":      {"delete", "/v1/aliases/{alias}"},
		"case start":                 {"post", "/v1/cases"},
		"case get":                   {"get", "/v1/cases/{caseId}"},
		"case list":                  {"get", "/v1/cases"},
		"case assign":                {"patch", "/v1/cases/{caseId}/assignee"},
		"case acl":                   {"patch", "/v1/cases/{caseId}/acl"},
		"case update":                {"patch", "/v1/cases/{caseId}"},
		"case handoff":               {"post", "/v1/cases/{caseId}/handoffs"},
		"case handoff graph":         {"get", "/v1/cases/{caseId}/handoff-graph"},
		"case close":                 {"post", "/v1/cases/{caseId}/close"},
		"case reopen":                {"post", "/v1/cases/{caseId}/reopen"},
		"case task create":           {"post", "/v1/cases/{caseId}/tasks"},
		"case task list":             {"get", "/v1/cases/{caseId}/tasks"},
		"case record append":         {"post", "/v1/cases/{caseId}/records"},
		"case record list":           {"get", "/v1/cases/{caseId}/records"},
		"task list":                  {"get", "/v1/tasks"},
		"task get":                   {"get", "/v1/tasks/{taskId}"},
		"task assign":                {"patch", "/v1/tasks/{taskId}/assignee"},
		"task update":                {"patch", "/v1/tasks/{taskId}"},
		"task close":                 {"post", "/v1/tasks/{taskId}/close"},
		"task reopen":                {"post", "/v1/tasks/{taskId}/reopen"},
		"playbook suggestion create": {"post", "/v1/playbooks/{playbookId}/suggestions"},
		"playbook suggestion list":   {"get", "/v1/playbooks/{playbookId}/suggestions"},
		"suggestion get":             {"get", "/v1/suggestions/{suggestionId}"},
		"suggestion list":            {"get", "/v1/suggestions"},
		"suggestion update":          {"patch", "/v1/suggestions/{suggestionId}"},
		"suggestion resolve":         {"post", "/v1/suggestions/{suggestionId}/resolve"},
	}
	localOrAuth := map[string]bool{
		"login": true, "logout": true, "whoami": true, "update": true, "completion": true, "doctor": true, "examples": true, "docs": true, "playbook init": true,
		"workspace current": true, "workspace clear": true,
	}
	for _, cmd := range buildCommands() {
		operation, remote := operations[cmd.Path]
		if !remote {
			if !localOrAuth[cmd.Path] {
				t.Errorf("command %q has no API contract mapping", cmd.Path)
			}
			continue
		}
		if contractOperation(contract, operation.path, operation.method) == nil {
			t.Errorf("command %q uses undocumented operation %s %s", cmd.Path, strings.ToUpper(operation.method), operation.path)
		}
		delete(operations, cmd.Path)
	}
	if len(operations) > 0 {
		missing := make([]string, 0, len(operations))
		for path := range operations {
			missing = append(missing, path)
		}
		sort.Strings(missing)
		t.Errorf("contract mappings have no command: %v", missing)
	}

	for _, operation := range []commandOperation{
		{"post", "/v1/login-options"},
		{"post", "/v1/otp-tokens"},
		{"post", "/oauth/token"},
		{"get", "/oauth/userinfo"},
		{"get", "/v1/workspaces"},
		{"post", "/oauth/revoke"},
		{"get", "/v1/aliases/resolve"},
	} {
		if contractOperation(contract, operation.path, operation.method) == nil {
			t.Errorf("authentication/helper operation is undocumented: %s %s", strings.ToUpper(operation.method), operation.path)
		}
	}
}

func TestQueryCommandOptionsMatchOpenAPI(t *testing.T) {
	contract := readOpenAPIContract(t)
	routes := map[string]string{
		"playbook search":          "/v1/playbooks",
		"playbook list":            "/v1/playbooks",
		"playbook resource list":   "/v1/playbook-resources",
		"playbook version list":    "/v1/playbooks/{playbookId}/versions",
		"playbook starred":         "/v1/me/starred-playbooks",
		"playbook alias list":      "/v1/aliases",
		"case list":                "/v1/cases",
		"case handoff graph":       "/v1/cases/{caseId}/handoff-graph",
		"case task list":           "/v1/cases/{caseId}/tasks",
		"case record list":         "/v1/cases/{caseId}/records",
		"task list":                "/v1/tasks",
		"playbook suggestion list": "/v1/playbooks/{playbookId}/suggestions",
		"suggestion list":          "/v1/suggestions",
		"token list":               "/v1/cli/tokens",
	}
	commands := map[string]*command{}
	for _, cmd := range buildCommands() {
		commands[cmd.Path] = cmd
	}
	for commandPath, route := range routes {
		t.Run(commandPath, func(t *testing.T) {
			operation := contractOperation(contract, route, "get")
			if operation == nil {
				t.Fatalf("GET %s is missing", route)
			}
			documented := map[string]bool{}
			if parameters, ok := operation["parameters"].([]any); ok {
				for _, raw := range parameters {
					parameter, _ := raw.(map[string]any)
					if parameter["in"] == "query" {
						documented[stringField(parameter, "name")] = true
					}
				}
			}
			for _, option := range commands[commandPath].Options {
				if strings.HasPrefix(option.Field, "_") {
					continue
				}
				if !documented[option.Field] {
					t.Errorf("option %s (%s) is absent from GET %s query parameters", option.Name, option.Field, route)
				}
			}
		})
	}
}

func readOpenAPIContract(t *testing.T) map[string]any {
	t.Helper()
	path := os.Getenv("EPISMO_OPENAPI_CONTRACT")
	if path == "" {
		path = filepath.Join("..", "..", "contracts", "openapi.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]any
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	return contract
}

func contractOperation(contract map[string]any, path, method string) map[string]any {
	paths, _ := contract["paths"].(map[string]any)
	pathItem, _ := paths[path].(map[string]any)
	operation, _ := pathItem[method].(map[string]any)
	return operation
}
