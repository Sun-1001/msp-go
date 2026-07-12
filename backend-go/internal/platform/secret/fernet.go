package secret

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

const fernetVersion byte = 0x80

// Fernet encrypts and decrypts Python cryptography.fernet-compatible tokens.
type Fernet struct {
	signingKey    []byte
	encryptionKey []byte
	now           func() time.Time
	random        io.Reader
}

// NewFernet creates a Fernet service from a URL-safe base64 32-byte key.
func NewFernet(secretKey string) (*Fernet, error) {
	key, err := base64.URLEncoding.DecodeString(secretKey)
	if err != nil {
		key, err = base64.RawURLEncoding.DecodeString(secretKey)
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("fernet key must decode to 32 bytes, got %d", len(key))
	}
	return &Fernet{
		signingKey:    append([]byte(nil), key[:16]...),
		encryptionKey: append([]byte(nil), key[16:]...),
		now:           func() time.Time { return time.Now().UTC() },
		random:        rand.Reader,
	}, nil
}

// GenerateFernetKey returns a new URL-safe base64 32-byte Fernet key.
func GenerateFernetKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

// Encrypt returns a Fernet token for plaintext.
func (f *Fernet) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	unixSeconds := f.now().Unix()
	if unixSeconds < 0 {
		return "", errors.New("fernet timestamp must not precede Unix epoch")
	}
	block, err := aes.NewCipher(f.encryptionKey)
	if err != nil {
		return "", err
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(f.random, iv); err != nil {
		return "", err
	}
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	message := make([]byte, 0, 1+8+len(iv)+len(ciphertext)+sha256.Size)
	message = append(message, fernetVersion)
	timestamp := make([]byte, 8)
	// #nosec G115 -- the negative range is rejected above.
	binary.BigEndian.PutUint64(timestamp, uint64(unixSeconds))
	message = append(message, timestamp...)
	message = append(message, iv...)
	message = append(message, ciphertext...)
	signature := hmacSHA256Bytes(f.signingKey, message)
	message = append(message, signature...)
	return base64.URLEncoding.EncodeToString(message), nil
}

// Decrypt validates a Fernet token and returns its plaintext.
func (f *Fernet) Decrypt(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		data, err = base64.RawURLEncoding.DecodeString(token)
	}
	if err != nil {
		return "", err
	}
	if len(data) < 1+8+aes.BlockSize+sha256.Size || data[0] != fernetVersion {
		return "", errors.New("invalid fernet token")
	}
	signed := data[:len(data)-sha256.Size]
	gotSignature := data[len(data)-sha256.Size:]
	wantSignature := hmacSHA256Bytes(f.signingKey, signed)
	if subtle.ConstantTimeCompare(gotSignature, wantSignature) != 1 {
		return "", errors.New("invalid fernet signature")
	}
	ciphertext := data[1+8+aes.BlockSize : len(data)-sha256.Size]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("invalid fernet ciphertext")
	}
	block, err := aes.NewCipher(f.encryptionKey)
	if err != nil {
		return "", err
	}
	plaintext := make([]byte, len(ciphertext))
	iv := data[1+8 : 1+8+aes.BlockSize]
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func hmacSHA256Bytes(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	// #nosec G115 -- callers use AES.BlockSize, so padding is in [1, 16].
	return append(append([]byte(nil), data...), bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid pkcs7 block size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, errors.New("invalid pkcs7 padding")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, errors.New("invalid pkcs7 padding")
		}
	}
	return data[:len(data)-padding], nil
}
