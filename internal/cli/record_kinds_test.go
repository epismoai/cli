package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRetiredReviewRecordKindCannotBeSelectedForWrites(t *testing.T) {
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	t.Setenv("EPISMO_TOKEN", "test-token")
	for _, args := range [][]string{
		{"case", "record", "append", "case-1", "--content", "Assessment"},
		{"case", "record", "update", "record-1", "--content", "Assessment"},
		{"record", "append", "--case-id", "case-1", "--content", "Assessment"},
		{"record", "update", "record-1", "--content", "Assessment"},
	} {
		for _, kind := range []string{"note", "output", "review"} {
			t.Run(strings.Join(args, " ")+"/"+kind, func(t *testing.T) {
				var out, err bytes.Buffer
				command := append(append([]string{}, args...), "--kind", kind, "--dry-run")
				exit := Main(command, "test", strings.NewReader(""), &out, &err)
				if kind == "review" {
					if exit == 0 {
						t.Fatalf("retired kind accepted: %s", out.String())
					}
				} else if exit != 0 {
					t.Fatalf("supported kind rejected: %s", err.String())
				}
			})
		}
	}
}
