package usecase_test

import (
	"context"
	"errors"
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

func (m *MockClientRepository) GetActiveClientSecret(ctx context.Context, clientID string) (string, error) {
	args := m.Called(ctx, clientID)
	return args.String(0), args.Error(1)
}

func (m *MockClientRepository) CreateClientSecret(ctx context.Context, secret *domain.ClientSecret) error {
	args := m.Called(ctx, secret)
	return args.Error(0)
}

func (m *MockClientRepository) RevokeClientSecret(ctx context.Context, clientID, secretID string) error {
	args := m.Called(ctx, clientID, secretID)
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

	uc := usecase.NewTokenUsecase(mockRepo, mockVerifier, mockIssuer, false)
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

	t.Run("Invalid grantType", func(t *testing.T) {
		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "wrong_grant")
		assert.Error(t, err)
		assert.ErrorIs(t, err.(*domain.DomainError).Err, domain.ErrInvalidGrantType)
	})

	t.Run("Missing clientID", func(t *testing.T) {
		_, err := uc.GenerateB2BToken(ctx, "", timestamp, signature, "client_credentials")
		assert.Error(t, err)
		assert.ErrorIs(t, err.(*domain.DomainError).Err, domain.ErrMissingHeader)
	})

	t.Run("Missing timestamp", func(t *testing.T) {
		_, err := uc.GenerateB2BToken(ctx, clientID, "", signature, "client_credentials")
		assert.Error(t, err)
		assert.ErrorIs(t, err.(*domain.DomainError).Err, domain.ErrMissingHeader)
	})

	t.Run("Missing signature", func(t *testing.T) {
		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, "", "client_credentials")
		assert.Error(t, err)
		assert.ErrorIs(t, err.(*domain.DomainError).Err, domain.ErrMissingHeader)
	})

	t.Run("Invalid timestamp format", func(t *testing.T) {
		_, err := uc.GenerateB2BToken(ctx, clientID, "not-a-timestamp", signature, "client_credentials")
		assert.Error(t, err)
	})

	t.Run("Timestamp skew too far in future", func(t *testing.T) {
		futureTS := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
		_, err := uc.GenerateB2BToken(ctx, clientID, futureTS, signature, "client_credentials")
		assert.Error(t, err)
		assert.ErrorIs(t, err.(*domain.DomainError).Err, domain.ErrInvalidTimestamp)
	})

	t.Run("Timestamp skew too far in past", func(t *testing.T) {
		pastTS := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
		_, err := uc.GenerateB2BToken(ctx, clientID, pastTS, signature, "client_credentials")
		assert.Error(t, err)
		assert.ErrorIs(t, err.(*domain.DomainError).Err, domain.ErrInvalidTimestamp)
	})

	t.Run("GetClientByID returns ErrClientNotFound", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(nil, domain.ErrClientNotFound).Once()
		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")
		assert.Error(t, err)
		assert.Equal(t, "4017300", err.(*domain.DomainError).SNAPCode)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetClientByID returns generic error", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(nil, errors.New("db down")).Once()
		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")
		assert.Error(t, err)
		assert.Equal(t, "5007300", err.(*domain.DomainError).SNAPCode)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetActiveClientPublicKey error", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(&domain.ClientApp{
			ClientID: clientID,
			Status:   domain.ClientStatusActive,
		}, nil).Once()
		mockRepo.On("GetActiveClientPublicKey", ctx, clientID).Return("", errors.New("no key")).Once()

		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")
		assert.Error(t, err)
		assert.Equal(t, "4017300", err.(*domain.DomainError).SNAPCode)
		mockRepo.AssertExpectations(t)
	})

	t.Run("VerifySignature error", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(&domain.ClientApp{
			ClientID: clientID,
			Status:   domain.ClientStatusActive,
		}, nil).Once()
		mockRepo.On("GetActiveClientPublicKey", ctx, clientID).Return(pubKeyPEM, nil).Once()

		stringToSign := clientID + "|" + timestamp
		mockVerifier.On("VerifySignature", pubKeyPEM, stringToSign, signature).Return(errors.New("bad sig")).Once()

		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")
		assert.Error(t, err)
		assert.Equal(t, "4017300", err.(*domain.DomainError).SNAPCode)
		mockRepo.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
	})

	t.Run("jwtIssuer.GenerateB2BToken error", func(t *testing.T) {
		mockRepo.On("GetClientByID", ctx, clientID).Return(&domain.ClientApp{
			ClientID: clientID,
			Status:   domain.ClientStatusActive,
		}, nil).Once()
		mockRepo.On("GetActiveClientPublicKey", ctx, clientID).Return(pubKeyPEM, nil).Once()

		stringToSign := clientID + "|" + timestamp
		mockVerifier.On("VerifySignature", pubKeyPEM, stringToSign, signature).Return(nil).Once()

		mockIssuer.On("GenerateB2BToken", clientID, 900*time.Second).Return("", "", errors.New("sign fail")).Once()

		_, err := uc.GenerateB2BToken(ctx, clientID, timestamp, signature, "client_credentials")
		assert.Error(t, err)
		assert.Equal(t, "5007300", err.(*domain.DomainError).SNAPCode)
		mockRepo.AssertExpectations(t)
		mockVerifier.AssertExpectations(t)
		mockIssuer.AssertExpectations(t)
	})
}

func TestTokenUsecase_ValidateToken(t *testing.T) {
	mockRepo := new(MockClientRepository)
	mockVerifier := new(MockRSAVerifier)
	mockIssuer := new(MockJWTIssuer)
	uc := usecase.NewTokenUsecase(mockRepo, mockVerifier, mockIssuer, false)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		claims := &domain.TokenClaims{ClientID: "client-001", JTI: "jti-1"}
		mockIssuer.On("ValidateToken", "good-token").Return(claims, nil).Once()

		result, err := uc.ValidateToken(ctx, "good-token")
		assert.NoError(t, err)
		assert.Equal(t, claims, result)
		mockIssuer.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockIssuer.On("ValidateToken", "bad-token").Return(nil, errors.New("invalid token")).Once()

		result, err := uc.ValidateToken(ctx, "bad-token")
		assert.Error(t, err)
		assert.Nil(t, result)
		mockIssuer.AssertExpectations(t)
	})
}

func TestTokenUsecase_GenerateB2BToken_SkewCheckSkippedWhenFlagSet(t *testing.T) {
	mockRepo := new(MockClientRepository)
	mockVerifier := new(MockRSAVerifier)
	mockIssuer := new(MockJWTIssuer)
	uc := usecase.NewTokenUsecase(mockRepo, mockVerifier, mockIssuer, true)
	ctx := context.Background()

	clientID := "client-001"
	staleTimestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	signature := "valid-base64-sig"
	pubKeyPEM := "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----"

	mockRepo.On("GetClientByID", ctx, clientID).Return(&domain.ClientApp{
		ClientID: clientID,
		Status:   domain.ClientStatusActive,
	}, nil).Once()
	mockRepo.On("GetActiveClientPublicKey", ctx, clientID).Return(pubKeyPEM, nil).Once()
	stringToSign := clientID + "|" + staleTimestamp
	mockVerifier.On("VerifySignature", pubKeyPEM, stringToSign, signature).Return(nil).Once()
	mockIssuer.On("GenerateB2BToken", clientID, 900*time.Second).Return("mock-jwt-token", "jti-123", nil).Once()

	resp, err := uc.GenerateB2BToken(ctx, clientID, staleTimestamp, signature, "client_credentials")

	assert.NoError(t, err)
	assert.Equal(t, "mock-jwt-token", resp.AccessToken)
	mockRepo.AssertExpectations(t)
	mockVerifier.AssertExpectations(t)
	mockIssuer.AssertExpectations(t)
}
