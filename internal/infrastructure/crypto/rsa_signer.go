package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
)

// RSASigner signs strings with an RSA private key using SHA256withRSA (PKCS1v15),
// the asymmetric signature algorithm mandated by SNAP for access-token signing.
type RSASigner struct{}

func NewRSASigner() *RSASigner {
	return &RSASigner{}
}

// parseRSAPrivateKey accepts a PEM-encoded key, or a raw base64-encoded PEM/DER
// key (per SNAP's "Base64-encoded private key" header convention).
func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	keyBytes := []byte(raw)

	if block, _ := pem.Decode(keyBytes); block != nil {
		keyBytes = block.Bytes
	} else if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		if block, _ := pem.Decode(decoded); block != nil {
			keyBytes = block.Bytes
		} else {
			keyBytes = decoded
		}
	}

	if privKey, err := x509.ParsePKCS1PrivateKey(keyBytes); err == nil {
		return privKey, nil
	}

	pkcs8Key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}
	privKey, ok := pkcs8Key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return privKey, nil
}

// Sign computes SHA256withRSA(privateKey, stringToSign) and returns it
// base64-encoded, matching the encoding RSAVerifier.VerifySignature expects
// and the SNAP spec's signature convention.
func (s *RSASigner) Sign(rawPrivateKey, stringToSign string) (string, error) {
	privKey, err := parseRSAPrivateKey(rawPrivateKey)
	if err != nil {
		return "", err
	}

	hashed := HashSHA256([]byte(stringToSign))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, CryptoSHA256, hashed)
	if err != nil {
		return "", fmt.Errorf("rsa signing failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(sig), nil
}

// PublicKeyFingerprint derives a short, stable identifier for the public half
// of a PEM-encoded RSA private key: the first 8 bytes of SHA-256 over the
// DER-encoded public key, hex-encoded.
//
// It exists so a log line can say WHICH key signed a request without ever
// putting key material in the log. A vendor rejecting our signature is the
// one failure that config alone cannot diagnose — VENDOR_PRIVATE_KEY_PATH
// tells you which file was read, not whether its public half is the one the
// vendor registered. Comparing this fingerprint against the registered
// public key (openssl rsa -pubin -in key.pem -pubout -outform DER | sha256sum)
// answers that directly.
func PublicKeyFingerprint(rawPrivateKey string) string {
	privKey, err := parseRSAPrivateKey(rawPrivateKey)
	if err != nil {
		return "unparseable"
	}
	der, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return "unmarshalable"
	}
	sum := HashSHA256(der)
	return hex.EncodeToString(sum[:8])
}
