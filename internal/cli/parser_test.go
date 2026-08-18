package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseInvocationSupportsEveryInputKind(t *testing.T) {
	cmd := &command{
		Path: "test parse",
		Args: []string{"id"},
		Options: []optionSpec{
			str("--text", "text", ""),
			integer("--count", "count", ""),
			object("--object", "object", ""),
			objectSource("--source", "source", ""),
			array("--array", "array", ""),
			csv("--csv", "csv", ""),
			choice("--choice", "choice", "", "a", "b"),
		},
	}
	invocation, err := parseInvocation(cmd, []string{
		"resource-1",
		"--text=value",
		"--count", "42",
		"--object", `{"enabled":true}`,
		"--source", "-",
		"--array", `[1,"two"]`,
		"--csv", `one, two`,
		"--choice", "b",
	}, strings.NewReader(`{"from":"stdin"}`))
	if err != nil {
		t.Fatal(err)
	}
	if invocation.positional(0) != "resource-1" || invocation.text("text") != "value" || invocation.value("count") != 42 || invocation.text("choice") != "b" {
		t.Fatalf("invocation = %#v", invocation)
	}
	if !reflect.DeepEqual(invocation.value("object"), map[string]any{"enabled": true}) || !reflect.DeepEqual(invocation.value("source"), map[string]any{"from": "stdin"}) || !reflect.DeepEqual(invocation.value("array"), []any{json.Number("1"), "two"}) || !reflect.DeepEqual(invocation.value("csv"), []string{"one", "two"}) {
		t.Fatalf("values = %#v", invocation.Values)
	}
}

func TestParseInvocationErrors(t *testing.T) {
	cmd := &command{Path: "test", Args: []string{"id"}, Options: []optionSpec{integer("--count", "count", ""), choice("--choice", "choice", "", "a", "b")}}
	tests := []struct {
		name string
		args []string
		code string
	}{
		{name: "unknown option", args: []string{"id", "--unknown", "x"}, code: "UNKNOWN_OPTION"},
		{name: "missing value", args: []string{"id", "--count"}, code: "MISSING_OPTION_VALUE"},
		{name: "option followed by option", args: []string{"id", "--count", "--choice", "a"}, code: "MISSING_OPTION_VALUE"},
		{name: "invalid integer", args: []string{"id", "--count", "many"}, code: "INVALID_ARGUMENT"},
		{name: "invalid choice", args: []string{"id", "--choice", "c"}, code: "INVALID_ARGUMENT"},
		{name: "missing positional", args: nil, code: "MISSING_ARGUMENT"},
		{name: "extra positional", args: []string{"id", "extra"}, code: "COMMAND_ERROR"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseInvocation(cmd, test.args, strings.NewReader(""))
			if got := errorCode(err); got != test.code {
				t.Fatalf("error = %v, code = %q, want %q", err, got, test.code)
			}
		})
	}
}

func TestDoubleDashStopsOptionParsing(t *testing.T) {
	cmd := &command{Path: "test", Args: []string{"value"}, Options: []optionSpec{str("--title", "title", "")}}
	invocation, err := parseInvocation(cmd, []string{"--", "--title"}, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if invocation.positional(0) != "--title" || invocation.Present["title"] {
		t.Fatalf("invocation = %#v", invocation)
	}
}

func TestMainHelpVersionAndCommandErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantOutput string
		stderr     bool
	}{
		{name: "root help", args: []string{"--help"}, wantOutput: "Usage: epismo <command>"},
		{name: "group help", args: []string{"playbook", "--help"}, wantOutput: "Usage: epismo playbook <command>"},
		{name: "command help", args: []string{"case", "close", "--help"}, wantOutput: "Usage: epismo case close"},
		{name: "version", args: []string{"--version"}, wantOutput: "9.9.9"},
		{name: "unknown command", args: []string{"does-not-exist"}, wantExit: 1, wantOutput: "UNKNOWN_COMMAND", stderr: true},
		{name: "unknown option", args: []string{"case", "get", "case-1", "--bogus", "x"}, wantExit: 1, wantOutput: "UNKNOWN_OPTION", stderr: true},
		{name: "missing argument", args: []string{"case", "get"}, wantExit: 1, wantOutput: "MISSING_ARGUMENT", stderr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := Main(test.args, "9.9.9", strings.NewReader(""), &stdout, &stderr)
			if exit != test.wantExit {
				t.Fatalf("exit = %d, want %d", exit, test.wantExit)
			}
			output := stdout.String()
			if test.stderr {
				output = stderr.String()
			}
			if !strings.Contains(output, test.wantOutput) {
				t.Fatalf("output = %q, want substring %q", output, test.wantOutput)
			}
		})
	}
}

func TestHelpFlagIsNotMistakenForOptionValue(t *testing.T) {
	cmd := caseCommands()[0]
	if !containsHelpFlag(cmd, []string{"--title", "value", "-h"}) {
		t.Fatal("standalone -h was not recognized")
	}
	if containsHelpFlag(cmd, []string{"--title", "-h"}) {
		t.Fatal("-h used as an option value was mistaken for help")
	}
}
