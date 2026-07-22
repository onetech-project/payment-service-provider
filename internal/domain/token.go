package domain

import (
	"context"
	"time"
)

type TokenClaims struct {
	ClientID string `json:"client_id"`
	JTI      string `json:"jti"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

type SNAPTokenRequest struct {
	GrantType      string                 `json:"grantType"`
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

type SNAPTokenResponse struct {
	ResponseCode    string                 `json:"responseCode"`
	ResponseMessage string                 `json:"responseMessage"`
	AccessToken     string                 `json:"accessToken,omitempty"`
	TokenType       string                 `json:"tokenType,omitempty"`
	ExpiresIn       string                 `json:"expiresIn,omitempty"`
	AdditionalInfo  map[string]interface{} `json:"additionalInfo,omitempty"`
}

type RSASignatureVerifier interface {
	VerifySignature(pubKeyPEM, stringToSign, signatureBase64 string) error
}

type JWTIssuer interface {
	GenerateB2BToken(clientID string, ttl time.Duration) (tokenString string, jti string, err error)
	ValidateToken(tokenString string) (*TokenClaims, error)
}

type TokenUsecase interface {
	GenerateB2BToken(ctx context.Context, clientID, timestamp, signature, grantType string) (*SNAPTokenResponse, error)
	ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error)
}
