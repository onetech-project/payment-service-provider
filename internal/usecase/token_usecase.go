package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backbone-new/internal/domain"
)

type TokenUsecase struct {
	clientRepo    domain.ClientRepository
	verifier      domain.RSASignatureVerifier
	jwtIssuer     domain.JWTIssuer
	skipSkewCheck bool
}

// NewTokenUsecase constructs a TokenUsecase. skipSkewCheck disables the
// ±5 minute X-TIMESTAMP freshness check — intended for APP_ENV=dev/uat only,
// where replaying stale sample requests during testing is common.
func NewTokenUsecase(
	clientRepo domain.ClientRepository,
	verifier domain.RSASignatureVerifier,
	jwtIssuer domain.JWTIssuer,
	skipSkewCheck bool,
) *TokenUsecase {
	return &TokenUsecase{
		clientRepo:    clientRepo,
		verifier:      verifier,
		jwtIssuer:     jwtIssuer,
		skipSkewCheck: skipSkewCheck,
	}
}

func (u *TokenUsecase) GenerateB2BToken(ctx context.Context, clientID, timestamp, signature, grantType string) (*domain.SNAPTokenResponse, error) {
	if grantType != "client_credentials" {
		return nil, domain.NewDomainError(domain.CodeTokenInvalidField,
			"Invalid field format [clientId/clientSecret/grantType]", domain.ErrInvalidGrantType)
	}

	// X-CLIENT-KEY has its own code (4007302); the other two headers have none
	// of their own, so they fall back to the endpoint's general field-format
	// code rather than borrowing X-CLIENT-KEY's and misnaming the field.
	if clientID == "" {
		return nil, domain.NewDomainError(domain.CodeTokenMissingClientKey,
			"Invalid mandatory field [X-CLIENT-KEY]", domain.ErrMissingHeader)
	}
	if timestamp == "" {
		return nil, domain.NewDomainError(domain.CodeTokenInvalidTimestamp,
			"Invalid field format [X-TIMESTAMP]", domain.ErrMissingHeader)
	}
	if signature == "" {
		return nil, domain.NewDomainError(domain.CodeTokenInvalidField,
			"Invalid field format [X-SIGNATURE]", domain.ErrMissingHeader)
	}

	// A malformed X-TIMESTAMP and a stale one both answer 4007301: BCA
	// publishes exactly one X-TIMESTAMP code for this endpoint, and a
	// timestamp outside the freshness window is as unusable as one that will
	// not parse. The message distinguishes them for our own logs.
	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil, domain.NewDomainError(domain.CodeTokenInvalidTimestamp,
			"Invalid field format [X-TIMESTAMP]", err)
	}

	if !u.skipSkewCheck && (time.Since(parsedTime) > 5*time.Minute || time.Until(parsedTime) > 5*time.Minute) {
		return nil, domain.NewDomainError(domain.CodeTokenInvalidTimestamp,
			"Invalid field format [X-TIMESTAMP]", domain.ErrInvalidTimestamp)
	}

	// Every rejection from here down is 4017300 with a bracketed reason, the
	// form BCA's own error table uses ("Unauthorized. [Signature]",
	// "Unauthorized. [Unknown client]"). The bracketed token is what the
	// caller matches on, so it is not free text.
	client, err := u.clientRepo.GetClientByID(ctx, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrClientNotFound) {
			return nil, domain.NewDomainError(domain.CodeTokenUnauthorized, "Unauthorized. [Unknown client]", err)
		}
		return nil, domain.NewDomainError(domain.CodeTokenInternalError, "Internal Server Error", err)
	}

	if client.Status != domain.ClientStatusActive {
		return nil, domain.NewDomainError(domain.CodeTokenUnauthorized, "Unauthorized. [Unknown client]", domain.ErrClientRevoked)
	}

	// Fetch active public key
	pubKeyPEM, err := u.clientRepo.GetActiveClientPublicKey(ctx, clientID)
	if err != nil {
		return nil, domain.NewDomainError(domain.CodeTokenUnauthorized, "Unauthorized. [Unknown client]", err)
	}

	// Verify signature over stringToSign: clientID|timestamp
	stringToSign := fmt.Sprintf("%s|%s", clientID, timestamp)
	if err := u.verifier.VerifySignature(pubKeyPEM, stringToSign, signature); err != nil {
		return nil, domain.NewDomainError(domain.CodeTokenUnauthorized, "Unauthorized. [Signature]", err)
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
