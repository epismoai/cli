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
	converted, err := cliJSON(value)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(converted)
}

func printDiagnosticJSON(w io.Writer, value any) error {
	converted, err := cliJSON(value)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(converted)
}

func errorPayload(normalized *Error) map[string]any {
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
	return payload
}

func printError(w io.Writer, err error) int {
	normalized := normalizeError(err)
	_ = printDiagnosticJSON(w, map[string]any{"error": errorPayload(normalized)})
	return normalized.ExitCode
}

func printAppError(a *app, err error) int {
	if a.options.DiagnosticFormat != "human" {
		return printError(a.stderr, err)
	}
	normalized := normalizeError(err)
	_, _ = fmt.Fprintf(a.stderr, "error: %s\n", normalized.Message)
	if normalized.Hint != "" {
		_, _ = fmt.Fprintf(a.stderr, "hint: %s\n", normalized.Hint)
	}
	return normalized.ExitCode
}

func printWarning(w io.Writer, code, message string) {
	printEvent(w, "warning", code, message, nil)
}

func printEvent(w io.Writer, level, code, message string, details map[string]any) {
	payload := map[string]any{"level": level, "code": code, "message": message}
	if len(details) > 0 {
		payload["details"] = details
	}
	_ = printDiagnosticJSON(w, map[string]any{"event": payload})
}

func required(name string) *Error {
	return &Error{
		Code:     "MISSING_OPTION_VALUE",
		Message:  fmt.Sprintf("Required option %q was not specified.", name+" <value>"),
		Hint:     "Run the command again with --help to see option usage.",
		ExitCode: 1,
	}
}
