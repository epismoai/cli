package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type optionSpec struct {
	Name     string
	Field    string
	Help     string
	Kind     optionKind
	Required bool
	Choices  []string
	Default  any
}

type inputSpec struct {
	Help string
}

type commandSafety struct {
	DryRun         bool
	IdempotencyKey bool
	Confirmation   bool
}

type outputMode uint8

const (
	outputStructured outputMode = iota
	outputRaw
)

type command struct {
	Path     string
	Summary  string
	Examples []string
	Args     []string
	Options  []optionSpec
	Input    *inputSpec
	Safety   commandSafety
	Output   outputMode
	Run      func(*app, invocation) (any, error)
}

type invocation struct {
	Command     *command
	Positionals []string
	Values      map[string]any
	Present     map[string]bool
}

func (i invocation) value(field string) any { return i.Values[field] }
func (i invocation) text(field string) string {
	value, _ := i.Values[field].(string)
	return value
}
func (i invocation) positional(index int) string {
	if index >= len(i.Positionals) {
		return ""
	}
	return i.Positionals[index]
}

func baseOptions(cmd *command) []optionSpec {
	options := append([]optionSpec{}, cmd.Options...)
	if cmd.Input != nil {
		help := cmd.Input.Help
		if help == "" {
			help = "JSON object, @file, or - for stdin"
		}
		options = append([]optionSpec{{Name: "--input", Field: "_input", Help: help, Kind: kindString}}, options...)
	}
	if cmd.Safety.IdempotencyKey {
		options = append(options, optionSpec{Name: "--idempotency-key", Field: "idempotencyKey", Help: "retry key; generated automatically when omitted", Kind: kindString})
	}
	return options
}

func parseInvocation(cmd *command, args []string, stdin io.Reader) (invocation, error) {
	options := baseOptions(cmd)
	byName := map[string]optionSpec{}
	values := map[string]any{}
	for _, option := range options {
		byName[option.Name] = option
		if option.Default != nil {
			values[option.Field] = option.Default
		}
	}
	present := map[string]bool{}
	positionals := []string{}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if !strings.HasPrefix(argument, "--") {
			positionals = append(positionals, argument)
			continue
		}
		name, raw, hasEquals := strings.Cut(argument, "=")
		option, ok := byName[name]
		if !ok {
			return invocation{}, &Error{Code: "UNKNOWN_OPTION", Message: fmt.Sprintf("Unknown option %q.", name), Hint: "Run the command again with --help to see available options.", ExitCode: 1}
		}
		if !hasEquals {
			if option.Kind == kindBoolean {
				values[option.Field] = true
				present[option.Field] = true
				continue
			}
			index++
			if index >= len(args) || strings.HasPrefix(args[index], "--") {
				return invocation{}, required(name)
			}
			raw = args[index]
		}
		value, err := parseValue(raw, option.Name, option.Kind, stdin)
		if err != nil {
			return invocation{}, err
		}
		if len(option.Choices) > 0 && !contains(option.Choices, fmt.Sprint(value)) {
			return invocation{}, &Error{Code: "INVALID_ARGUMENT", Message: fmt.Sprintf("Option %s argument %q is invalid. Allowed choices are %s.", name, raw, strings.Join(option.Choices, ", ")), ExitCode: 1}
		}
		values[option.Field] = value
		present[option.Field] = true
	}
	if len(positionals) < len(cmd.Args) {
		return invocation{}, &Error{Code: "MISSING_ARGUMENT", Message: fmt.Sprintf("Missing required argument %q.", cmd.Args[len(positionals)]), Hint: "Run the command again with --help to see required arguments.", ExitCode: 1}
	}
	if len(positionals) > len(cmd.Args) {
		return invocation{}, &Error{Code: "UNEXPECTED_ARGUMENT", Message: fmt.Sprintf("Too many arguments for %s.", cmd.Path), Hint: "Run the command again with --help to inspect the expected arguments.", ExitCode: 1}
	}
	if cmd.Input == nil {
		for _, option := range options {
			if option.Required && !present[option.Field] {
				return invocation{}, required(option.Name)
			}
		}
	}
	return invocation{Command: cmd, Positionals: positionals, Values: values, Present: present}, nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func printCommandHelp(w io.Writer, cmd *command) {
	fmt.Fprintf(w, "Usage: epismo %s", cmd.Path)
	for _, argument := range cmd.Args {
		fmt.Fprintf(w, " <%s>", argument)
	}
	if len(baseOptions(cmd)) > 0 {
		fmt.Fprint(w, " [options]")
	}
	fmt.Fprintf(w, "\n\n%s\n", cmd.Summary)
	if options := baseOptions(cmd); len(options) > 0 {
		fmt.Fprintln(w, "\nOptions:")
		for _, option := range options {
			required := ""
			if option.Required {
				required = " (required)"
			}
			name := option.Name + " <value>"
			if option.Kind == kindBoolean {
				name = option.Name
			}
			details := option.Help
			if len(option.Choices) > 0 {
				details += " (values: " + strings.Join(option.Choices, ", ") + ")"
			}
			if option.Default != nil {
				details += fmt.Sprintf(" (default: %v)", option.Default)
			}
			fmt.Fprintf(w, "  %-28s %s%s\n", name, details, required)
		}
	}
	if len(cmd.Examples) > 0 {
		fmt.Fprintln(w, "\nExamples:")
		for _, example := range cmd.Examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
	}
	fmt.Fprintln(w, "\nGlobal options: --workspace/-w <id-or-handle>, --output/-o <json|table|yaml|jsonl|value>, --diagnostic-format <json|human>, --field <path>, --jq <projection>, --dry-run, --yes/-y, --schema")
}

func printGroupHelp(w io.Writer, prefix string, commands []*command) {
	if prefix == "" {
		fmt.Fprintln(w, "Epismo — agent-first CLI for discovering, authoring, and coordinating reusable AI Playbooks.")
		fmt.Fprintln(w, "\nUsage: epismo <command> [options]")
	} else {
		fmt.Fprintf(w, "Usage: epismo %s <command> [options]\n", prefix)
	}
	children := map[string]string{}
	groupDescriptions := map[string]string{
		"workspace":        "manage workspaces and the saved default workspace",
		"workspace member": "manage workspace members",
		"team":             "manage workspace teams",
		"team member":      "manage team members",
		"credit":           "view balance and purchase credits",
		"token":            "manage CLI tokens for CI/CD",
		"playbook":         "manage reusable Playbooks",
		"playbook version": "read and publish immutable Versions",
		"playbook draft":   "edit a mutable Draft before publishing",
		"playbook alias":   "manage Playbook aliases in the active namespace",
		"case":             "start, assign, and close Cases",
		"task":             "manage materialized Case Tasks",
		"record":           "append Records and browse the ACL-scoped activity feed",
		"suggestion":       "manage Playbook Suggestions",
	}
	needle := strings.TrimSpace(prefix)
	for _, cmd := range commands {
		parts := strings.Fields(cmd.Path)
		prefixParts := strings.Fields(needle)
		if len(parts) <= len(prefixParts) || !strings.HasPrefix(cmd.Path, needle) {
			continue
		}
		match := true
		for index := range prefixParts {
			if parts[index] != prefixParts[index] {
				match = false
			}
		}
		if match {
			child := parts[len(prefixParts)]
			childPath := strings.TrimSpace(strings.Join(append(prefixParts, child), " "))
			if description, ok := groupDescriptions[childPath]; ok {
				children[child] = description
			} else if _, exists := children[child]; !exists {
				children[child] = cmd.Summary
			}
		}
	}
	if len(children) == 0 {
		return
	}
	names := make([]string, 0, len(children))
	for name := range children {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprintln(w, "\nCommands:")
	for _, name := range names {
		fmt.Fprintf(w, "  %-16s %s\n", name, children[name])
	}
}
