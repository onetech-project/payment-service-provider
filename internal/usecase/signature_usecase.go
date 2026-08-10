package usecase

import (
	"context"
	"encoding/base64"
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
		return nil, domain.NewDomainError("4000000", "Bad Request. Missing required fields (X-CLIENT-KEY, X-TIMESTAMP, X-PRIVATE-KEY)", domain.ErrMissingHeader)
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

// GenerateServiceSignature computes a symmetric X-SIGNATURE for a transaction
// endpoint. bodyHashEncoding selects how the RequestBody component is encoded;
// an empty value means hex, the BCA/SNAP spec form. It must agree with the
// vendor's VENDOR_BODY_HASH_ENCODING, or the signature this helper hands out
// will not verify.
func (u *SignatureUsecase) GenerateServiceSignatureWithEncoding(ctx context.Context, clientSecret, httpMethod, endpointURL, accessToken, timestamp string, requestBody []byte, bodyHashEncoding string) (*domain.SignatureServiceResponse, error) {
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

	bodyHash := crypto.HashRequestBody(requestBody, bodyHashEncoding)
	stringToSign := fmt.Sprintf("%s:%s:%s:%s:%s", httpMethod, endpointURL, accessToken, bodyHash, timestamp)

	hmacSigner := crypto.NewHMACSigner(secret, "HMAC-SHA512")
	signature := hmacSigner.Sign(stringToSign)

	return &domain.SignatureServiceResponse{
		ResponseCode:    "2000000",
		ResponseMessage: "Successful",
		Signature:       signature,
	}, nil
}

// GenerateServiceSignature computes a symmetric X-SIGNATURE using the
// spec-conformant hex body-hash encoding.
func (u *SignatureUsecase) GenerateServiceSignature(ctx context.Context, clientSecret, httpMethod, endpointURL, accessToken, timestamp string, requestBody []byte) (*domain.SignatureServiceResponse, error) {
	return u.GenerateServiceSignatureWithEncoding(ctx, clientSecret, httpMethod, endpointURL, accessToken, timestamp, requestBody, crypto.BodyHashHex)
}
