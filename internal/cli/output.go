package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

func printResult(w io.Writer, value any, o globalOptions) error {
	converted, err := cliJSON(value)
	if err != nil {
		return err
	}
	value = converted
	if o.JQ != "" {
		selected, err := selectJQ(value, o.JQ)
		if err != nil {
			return err
		}
		return printJSON(w, selected)
	}
	if o.Output == "value" {
		selected, err := selectField(value, o.Field)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, scalarString(selected))
		return err
	}
	switch o.Output {
	case "json":
		return printJSON(w, value)
	case "jsonl":
		return printJSONL(w, value)
	case "table":
		return printTable(w, value)
	case "yaml":
		return printYAML(w, value, 0)
	}
	return printJSON(w, value)
}

func selectField(value any, field string) (any, error) {
	if field == "" {
		return value, nil
	}
	current := value
	for _, part := range strings.Split(strings.TrimPrefix(field, "."), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, &Error{Code: "FIELD_NOT_FOUND", Message: "Field not found: " + field + ".", ExitCode: 1}
		}
		var found bool
		current, found = object[part]
		if !found {
			return nil, &Error{Code: "FIELD_NOT_FOUND", Message: "Field not found: " + field + ".", ExitCode: 1}
		}
	}
	return current, nil
}

// selectJQ supports the common projection subset without a third-party
// runtime: .field, .field.nested, .items[], and .items[] | .id.
func selectJQ(value any, expression string) (any, error) {
	current := value
	for _, raw := range strings.Split(expression, "|") {
		path := strings.TrimSpace(raw)
		if !strings.HasPrefix(path, ".") {
			return nil, &Error{Code: "UNSUPPORTED_JQ", Message: "--jq supports field projections beginning with '.'.", Hint: "Use .checkout_url or .playbooks[] | .id.", ExitCode: 1}
		}
		array := strings.HasSuffix(path, "[]")
		path = strings.TrimSuffix(strings.TrimPrefix(path, "."), "[]")
		if items, ok := current.([]any); ok && !array {
			values := make([]any, 0, len(items))
			for _, item := range items {
				selected, err := selectField(item, path)
				if err != nil {
					return nil, err
				}
				values = append(values, selected)
			}
			current = values
			continue
		}
		selected, err := selectField(current, path)
		if err != nil {
			return nil, err
		}
		if array {
			if _, ok := selected.([]any); !ok {
				return nil, &Error{Code: "UNSUPPORTED_JQ", Message: "The [] operator requires an array value.", ExitCode: 1}
			}
		}
		current = selected
	}
	return current, nil
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	}
}
func printJSONL(w io.Writer, value any) error {
	if object, ok := value.(map[string]any); ok {
		for key, raw := range object {
			if items, ok := raw.([]any); ok && jsonLCollection(object, key) {
				for _, item := range items {
					if err := printCompactJSON(w, item); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}
	if items, ok := value.([]any); ok {
		for _, item := range items {
			if err := printCompactJSON(w, item); err != nil {
				return err
			}
		}
		return nil
	}
	return printCompactJSON(w, value)
}

func jsonLCollection(object map[string]any, arrayKey string) bool {
	for key := range object {
		if key != arrayKey && key != "next_cursor" {
			return false
		}
	}
	return true
}

func printCompactJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
func printTable(w io.Writer, value any) error {
	if object, ok := value.(map[string]any); ok {
		for key, raw := range object {
			if items, ok := raw.([]any); ok && jsonLCollection(object, key) {
				return printTableRows(w, items)
			}
		}
		for _, key := range sortedKeys(object) {
			if _, err := fmt.Fprintf(w, "%s\t%s\n", key, scalarString(object[key])); err != nil {
				return err
			}
		}
		return nil
	}
	if items, ok := value.([]any); ok {
		return printTableRows(w, items)
	}
	_, err := fmt.Fprintln(w, scalarString(value))
	return err
}
func printTableRows(w io.Writer, items []any) error {
	columns := map[string]bool{}
	for _, raw := range items {
		if object, ok := raw.(map[string]any); ok {
			for key, value := range object {
				if _, nested := value.(map[string]any); !nested {
					columns[key] = true
				}
			}
		}
	}
	keys := make([]string, 0, len(columns))
	for key := range columns {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		for _, item := range items {
			if _, err := fmt.Fprintln(w, scalarString(item)); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(w, strings.Join(keys, "\t")); err != nil {
		return err
	}
	for _, raw := range items {
		object, _ := raw.(map[string]any)
		row := make([]string, len(keys))
		for i, key := range keys {
			row[i] = scalarString(object[key])
		}
		if _, err := fmt.Fprintln(w, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}
func printYAML(w io.Writer, value any, indent int) error {
	prefix := strings.Repeat("  ", indent)
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			_, err := fmt.Fprintln(w, prefix+"{}")
			return err
		}
		for _, key := range sortedKeys(typed) {
			switch child := typed[key].(type) {
			case map[string]any:
				if len(child) == 0 {
					if _, err := fmt.Fprintf(w, "%s%s: {}\n", prefix, yamlKey(key)); err != nil {
						return err
					}
					continue
				}
				if _, err := fmt.Fprintf(w, "%s%s:\n", prefix, yamlKey(key)); err != nil {
					return err
				}
				if err := printYAML(w, child, indent+1); err != nil {
					return err
				}
			case []any:
				if len(child) == 0 {
					if _, err := fmt.Fprintf(w, "%s%s: []\n", prefix, yamlKey(key)); err != nil {
						return err
					}
					continue
				}
				if _, err := fmt.Fprintf(w, "%s%s:\n", prefix, yamlKey(key)); err != nil {
					return err
				}
				if err := printYAML(w, child, indent+1); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%s%s: %s\n", prefix, yamlKey(key), yamlScalar(typed[key])); err != nil {
					return err
				}
			}
		}
	case []any:
		if len(typed) == 0 {
			_, err := fmt.Fprintln(w, prefix+"[]")
			return err
		}
		for _, item := range typed {
			switch child := item.(type) {
			case map[string]any:
				if len(child) == 0 {
					if _, err := fmt.Fprintf(w, "%s- {}\n", prefix); err != nil {
						return err
					}
					continue
				}
				if _, err := fmt.Fprintf(w, "%s-\n", prefix); err != nil {
					return err
				}
				if err := printYAML(w, child, indent+1); err != nil {
					return err
				}
			case []any:
				if len(child) == 0 {
					if _, err := fmt.Fprintf(w, "%s- []\n", prefix); err != nil {
						return err
					}
					continue
				}
				if _, err := fmt.Fprintf(w, "%s-\n", prefix); err != nil {
					return err
				}
				if err := printYAML(w, child, indent+1); err != nil {
					return err
				}
			default:
				if _, err := fmt.Fprintf(w, "%s- %s\n", prefix, yamlScalar(item)); err != nil {
					return err
				}
			}
		}
	default:
		_, err := fmt.Fprintln(w, prefix+scalarString(value))
		return err
	}
	return nil
}
func yamlScalar(value any) string {
	if text, ok := value.(string); ok {
		encoded, _ := json.Marshal(text)
		return string(encoded)
	}
	return scalarString(value)
}
func yamlKey(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
