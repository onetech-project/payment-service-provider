package crypto_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"backbone-new/internal/infrastructure/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestRSAKeypair(t *testing.T) (*rsa.PrivateKey, string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return privateKey, string(pemBytes)
}

func TestRSASignatureVerifier_Verify(t *testing.T) {
	privateKey, pubKeyPEM := generateTestRSAKeypair(t)
	verifier := crypto.NewRSAVerifier()

	clientKey := "client-test-123"
	timestamp := "2026-07-22T21:29:07+07:00"
	stringToSign := clientKey + "|" + timestamp

	// Create valid signature
	hashed := crypto.HashSHA256([]byte(stringToSign))
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.CryptoSHA256, hashed)
	require.NoError(t, err)
	validSignatureBase64 := base64.StdEncoding.EncodeToString(sigBytes)

	t.Run("Valid Signature", func(t *testing.T) {
		err := verifier.VerifySignature(pubKeyPEM, stringToSign, validSignatureBase64)
		assert.NoError(t, err)
	})

	t.Run("Tampered Signature", func(t *testing.T) {
		tamperedSig := validSignatureBase64[:len(validSignatureBase64)-4] + "AAAA"
		err := verifier.VerifySignature(pubKeyPEM, stringToSign, tamperedSig)
		assert.Error(t, err)
	})

	t.Run("Invalid Public Key PEM", func(t *testing.T) {
		err := verifier.VerifySignature("INVALID_PEM", stringToSign, validSignatureBase64)
		assert.Error(t, err)
	})
}
