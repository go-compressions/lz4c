package cmdio

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadInput_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "in.bin")
	want := []byte("payload")
	if err := os.WriteFile(f, want, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadInput(f)
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReadInput_Stdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = prev }()
	want := []byte("from stdin")
	go func() {
		w.Write(want)
		w.Close()
	}()
	got, err := ReadInput("")
	if err != nil {
		t.Fatalf("ReadInput(stdin): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReadInput_FileNotFound(t *testing.T) {
	if _, err := ReadInput("/does/not/exist/cmdio-test"); err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteOutput_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "out.bin")
	data := []byte{1, 2, 3}
	if err := WriteOutput(f, data); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %v, want %v", got, data)
	}
}

func TestWriteOutput_Stdout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = prev }()

	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		buf.ReadFrom(r)
		done <- buf.Bytes()
	}()

	want := []byte("stdout payload")
	if err := WriteOutput("", want); err != nil {
		t.Fatalf("WriteOutput(stdout): %v", err)
	}
	w.Close()
	got := <-done
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteOutput_PathError(t *testing.T) {
	if err := WriteOutput("/does/not/exist/cmdio-test", []byte("x")); err == nil {
		t.Fatal("expected error for unwriteable path")
	}
}
