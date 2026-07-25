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

func TestNewJWTIssuerFromPEM_Errors(t *testing.T) {
	_, validPub := generateRSAPrivateKeyPEM(t)
	validPriv, _ := generateRSAPrivateKeyPEM(t)

	t.Run("invalid private key PEM", func(t *testing.T) {
		_, err := crypto.NewJWTIssuerFromPEM("not-a-pem", validPub)
		assert.Error(t, err)
	})

	t.Run("invalid public key PEM", func(t *testing.T) {
		_, err := crypto.NewJWTIssuerFromPEM(validPriv, "not-a-pem")
		assert.Error(t, err)
	})

	t.Run("private key PKCS8 format", func(t *testing.T) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}))

		pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(t, err)
		pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))

		issuer, err := crypto.NewJWTIssuerFromPEM(privPEM, pubPEM)
		require.NoError(t, err)
		assert.NotNil(t, issuer)
	})

	t.Run("private key not RSA (PKCS8 EC key)", func(t *testing.T) {
		ecPKCS8PEM := `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgM5yf3HPRDRR6EfhT
00ZnSa1RjMG9z3gWqROXpvUs97ehRANCAASIMtFfAmVML21qaI2SnUltqInNFefw
+3bsfWQL0M+UylLhhZWQbnxDGFR2VWj8CKnWWE60WhS1CMaqX61rU6fY
-----END PRIVATE KEY-----`
		_, err := crypto.NewJWTIssuerFromPEM(ecPKCS8PEM, validPub)
		assert.Error(t, err)
	})

	t.Run("public key not RSA", func(t *testing.T) {
		ecPubPEM := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEiDLRXwJlTC9tamiNkp1JbaiJzRXn
8Pt27H1kC9DPlMpS4YWVkG58QxhUdlVo/Aip1lhOtFoUtQjGql+ta1On2A==
-----END PUBLIC KEY-----`
		_, err := crypto.NewJWTIssuerFromPEM(validPriv, ecPubPEM)
		assert.Error(t, err)
	})
}

func TestJWTIssuer_ValidateToken_Errors(t *testing.T) {
	privPEM, pubPEM := generateRSAPrivateKeyPEM(t)
	issuer, err := crypto.NewJWTIssuerFromPEM(privPEM, pubPEM)
	require.NoError(t, err)

	t.Run("malformed token", func(t *testing.T) {
		_, err := issuer.ValidateToken("not-a-jwt")
		assert.Error(t, err)
	})

	t.Run("token signed with different key", func(t *testing.T) {
		otherPriv, otherPub := generateRSAPrivateKeyPEM(t)
		otherIssuer, err := crypto.NewJWTIssuerFromPEM(otherPriv, otherPub)
		require.NoError(t, err)

		tokenString, _, err := otherIssuer.GenerateB2BToken("client-x", time.Minute)
		require.NoError(t, err)

		_, err = issuer.ValidateToken(tokenString)
		assert.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		tokenString, _, err := issuer.GenerateB2BToken("client-x", -time.Minute)
		require.NoError(t, err)

		_, err = issuer.ValidateToken(tokenString)
		assert.Error(t, err)
	})
}
