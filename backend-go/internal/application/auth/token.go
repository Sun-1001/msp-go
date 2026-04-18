package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"

	"mathstudy/backend-go/internal/domain/user"
)

const (
	jwtIssuer   = "math-study-platform"
	jwtAudience = "msp-api"
)

var errInvalidToken = errors.New("invalid token")

// TokenClaims contains the JWT claims shared with the Python backend.
type TokenClaims struct {
	Subject string
	Role    user.Role
	Type    string
	JTI     string
	Issued  time.Time
	Expires time.Time
}

// TokenService creates and verifies HMAC JWTs compatible with the Python PyJWT setup.
type TokenService struct {
	secret     []byte
	algorithm  string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// NewTokenService builds a JWT service for HS256, HS384, or HS512.
func NewTokenService(secret, algorithm string, accessTTL, refreshTTL time.Duration) (TokenService, error) {
	algorithm = strings.ToUpper(strings.TrimSpace(algorithm))
	if _, err := hashForAlgorithm(algorithm); err != nil {
		return TokenService{}, err
	}
	if strings.TrimSpace(secret) == "" {
		return TokenService{}, errors.New("jwt secret key is empty")
	}
	if accessTTL <= 0 {
		return TokenService{}, errors.New("jwt access token ttl must be greater than zero")
	}
	if refreshTTL <= 0 {
		return TokenService{}, errors.New("jwt refresh token ttl must be greater than zero")
	}
	return TokenService{
		secret:     []byte(secret),
		algorithm:  algorithm,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        func() time.Time { return time.Now().UTC() },
	}, nil
}

func newTokenServiceWithClock(secret, algorithm string, accessTTL, refreshTTL time.Duration, now func() time.Time) (TokenService, error) {
	service, err := NewTokenService(secret, algorithm, accessTTL, refreshTTL)
	if err != nil {
		return TokenService{}, err
	}
	service.now = now
	return service, nil
}

// CreateAccessToken returns a signed access token with a lower-case role claim.
func (s TokenService) CreateAccessToken(subject string, role user.Role) (string, error) {
	return s.createToken(subject, "access", s.accessTTL, map[string]any{"role": string(role)})
}

// CreateRefreshToken returns a signed refresh token.
func (s TokenService) CreateRefreshToken(subject string) (string, error) {
	return s.createToken(subject, "refresh", s.refreshTTL, nil)
}

// Decode verifies a token and returns its compatible claims.
func (s TokenService) Decode(token string) (TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenClaims{}, errInvalidToken
	}

	var header map[string]any
	if err := decodeSegment(parts[0], &header); err != nil {
		return TokenClaims{}, errInvalidToken
	}
	alg, ok := header["alg"].(string)
	if !ok || strings.ToUpper(alg) != s.algorithm {
		return TokenClaims{}, errInvalidToken
	}

	expected, err := s.sign(parts[0] + "." + parts[1])
	if err != nil {
		return TokenClaims{}, err
	}
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) != 1 {
		return TokenClaims{}, errInvalidToken
	}

	var rawClaims map[string]any
	if err := decodeSegment(parts[1], &rawClaims); err != nil {
		return TokenClaims{}, errInvalidToken
	}
	if !claimMatches(rawClaims["iss"], jwtIssuer) || !audienceMatches(rawClaims["aud"], jwtAudience) {
		return TokenClaims{}, errInvalidToken
	}
	subject, _ := rawClaims["sub"].(string)
	tokenType, _ := rawClaims["type"].(string)
	jti, _ := rawClaims["jti"].(string)
	if subject == "" || tokenType == "" || jti == "" {
		return TokenClaims{}, errInvalidToken
	}

	issued, ok := numericDate(rawClaims["iat"])
	if !ok {
		return TokenClaims{}, errInvalidToken
	}
	expires, ok := numericDate(rawClaims["exp"])
	if !ok || s.now().After(expires) {
		return TokenClaims{}, errInvalidToken
	}

	var role user.Role
	if roleValue, ok := rawClaims["role"].(string); ok && roleValue != "" {
		parsedRole, err := user.ParseRole(roleValue)
		if err != nil {
			return TokenClaims{}, errInvalidToken
		}
		role = parsedRole
	}

	return TokenClaims{
		Subject: subject,
		Role:    role,
		Type:    tokenType,
		JTI:     jti,
		Issued:  issued,
		Expires: expires,
	}, nil
}

func (s TokenService) createToken(subject string, tokenType string, ttl time.Duration, extra map[string]any) (string, error) {
	now := s.now().UTC()
	jti, err := randomHex(16)
	if err != nil {
		return "", err
	}
	claims := map[string]any{
		"exp":  now.Add(ttl).Unix(),
		"iat":  now.Unix(),
		"sub":  subject,
		"iss":  jwtIssuer,
		"aud":  jwtAudience,
		"jti":  jti,
		"type": tokenType,
	}
	for key, value := range extra {
		claims[key] = value
	}
	header := map[string]string{"alg": s.algorithm, "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature, err := s.sign(unsigned)
	if err != nil {
		return "", err
	}
	return unsigned + "." + signature, nil
}

func (s TokenService) sign(unsigned string) (string, error) {
	hashFn, err := hashForAlgorithm(s.algorithm)
	if err != nil {
		return "", err
	}
	mac := hmac.New(hashFn, s.secret)
	_, _ = mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func hashForAlgorithm(algorithm string) (func() hash.Hash, error) {
	switch algorithm {
	case "HS256":
		return sha256.New, nil
	case "HS384":
		return sha512.New384, nil
	case "HS512":
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("unsupported jwt algorithm %q", algorithm)
	}
}

func decodeSegment(segment string, target any) error {
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func randomHex(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func claimMatches(value any, want string) bool {
	got, ok := value.(string)
	return ok && got == want
}

func audienceMatches(value any, want string) bool {
	if got, ok := value.(string); ok {
		return got == want
	}
	values, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range values {
		if got, ok := item.(string); ok && got == want {
			return true
		}
	}
	return false
}

func numericDate(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case json.Number:
		seconds, err := typed.Int64()
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(seconds, 0).UTC(), true
	case float64:
		return time.Unix(int64(typed), 0).UTC(), true
	case int64:
		return time.Unix(typed, 0).UTC(), true
	default:
		return time.Time{}, false
	}
}
