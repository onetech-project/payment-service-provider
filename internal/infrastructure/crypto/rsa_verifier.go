package crypto

import (
	cryptoStd "crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

const CryptoSHA256 = cryptoStd.SHA256

func HashSHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

type RSAVerifier struct{}

func NewRSAVerifier() *RSAVerifier {
	return &RSAVerifier{}
}

func (v *RSAVerifier) VerifySignature(pubKeyPEM, stringToSign, signatureBase64 string) error {
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return errors.New("failed to parse PEM block containing public key")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse PKIX public key: %w", err)
	}

	rsaPubKey, ok := pubKeyInterface.(*rsa.PublicKey)
	if !ok {
		return errors.New("public key is not an RSA public key")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to base64 decode signature: %w", err)
	}

	hashed := HashSHA256([]byte(stringToSign))

	err = rsa.VerifyPKCS1v15(rsaPubKey, CryptoSHA256, hashed, sigBytes)
	if err != nil {
		return fmt.Errorf("rsa signature verification failed: %w", err)
	}

	return nil
}
