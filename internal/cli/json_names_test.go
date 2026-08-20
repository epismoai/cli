package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPrintJSONUsesSnakeCaseRecursively(t *testing.T) {
	input := map[string]any{
		"currentWorkspace": map[string]any{
			"accountId":   "account-1",
			"checkoutURL": "https://example.com",
		},
		"items": []any{map[string]any{"createdAt": "now"}},
	}
	var output bytes.Buffer
	if err := printJSON(&output, input); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"current_workspace": map[string]any{
			"account_id":   "account-1",
			"checkout_url": "https://example.com",
		},
		"items": []any{map[string]any{"created_at": "now"}},
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded = %#v, want %#v", decoded, want)
	}
}

func TestAPIJSONAcceptsSnakeCaseAndExistingCamelCase(t *testing.T) {
	for name, input := range map[string]map[string]any{
		"snake case": {"expected_lock_version": json.Number("3"), "definition": map[string]any{"schema_version": json.Number("1")}},
		"camel case": {"expectedLockVersion": json.Number("3"), "definition": map[string]any{"schemaVersion": json.Number("1")}},
	} {
		t.Run(name, func(t *testing.T) {
			converted, err := apiJSON(input)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"expectedLockVersion": json.Number("3"), "definition": map[string]any{"schemaVersion": json.Number("1")}}
			if !reflect.DeepEqual(converted, want) {
				t.Fatalf("converted = %#v, want %#v", converted, want)
			}
		})
	}
}

func TestAPIJSONRejectsNamingCollisions(t *testing.T) {
	_, err := apiJSON(map[string]any{"account_id": "one", "accountId": "two"})
	if errorCode(err) != "AMBIGUOUS_INPUT" || !strings.Contains(err.Error(), "accountId") {
		t.Fatalf("error = %v", err)
	}
}

func TestCLIJSONRejectsNamingCollisions(t *testing.T) {
	_, err := cliJSON(map[string]any{"account_id": "one", "accountId": "two"})
	if errorCode(err) != "AMBIGUOUS_OUTPUT" || !strings.Contains(err.Error(), "account_id") {
		t.Fatalf("error = %v", err)
	}
}
