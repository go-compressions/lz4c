// Package lz4io adapts the raw LZ4 *block* codec from
// github.com/go-compressions/lz4 into a tiny self-describing container so the
// lz4c CLI can round-trip arbitrary input through a single file or pipe.
//
// The underlying library exposes only the LZ4 block format
// (CompressBlock / DecompressBlock). A raw LZ4 block does not record the
// decompressed length, so a standalone decoder cannot size its output buffer
// up front. lz4io therefore frames each block with a fixed 12-byte header:
//
//	magic   [4]byte  // "LZ4C"
//	rawLen  uint64   // little-endian length of the original (uncompressed) data
//	...block...      // the standard LZ4 block, byte-for-byte
//
// The block payload after the header is an unmodified, standard LZ4 block and
// stays wire-compatible with pierrec/lz4 (see the library's
// TestCrossCompatPierrec): strip the 12-byte header and any LZ4 block decoder
// can consume the remainder.
package lz4io

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/go-compressions/lz4"
)

// Magic is the 4-byte container identifier.
var Magic = [4]byte{'L', 'Z', '4', 'C'}

// HeaderLen is the size in bytes of the container header (magic + rawLen).
const HeaderLen = 4 + 8

// ErrBadMagic is returned when the input does not start with the lz4c magic.
var ErrBadMagic = errors.New("lz4io: not an lz4c stream (bad magic)")

// ErrTruncated is returned when the input is shorter than a full header.
var ErrTruncated = errors.New("lz4io: truncated lz4c stream (short header)")

// ErrSizeMismatch is returned when the decoded block length does not match the
// length recorded in the header, signalling a corrupt stream.
var ErrSizeMismatch = errors.New("lz4io: decompressed size does not match header")

// Compress frames data into an lz4c container: the 12-byte header followed by
// the standard LZ4 block produced by the library.
func Compress(data []byte) []byte {
	block := lz4.CompressBlock(data)
	out := make([]byte, HeaderLen, HeaderLen+len(block))
	copy(out[0:4], Magic[:])
	binary.LittleEndian.PutUint64(out[4:HeaderLen], uint64(len(data)))
	return append(out, block...)
}

// Decompress validates the container header and decodes the trailing LZ4 block
// back to the original bytes.
func Decompress(data []byte) ([]byte, error) {
	if len(data) < HeaderLen {
		return nil, ErrTruncated
	}
	if [4]byte(data[0:4]) != Magic {
		return nil, ErrBadMagic
	}
	rawLen := binary.LittleEndian.Uint64(data[4:HeaderLen])
	out, err := lz4.DecompressBlock(data[HeaderLen:], int(rawLen))
	if err != nil {
		return nil, fmt.Errorf("lz4io: %w", err)
	}
	if uint64(len(out)) != rawLen {
		return nil, ErrSizeMismatch
	}
	return out, nil
}
