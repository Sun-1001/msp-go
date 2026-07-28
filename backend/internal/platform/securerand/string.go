package securerand

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

var randomReader io.Reader = rand.Reader

// Bytes returns length cryptographically secure random bytes.
func Bytes(length int) ([]byte, error) {
	if length < 0 {
		return nil, errors.New("secure random byte length is negative")
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(randomReader, data); err != nil {
		return nil, err
	}
	return data, nil
}

// Hex returns length random bytes encoded as lower-case hexadecimal text.
func Hex(length int) (string, error) {
	data, err := Bytes(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// Byte returns one byte selected uniformly from an ASCII alphabet.
func Byte(alphabet string) (byte, error) {
	index, err := index(len(alphabet))
	if err != nil {
		return 0, err
	}
	return alphabet[index], nil
}

// String returns a random string of length bytes selected uniformly from an ASCII alphabet.
func String(length int, alphabet string) (string, error) {
	if length < 0 {
		return "", errors.New("secure random string length is negative")
	}
	var builder strings.Builder
	builder.Grow(length)
	for i := 0; i < length; i++ {
		char, err := Byte(alphabet)
		if err != nil {
			return "", err
		}
		builder.WriteByte(char)
	}
	return builder.String(), nil
}

// ShuffleString returns value with its bytes shuffled using Fisher-Yates.
func ShuffleString(value string) (string, error) {
	data := []byte(value)
	for i := len(data) - 1; i > 0; i-- {
		j, err := index(i + 1)
		if err != nil {
			return "", err
		}
		data[i], data[j] = data[j], data[i]
	}
	return string(data), nil
}

func index(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("secure random alphabet is empty")
	}
	if max > 256 {
		return 0, errors.New("secure random alphabet exceeds 256 bytes")
	}
	limit := 256 - (256 % max)
	var data [1]byte
	for {
		if _, err := io.ReadFull(randomReader, data[:]); err != nil {
			return 0, err
		}
		if int(data[0]) < limit {
			return int(data[0]) % max, nil
		}
	}
}
