package cli

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// cliJSON converts API/internal camelCase JSON keys to the snake_case public
// representation exposed by the CLI. Values are left unchanged.
func cliJSON(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			publicKey := snakeCase(key)
			if _, exists := result[publicKey]; exists {
				return nil, &Error{
					Code:     "AMBIGUOUS_OUTPUT",
					Message:  fmt.Sprintf("Response contains multiple fields that normalize to %q.", publicKey),
					Details:  map[string]any{"field": publicKey},
					ExitCode: 1,
				}
			}
			item, err := cliJSON(typed[key])
			if err != nil {
				return nil, err
			}
			result[publicKey] = item
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			converted, err := cliJSON(item)
			if err != nil {
				return nil, err
			}
			result[index] = converted
		}
		return result, nil
	case []map[string]any:
		result := make([]any, len(typed))
		for index, item := range typed {
			converted, err := cliJSON(item)
			if err != nil {
				return nil, err
			}
			result[index] = converted
		}
		return result, nil
	default:
		return value, nil
	}
}

// apiJSON converts snake_case keys accepted at the CLI boundary to the
// camelCase wire representation used by the Epismo API. Existing camelCase
// input remains valid. Ambiguous objects are rejected rather than resolving a
// collision according to Go's randomized map iteration order.
func apiJSON(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			wireKey := lowerCamelCase(key)
			if _, exists := result[wireKey]; exists {
				return nil, &Error{
					Code:     "AMBIGUOUS_INPUT",
					Message:  fmt.Sprintf("Input contains multiple fields that normalize to %q.", wireKey),
					Hint:     "Use snake_case field names consistently.",
					Details:  map[string]any{"field": wireKey},
					ExitCode: 1,
				}
			}
			item, err := apiJSON(typed[key])
			if err != nil {
				return nil, err
			}
			result[wireKey] = item
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			converted, err := apiJSON(item)
			if err != nil {
				return nil, err
			}
			result[index] = converted
		}
		return result, nil
	default:
		return value, nil
	}
}

func snakeCase(value string) string {
	runes := []rune(value)
	var result strings.Builder
	lastWasUnderscore := false
	for index, current := range runes {
		if current == '-' || unicode.IsSpace(current) {
			if result.Len() > 0 && !lastWasUnderscore {
				result.WriteRune('_')
				lastWasUnderscore = true
			}
			continue
		}
		if unicode.IsUpper(current) {
			previousIsLowerOrDigit := index > 0 && (unicode.IsLower(runes[index-1]) || unicode.IsDigit(runes[index-1]))
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			previousIsUpper := index > 0 && unicode.IsUpper(runes[index-1])
			if result.Len() > 0 && (previousIsLowerOrDigit || previousIsUpper && nextIsLower) {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(current))
			lastWasUnderscore = false
			continue
		}
		result.WriteRune(current)
		lastWasUnderscore = current == '_'
	}
	return result.String()
}

func lowerCamelCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' || unicode.IsSpace(r) })
	if len(parts) <= 1 {
		return value
	}
	var result strings.Builder
	result.WriteString(strings.ToLower(parts[0]))
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		runes := []rune(strings.ToLower(part))
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	return result.String()
}
