package usecase_test

import (
	"context"
	"testing"
	"time"

	"backbone-new/internal/domain"
	"backbone-new/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockClientRepository struct {
	mock.Mock
}

func (m *MockClientRepository) GetClientByID(ctx context.Context, clientID string) (*domain.ClientApp, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ClientApp), args.Error(1)
}

func (m *MockClientRepository) GetActiveClientPublicKey(ctx context.Context, clientID string) (string, error) {
	args := m.Called(ctx, clientID)
	return args.String(0), args.Error(1)
}

func (m *MockClientRepository) CreateClient(ctx context.Context, client *domain.ClientApp) error {
	args := m.Called(ctx, client)
	return args.Error(0)
}

func (m *MockClientRepository) CreateClientKey(ctx context.Context, key *domain.ClientKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockClientRepository) RevokeClientKey(ctx context.Context, clientID, keyID string) error {
	args := m.Called(ctx, clientID, keyID)
	return args.Error(0)
}

type MockRSAVerifier struct {
	mock.Mock
}

func (m *MockRSAVerifier) VerifySignature(pubKeyPEM, stringToSign, signatureBase64 string) error {
	args := m.Called(pubKeyPEM, stringToSign, signatureBase64)
	return args.Error(0)
}

type MockJWTIssuer struct {
	mock.Mock
}

func (m *MockJWTIssuer) GenerateB2BToken(clientID string, ttl time.Duration) (string, string, error) {
	args := m.Called(clientID, ttl)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockJWTIssuer) ValidateToken(tokenString string) (*domain.TokenClaims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenClaims), args.Error(1)
}

func TestTokenUsecase_GenerateB2BToken(t *testing.T) {
	mockRepo := new(MockClientRepository)
	mockVerifier := new(MockRSAVerifier)
	mockIssuer := new(MockJWTIssuer)

	uc := usecase.NewTokenUsecase(mockRepo, mockVerifier, mockIssuer)
	ctx := context.Background()

	clientID := "client-001"
	timestamp := time.Now().Format(time.RFC3339)
	signature := "valid-base64-sig"
	pubKeyPEM := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----"

	t.Run("Successful Token Generation", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(&domain.ClientApp{
			ClientID: clientID,
			Status:   domain.ClientStatusActive,
		}, nil).Once()

		mockRepo.On("GetActiveClientPublicKey", ctx, clientID).Return(pubKeyPEM, nil).Once()

		stringToSign := clientID + "|" + timestamp
		mockVerifier.On("VerifySignature", pubKeyPEM, stringToSign, signature).Return(nil).Once()

		mockIssuer.On("GenerateB2BToken", clientID, 900*time.Second).Return("mock-jwt-token", "jti-123", nil).Once()

		resp, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")

		assert.NoError(t, err)
		assert.Equal(t, "2007300", resp.ResponseCode)
		assert.Equal(t, "Successful", resp.ResponseMessage)
		assert.Equal(t, "mock-jwt-token", resp.AccessToken)
		assert.Equal(t, "Bearer", resp.TokenType)
		assert.Equal(t, "900", resp.ExpiresIn)

		mockRepo.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockIssuer.AssertExpectations(t)
	})

	t.Run("Revoked Client Rejection", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(&domain.ClientApp{
			ClientID: clientID,
			Status:   domain.ClientStatusRevoked,
		}, nil).Once()

		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")
		assert.Error(t, err)
	})
}
