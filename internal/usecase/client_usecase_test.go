package usecase_test

import (
	"context"
	"errors"
	"testing"

	"backbone-new/internal/domain"
	"backbone-new/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockClientKeyCache struct {
	mock.Mock
}

func (m *MockClientKeyCache) GetClientPublicKey(ctx context.Context, clientID string) (string, error) {
	args := m.Called(ctx, clientID)
	return args.String(0), args.Error(1)
}

func (m *MockClientKeyCache) SetClientPublicKey(ctx context.Context, clientID, pubKeyPEM string) error {
	args := m.Called(ctx, clientID, pubKeyPEM)
	return args.Error(0)
}

func TestClientUsecase_RegisterClient(t *testing.T) {
	mockRepo := new(MockClientRepository)
	mockCache := new(MockClientKeyCache)
	uc := usecase.NewClientUsecase(mockRepo, mockCache)
	ctx := context.Background()

	client := &domain.ClientApp{
		ClientID:   "client-partner-002",
		ClientName: "Partner Beta",
		Status:     domain.ClientStatusActive,
	}

	key := &domain.ClientKey{
		ClientID:     "client-partner-002",
		KeyID:        "key-01",
		PublicKeyPEM: "pem-content",
		Algorithm:    "SHA256withRSA",
		IsActive:     true,
	}

	mockRepo.On("CreateClient", ctx, mock.Anything).Return(nil).Once()
	mockRepo.On("CreateClientKey", ctx, mock.Anything).Return(nil).Once()
	mockCache.On("SetClientPublicKey", ctx, "client-partner-002", "pem-content").Return(nil).Once()

	err := uc.RegisterClient(ctx, client, key)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestClientUsecase_RegisterClient_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty client_id", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		mockCache := new(MockClientKeyCache)
		uc := usecase.NewClientUsecase(mockRepo, mockCache)

		err := uc.RegisterClient(ctx, &domain.ClientApp{ClientName: "name"}, nil)
		assert.Error(t, err)
	})

	t.Run("Empty client_name", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		mockCache := new(MockClientKeyCache)
		uc := usecase.NewClientUsecase(mockRepo, mockCache)

		err := uc.RegisterClient(ctx, &domain.ClientApp{ClientID: "id"}, nil)
		assert.Error(t, err)
	})

	t.Run("CreateClient error", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		mockCache := new(MockClientKeyCache)
		uc := usecase.NewClientUsecase(mockRepo, mockCache)

		mockRepo.On("CreateClient", ctx, mock.Anything).Return(errors.New("db error")).Once()

		err := uc.RegisterClient(ctx, &domain.ClientApp{ClientID: "id", ClientName: "name"}, nil)
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CreateClientKey error", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		mockCache := new(MockClientKeyCache)
		uc := usecase.NewClientUsecase(mockRepo, mockCache)

		mockRepo.On("CreateClient", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("CreateClientKey", ctx, mock.Anything).Return(errors.New("key error")).Once()

		key := &domain.ClientKey{ClientID: "id", KeyID: "k1", PublicKeyPEM: "pem"}
		err := uc.RegisterClient(ctx, &domain.ClientApp{ClientID: "id", ClientName: "name"}, key)
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Nil key param skips key creation", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		mockCache := new(MockClientKeyCache)
		uc := usecase.NewClientUsecase(mockRepo, mockCache)

		mockRepo.On("CreateClient", ctx, mock.Anything).Return(nil).Once()

		err := uc.RegisterClient(ctx, &domain.ClientApp{ClientID: "id", ClientName: "name"}, nil)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "CreateClientKey", mock.Anything, mock.Anything)
	})

	t.Run("Nil keyCache skips SetClientPublicKey", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("CreateClient", ctx, mock.Anything).Return(nil).Once()
		mockRepo.On("CreateClientKey", ctx, mock.Anything).Return(nil).Once()

		key := &domain.ClientKey{ClientID: "id", KeyID: "k1", PublicKeyPEM: "pem"}
		err := uc.RegisterClient(ctx, &domain.ClientApp{ClientID: "id", ClientName: "name"}, key)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestClientUsecase_RevokeClientKey(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("RevokeClientKey", ctx, "client-1", "key-1").Return(nil).Once()

		err := uc.RevokeClientKey(ctx, "client-1", "key-1")
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Repo error", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("RevokeClientKey", ctx, "client-1", "key-1").Return(errors.New("db error")).Once()

		err := uc.RevokeClientKey(ctx, "client-1", "key-1")
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestClientUsecase_AddClientKey(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		mockCache := new(MockClientKeyCache)
		uc := usecase.NewClientUsecase(mockRepo, mockCache)

		mockRepo.On("CreateClientKey", ctx, mock.Anything).Return(nil).Once()
		mockCache.On("SetClientPublicKey", ctx, "client-1", "pem-data").Return(nil).Once()

		key := &domain.ClientKey{ClientID: "client-1", KeyID: "key-1", PublicKeyPEM: "pem-data"}
		err := uc.AddClientKey(ctx, key)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("Missing client_id", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		err := uc.AddClientKey(ctx, &domain.ClientKey{KeyID: "key-1", PublicKeyPEM: "pem"})
		assert.Error(t, err)
	})

	t.Run("Missing key_id", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		err := uc.AddClientKey(ctx, &domain.ClientKey{ClientID: "client-1", PublicKeyPEM: "pem"})
		assert.Error(t, err)
	})

	t.Run("Missing public_key_pem", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		err := uc.AddClientKey(ctx, &domain.ClientKey{ClientID: "client-1", KeyID: "key-1"})
		assert.Error(t, err)
	})

	t.Run("CreateClientKey repo error", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("CreateClientKey", ctx, mock.Anything).Return(errors.New("db error")).Once()

		key := &domain.ClientKey{ClientID: "client-1", KeyID: "key-1", PublicKeyPEM: "pem"}
		err := uc.AddClientKey(ctx, key)
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Nil keyCache skips SetClientPublicKey", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("CreateClientKey", ctx, mock.Anything).Return(nil).Once()

		key := &domain.ClientKey{ClientID: "client-1", KeyID: "key-1", PublicKeyPEM: "pem"}
		err := uc.AddClientKey(ctx, key)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestClientUsecase_AddClientSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("CreateClientSecret", ctx, mock.Anything).Return(nil).Once()

		secret := &domain.ClientSecret{ClientID: "client-1", SecretID: "secret-1", SecretValue: "shh"}
		err := uc.AddClientSecret(ctx, secret)
		assert.NoError(t, err)
		assert.True(t, secret.IsActive)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Missing client_id", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		err := uc.AddClientSecret(ctx, &domain.ClientSecret{SecretID: "secret-1", SecretValue: "shh"})
		assert.Error(t, err)
		mockRepo.AssertNotCalled(t, "CreateClientSecret", mock.Anything, mock.Anything)
	})

	t.Run("Missing secret_id", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		err := uc.AddClientSecret(ctx, &domain.ClientSecret{ClientID: "client-1", SecretValue: "shh"})
		assert.Error(t, err)
	})

	t.Run("Missing secret_value", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		err := uc.AddClientSecret(ctx, &domain.ClientSecret{ClientID: "client-1", SecretID: "secret-1"})
		assert.Error(t, err)
	})

	t.Run("CreateClientSecret repo error", func(t *testing.T) {
		mockRepo := new(MockClientRepository)
		uc := usecase.NewClientUsecase(mockRepo, nil)

		mockRepo.On("CreateClientSecret", ctx, mock.Anything).Return(errors.New("db error")).Once()

		secret := &domain.ClientSecret{ClientID: "client-1", SecretID: "secret-1", SecretValue: "shh"}
		err := uc.AddClientSecret(ctx, secret)
		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestClientUsecase_RevokeClientSecret(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockClientRepository)
	uc := usecase.NewClientUsecase(mockRepo, nil)

	mockRepo.On("RevokeClientSecret", ctx, "client-1", "secret-1").Return(nil).Once()

	err := uc.RevokeClientSecret(ctx, "client-1", "secret-1")
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
