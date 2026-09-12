package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestHumanFriendlyOutput(t *testing.T) {
	var output bytes.Buffer
	if err := printResult(&output, map[string]any{"workspaces": []any{map[string]any{"id": "workspace-1", "handle": "acme"}}}, globalOptions{Output: "table"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "handle\tid") || !strings.Contains(output.String(), "acme\tworkspace-1") {
		t.Fatalf("table = %q", output.String())
	}
}

func TestTableDoesNotDiscardFieldsFromArrayBearingObject(t *testing.T) {
	var output bytes.Buffer
	value := map[string]any{"id": "playbook-1", "title": "Onboarding", "acl": []any{map[string]any{"principal": "user-1"}}}
	if err := printResult(&output, value, globalOptions{Output: "table"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"id\tplaybook-1", "title\tOnboarding", "acl\t[{\"principal\":\"user-1\"}]"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output %q does not contain %q", output.String(), expected)
		}
	}
}

func TestOutputFormatsAndProjectionsUseSnakeCase(t *testing.T) {
	value := map[string]any{
		"checkoutUrl": "https://example.com/checkout",
		"items":       []any{map[string]any{"createdAt": "now"}},
	}
	for name, test := range map[string]struct {
		value   any
		options globalOptions
		want    string
	}{
		"table": {value: map[string]any{"items": value["items"]}, options: globalOptions{Output: "table"}, want: "created_at"},
		"yaml":  {value: value, options: globalOptions{Output: "yaml"}, want: "checkout_url"},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := printResult(&output, test.value, test.options); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "checkoutUrl") || strings.Contains(output.String(), "createdAt") || !strings.Contains(output.String(), test.want) {
				t.Fatalf("output = %q", output.String())
			}
		})
	}
	for name, test := range map[string]struct {
		options globalOptions
		want    string
	}{
		"field": {options: globalOptions{Output: "value", Field: "checkout_url"}, want: "https://example.com/checkout"},
		"jq":    {options: globalOptions{Output: "json", JQ: ".items[] | .created_at"}, want: "now"},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := printResult(&output, value, test.options); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), test.want) {
				t.Fatalf("output = %q", output.String())
			}
		})
	}
}

func TestJSONLEmitsOneCompactValuePerLine(t *testing.T) {
	var output bytes.Buffer
	value := map[string]any{
		"playbooks":  []any{map[string]any{"id": "one", "createdAt": "now"}, map[string]any{"id": "two"}},
		"nextCursor": "three",
	}
	if err := printResult(&output, value, globalOptions{Output: "jsonl"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("output = %q", output.String())
	}
	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line is not compact JSON: %q: %v", line, err)
		}
	}

	output.Reset()
	if err := printResult(&output, map[string]any{"title": "Demo", "steps": []any{}}, globalOptions{Output: "jsonl"}); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(output.String()), "\n"); len(lines) != 1 || !strings.Contains(lines[0], `"steps":[]`) {
		t.Fatalf("non-collection output = %q", output.String())
	}
}

func TestYAMLPreservesEmptyCollections(t *testing.T) {
	var output bytes.Buffer
	value := map[string]any{"emptyMap": map[string]any{}, "emptyList": []any{}, "items": []any{map[string]any{}, []any{}}}
	if err := printResult(&output, value, globalOptions{Output: "yaml"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"empty_map": {}`, `"empty_list": []`, "- {}", "- []"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output %q does not contain %q", output.String(), expected)
		}
	}
}

func TestYAMLQuotesMappingKeys(t *testing.T) {
	var output bytes.Buffer
	if err := printResult(&output, map[string]any{"#note": "hidden"}, globalOptions{Output: "yaml"}); err != nil {
		t.Fatal(err)
	}
	if output.String() != `"#note": "hidden"`+"\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestTableAndYAMLPropagateWriteErrors(t *testing.T) {
	want := errors.New("write failed")
	for _, options := range []globalOptions{{Output: "table"}, {Output: "yaml"}} {
		if err := printResult(failingWriter{err: want}, map[string]any{"id": "playbook-1"}, options); !errors.Is(err, want) {
			t.Fatalf("output %q error = %v, want %v", options.Output, err, want)
		}
	}
}
