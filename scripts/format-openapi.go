package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s INPUT OUTPUT\n", os.Args[0])
		os.Exit(2)
	}

	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	input = bytes.TrimSpace(input)
	if !json.Valid(input) {
		fmt.Fprintln(os.Stderr, "downloaded OpenAPI contract is not valid JSON")
		os.Exit(1)
	}

	var output bytes.Buffer
	if err := json.Indent(&output, input, "", "  "); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output.WriteByte('\n')
	if err := os.WriteFile(os.Args[2], output.Bytes(), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Chmod(os.Args[2], 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
