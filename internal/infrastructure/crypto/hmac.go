package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"strings"
)

// HMACSigner creates HMAC signatures for vendor API requests
type HMACSigner struct {
	secret string
	hashFn func() hash.Hash
}

// NewHMACSigner creates a new HMAC signer with the given secret
// algorithm should be "HMAC-SHA256" or "HMAC-SHA512"
func NewHMACSigner(secret string, algorithm string) *HMACSigner {
	var hashFn func() hash.Hash
	switch strings.ToUpper(algorithm) {
	case "HMAC-SHA256":
		hashFn = sha256.New
	case "HMAC-SHA512":
		hashFn = sha512.New
	default:
		hashFn = sha512.New
	}
	return &HMACSigner{
		secret: secret,
		hashFn: hashFn,
	}
}

// Sign generates an HMAC signature for the given string-to-sign, base64 encoded
func (s *HMACSigner) Sign(stringToSign string) string {
	mac := hmac.New(s.hashFn, []byte(s.secret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Verify verifies an HMAC signature
func (s *HMACSigner) Verify(stringToSign, signature string) bool {
	expected := s.Sign(stringToSign)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// BuildStringToSign builds the string-to-sign for vendor SNAP API
// Format: HTTPMethod:RelativeURL:AccessToken:SHA256(RequestBody):Timestamp
func BuildStringToSign(method, relativeURL, accessToken, requestBodyHash, timestamp string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", method, relativeURL, accessToken, requestBodyHash, timestamp)
}

// HashSHA256Base64 computes SHA256 hash of the input and returns standard base64
func HashSHA256Base64(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// HashSHA256ReaderBase64 computes SHA256 hash from a reader, returning standard base64
func HashSHA256ReaderBase64(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
