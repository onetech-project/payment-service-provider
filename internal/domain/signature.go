package domain

import "context"

type SignatureAuthResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	Signature       string `json:"signature,omitempty"`
}

type SignatureServiceResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	Signature       string `json:"signature,omitempty"`
}

// RSASignatureSigner signs a string-to-sign with an RSA private key (SHA256withRSA).
type RSASignatureSigner interface {
	Sign(privateKeyPEMOrBase64, stringToSign string) (signatureHex string, err error)
}

// SignatureUsecase implements the SNAP security "utilities" endpoints that
// help partners compute the asymmetric (access-token) and symmetric (service)
// signatures described at https://apidevportal.aspi-indonesia.or.id/api-services/keamanan.
type SignatureUsecase interface {
	GenerateAccessTokenSignature(ctx context.Context, clientKey, timestamp, privateKey string) (*SignatureAuthResponse, error)
	GenerateServiceSignature(ctx context.Context, clientSecret, httpMethod, endpointURL, accessToken, timestamp string, requestBody []byte) (*SignatureServiceResponse, error)
}
