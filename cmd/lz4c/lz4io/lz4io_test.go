package lz4io

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/go-compressions/lz4"
	plz4 "github.com/pierrec/lz4/v4"
)

// inputs returns a representative spread: empty, tiny, text, highly
// compressible, and incompressible (pseudo-random) data.
func inputs() [][]byte {
	rng := make([]byte, 64*1024)
	// Deterministic LCG so the test is reproducible across runs/arches.
	x := uint32(0x9e3779b9)
	for i := range rng {
		x = x*1664525 + 1013904223
		rng[i] = byte(x >> 24)
	}
	return [][]byte{
		{},
		[]byte("x"),
		[]byte("the quick brown fox jumps over the lazy dog"),
		bytes.Repeat([]byte("ABCDEFGH"), 8192),
		rng,
	}
}

func TestRoundTrip(t *testing.T) {
	for i, src := range inputs() {
		framed := Compress(src)
		if !bytes.HasPrefix(framed, Magic[:]) {
			t.Fatalf("input %d: output missing magic", i)
		}
		got, err := Decompress(framed)
		if err != nil {
			t.Fatalf("input %d: Decompress: %v", i, err)
		}
		if !bytes.Equal(got, src) {
			t.Fatalf("input %d: round-trip mismatch (%d vs %d bytes)", i, len(got), len(src))
		}
	}
}

// TestBlockIsPierrecCompatible confirms the payload after the 12-byte header is
// a standard LZ4 block that pierrec/lz4 can decode unchanged.
func TestBlockIsPierrecCompatible(t *testing.T) {
	for i, src := range inputs() {
		if len(src) == 0 {
			continue
		}
		framed := Compress(src)
		block := framed[HeaderLen:]
		out := make([]byte, len(src))
		n, err := plz4.UncompressBlock(block, out)
		if err != nil {
			t.Fatalf("input %d: pierrec could not decode our block: %v", i, err)
		}
		if !bytes.Equal(out[:n], src) {
			t.Fatalf("input %d: pierrec decode mismatch", i)
		}
	}
}

func TestDecompress_Truncated(t *testing.T) {
	if _, err := Decompress([]byte{'L', 'Z', '4'}); !errors.Is(err, ErrTruncated) {
		t.Fatalf("got %v, want ErrTruncated", err)
	}
}

func TestDecompress_BadMagic(t *testing.T) {
	bad := make([]byte, HeaderLen)
	copy(bad, []byte("XXXX"))
	if _, err := Decompress(bad); !errors.Is(err, ErrBadMagic) {
		t.Fatalf("got %v, want ErrBadMagic", err)
	}
}

func TestDecompress_CorruptBlock(t *testing.T) {
	// Valid header, but the trailing block is malformed: a token claiming 15
	// literals with no following length/literal bytes.
	buf := make([]byte, HeaderLen+1)
	copy(buf[0:4], Magic[:])
	binary.LittleEndian.PutUint64(buf[4:HeaderLen], 100)
	buf[HeaderLen] = 0xF0 // literal length 15, needs continuation bytes
	_, err := Decompress(buf)
	if err == nil || !strings.Contains(err.Error(), "lz4io:") {
		t.Fatalf("got %v, want wrapped lz4io error", err)
	}
	if errors.Is(err, ErrBadMagic) || errors.Is(err, ErrTruncated) || errors.Is(err, ErrSizeMismatch) {
		t.Fatalf("unexpected sentinel: %v", err)
	}
}

func TestDecompress_SizeMismatch(t *testing.T) {
	// A genuine block whose header lies about the original length.
	block := lz4.CompressBlock([]byte("hello hello hello"))
	buf := make([]byte, HeaderLen, HeaderLen+len(block))
	copy(buf[0:4], Magic[:])
	binary.LittleEndian.PutUint64(buf[4:HeaderLen], 999) // wrong length
	buf = append(buf, block...)
	if _, err := Decompress(buf); !errors.Is(err, ErrSizeMismatch) {
		t.Fatalf("got %v, want ErrSizeMismatch", err)
	}
}
