// Package cmdio holds the input/output helpers shared by the lz4c CLI.
package cmdio

import (
	"io"
	"os"
)

// ReadInput reads all of path's bytes; an empty path reads from stdin.
func ReadInput(path string) ([]byte, error) {
	if path == "" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// WriteOutput writes data to path; an empty path writes to stdout.
func WriteOutput(path string, data []byte) error {
	if path == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
