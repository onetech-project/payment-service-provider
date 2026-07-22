package crypto_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"backbone-new/internal/infrastructure/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateRSAPrivateKeyPEM(t *testing.T) (string, string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return string(privPEM), string(pubPEM)
}

func TestJWTIssuer_GenerateAndValidate(t *testing.T) {
	privPEM, pubPEM := generateRSAPrivateKeyPEM(t)
	issuer, err := crypto.NewJWTIssuerFromPEM(privPEM, pubPEM)
	require.NoError(t, err)

	clientID := "client-test-001"
	ttl := 900 * time.Second

	tokenString, jti, err := issuer.GenerateB2BToken(clientID, ttl)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)
	assert.NotEmpty(t, jti)

	claims, err := issuer.ValidateToken(tokenString)
	require.NoError(t, err)
	assert.Equal(t, clientID, claims.ClientID)
	assert.Equal(t, jti, claims.JTI)
}
