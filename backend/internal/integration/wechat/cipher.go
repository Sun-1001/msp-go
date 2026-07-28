package wechat

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
)

const (
	encodingAESKeyLength     = 43
	messageAESKeyLength      = 32
	messageRandomPrefixBytes = 16
	messageLengthPrefixBytes = 4
	wechatPKCS7BlockSize     = 32
	maxPlaintextBytes        = int(MaxCallbackBodyBytes)
)

type messageCipher struct {
	appID  []byte
	key    []byte
	random io.Reader
}

func newMessageCipher(appID, encodingAESKey string) (*messageCipher, error) {
	if len(encodingAESKey) != encodingAESKeyLength {
		return nil, errors.New("WeChat EncodingAESKey must contain exactly 43 characters")
	}
	key, err := base64.StdEncoding.Strict().DecodeString(encodingAESKey + "=")
	if err != nil || len(key) != messageAESKeyLength {
		return nil, errors.New("WeChat EncodingAESKey is invalid")
	}
	return &messageCipher{
		appID:  []byte(appID),
		key:    key,
		random: cryptorand.Reader,
	}, nil
}

// Decrypt decrypts an AES safe-mode message or encrypted echostr and verifies
// that its embedded receiver ID matches this protocol's AppID.
func (p *Protocol) Decrypt(encrypted string) ([]byte, error) {
	if p == nil || p.cipher == nil {
		return nil, ErrCipherUnavailable
	}
	return p.cipher.decrypt(encrypted)
}

// Encrypt encrypts one plaintext XML message or echostr for AES safe mode.
func (p *Protocol) Encrypt(plaintext []byte) (string, error) {
	if p == nil || p.cipher == nil {
		return "", ErrCipherUnavailable
	}
	return p.cipher.encrypt(plaintext)
}

func (c *messageCipher) decrypt(encrypted string) ([]byte, error) {
	if encrypted == "" || len(encrypted) > maxEncryptedTextBytes {
		return nil, ErrInvalidCiphertext
	}
	ciphertext, err := base64.StdEncoding.Strict().DecodeString(encrypted)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%wechatPKCS7BlockSize != 0 {
		return nil, ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	plainPadded := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, c.key[:aes.BlockSize]).CryptBlocks(plainPadded, ciphertext)
	plain, ok := removePKCS7Padding(plainPadded)
	if !ok || len(plain) < messageRandomPrefixBytes+messageLengthPrefixBytes {
		return nil, ErrInvalidCiphertext
	}

	lengthOffset := messageRandomPrefixBytes
	messageLength := uint64(binary.BigEndian.Uint32(plain[lengthOffset : lengthOffset+messageLengthPrefixBytes]))
	messageOffset := lengthOffset + messageLengthPrefixBytes
	if messageLength > uint64(maxPlaintextBytes) || messageLength > uint64(len(plain)-messageOffset) {
		return nil, ErrInvalidCiphertext
	}
	messageEnd := messageOffset + int(messageLength)
	receiverID := plain[messageEnd:]
	if len(receiverID) != len(c.appID) || subtle.ConstantTimeCompare(receiverID, c.appID) != 1 {
		return nil, ErrInvalidCiphertext
	}

	message := make([]byte, int(messageLength))
	copy(message, plain[messageOffset:messageEnd])
	return message, nil
}

func (c *messageCipher) encrypt(plaintext []byte) (string, error) {
	if len(plaintext) > maxPlaintextBytes {
		return "", errors.New("WeChat plaintext exceeds 64 KiB")
	}
	length := messageRandomPrefixBytes + messageLengthPrefixBytes + len(plaintext) + len(c.appID)
	plain := make([]byte, length)
	if _, err := io.ReadFull(c.random, plain[:messageRandomPrefixBytes]); err != nil {
		return "", errors.New("generate WeChat message randomness")
	}
	binary.BigEndian.PutUint32(plain[messageRandomPrefixBytes:], uint32(len(plaintext)))
	copy(plain[messageRandomPrefixBytes+messageLengthPrefixBytes:], plaintext)
	copy(plain[messageRandomPrefixBytes+messageLengthPrefixBytes+len(plaintext):], c.appID)
	plain = addPKCS7Padding(plain)

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", errors.New("initialize WeChat message cipher")
	}
	ciphertext := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, c.key[:aes.BlockSize]).CryptBlocks(ciphertext, plain)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func addPKCS7Padding(data []byte) []byte {
	paddingLength := wechatPKCS7BlockSize - len(data)%wechatPKCS7BlockSize
	result := make([]byte, len(data)+paddingLength)
	copy(result, data)
	for index := len(data); index < len(result); index++ {
		result[index] = byte(paddingLength)
	}
	return result
}

func removePKCS7Padding(data []byte) ([]byte, bool) {
	if len(data) == 0 || len(data)%wechatPKCS7BlockSize != 0 {
		return nil, false
	}
	paddingLength := int(data[len(data)-1])
	if paddingLength < 1 || paddingLength > wechatPKCS7BlockSize || paddingLength > len(data) {
		return nil, false
	}
	invalid := byte(0)
	for _, value := range data[len(data)-paddingLength:] {
		invalid |= value ^ byte(paddingLength)
	}
	if invalid != 0 {
		return nil, false
	}
	return data[:len(data)-paddingLength], true
}
