package crypto

import (
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
	assert.Len(t, signature, 64) // SHA256 hex is 64 chars
}

func TestHMACSigner_SignBase64(t *testing.T) {
	signer := NewHMACSigner("test-secret", "HMAC-SHA512")

	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"
	signature := signer.SignBase64(stringToSign)

	assert.NotEmpty(t, signature)
}

func TestHMACSigner_Verify(t *testing.T) {
	signer := NewHMACSigner("test-secret", "HMAC-SHA256")

	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"
	signature := signer.Sign(stringToSign)

	assert.True(t, signer.Verify(stringToSign, signature))
	assert.False(t, signer.Verify(stringToSign, "invalid-signature"))
}

func TestBuildStringToSign(t *testing.T) {
	result := BuildStringToSign("POST", "/api/test", "token123", "abc123", "2026-07-22T10:00:00+07:00")

	assert.Equal(t, "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00", result)
}

func TestHashSHA256Hex(t *testing.T) {
	hash := HashSHA256Hex("test data")

	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64)
}

func TestHashSHA256Reader(t *testing.T) {
	hash, err := HashSHA256Reader(strings.NewReader("test data"))
	assert.NoError(t, err)
	assert.Equal(t, HashSHA256Hex("test data"), hash)
}

func TestHashSHA256Reader_Error(t *testing.T) {
	_, err := HashSHA256Reader(&errorReader{})
	assert.Error(t, err)
}

type errorReader struct{}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestNewHMACSigner_UnknownAlgorithmDefaultsToSHA512(t *testing.T) {
	signer := NewHMACSigner("secret", "unknown-algo")
	sig := signer.SignBase64("test")
	assert.NotEmpty(t, sig)

	sha512Signer := NewHMACSigner("secret", "HMAC-SHA512")
	assert.Equal(t, sha512Signer.SignBase64("test"), sig)
}

func TestHMACSigner_DifferentSecrets(t *testing.T) {
	signer1 := NewHMACSigner("secret1", "HMAC-SHA256")
	signer2 := NewHMACSigner("secret2", "HMAC-SHA256")

	stringToSign := "POST:/api/test:token123:abc123:2026-07-22T10:00:00+07:00"

	sig1 := signer1.Sign(stringToSign)
	sig2 := signer2.Sign(stringToSign)

	assert.NotEqual(t, sig1, sig2)
}
