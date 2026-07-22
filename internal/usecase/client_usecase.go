package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backbone-new/internal/domain"

	"github.com/google/uuid"
)

type ClientKeyCache interface {
	GetClientPublicKey(ctx context.Context, clientID string) (string, error)
	SetClientPublicKey(ctx context.Context, clientID, pubKeyPEM string) error
}

type ClientUsecase struct {
	clientRepo domain.ClientRepository
	keyCache   ClientKeyCache
}

func NewClientUsecase(clientRepo domain.ClientRepository, keyCache ClientKeyCache) *ClientUsecase {
	return &ClientUsecase{
		clientRepo: clientRepo,
		keyCache:   keyCache,
	}
}

func (u *ClientUsecase) RegisterClient(ctx context.Context, client *domain.ClientApp, key *domain.ClientKey) error {
	if client.ClientID == "" || client.ClientName == "" {
		return errors.New("client_id and client_name are required")
	}

	if client.ID == "" {
		client.ID = uuid.New().String()
	}
	client.Status = domain.ClientStatusActive
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()

	if err := u.clientRepo.CreateClient(ctx, client); err != nil {
		return fmt.Errorf("failed to register client: %w", err)
	}

	if key != nil {
		if key.ID == "" {
			key.ID = uuid.New().String()
		}
		key.ClientID = client.ClientID
		key.IsActive = true
		key.CreatedAt = time.Now()
		key.UpdatedAt = time.Now()

		if err := u.clientRepo.CreateClientKey(ctx, key); err != nil {
			return fmt.Errorf("failed to register client key: %w", err)
		}

		if u.keyCache != nil {
			_ = u.keyCache.SetClientPublicKey(ctx, client.ClientID, key.PublicKeyPEM)
		}
	}

	return nil
}

func (u *ClientUsecase) RevokeClientKey(ctx context.Context, clientID, keyID string) error {
	return u.clientRepo.RevokeClientKey(ctx, clientID, keyID)
}
