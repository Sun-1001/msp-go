package wechat

import (
	"crypto/sha1" // #nosec G505 -- the WeChat callback protocol mandates SHA-1 signatures.
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
)

const (
	// MaxCallbackBodyBytes is the maximum callback XML size HTTP adapters should accept.
	MaxCallbackBodyBytes  int64 = 64 << 10
	maxAppIDBytes               = 128
	maxNonceBytes               = 128
	maxEncryptedTextBytes       = 128 << 10
)

var (
	// ErrInvalidSignature reports a missing, malformed, or mismatched WeChat signature.
	ErrInvalidSignature = errors.New("invalid WeChat signature")
	// ErrCipherUnavailable reports an AES operation attempted without an EncodingAESKey.
	ErrCipherUnavailable = errors.New("WeChat message cipher is not configured")
	// ErrInvalidCiphertext reports an invalid encrypted WeChat payload.
	ErrInvalidCiphertext = errors.New("invalid encrypted WeChat payload")
	// ErrInvalidXML reports malformed or structurally invalid WeChat XML.
	ErrInvalidXML = errors.New("invalid WeChat XML payload")
	// ErrInvalidMessage reports a well-formed XML document with invalid message fields.
	ErrInvalidMessage = errors.New("invalid WeChat message")
)

// Protocol implements the cryptographic portions of the WeChat Official Account
// callback protocol. An empty EncodingAESKey configures plaintext-only operation.
type Protocol struct {
	appID  string
	token  string
	cipher *messageCipher
}

// NewProtocol validates account credentials used by callback verification.
// It never includes credential values in returned errors.
func NewProtocol(appID, token, encodingAESKey string) (*Protocol, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" || len(appID) > maxAppIDBytes {
		return nil, errors.New("WeChat AppID must be between 1 and 128 bytes")
	}
	if !validToken(token) {
		return nil, errors.New("WeChat callback token must contain 3 to 32 ASCII letters or digits")
	}

	protocol := &Protocol{appID: appID, token: token}
	encodingAESKey = strings.TrimSpace(encodingAESKey)
	if encodingAESKey == "" {
		return protocol, nil
	}
	cipher, err := newMessageCipher(appID, encodingAESKey)
	if err != nil {
		return nil, err
	}
	protocol.cipher = cipher
	return protocol, nil
}

// HasCipher reports whether the protocol can encrypt and decrypt safe-mode messages.
func (p *Protocol) HasCipher() bool {
	return p != nil && p.cipher != nil
}

// VerifySignature verifies a plaintext callback signature. The timestamp and
// nonce are treated exactly as their query-string representations.
func (p *Protocol) VerifySignature(signature, timestamp, nonce string) error {
	if p == nil || p.token == "" || !validCallbackParameters(timestamp, nonce) {
		return ErrInvalidSignature
	}
	return verifySHA1Signature(signature, p.token, timestamp, nonce)
}

// VerifyMessageSignature verifies an AES callback signature. The encrypted
// value must be the exact base64 text carried by Encrypt or echostr.
func (p *Protocol) VerifyMessageSignature(signature, timestamp, nonce, encrypted string) error {
	if p == nil || p.token == "" || !validCallbackParameters(timestamp, nonce) {
		return ErrInvalidSignature
	}
	if encrypted == "" || len(encrypted) > maxEncryptedTextBytes {
		return ErrInvalidSignature
	}
	return verifySHA1Signature(signature, p.token, timestamp, nonce, encrypted)
}

func verifySHA1Signature(signature string, parts ...string) error {
	if len(signature) != sha1.Size*2 {
		return ErrInvalidSignature
	}
	provided, err := hex.DecodeString(signature)
	if err != nil || len(provided) != sha1.Size {
		return ErrInvalidSignature
	}

	digest := calculateSHA1Signature(parts...)
	if subtle.ConstantTimeCompare(provided, digest[:]) != 1 {
		return ErrInvalidSignature
	}
	return nil
}

func calculateSHA1Signature(parts ...string) [sha1.Size]byte {
	sorted := append([]string(nil), parts...)
	sort.Strings(sorted)
	return sha1.Sum([]byte(strings.Join(sorted, ""))) // #nosec G401 -- WeChat mandates this SHA-1 construction.
}

func validCallbackParameters(timestamp, nonce string) bool {
	if timestamp == "" || len(timestamp) > 20 || nonce == "" || len(nonce) > maxNonceBytes {
		return false
	}
	value, err := strconv.ParseInt(timestamp, 10, 64)
	return err == nil && value > 0
}

func validToken(token string) bool {
	if len(token) < 3 || len(token) > 32 {
		return false
	}
	for _, value := range []byte(token) {
		if (value < 'a' || value > 'z') &&
			(value < 'A' || value > 'Z') &&
			(value < '0' || value > '9') {
			return false
		}
	}
	return true
}
