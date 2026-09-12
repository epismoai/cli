package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandHelpIncludesChoicesAndExamples(t *testing.T) {
	command := playbookCommands()[0]
	command.Examples = []string{"epismo playbook search --query onboarding"}
	var output bytes.Buffer
	printCommandHelp(&output, command)
	if !strings.Contains(output.String(), "values: productivity") || !strings.Contains(output.String(), "Examples:") {
		t.Fatalf("help = %q", output.String())
	}
}

func TestInputAndIdempotencyKeyAreIndependent(t *testing.T) {
	inputOnly := &command{Input: &inputSpec{}}
	idempotencyOnly := &command{Safety: commandSafety{IdempotencyKey: true}}

	if optionsHaveField(baseOptions(inputOnly), "idempotencyKey") || !optionsHaveField(baseOptions(inputOnly), "_input") {
		t.Fatalf("input-only options = %#v", baseOptions(inputOnly))
	}
	if !optionsHaveField(baseOptions(idempotencyOnly), "idempotencyKey") || optionsHaveField(baseOptions(idempotencyOnly), "_input") {
		t.Fatalf("idempotency-only options = %#v", baseOptions(idempotencyOnly))
	}
}

func optionsHaveField(options []optionSpec, field string) bool {
	for _, option := range options {
		if option.Field == field {
			return true
		}
	}
	return false
}
