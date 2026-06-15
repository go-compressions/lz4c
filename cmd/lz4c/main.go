// lz4c is a small CLI wrapper around the pure-Go LZ4 block codec in
// github.com/go-compressions/lz4.
//
// By default lz4c compresses; pass -d/--decompress to go the other way.
// Input defaults to stdin and output to stdout, so lz4c composes in pipes:
//
//	cat big.bin | lz4c | lz4c -d > restored.bin
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-compressions/lz4c/cmd/lz4c/internal/cmdio"
	"github.com/go-compressions/lz4c/cmd/lz4c/lz4io"
	"github.com/spf13/cobra"
)

// osExit allows tests to override os.Exit.
var osExit = os.Exit

func main() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}

// RootCmd returns the top-level cobra command. Exported so tests in the same
// package can exercise the wiring without spawning a child process.
func RootCmd() *cobra.Command {
	var (
		decompress bool
		inputPath  string
		outputPath string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:   "lz4c",
		Short: "Compress and decompress data using the LZ4 block format",
		Long: `lz4c compresses and decompresses data using the LZ4 block format
with a pure-Go implementation (github.com/go-compressions/lz4).

By default it compresses; pass -d/--decompress to decompress. Input
defaults to stdin and output to stdout.`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, decompress, inputPath, outputPath, verbose)
		},
	}

	cmd.Flags().BoolVarP(&decompress, "decompress", "d", false,
		"Decompress (default is compress)")
	cmd.Flags().StringVarP(&inputPath, "input", "i", "",
		"Input file (default: stdin)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "",
		"Output file (default: stdout)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Print byte counts, ratio, and elapsed time to stderr")
	return cmd
}

// run executes one compress-or-decompress pass and, when verbose is set, prints
// a one-line summary to stderr.
func run(cmd *cobra.Command, decompress bool, inputPath, outputPath string, verbose bool) error {
	data, err := cmdio.ReadInput(inputPath)
	if err != nil {
		return err
	}

	var (
		out     []byte
		started = time.Now()
	)
	if decompress {
		out, err = lz4io.Decompress(data)
	} else {
		out = lz4io.Compress(data)
	}
	elapsed := time.Since(started)
	if err != nil {
		return err
	}

	if err := cmdio.WriteOutput(outputPath, out); err != nil {
		return err
	}

	if verbose {
		if decompress {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"decompressed %d → %d bytes in %s\n",
				len(data), len(out), FormatDuration(elapsed))
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"compressed %d → %d bytes (%.1f%%) in %s\n",
				len(data), len(out), Ratio(len(data), len(out)),
				FormatDuration(elapsed))
		}
	}
	return nil
}

// Ratio returns 100 × compressed / original as a percentage; 0 when original is
// zero so the caller doesn't print NaN.
func Ratio(original, compressed int) float64 {
	if original == 0 {
		return 0
	}
	return float64(compressed) / float64(original) * 100
}

// FormatDuration renders d rounded to the nearest microsecond so trailing
// nanosecond noise doesn't leak into the output.
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return d.String()
	}
	return d.Round(time.Microsecond).String()
}
