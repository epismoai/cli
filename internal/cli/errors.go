package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Error struct {
	Code      string         `json:"-"`
	Message   string         `json:"-"`
	Retryable bool           `json:"-"`
	Hint      string         `json:"-"`
	Details   map[string]any `json:"-"`
	ExitCode  int            `json:"-"`
	Cause     error          `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func cliError(code, message string) *Error {
	return &Error{Code: code, Message: message, ExitCode: 1}
}

func normalizeError(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		if target.ExitCode == 0 {
			target.ExitCode = 1
		}
		return target
	}
	return &Error{Code: "UNEXPECTED_ERROR", Message: err.Error(), ExitCode: 1, Cause: err}
}

func printJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func printError(w io.Writer, err error) int {
	normalized := normalizeError(err)
	payload := map[string]any{
		"code":      normalized.Code,
		"message":   normalized.Message,
		"retryable": normalized.Retryable,
	}
	if normalized.Hint != "" {
		payload["hint"] = normalized.Hint
	}
	if len(normalized.Details) > 0 {
		payload["details"] = normalized.Details
	}
	_ = printJSON(w, map[string]any{"error": payload})
	return normalized.ExitCode
}

func printWarning(w io.Writer, code, message string) {
	_ = printJSON(w, map[string]any{"warning": map[string]any{"code": code, "message": message}})
}

func required(name string) *Error {
	return &Error{
		Code:     "MISSING_OPTION_VALUE",
		Message:  fmt.Sprintf("required option '%s <value>' not specified", name),
		Hint:     "Run the command again with --help to see option usage.",
		ExitCode: 1,
	}
}
