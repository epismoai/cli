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

func TestTopLevelHelpListsCaseBeforePlaybook(t *testing.T) {
	words := buildCommandWords()
	caseIdx, playbookIdx := -1, -1
	for index, word := range words {
		if word == "case" {
			caseIdx = index
		}
		if word == "playbook" {
			playbookIdx = index
		}
	}
	if caseIdx < 0 || playbookIdx < 0 || caseIdx > playbookIdx {
		t.Fatalf("buildCommandWords case=%d playbook=%d in %v", caseIdx, playbookIdx, words)
	}

	var output bytes.Buffer
	printGroupHelp(&output, "", buildCommands())
	help := output.String()
	casePos := strings.Index(help, "\n  case ")
	playbookPos := strings.Index(help, "\n  playbook ")
	if casePos < 0 || playbookPos < 0 || casePos > playbookPos {
		t.Fatalf("help order case=%d playbook=%d\n%s", casePos, playbookPos, help)
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
