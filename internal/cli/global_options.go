package cli

import (
	"strconv"
	"strings"
)

// globalOptions stay independent from API request options so they can appear
// before or after a command without altering the API contract.
type globalOptions struct {
	Workspace, Output, DiagnosticFormat, Field, JQ string
	Quiet, Yes, DryRun, Schema                     bool
}

func boolOption(name, field, help string) optionSpec {
	return optionSpec{Name: name, Field: field, Help: help, Kind: kindBoolean}
}

func extractGlobalOptions(args []string) ([]string, globalOptions, error) {
	o := globalOptions{Output: "json", DiagnosticFormat: "json"}
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, equals := strings.Cut(arg, "=")
		switch name {
		case "--quiet":
			parsed, err := globalBoolValue(name, value, equals)
			if err != nil {
				return nil, o, err
			}
			o.Quiet = parsed
			continue
		case "--yes", "-y":
			parsed, err := globalBoolValue(name, value, equals)
			if err != nil {
				return nil, o, err
			}
			o.Yes = parsed
			continue
		case "--dry-run":
			parsed, err := globalBoolValue(name, value, equals)
			if err != nil {
				return nil, o, err
			}
			o.DryRun = parsed
			continue
		case "--schema":
			parsed, err := globalBoolValue(name, value, equals)
			if err != nil {
				return nil, o, err
			}
			o.Schema = parsed
			continue
		}
		var destination *string
		switch name {
		case "--workspace", "-w":
			destination = &o.Workspace
		case "--output", "-o":
			destination = &o.Output
		case "--diagnostic-format":
			destination = &o.DiagnosticFormat
		case "--field":
			destination = &o.Field
		case "--jq":
			destination = &o.JQ
		}
		if destination == nil {
			remaining = append(remaining, arg)
			continue
		}
		if !equals {
			i++
			if i >= len(args) {
				return nil, o, required(name)
			}
			value = args[i]
		}
		*destination = strings.TrimSpace(value)
	}
	if !contains([]string{"json", "table", "yaml", "jsonl", "value"}, o.Output) {
		return nil, o, &Error{Code: "INVALID_ARGUMENT", Message: "Invalid --output value. Allowed values are json, table, yaml, jsonl, value.", ExitCode: 1}
	}
	if !contains([]string{"json", "human"}, o.DiagnosticFormat) {
		return nil, o, &Error{Code: "INVALID_ARGUMENT", Message: "Invalid --diagnostic-format value. Allowed values are json, human.", ExitCode: 1}
	}
	if o.Field != "" && o.Output != "value" {
		return nil, o, &Error{Code: "INVALID_ARGUMENT", Message: "--field requires --output value.", ExitCode: 1}
	}
	return remaining, o, nil
}

// Global boolean flags follow the conventional Go CLI syntax: a bare flag is
// true, while --flag=true and --flag=false set an explicit value.
func globalBoolValue(name, value string, hasValue bool) (bool, error) {
	if !hasValue {
		return true, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, &Error{Code: "INVALID_ARGUMENT", Message: "Invalid " + name + ": boolean expected.", ExitCode: 1}
	}
	return parsed, nil
}
