package crypto

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"strings"
)

// Body-hash encodings for the RequestBody component of stringToSign.
//
// BCA's SNAP spec (Developer API BCA, "Signature Symmetric") defines the
// component as Lowercase(HexEncode(SHA-256(MinifyJson(RequestBody)))), so
// BodyHashHex is the spec-conformant choice and the default. BodyHashBase64
// exists because vendors onboarded under feature 012-base64-hash-encoding sign
// with base64 and would break if the encoding were changed under them — this
// is a per-vendor setting precisely so one non-conformant vendor cannot force
// every other vendor off spec.
const (
	BodyHashHex    = "hex"
	BodyHashBase64 = "base64"
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

// Verify verifies an HMAC signature. The comparison is done on the decoded
// bytes so a signature sent without base64 padding — which some SNAP clients
// do — still matches; an undecodable signature falls back to comparing the
// raw strings so the result is still constant-time rather than an early exit.
func (s *HMACSigner) Verify(stringToSign, signature string) bool {
	expected := s.Sign(stringToSign)
	if hmac.Equal([]byte(expected), []byte(signature)) {
		return true
	}

	expectedBytes, err := base64.StdEncoding.DecodeString(expected)
	if err != nil {
		return false
	}
	actualBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		if actualBytes, err = base64.RawStdEncoding.DecodeString(signature); err != nil {
			return false
		}
	}
	return hmac.Equal(expectedBytes, actualBytes)
}

// BuildStringToSign builds the string-to-sign for vendor SNAP API
// Format: HTTPMethod:RelativeURL:AccessToken:SHA256(RequestBody):Timestamp
func BuildStringToSign(method, relativeURL, accessToken, requestBodyHash, timestamp string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", method, relativeURL, accessToken, requestBodyHash, timestamp)
}

// MinifyJSON removes insignificant whitespace from a JSON document, per the
// MinifyJson step BCA applies before hashing the request body. A body that is
// not valid JSON (an empty body, or a malformed one we are about to reject
// anyway) is returned unchanged rather than erroring: the signature check must
// still run and fail closed on its own terms, not short-circuit into a
// different error class.
func MinifyJSON(data []byte) []byte {
	if len(bytes.TrimSpace(data)) == 0 {
		return data
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return data
	}
	return buf.Bytes()
}

// HashRequestBody computes the RequestBody component of stringToSign:
// minify the JSON, SHA-256 it, then encode per the vendor's configured
// encoding. An unrecognised encoding falls back to hex — the spec-conformant
// form — so a typo in vendor config degrades toward the standard rather than
// away from it.
func HashRequestBody(body []byte, encoding string) string {
	sum := sha256.Sum256(MinifyJSON(body))
	if strings.EqualFold(encoding, BodyHashBase64) {
		return base64.StdEncoding.EncodeToString(sum[:])
	}
	return hex.EncodeToString(sum[:])
}

// HashSHA256Hex computes the SHA256 of the input as lowercase hex, the
// encoding BCA's SNAP signature spec requires.
func HashSHA256Hex(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
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
