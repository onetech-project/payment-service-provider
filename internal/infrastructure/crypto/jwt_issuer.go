package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"backbone-new/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTIssuer struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewJWTIssuerFromPEM(privateKeyPEM, publicKeyPEM string) (*JWTIssuer, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("failed to parse private key PEM")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// try PKCS8
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
		}
		var ok bool
		privKey, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
	}

	pubBlock, _ := pem.Decode([]byte(publicKeyPEM))
	if pubBlock == nil {
		return nil, errors.New("failed to parse public key PEM")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pubKey, ok := pubKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}

	return &JWTIssuer{
		privateKey: privKey,
		publicKey:  pubKey,
	}, nil
}

func (j *JWTIssuer) GenerateB2BToken(clientID string, ttl time.Duration) (string, string, error) {
	jti := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(ttl)

	claims := jwt.MapClaims{
		"client_id": clientID,
		"jti":       jti,
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
		"iss":       "payment-gateway",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, jti, nil
}

func (j *JWTIssuer) ValidateToken(tokenString string) (*domain.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.publicKey, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claimsMap, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims format")
	}

	clientID, _ := claimsMap["client_id"].(string)
	jti, _ := claimsMap["jti"].(string)

	return &domain.TokenClaims{
		ClientID: clientID,
		JTI:      jti,
	}, nil
}
