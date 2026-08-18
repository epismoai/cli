package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type optionKind int

const (
	kindString optionKind = iota
	kindInteger
	kindObject
	kindObjectSource
	kindArray
	kindCSV
)

func parseJSON(data []byte, label string) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, &Error{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid JSON for %s: %s", label, err), Hint: "Pass a valid JSON value, @path/to/file.json, or - to read from stdin.", ExitCode: 1}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values are not allowed")
		}
		return nil, &Error{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid JSON for %s: %s", label, err), Hint: "Pass exactly one valid JSON value, @path/to/file.json, or - to read from stdin.", ExitCode: 1}
	}
	return value, nil
}

func readObjectSource(value, label string, stdin io.Reader) (map[string]any, error) {
	trimmed := strings.TrimSpace(value)
	var data []byte
	var err error
	switch {
	case trimmed == "-":
		data, err = io.ReadAll(stdin)
	case strings.HasPrefix(trimmed, "@"):
		path := strings.TrimPrefix(trimmed, "@")
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, &Error{Code: "INPUT_FILE_READ_FAILED", Message: fmt.Sprintf("Failed to read input file %q: %s", path, err), Hint: fmt.Sprintf("Check that the file exists and is readable, or pass %s - to read from stdin.", label), Details: map[string]any{"path": path}, ExitCode: 1}
		}
	default:
		data = []byte(trimmed)
	}
	if err != nil {
		return nil, err
	}
	parsed, err := parseJSON(data, label)
	if err != nil {
		return nil, err
	}
	object, ok := parsed.(map[string]any)
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: fmt.Sprintf("Invalid JSON for %s: object expected.", label), Hint: `Pass a JSON object like '{"key":"value"}'.`, ExitCode: 1}
	}
	return object, nil
}

func parseValue(raw, label string, kind optionKind, stdin io.Reader) (any, error) {
	switch kind {
	case kindString:
		return raw, nil
	case kindInteger:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, &Error{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("Invalid %s: number expected.", label), ExitCode: 1}
		}
		return value, nil
	case kindObjectSource:
		return readObjectSource(raw, label, stdin)
	case kindObject:
		if strings.TrimSpace(raw) == "" {
			return map[string]any{}, nil
		}
		value, err := parseJSON([]byte(raw), label)
		if err != nil {
			return nil, err
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, &Error{Code: "INVALID_INPUT", Message: fmt.Sprintf("Invalid %s: JSON object expected.", label), ExitCode: 1}
		}
		return object, nil
	case kindArray:
		if strings.TrimSpace(raw) == "" {
			return []any{}, nil
		}
		value, err := parseJSON([]byte(raw), label)
		if err != nil {
			return nil, err
		}
		array, ok := value.([]any)
		if !ok {
			return nil, &Error{Code: "INVALID_INPUT", Message: fmt.Sprintf("Invalid %s: JSON array expected.", label), ExitCode: 1}
		}
		return array, nil
	case kindCSV:
		return stringArray(raw, label)
	default:
		return raw, nil
	}
}

func stringArray(value any, label string) ([]string, error) {
	var parts []string
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "[") {
			parsed, err := parseJSON([]byte(trimmed), label)
			if err != nil {
				return nil, err
			}
			array, ok := parsed.([]any)
			if !ok {
				return nil, &Error{Code: "INVALID_INPUT", Message: fmt.Sprintf("Invalid %s: string array expected.", label), ExitCode: 1}
			}
			for _, item := range array {
				parts = append(parts, strings.Split(fmt.Sprint(item), ",")...)
			}
		} else {
			parts = strings.Split(trimmed, ",")
		}
	case []any:
		for _, item := range typed {
			parts = append(parts, strings.Split(fmt.Sprint(item), ",")...)
		}
	case []string:
		parts = typed
	default:
		return nil, &Error{Code: "INVALID_INPUT", Message: fmt.Sprintf("Invalid %s: string array expected.", label), ExitCode: 1}
	}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}

func exactInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil && int64(int(parsed)) == parsed
	case float64:
		parsed := int(typed)
		return parsed, float64(parsed) == typed
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func mergeDefined(base map[string]any, overrides map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(overrides))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overrides {
		if value != nil {
			result[key] = value
		}
	}
	return result
}
