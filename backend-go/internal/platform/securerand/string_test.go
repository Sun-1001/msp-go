package securerand

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestStringUsesUniformAlphabetIndexes(t *testing.T) {
	restore := replaceRandomReader(bytes.NewReader([]byte{0, 1, 2, 3, 4, 5}))
	defer restore()

	got, err := String(6, "ABC")
	if err != nil {
		t.Fatalf("String() error = %v", err)
	}
	if got != "ABCABC" {
		t.Fatalf("String() = %q", got)
	}
}

func TestBytesAndHexUseRandomReader(t *testing.T) {
	restore := replaceRandomReader(bytes.NewReader([]byte{0x00, 0x0f, 0x10, 0xff}))
	defer restore()

	data, err := Bytes(2)
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}
	if !bytes.Equal(data, []byte{0x00, 0x0f}) {
		t.Fatalf("Bytes() = %x", data)
	}

	encoded, err := Hex(2)
	if err != nil {
		t.Fatalf("Hex() error = %v", err)
	}
	if encoded != "10ff" {
		t.Fatalf("Hex() = %q", encoded)
	}
}

func TestBytesRejectsNegativeLength(t *testing.T) {
	if _, err := Bytes(-1); err == nil {
		t.Fatal("Bytes() error = nil for negative length")
	}
	if _, err := Hex(-1); err == nil {
		t.Fatal("Hex() error = nil for negative length")
	}
}

func TestBytesReturnsReaderErrors(t *testing.T) {
	want := errors.New("entropy unavailable")
	restore := replaceRandomReader(errReader{err: want})
	defer restore()

	got, err := Bytes(1)
	if !errors.Is(err, want) {
		t.Fatalf("Bytes() error = %v, want %v", err, want)
	}
	if got != nil {
		t.Fatalf("Bytes() = %x, want nil", got)
	}
}

func TestStringRejectsInvalidInputs(t *testing.T) {
	if _, err := String(-1, "ABC"); err == nil {
		t.Fatal("String() error = nil for negative length")
	}
	if _, err := String(1, ""); err == nil {
		t.Fatal("String() error = nil for empty alphabet")
	}
	if _, err := String(1, strings.Repeat("A", 257)); err == nil {
		t.Fatal("String() error = nil for alphabet over 256 bytes")
	}
}

func TestStringSkipsModuloBiasRejectedBytes(t *testing.T) {
	restore := replaceRandomReader(bytes.NewReader([]byte{255, 254, 2}))
	defer restore()

	got, err := String(1, "ABC")
	if err != nil {
		t.Fatalf("String() error = %v", err)
	}
	if got != "C" {
		t.Fatalf("String() = %q, want C", got)
	}
}

func TestStringReturnsReaderErrors(t *testing.T) {
	want := errors.New("entropy unavailable")
	restore := replaceRandomReader(errReader{err: want})
	defer restore()

	got, err := String(1, "ABC")
	if !errors.Is(err, want) {
		t.Fatalf("String() error = %v, want %v", err, want)
	}
	if got != "" {
		t.Fatalf("String() = %q, want empty value", got)
	}
}

func TestShuffleStringUsesFisherYatesIndexes(t *testing.T) {
	restore := replaceRandomReader(bytes.NewReader([]byte{1, 0, 1}))
	defer restore()

	got, err := ShuffleString("ABCD")
	if err != nil {
		t.Fatalf("ShuffleString() error = %v", err)
	}
	if got != "CDAB" {
		t.Fatalf("ShuffleString() = %q", got)
	}
}

func TestShuffleStringReturnsReaderErrors(t *testing.T) {
	want := errors.New("entropy unavailable")
	restore := replaceRandomReader(errReader{err: want})
	defer restore()

	got, err := ShuffleString("AB")
	if !errors.Is(err, want) {
		t.Fatalf("ShuffleString() error = %v, want %v", err, want)
	}
	if got != "" {
		t.Fatalf("ShuffleString() = %q, want empty value", got)
	}
}

func replaceRandomReader(reader io.Reader) func() {
	previous := randomReader
	randomReader = reader
	return func() {
		randomReader = previous
	}
}

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}
