package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// run drives RootCmd with the given args, capturing stderr. Binary output goes
// to a temp file via -o so it never pollutes the test stream.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := RootCmd()
	var stderr bytes.Buffer
	c.SetArgs(args)
	c.SetOut(&bytes.Buffer{})
	c.SetErr(&stderr)
	err := c.Execute()
	return stderr.String(), err
}

func TestRoundtrip_Files(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	comp := filepath.Join(dir, "src.lz4")
	round := filepath.Join(dir, "round.bin")
	original := bytes.Repeat([]byte("roundtrip via cobra "), 100)
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "-i", src, "-o", comp); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if _, err := runCmd(t, "-d", "-i", comp, "-o", round); err != nil {
		t.Fatalf("decompress: %v", err)
	}
	got, err := os.ReadFile(round)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("roundtrip mismatch: got %d bytes, want %d", len(got), len(original))
	}
}

func TestVerbose_Compress(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	out := filepath.Join(dir, "src.lz4")
	if err := os.WriteFile(src, bytes.Repeat([]byte("verbose "), 200), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, err := runCmd(t, "-v", "-i", src, "-o", out)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if !strings.Contains(stderr, "compressed") || !strings.Contains(stderr, " in ") ||
		!strings.Contains(stderr, "%") {
		t.Fatalf("compress summary missing fragments: %q", stderr)
	}
}

func TestVerbose_Decompress(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	comp := filepath.Join(dir, "src.lz4")
	round := filepath.Join(dir, "round.bin")
	if err := os.WriteFile(src, bytes.Repeat([]byte("verbose dec "), 200), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "-i", src, "-o", comp); err != nil {
		t.Fatalf("compress: %v", err)
	}
	stderr, err := runCmd(t, "-v", "-d", "-i", comp, "-o", round)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if !strings.Contains(stderr, "decompressed") || !strings.Contains(stderr, " in ") {
		t.Fatalf("decompress summary missing fragments: %q", stderr)
	}
}

func TestCompressDecompress_Stdout(t *testing.T) {
	// Exercise the stdout (empty -o) write path for both directions by piping
	// through real os.Stdin/os.Stdout swaps.
	original := []byte("piped through stdout path")

	comp := withStdin(t, original, func() []byte {
		return captureStdout(t, func() {
			if _, err := runCmd(t); err != nil {
				t.Fatalf("compress: %v", err)
			}
		})
	})

	got := withStdin(t, comp, func() []byte {
		return captureStdout(t, func() {
			if _, err := runCmd(t, "-d"); err != nil {
				t.Fatalf("decompress: %v", err)
			}
		})
	})

	if !bytes.Equal(got, original) {
		t.Fatalf("stdout roundtrip mismatch: %q vs %q", got, original)
	}
}

func TestReadError(t *testing.T) {
	if _, err := runCmd(t, "-i", "/does/not/exist/lz4c.input"); err == nil {
		t.Fatal("expected error for missing input")
	}
}

func TestDecompressError(t *testing.T) {
	// A file that is not an lz4c stream must fail decompression.
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk")
	if err := os.WriteFile(junk, []byte("not lz4c"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "-d", "-i", junk); err == nil {
		t.Fatal("expected error decompressing junk")
	}
}

func TestWriteError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "-i", src, "-o", "/does/not/exist/out.lz4"); err == nil {
		t.Fatal("expected error for unwriteable output")
	}
}

func TestRatio(t *testing.T) {
	if got := Ratio(0, 10); got != 0 {
		t.Fatalf("Ratio(0,10) = %v, want 0", got)
	}
	if got := Ratio(100, 50); got != 50 {
		t.Fatalf("Ratio(100,50) = %v, want 50", got)
	}
}

func TestFormatDuration(t *testing.T) {
	if got := FormatDuration(500 * time.Nanosecond); got != "500ns" {
		t.Fatalf("sub-microsecond: got %q", got)
	}
	if got := FormatDuration(1500 * time.Nanosecond); !strings.HasSuffix(got, "µs") {
		t.Fatalf("microsecond rounding: got %q", got)
	}
}

func TestMain_OsExitOnError(t *testing.T) {
	prevExit := osExit
	prevArgs := os.Args
	defer func() {
		osExit = prevExit
		os.Args = prevArgs
	}()

	called := false
	osExit = func(code int) {
		called = true
		if code != 1 {
			t.Errorf("osExit code: got %d, want 1", code)
		}
	}
	os.Args = []string{"lz4c", "-i", "/does/not/exist"}
	main()
	if !called {
		t.Fatal("osExit not called")
	}
}

func TestMain_CleanRun(t *testing.T) {
	prevExit := osExit
	prevArgs := os.Args
	defer func() {
		osExit = prevExit
		os.Args = prevArgs
	}()
	osExit = func(code int) {
		t.Fatalf("osExit called with %d on a clean run", code)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	out := filepath.Join(dir, "out.lz4")
	if err := os.WriteFile(src, []byte("clean run payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"lz4c", "-i", src, "-o", out}
	main()
}

// --- stdin/stdout swap helpers ---

func withStdin(t *testing.T, data []byte, fn func() []byte) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = prev }()
	go func() {
		w.Write(data)
		w.Close()
	}()
	return fn()
}

func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		buf.ReadFrom(r)
		done <- buf.Bytes()
	}()
	fn()
	w.Close()
	os.Stdout = prev
	return <-done
}
