package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/infrastructure/crypto"
)

type SignatureUsecase struct {
	signer domain.RSASignatureSigner
}

func NewSignatureUsecase(signer domain.RSASignatureSigner) *SignatureUsecase {
	return &SignatureUsecase{signer: signer}
}

func (u *SignatureUsecase) GenerateAccessTokenSignature(ctx context.Context, clientKey, timestamp, privateKey string) (*domain.SignatureAuthResponse, error) {
	if clientKey == "" || timestamp == "" || privateKey == "" {
		return nil, domain.NewDomainError("4000000", "Bad Request. Missing required fields (X-CLIENT-KEY, X-TIMESTAMP, Private_Key)", domain.ErrMissingHeader)
	}

	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		return nil, domain.NewDomainError("4000000", "Bad Request. Invalid timestamp format (must be ISO 8601)", err)
	}

	stringToSign := fmt.Sprintf("%s|%s", clientKey, timestamp)
	signature, err := u.signer.Sign(privateKey, stringToSign)
	if err != nil {
		return nil, domain.NewDomainError("4000000", "Bad Request. Failed to sign stringToSign with provided private key", err)
	}

	return &domain.SignatureAuthResponse{
		ResponseCode:    "2000000",
		ResponseMessage: "Successful",
		Signature:       signature,
	}, nil
}

func (u *SignatureUsecase) GenerateServiceSignature(ctx context.Context, clientSecret, httpMethod, endpointURL, accessToken, timestamp string, requestBody []byte) (*domain.SignatureServiceResponse, error) {
	if clientSecret == "" || httpMethod == "" || endpointURL == "" || timestamp == "" {
		return nil, domain.NewDomainError("4000000", "Bad Request. Missing required fields (X-CLIENT-SECRET, HttpMethod, EndpointUrl, X-TIMESTAMP)", domain.ErrMissingHeader)
	}

	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		return nil, domain.NewDomainError("4000000", "Bad Request. Invalid timestamp format (must be ISO 8601)", err)
	}

	secret := clientSecret
	if decoded, err := base64.StdEncoding.DecodeString(clientSecret); err == nil {
		secret = string(decoded)
	}

	bodyHash := crypto.HashSHA256Hex(minifyJSON(requestBody))
	stringToSign := fmt.Sprintf("%s:%s:%s:%s:%s", httpMethod, endpointURL, accessToken, bodyHash, timestamp)

	hmacSigner := crypto.NewHMACSigner(secret, "HMAC-SHA512")
	signature := hmacSigner.Sign(stringToSign)

	return &domain.SignatureServiceResponse{
		ResponseCode:    "2000000",
		ResponseMessage: "Successful",
		Signature:       signature,
	}, nil
}

// minifyJSON compacts a JSON request body before hashing, per SNAP's
// stringToSign spec. Non-JSON or empty bodies hash as an empty string.
func minifyJSON(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		return string(body)
	}
	return buf.String()
}
