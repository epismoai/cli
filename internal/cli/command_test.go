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
