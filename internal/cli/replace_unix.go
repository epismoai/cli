//go:build !windows

package cli

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
