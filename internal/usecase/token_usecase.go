package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backbone-new/internal/domain"
)

type TokenUsecase struct {
	clientRepo  domain.ClientRepository
	verifier    domain.RSASignatureVerifier
	jwtIssuer   domain.JWTIssuer
}

func NewTokenUsecase(
	clientRepo domain.ClientRepository,
	verifier domain.RSASignatureVerifier,
	jwtIssuer domain.JWTIssuer,
) *TokenUsecase {
	return &TokenUsecase{
		clientRepo: clientRepo,
		verifier:   verifier,
		jwtIssuer:  jwtIssuer,
	}
}

func (u *TokenUsecase) GenerateB2BToken(ctx context.Context, clientID, timestamp, signature, grantType string) (*domain.SNAPTokenResponse, error) {
	if grantType != "client_credentials" {
		return nil, domain.NewDomainError("4007300", "Bad Request. Invalid grantType", domain.ErrInvalidGrantType)
	}

	if clientID == "" || timestamp == "" || signature == "" {
		return nil, domain.NewDomainError("4007300", "Bad Request. Missing required SNAP headers", domain.ErrMissingHeader)
	}

	// Validate timestamp format (ISO 8601) and skew (within 5 minutes)
	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil, domain.NewDomainError("4007300", "Bad Request. Invalid timestamp format (must be ISO 8601)", err)
	}

	if time.Since(parsedTime) > 5*time.Minute || time.Until(parsedTime) > 5*time.Minute {
		return nil, domain.NewDomainError("4007300", "Bad Request. Timestamp skew exceeds 5 minutes", domain.ErrInvalidTimestamp)
	}

	// Check client status
	client, err := u.clientRepo.GetClientByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrClientNotFound) {
			return nil, domain.NewDomainError("4017300", "Unauthorized. Unknown client", err)
		}
		return nil, domain.NewDomainError("5007300", "Internal Server Error", err)
	}

	if client.Status != domain.ClientStatusActive {
		return nil, domain.NewDomainError("4017300", "Unauthorized. Client account is not active", domain.ErrClientRevoked)
	}

	// Fetch active public key
	pubKeyPEM, err := u.clientRepo.GetActiveClientPublicKey(ctx, clientID)
	if err != nil {
		return nil, domain.NewDomainError("4017300", "Unauthorized. No active public key registered", err)
	}

	// Verify signature over stringToSign: clientID|timestamp
	stringToSign := fmt.Sprintf("%s|%s", clientID, timestamp)
	if err := u.verifier.VerifySignature(pubKeyPEM, stringToSign, signature); err != nil {
		return nil, domain.NewDomainError("4017300", "Unauthorized. Invalid Signature", err)
	}

	// Issue JWT token with 900s (15m) expiry
	ttl := 900 * time.Second
	accessToken, _, err := u.jwtIssuer.GenerateB2BToken(clientID, ttl)
	if err != nil {
		return nil, domain.NewDomainError("5007300", "Failed to generate token", err)
	}

	return &domain.SNAPTokenResponse{
		ResponseCode:    "2007300",
		ResponseMessage: "Successful",
		AccessToken:     accessToken,
		TokenType:       "Bearer",
		ExpiresIn:       "900",
		AdditionalInfo:  make(map[string]interface{}),
	}, nil
}

func (u *TokenUsecase) ValidateToken(ctx context.Context, tokenString string) (*domain.TokenClaims, error) {
	return u.jwtIssuer.ValidateToken(tokenString)
}
