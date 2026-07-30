package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHMACSigner_Sign(t *testing.T) {
	signer := NewHMACSigner("test-secret", "HMAC-SHA256")

	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"
	signature := signer.Sign(stringToSign)

	assert.NotEmpty(t, signature)
	assert.Len(t, signature, 44) // SHA256 (32 bytes) standard base64 is 44 chars, incl. padding
}

func TestHMACSigner_Verify(t *testing.T) {
	signer := NewHMACSigner("test-secret", "HMAC-SHA256")

	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"
	signature := signer.Sign(stringToSign)

	assert.True(t, signer.Verify(stringToSign, signature))
	assert.False(t, signer.Verify(stringToSign, "invalid-signature"))
}

func TestHMACSigner_Verify_RejectsHexEncodedSignature(t *testing.T) {
	// Feature 012-base64-hash-encoding: a signature computed with the old
	// hex convention must not verify against the new base64-only Sign/Verify.
	signer := NewHMACSigner("test-secret", "HMAC-SHA256")
	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"

	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(stringToSign))
	hexSignature := hex.EncodeToString(mac.Sum(nil))

	assert.False(t, signer.Verify(stringToSign, hexSignature))
}

func TestBuildStringToSign(t *testing.T) {
	result := BuildStringToSign("POST", "/api/test", "token123", "abc123", "2026-07-22T10:00:00+07:00")

	assert.Equal(t, "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00", result)
}

func TestHashSHA256Base64(t *testing.T) {
	hash := HashSHA256Base64("test data")

	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 44) // SHA256 (32 bytes) standard base64 is 44 chars, incl. padding
}

func TestHashSHA256ReaderBase64(t *testing.T) {
	hash, err := HashSHA256ReaderBase64(strings.NewReader("test data"))
	assert.NoError(t, err)
	assert.Equal(t, HashSHA256Base64("test data"), hash)
}

func TestHashSHA256ReaderBase64_Error(t *testing.T) {
	_, err := HashSHA256ReaderBase64(&errorReader{})
	assert.Error(t, err)
}

type errorReader struct{}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestNewHMACSigner_UnknownAlgorithmDefaultsToSHA512(t *testing.T) {
	signer := NewHMACSigner("secret", "unknown-algo")
	sig := signer.Sign("test")
	assert.NotEmpty(t, sig)

	sha512Signer := NewHMACSigner("secret", "HMAC-SHA512")
	assert.Equal(t, sha512Signer.Sign("test"), sig)
}

func TestHMACSigner_DifferentSecrets(t *testing.T) {
	signer1 := NewHMACSigner("secret1", "HMAC-SHA256")
	signer2 := NewHMACSigner("secret2", "HMAC-SHA256")

	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"

	sig1 := signer1.Sign(stringToSign)
	sig2 := signer2.Sign(stringToSign)

	assert.NotEqual(t, sig1, sig2)
}
