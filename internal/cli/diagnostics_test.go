package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticsAreOneJSONObjectPerLine(t *testing.T) {
	var output bytes.Buffer
	printEvent(&output, "info", "BROWSER_WAITING", "Waiting.", map[string]any{"timeoutSeconds": 300})
	printWarning(&output, "UPDATE_AVAILABLE", "Update available.")
	if exit := printError(&output, &Error{Code: "NOT_FOUND", Message: "Not found.", ExitCode: 1}); exit != 1 {
		t.Fatalf("exit = %d", exit)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("diagnostics = %q", output.String())
	}
	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line is not JSON: %q: %v", line, err)
		}
	}
	if strings.Contains(lines[0], `"warning"`) || !strings.Contains(lines[0], `"level":"info"`) || !strings.Contains(lines[0], `"timeout_seconds":300`) {
		t.Fatalf("info event = %s", lines[0])
	}
	if !strings.Contains(lines[1], `"level":"warning"`) || !strings.Contains(lines[2], `"error"`) {
		t.Fatalf("diagnostics = %q", output.String())
	}
}
