package identifier

import (
	"crypto/rand"
	"fmt"
	"io"
)

var uuidRandomReader io.Reader = rand.Reader

// NewUUID returns a random RFC 4122 version 4 UUID string.
func NewUUID() (string, error) {
	var data [16]byte
	if _, err := io.ReadFull(uuidRandomReader, data[:]); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		data[0:4],
		data[4:6],
		data[6:8],
		data[8:10],
		data[10:16],
	), nil
}
