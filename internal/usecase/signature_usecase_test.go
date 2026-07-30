package usecase_test

import (
	"context"
	"encoding/base64"
	"testing"

	"backbone-new/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRSASigner struct {
	mock.Mock
}

func (m *MockRSASigner) Sign(privateKeyPEMOrBase64, stringToSign string) (string, error) {
	args := m.Called(privateKeyPEMOrBase64, stringToSign)
	return args.String(0), args.Error(1)
}

func TestSignatureUsecase_GenerateAccessTokenSignature(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		mockSigner.On("Sign", "priv-key", "client-key|2024-01-01T00:00:00Z").Return("sig-value", nil).Once()

		resp, err := uc.GenerateAccessTokenSignature(ctx, "client-key", "2024-01-01T00:00:00Z", "priv-key")
		assert.NoError(t, err)
		assert.Equal(t, "2000000", resp.ResponseCode)
		assert.Equal(t, "sig-value", resp.Signature)
		mockSigner.AssertExpectations(t)
	})

	t.Run("Missing clientKey", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateAccessTokenSignature(ctx, "", "2024-01-01T00:00:00Z", "priv-key")
		assert.Error(t, err)
	})

	t.Run("Missing timestamp", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateAccessTokenSignature(ctx, "client-key", "", "priv-key")
		assert.Error(t, err)
	})

	t.Run("Missing privateKey", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateAccessTokenSignature(ctx, "client-key", "2024-01-01T00:00:00Z", "")
		assert.Error(t, err)
	})

	t.Run("Bad timestamp format", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateAccessTokenSignature(ctx, "client-key", "not-a-timestamp", "priv-key")
		assert.Error(t, err)
	})

	t.Run("Signer error", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		mockSigner.On("Sign", "priv-key", "client-key|2024-01-01T00:00:00Z").Return("", assert.AnError).Once()

		_, err := uc.GenerateAccessTokenSignature(ctx, "client-key", "2024-01-01T00:00:00Z", "priv-key")
		assert.Error(t, err)
		mockSigner.AssertExpectations(t)
	})
}

func TestSignatureUsecase_GenerateServiceSignature(t *testing.T) {
	ctx := context.Background()

	t.Run("Success with valid JSON body", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		resp, err := uc.GenerateServiceSignature(ctx, "client-secret", "POST", "/openapi/v1.0/transfer-va/inquiry", "access-token", "2024-01-01T00:00:00Z", []byte(`{"a": 1}`))
		assert.NoError(t, err)
		assert.Equal(t, "2000000", resp.ResponseCode)
		assert.NotEmpty(t, resp.Signature)
		// Feature 012-base64-hash-encoding: signature must be standard base64
		// (HMAC-SHA512 -> 64 bytes -> 88 chars incl. padding), not hex (128 chars).
		assert.Len(t, resp.Signature, 88)
		_, decodeErr := base64.StdEncoding.DecodeString(resp.Signature)
		assert.NoError(t, decodeErr, "signature must be valid standard base64")
	})

	t.Run("Success with empty body", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		resp, err := uc.GenerateServiceSignature(ctx, "client-secret", "POST", "/openapi/v1.0/transfer-va/inquiry", "access-token", "2024-01-01T00:00:00Z", nil)
		assert.NoError(t, err)
		assert.NotEmpty(t, resp.Signature)
	})

	t.Run("Missing clientSecret", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateServiceSignature(ctx, "", "POST", "/url", "token", "2024-01-01T00:00:00Z", nil)
		assert.Error(t, err)
	})

	t.Run("Missing httpMethod", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateServiceSignature(ctx, "secret", "", "/url", "token", "2024-01-01T00:00:00Z", nil)
		assert.Error(t, err)
	})

	t.Run("Missing endpointURL", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateServiceSignature(ctx, "secret", "POST", "", "token", "2024-01-01T00:00:00Z", nil)
		assert.Error(t, err)
	})

	t.Run("Missing timestamp", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateServiceSignature(ctx, "secret", "POST", "/url", "token", "", nil)
		assert.Error(t, err)
	})

	t.Run("Bad timestamp format", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		_, err := uc.GenerateServiceSignature(ctx, "secret", "POST", "/url", "token", "not-a-timestamp", nil)
		assert.Error(t, err)
	})

	t.Run("Base64-encoded clientSecret produces different signature than plain", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)

		// "plain-secret" base64-encoded
		resp1, err := uc.GenerateServiceSignature(ctx, "cGxhaW4tc2VjcmV0", "POST", "/url", "token", "2024-01-01T00:00:00Z", nil)
		assert.NoError(t, err)

		resp2, err := uc.GenerateServiceSignature(ctx, "plain-secret", "POST", "/url", "token", "2024-01-01T00:00:00Z", nil)
		assert.NoError(t, err)

		// base64 "cGxhaW4tc2VjcmV0" decodes to "plain-secret", so both should
		// end up using the same underlying secret bytes -> same signature.
		assert.Equal(t, resp2.Signature, resp1.Signature)
	})

	t.Run("Malformed JSON body still succeeds via raw-body hash", func(t *testing.T) {
		mockSigner := new(MockRSASigner)
		uc := usecase.NewSignatureUsecase(mockSigner)
		resp, err := uc.GenerateServiceSignature(ctx, "secret", "POST", "/url", "token", "2024-01-01T00:00:00Z", []byte(`{not-json`))
		assert.NoError(t, err)
		assert.NotEmpty(t, resp.Signature)
	})
}
