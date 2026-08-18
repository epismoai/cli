package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/epismoai/cli/internal/cli"
)

// version is replaced by release builds through -ldflags. For binaries built
// with `go install module@version`, resolvedVersion reads the module version
// embedded by the Go toolchain instead.
var version = "dev"

func main() {
	os.Exit(cli.MainWithDistribution(os.Args[1:], resolvedVersion(), resolvedDistribution(), os.Stdin, os.Stdout, os.Stderr))
}

func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return version
}

func resolvedDistribution() string {
	if os.Getenv("EPISMO_DISTRIBUTION") == "node" {
		return "node"
	}
	if version != "dev" {
		return "release"
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return "go"
	}
	return "development"
}
