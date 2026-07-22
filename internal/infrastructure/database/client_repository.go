package database

import (
	"context"
	"errors"
	"fmt"

	"backbone-new/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClientRepository struct {
	pool *pgxpool.Pool
}

func NewClientRepository(pool *pgxpool.Pool) *ClientRepository {
	return &ClientRepository{pool: pool}
}

func (r *ClientRepository) GetClientByID(ctx context.Context, clientID string) (*domain.ClientApp, error) {
	query := `SELECT id, client_id, client_name, status, created_at, updated_at FROM client_apps WHERE client_id = $1`

	var app domain.ClientApp
	err := r.pool.QueryRow(ctx, query, clientID).Scan(
		&app.ID, &app.ClientID, &app.ClientName, &app.Status, &app.CreatedAt, &app.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query client_apps: %w", err)
	}

	return &app, nil
}

func (r *ClientRepository) GetActiveClientPublicKey(ctx context.Context, clientID string) (string, error) {
	query := `SELECT public_key_pem FROM client_keys WHERE client_id = $1 AND is_active = true ORDER BY created_at DESC LIMIT 1`

	var pemStr string
	err := r.pool.QueryRow(ctx, query, clientID).Scan(&pemStr)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrClientNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to query client_keys: %w", err)
	}

	return pemStr, nil
}

func (r *ClientRepository) CreateClient(ctx context.Context, client *domain.ClientApp) error {
	query := `INSERT INTO client_apps (id, client_id, client_name, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, query, client.ID, client.ClientID, client.ClientName, client.Status, client.CreatedAt, client.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert client_apps: %w", err)
	}
	return nil
}

func (r *ClientRepository) CreateClientKey(ctx context.Context, key *domain.ClientKey) error {
	query := `INSERT INTO client_keys (id, client_id, key_id, public_key_pem, algorithm, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.pool.Exec(ctx, query, key.ID, key.ClientID, key.KeyID, key.PublicKeyPEM, key.Algorithm, key.IsActive, key.CreatedAt, key.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert client_keys: %w", err)
	}
	return nil
}

func (r *ClientRepository) RevokeClientKey(ctx context.Context, clientID, keyID string) error {
	query := `UPDATE client_keys SET is_active = false, updated_at = NOW() WHERE client_id = $1 AND key_id = $2`
	_, err := r.pool.Exec(ctx, query, clientID, keyID)
	if err != nil {
		return fmt.Errorf("failed to revoke client_key: %w", err)
	}
	return nil
}
