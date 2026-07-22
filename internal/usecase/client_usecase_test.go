package usecase_test

import (
	"context"
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
