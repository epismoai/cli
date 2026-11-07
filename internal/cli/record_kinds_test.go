package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRecordWriteKinds(t *testing.T) {
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	t.Setenv("EPISMO_TOKEN", "test-token")
	for _, args := range [][]string{
		{"case", "record", "append", "case-1", "--content", "Assessment"},
		{"case", "record", "update", "record-1", "--content", "Assessment"},
		{"record", "append", "--case-id", "case-1", "--content", "Assessment"},
		{"record", "update", "record-1", "--content", "Assessment"},
	} {
		for _, kind := range []string{"note", "output", "activity", "agreement", "invalid"} {
			t.Run(strings.Join(args, " ")+"/"+kind, func(t *testing.T) {
				var out, err bytes.Buffer
				command := append(append([]string{}, args...), "--kind", kind, "--dry-run")
				exit := Main(command, "test", strings.NewReader(""), &out, &err)
				if kind == "activity" || kind == "invalid" {
					if exit == 0 {
						t.Fatalf("unsupported kind accepted: %s", out.String())
					}
				} else {
					if exit != 0 {
						t.Fatalf("supported kind rejected: %s", err.String())
					}
					var preview struct {
						Request struct {
							Kind string `json:"kind"`
						} `json:"request"`
					}
					if json.Unmarshal(out.Bytes(), &preview) != nil || preview.Request.Kind != kind {
						t.Fatalf("kind missing from request: %s", out.String())
					}
				}
			})
		}
	}
}
