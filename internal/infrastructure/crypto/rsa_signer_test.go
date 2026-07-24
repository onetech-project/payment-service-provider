package crypto_test

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"backbone-new/internal/infrastructure/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRSASigner_Sign(t *testing.T) {
	privateKey, _ := generateTestRSAKeypair(t)
	privKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	}))

	signer := crypto.NewRSASigner()
	stringToSign := "client-test-123|2026-07-22T21:29:07+07:00"

	t.Run("returns a base64-encoded signature", func(t *testing.T) {
		sig, err := signer.Sign(privKeyPEM, stringToSign)
		require.NoError(t, err)

		_, err = base64.StdEncoding.DecodeString(sig)
		assert.NoError(t, err, "Sign() output must be valid base64, matching what RSAVerifier.VerifySignature expects")
	})

	t.Run("invalid private key returns an error", func(t *testing.T) {
		_, err := signer.Sign("not-a-valid-key", stringToSign)
		assert.Error(t, err)
	})
}

// TestRSASignAndVerify_RoundTrip guards against Sign() and VerifySignature()
// drifting onto incompatible encodings (regression: Sign() previously
// hex-encoded while VerifySignature() base64-decoded, so every real
// signature-auth -> access-token/b2b call failed verification).
func TestRSASignAndVerify_RoundTrip(t *testing.T) {
	privateKey, pubKeyPEM := generateTestRSAKeypair(t)
	privKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	}))

	signer := crypto.NewRSASigner()
	verifier := crypto.NewRSAVerifier()

	clientKey := "client-test-123"
	timestamp := "2026-07-22T21:29:07+07:00"
	stringToSign := clientKey + "|" + timestamp

	signature, err := signer.Sign(privKeyPEM, stringToSign)
	require.NoError(t, err)

	err = verifier.VerifySignature(pubKeyPEM, stringToSign, signature)
	assert.NoError(t, err, "a signature produced by RSASigner.Sign must be accepted by RSAVerifier.VerifySignature")
}
